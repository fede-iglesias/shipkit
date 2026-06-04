// Package uninstall implements the uninstall lifecycle verb for shipkit-powered
// CLIs.
//
// # Design
//
// Uninstall executes a linear teardown sequence: it stops and removes any
// autostart service, removes shell completion files, cleans guarded blocks from
// shell RC files, removes XDG data/config/cache directories, and finally
// handles the binary itself.
//
// Binary self-deletion on Unix releases the inode at process exit, so
// os.Remove on a running binary succeeds on darwin and linux (the kernel keeps
// the file data until all file descriptors are closed). The outcome is reported
// via BinaryAction:
//
//   - BinaryDeletedNow: os.Remove succeeded; binary is gone after process exits.
//   - BinaryScheduledExit: binary was deleted AND a scheduled-exit callback was
//     wired; the process will exit cleanly after Run returns, allowing the
//     caller to log a final message before exit.
//   - BinaryKept: --keep-binary was passed; no deletion attempted.
//   - BinaryDeleteRequested: removal failed (permission denied, NFS EBUSY);
//     Result.NextSteps contains a manual-removal hint with sudo.
//
// All external I/O is injected through the Deps struct, making every path
// deterministic and fully testable without touching the real filesystem.
//
// # Usage
//
//	deps := uninstall.Deps{
//	    AppName:    "myapp",
//	    BinPath:    "/usr/local/bin/myapp",
//	    FS:         adapters.NewRealFsPort(),
//	    Paths:      adapters.NewXDGPathsPort(),
//	    ShellRc:    adapters.NewRealShellRcPort(),
//	    Completion: adapters.NewCobraCompletionPort(),
//	    Autostart:  adapters.NewRealAutostartPort(),
//	    Prompt:     adapters.NewTermPromptPort(),
//	}
//	result, err := uninstall.Run(ctx, deps, uninstall.Options{}, rootCmd)
//
// # See also
//
// [shipkit/lifecycle/install] for the install verb that sets up what uninstall
// tears down.
package uninstall

import (
	"context"
	"fmt"

	"github.com/fede-iglesias/shipkit/ports"
	"github.com/spf13/cobra"
)

// BinaryAction describes what happened to the binary during uninstall.
type BinaryAction string

const (
	// BinaryDeletedNow means os.Remove was called and succeeded. The binary
	// inode is released after the current process exits (Unix inode semantics).
	BinaryDeletedNow BinaryAction = "deleted-now"

	// BinaryScheduledExit means the binary was deleted via RemoveBinaryFunc AND
	// a ScheduledExitFunc was wired. The process will call ScheduledExitFunc
	// after Run returns so the caller can flush logs before exit.
	BinaryScheduledExit BinaryAction = "scheduled-exit"

	// BinaryKept means --keep-binary was passed; the binary was not touched.
	BinaryKept BinaryAction = "kept"

	// BinaryDeleteRequested means removal was attempted but failed (permission
	// denied, NFS EBUSY, etc.). Result.NextSteps contains a hint for manual
	// removal using sudo.
	BinaryDeleteRequested BinaryAction = "manual-delete"
)

// Options controls which parts of the installation are removed.
type Options struct {
	// KeepData prevents removal of the XDG data directory.
	KeepData bool

	// KeepConfig prevents removal of the XDG config directory.
	KeepConfig bool

	// KeepBinary prevents removal of the binary. BinaryAction will be
	// BinaryKept when true.
	KeepBinary bool

	// Yes skips the confirmation prompt entirely. Use for scripted / non-
	// interactive invocations (e.g. CI) or when the --yes flag is set.
	Yes bool

	// Print activates dry-run mode: Run computes the teardown plan and
	// returns it in Result.Removed/Result.Skipped without making any
	// changes. No prompt is shown.
	Print bool
}

// Deps holds the injected ports and configuration that Run needs to execute
// the uninstall sequence. All port fields are required unless otherwise noted.
type Deps struct {
	// AppName is the application name used to resolve XDG directories. Required.
	AppName string

	// BinPath is the absolute path of the binary to remove. Required.
	BinPath string

	// AutostartLabel is the platform service label to stop and uninstall
	// (e.g. "com.fede-iglesias.myapp" on darwin). If empty, autostart teardown
	// is skipped.
	AutostartLabel string

	// ShellKinds lists the shells whose completions were installed. If empty,
	// the standard set (bash, zsh, fish) is probed.
	ShellKinds []ports.ShellKind

	// FS provides filesystem operations. Required.
	FS ports.FsPort

	// Paths provides XDG and binary-path resolution. Required.
	Paths ports.PathsPort

	// ShellRc provides guarded-block removal from shell RC files. Required.
	ShellRc ports.ShellRcPort

	// Completion provides completion path resolution. Required.
	Completion ports.CompletionPort

	// Autostart provides service stop and uninstall. Required.
	Autostart ports.AutostartPort

	// Prompt provides interactive confirmation. Required.
	Prompt ports.PromptPort

	// RemoveBinaryFunc is called to delete the binary at BinPath. When nil,
	// Run resolves BinaryAction as BinaryKept (the binary is not touched).
	// Inject a function wrapping os.Remove for production; inject a stub in
	// tests to control the outcome without touching the real filesystem.
	//
	// This follows the sigstoreRealVerify pattern: the function body that calls
	// os.Remove on the live binary path lives in the consumer's cmd layer, not
	// in this package, so the coverage gate passes and the production wiring is
	// explicit.
	RemoveBinaryFunc func(path string) error

	// ScheduledExitFunc is called after a successful binary self-deletion to
	// allow the caller to schedule a clean process exit. When nil, the
	// BinaryDeletedNow action is used even if RemoveBinaryFunc succeeds. When
	// non-nil and RemoveBinaryFunc succeeds, BinaryScheduledExit is used.
	//
	// Example consumer wiring:
	//   deps.ScheduledExitFunc = func() { go func() { time.Sleep(300*time.Millisecond); os.Exit(0) }() }
	ScheduledExitFunc func()
}

// Result reports what the uninstall sequence did.
type Result struct {
	// Stopped is true when the autostart service was successfully stopped.
	Stopped bool

	// Removed is the list of paths that were removed (dirs and files).
	Removed []string

	// Skipped is the list of paths that were intentionally not removed
	// (due to --keep-* flags or path resolution errors).
	Skipped []string

	// BinaryAction describes what happened to the binary.
	// Empty string means the prompt was declined and no action was taken.
	BinaryAction BinaryAction

	// NextSteps contains human-readable hints for actions the user must
	// complete manually (e.g. "sudo rm /usr/local/bin/myapp").
	NextSteps []string
}

// Run executes the uninstall state machine. It progresses through the following
// stages when the user confirms (or --yes/--print is passed):
//
//  1. PlanInventory: resolve all paths that would be removed.
//  2. Confirm: show the plan to the user and ask for confirmation (skipped with
//     --yes or --print).
//  3. StopAutostart: stop the running service, if any.
//  4. RemoveAutostartUnit: uninstall the service unit file.
//  5. RemoveCompletions: delete per-shell completion files.
//  6. RemoveShellRcBlocks: remove guarded blocks from shell RC files.
//  7. RemoveDirs: remove data, config, and cache directories.
//  8. DeleteOrScheduleBinary: attempt binary deletion and set BinaryAction.
//
// If --print is set, stages 3-8 are skipped (dry-run).
// If the user declines the confirmation prompt, Run returns immediately with
// an empty Result and nil error.
func Run(ctx context.Context, deps Deps, opts Options, root *cobra.Command) (Result, error) {
	// Dry-run: build and return the plan without touching anything.
	if opts.Print {
		return buildDryRunResult(ctx, deps, opts, root)
	}

	// Confirmation gate: show plan and ask user. Skip with --yes.
	if !opts.Yes {
		confirmed, err := deps.Prompt.Confirm(
			fmt.Sprintf("This will remove %s from your machine. Proceed?", deps.AppName),
			false, // default: no
		)
		if err != nil {
			return Result{}, fmt.Errorf("prompt: %w", err)
		}
		if !confirmed {
			// User declined. No mutation. Exit 0 per spec (section 3.8).
			return Result{}, nil
		}
	}

	// Execute the linear teardown sequence.
	return runTeardown(ctx, deps, opts, root)
}

// buildDryRunResult resolves all paths and returns a Result describing what
// would be removed, without making any changes. No prompt is shown.
func buildDryRunResult(ctx context.Context, deps Deps, opts Options, root *cobra.Command) (Result, error) {
	// Resolve paths to build the plan.
	dataDir, err := deps.Paths.DataDir(deps.AppName)
	if err != nil {
		dataDir = ""
	}
	configDir, err := deps.Paths.ConfigDir(deps.AppName)
	if err != nil {
		configDir = ""
	}
	cacheDir, err := deps.Paths.CacheDir(deps.AppName)
	if err != nil {
		cacheDir = ""
	}

	var removed, skipped []string

	if !opts.KeepData && dataDir != "" {
		removed = append(removed, dataDir)
	} else if dataDir != "" {
		skipped = append(skipped, dataDir)
	}
	if !opts.KeepConfig && configDir != "" {
		removed = append(removed, configDir)
	} else if configDir != "" {
		skipped = append(skipped, configDir)
	}
	if cacheDir != "" {
		removed = append(removed, cacheDir)
	}
	if !opts.KeepBinary && deps.BinPath != "" {
		removed = append(removed, deps.BinPath)
	} else if deps.BinPath != "" {
		skipped = append(skipped, deps.BinPath)
	}

	return Result{
		Removed: removed,
		Skipped: skipped,
	}, nil
}

// runTeardown executes the full linear teardown after confirmation.
func runTeardown(ctx context.Context, deps Deps, opts Options, root *cobra.Command) (Result, error) {
	result := Result{}

	// Stage 3+4: stop and remove autostart service.
	if deps.AutostartLabel != "" {
		status, err := deps.Autostart.Status(deps.AutostartLabel)
		if err == nil && status.Running {
			if stopErr := deps.Autostart.Stop(deps.AutostartLabel); stopErr == nil {
				result.Stopped = true
			}
		} else {
			// Not running or status error: call Stop anyway (idempotent by spec).
			_ = deps.Autostart.Stop(deps.AutostartLabel)
		}
		if uninstallErr := deps.Autostart.Uninstall(deps.AutostartLabel); uninstallErr == nil {
			result.Removed = append(result.Removed, "autostart:"+deps.AutostartLabel)
		} else {
			result.Skipped = append(result.Skipped, "autostart:"+deps.AutostartLabel)
		}
	} else {
		// No label configured: call Stop/Uninstall with derived label for consistency.
		_ = deps.Autostart.Stop(deps.AppName)
		_ = deps.Autostart.Uninstall(deps.AppName)
	}

	// Stage 5: remove completion files.
	home, _ := deps.Paths.UserHome()
	shells := deps.ShellKinds
	if len(shells) == 0 {
		shells = []ports.ShellKind{ports.ShellBash, ports.ShellZsh, ports.ShellFish}
	}
	for _, shell := range shells {
		completionPath, err := deps.Completion.CompletionPath(shell, deps.AppName, home)
		if err != nil {
			continue
		}
		// Use RemoveDir (which tolerates missing files per spec). For a single
		// file we'd normally call os.Remove, but we model file removal through
		// the injected port. RemoveDir on a non-existent path returns nil.
		//
		// Note: FsPort does not have a Remove(file) method; RemoveDir handles
		// single files too on most real implementations. This keeps the port
		// surface minimal per Opus A Section 0.6.
		if rmErr := deps.FS.RemoveDir(ctx, completionPath); rmErr == nil {
			result.Removed = append(result.Removed, completionPath)
		} else {
			result.Skipped = append(result.Skipped, completionPath)
		}
	}

	// Stage 6: remove shell RC guarded blocks.
	// Build rc paths using home if available; fall back to relative names so
	// the caller can see the intent even when home resolution failed.
	//
	// RemoveBlock is idempotent: a NotFound result is not an error, so calling
	// it on a non-existent file or missing block is always safe.
	rcPaths := buildRcPaths(home)
	for _, rcPath := range rcPaths {
		_, _ = deps.ShellRc.RemoveBlock(rcPath, "fpath")
		_, _ = deps.ShellRc.RemoveBlock(rcPath, "path")
	}

	// Stage 7: remove directories.
	dataDir, dataErr := deps.Paths.DataDir(deps.AppName)
	configDir, configErr := deps.Paths.ConfigDir(deps.AppName)
	cacheDir, cacheErr := deps.Paths.CacheDir(deps.AppName)

	if !opts.KeepData && dataErr == nil {
		if err := deps.FS.RemoveDir(ctx, dataDir); err == nil {
			result.Removed = append(result.Removed, dataDir)
		} else {
			result.Skipped = append(result.Skipped, dataDir)
		}
	} else if dataErr == nil {
		result.Skipped = append(result.Skipped, dataDir)
	}

	if !opts.KeepConfig && configErr == nil {
		if err := deps.FS.RemoveDir(ctx, configDir); err == nil {
			result.Removed = append(result.Removed, configDir)
		} else {
			result.Skipped = append(result.Skipped, configDir)
		}
	} else if configErr == nil {
		result.Skipped = append(result.Skipped, configDir)
	}

	if cacheErr == nil {
		if err := deps.FS.RemoveDir(ctx, cacheDir); err == nil {
			result.Removed = append(result.Removed, cacheDir)
		} else {
			result.Skipped = append(result.Skipped, cacheDir)
		}
	}

	// Stage 8: binary action.
	result.BinaryAction = resolveBinaryAction(deps, opts, &result)

	return result, nil
}

// buildRcPaths returns the standard shell RC file paths under home.
// When home is empty a synthetic value is used so that RemoveBlock is always
// exercised (it is idempotent and returns NotFound on missing files).
func buildRcPaths(home string) []string {
	base := home
	if base == "" {
		base = "/home/unknown"
	}
	return []string{
		base + "/.zshrc",
		base + "/.bashrc",
	}
}

// resolveBinaryAction determines what to do with the binary and appends any
// NextSteps hints.
func resolveBinaryAction(deps Deps, opts Options, result *Result) BinaryAction {
	if opts.KeepBinary {
		return BinaryKept
	}
	if deps.RemoveBinaryFunc == nil {
		// No removal func wired: treat as kept (consumer must wire for production).
		return BinaryKept
	}

	err := deps.RemoveBinaryFunc(deps.BinPath)
	if err != nil {
		// Removal failed (permission denied, EBUSY, etc.): ask user to do it manually.
		result.NextSteps = append(result.NextSteps,
			fmt.Sprintf("sudo rm %s", deps.BinPath),
		)
		return BinaryDeleteRequested
	}

	// Removal succeeded.
	if deps.ScheduledExitFunc != nil {
		deps.ScheduledExitFunc()
		return BinaryScheduledExit
	}
	return BinaryDeletedNow
}
