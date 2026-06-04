// Package install provides the install lifecycle verb for shipkit-based CLIs.
//
// # Design
//
// The install verb sets up the user-scope state for a CLI application after the
// binary has been placed on disk by install.sh. It creates XDG data/config/cache
// directories, installs shell completions, optionally registers an autostart unit,
// injects guarded blocks into the user's shell RC file, and writes a JSON
// marker file recording what was done.
//
// All external I/O is injected through port interfaces in Deps, enabling
// deterministic unit testing without real filesystem access or a tty.
//
// The state machine is linear and marker-gated:
//
//	Plan -> CreateDirs -> EmitCompletions -> EnsureShellHooks ->
//	  InstallAutostart? -> WriteMarker -> Done
//
// The marker is written last. A missing marker means install is incomplete and
// it is safe to re-run. Use Options.Force to re-run even when the marker exists.
//
// # Usage
//
//	deps := install.Deps{
//	    Cfg:   cfg,
//	    FS:    fsAdapter,
//	    Paths: pathsAdapter,
//	    // ... other ports
//	}
//	result, err := install.Run(ctx, deps, install.Options{}, rootCmd)
//
// # See also
//
// [shipkit/ports] for the port interfaces injected via Deps.
package install

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fede-iglesias/shipkit/ports"
	"github.com/spf13/cobra"
)

// Config holds the application-level configuration for the install verb.
// It mirrors the fields from shipkit.Config that the install verb needs.
type Config struct {
	// AppName is the canonical application name used to locate XDG directories
	// and name shell RC blocks. Required.
	AppName string

	// BinaryName is the executable file name. Defaults to AppName when empty.
	BinaryName string

	// Version is the current binary version, e.g. "v0.1.0". Required.
	Version string

	// EnableAutostart, when true, allows --autostart to register a service unit
	// via AutostartPort. When false, Options.Autostart returns an error.
	EnableAutostart bool

	// AutostartLabel is the platform service label, e.g. "com.fede-iglesias.myapp".
	// Defaults to "com.fede-iglesias.<AppName>" when empty.
	AutostartLabel string

	// AutostartArgs is the argument list passed to the binary when the autostart
	// unit runs. Defaults to ["daemon", "run"] when nil.
	AutostartArgs []string
}

// binaryName returns BinaryName, falling back to AppName.
func (c Config) binaryName() string {
	if c.BinaryName != "" {
		return c.BinaryName
	}
	return c.AppName
}

// autostartLabel returns the platform service label, defaulting to
// "com.fede-iglesias.<AppName>" when not configured.
func (c Config) autostartLabel() string {
	if c.AutostartLabel != "" {
		return c.AutostartLabel
	}
	return "com.fede-iglesias." + c.AppName
}

// autostartArgs returns AutostartArgs, defaulting to ["daemon", "run"].
func (c Config) autostartArgs() []string {
	if len(c.AutostartArgs) > 0 {
		return c.AutostartArgs
	}
	return []string{"daemon", "run"}
}

// Options controls runtime behavior of the install verb.
type Options struct {
	// Force re-runs all install steps even when the marker already exists,
	// overwriting files where content differs.
	Force bool

	// Autostart registers a platform autostart unit (LaunchAgent on darwin,
	// systemd-user on linux). Requires Config.EnableAutostart = true.
	Autostart bool

	// Completions is the explicit list of shells for which to install completion
	// scripts. When nil, the shell is autodetected via EnvPort.DetectShell.
	// An empty non-nil slice disables completion installation entirely.
	Completions []ports.ShellKind

	// Print activates dry-run mode: the plan is printed and no files are written.
	Print bool

	// Yes skips any interactive confirmation prompts.
	Yes bool

	// Stderr is where warnings (e.g. bash 3.2 skip) are written.
	// Defaults to os.Stderr when nil.
	Stderr io.Writer
}

// Deps holds the injected port implementations for the install verb.
// All fields are required unless documented otherwise.
type Deps struct {
	// Cfg is the application configuration. Required.
	Cfg Config

	// FS is the filesystem port. Required.
	FS ports.FsPort

	// Paths is the path-resolution port. Required.
	Paths ports.PathsPort

	// Env is the environment-variable and OS-detection port. Required.
	Env ports.EnvPort

	// ShellRc is the shell RC block management port. Required.
	ShellRc ports.ShellRcPort

	// Completion is the shell completion generation port. Required.
	Completion ports.CompletionPort

	// Autostart is the service management port. Required.
	Autostart ports.AutostartPort

	// Prompt is the interactive confirmation port. Required.
	Prompt ports.PromptPort

	// Clock is the time port for deterministic timestamps. Required.
	Clock ports.ClockPort
}

// InstallMarker is the JSON structure written to
// {DataRoot}/.shipkit.installed after a successful install.
type InstallMarker struct {
	// App is the application name.
	App string `json:"app"`

	// VersionInstalled is the version string at install time, e.g. "v0.1.0".
	VersionInstalled string `json:"version_installed"`

	// InstalledAt is the RFC3339 timestamp of install.
	InstalledAt string `json:"installed_at"`

	// BinPath is the absolute path of the installed binary.
	BinPath string `json:"bin_path"`

	// Completions lists the shells for which completions were installed.
	Completions []ports.ShellKind `json:"completions"`

	// Autostart records whether an autostart unit was installed.
	Autostart bool `json:"autostart"`
}

// Result describes the outcome of a successful install run.
type Result struct {
	// Marker is the JSON marker that was written (or that existed on noop).
	Marker InstallMarker

	// AlreadyInstalled is true when the marker already existed for the same
	// version and Options.Force was not set (idempotent no-op).
	AlreadyInstalled bool

	// PathEnsured is true when the binary directory is in $PATH.
	PathEnsured bool

	// CompletionsWritten maps each shell to the path where its completion
	// script was written. Empty when dry-run or no shells detected.
	CompletionsWritten map[ports.ShellKind]string

	// AutostartInstalled is true when the autostart unit was installed.
	AutostartInstalled bool

	// Manifest is a list of filesystem paths that were created or modified.
	Manifest []string
}

// markerFileName is the name of the JSON marker file placed in the data directory.
const markerFileName = ".shipkit.installed"

// Run executes the install state machine: Plan -> CreateDirs ->
// EmitCompletions -> EnsureShellHooks -> InstallAutostart? -> WriteMarker -> Done.
//
// All I/O goes through the ports in deps. root is the cobra root command used
// for generating shell completion scripts.
//
// Returns an error if any step fails. The marker is written last; a missing
// marker indicates an incomplete install and a re-run is safe.
func Run(ctx context.Context, deps Deps, opts Options, root *cobra.Command) (Result, error) {
	stderr := opts.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}

	// Step 0: validate autostart option.
	if opts.Autostart && !deps.Cfg.EnableAutostart {
		return Result{}, fmt.Errorf("install: --autostart requested but Config.EnableAutostart is false; enable it in the app shipkit.Config")
	}

	// Step 1: resolve binary path.
	binPath, err := deps.Paths.Executable()
	if err != nil {
		return Result{}, fmt.Errorf("install: resolve binary path: %w", err)
	}

	// Step 2: resolve data directory.
	dataDir, err := deps.Paths.DataDir(deps.Cfg.AppName)
	if err != nil {
		return Result{}, fmt.Errorf("install: resolve data dir: %w", err)
	}

	markerPath := filepath.Join(dataDir, markerFileName)

	// Step 3: idempotency check.
	if !opts.Force {
		if existing, err := readMarker(ctx, deps, markerPath); err == nil {
			// Marker exists - already installed.
			binDir := filepath.Dir(binPath)
			return Result{
				Marker:           existing,
				AlreadyInstalled: true,
				PathEnsured:      deps.Paths.InPATH(binDir),
			}, nil
		}
	}

	// Dry-run: print plan and return without mutations.
	if opts.Print {
		fmt.Fprintf(stderr, "install plan for %s:\n", deps.Cfg.AppName)
		fmt.Fprintf(stderr, "  data dir:   %s\n", dataDir)
		fmt.Fprintf(stderr, "  binary:     %s\n", binPath)
		return Result{}, nil
	}

	var manifest []string
	completionsWritten := map[ports.ShellKind]string{}
	var autostartInstalled bool
	var completionShells []ports.ShellKind

	// Step 4: create data directory.
	if err := deps.FS.MkdirAll(ctx, dataDir, 0o755); err != nil {
		return Result{}, fmt.Errorf("install: create data dir %s: %w", dataDir, err)
	}
	manifest = append(manifest, dataDir)

	// Step 5: resolve the list of shells for completions.
	if opts.Completions == nil {
		// Autodetect.
		detected := deps.Env.DetectShell()
		if detected != ports.ShellUnknown {
			completionShells = []ports.ShellKind{detected}
		}
	} else {
		completionShells = opts.Completions
	}

	// Step 6: emit completions.
	home, err := deps.Paths.UserHome()
	if err != nil {
		home = ""
	}

	for _, shell := range completionShells {
		// Darwin bash 3.2 skip.
		if shell == ports.ShellBash && deps.Env.DetectOS() == "darwin" {
			if shouldSkipBash32(deps.Env, stderr) {
				continue
			}
		}

		// Resolve completion path.
		compPath, err := deps.Completion.CompletionPath(shell, deps.Cfg.AppName, home)
		if err != nil {
			return Result{}, fmt.Errorf("install: completion path for %s: %w", shell, err)
		}

		// Ensure parent dir exists.
		if err := deps.FS.MkdirAll(ctx, filepath.Dir(compPath), 0o755); err != nil {
			return Result{}, fmt.Errorf("install: create completion dir: %w", err)
		}

		// Write completion to a temp buffer then atomically write via a temp file.
		var buf bytes.Buffer
		if err := deps.Completion.EmitCompletion(shell, root, &buf); err != nil {
			return Result{}, fmt.Errorf("install: emit completion for %s: %w", shell, err)
		}
		if err := deps.FS.AtomicWrite(ctx, compPath, buf.Bytes(), 0o644); err != nil {
			return Result{}, fmt.Errorf("install: write completion %s: %w", compPath, err)
		}
		completionsWritten[shell] = compPath
		manifest = append(manifest, compPath)
	}

	// Step 7: inject shell RC hooks (skip fish - fish autoloads completions).
	detectedShell := deps.Env.DetectShell()
	if detectedShell != ports.ShellFish && detectedShell != ports.ShellUnknown {
		rcPath, err := shellRcPath(detectedShell, home)
		if err == nil && rcPath != "" {
			blockContent := fpathBlock(deps.Cfg.AppName, completionsWritten[detectedShell])
			if _, err := deps.ShellRc.EnsureBlock(rcPath, "fpath", blockContent); err != nil {
				return Result{}, fmt.Errorf("install: ensure shell rc block: %w", err)
			}
			manifest = append(manifest, rcPath)
		}
	}

	// Step 8: install autostart unit.
	if opts.Autostart && deps.Cfg.EnableAutostart {
		unit := ports.AutostartUnit{
			Label:     deps.Cfg.autostartLabel(),
			Program:   binPath,
			Args:      deps.Cfg.autostartArgs(),
			KeepAlive: true,
			RunAtLoad: true,
		}
		if err := deps.Autostart.Install(unit); err != nil {
			return Result{}, fmt.Errorf("install: autostart install: %w", err)
		}
		autostartInstalled = true
	}

	// Step 9: write marker (last step).
	now := deps.Clock.NowUTC()
	shells := make([]ports.ShellKind, 0, len(completionsWritten))
	for s := range completionsWritten {
		shells = append(shells, s)
	}

	marker := InstallMarker{
		App:              deps.Cfg.AppName,
		VersionInstalled: deps.Cfg.Version,
		InstalledAt:      now.Format(time.RFC3339),
		BinPath:          binPath,
		Completions:      shells,
		Autostart:        autostartInstalled,
	}

	// marshalInstallMarker can only fail if the struct contains non-serialisable
	// types (functions, channels). InstallMarker contains only strings, bools,
	// and a slice of ShellKind (string), so this panic is a compile-time
	// invariant guard, not a runtime error path.
	markerJSON := marshalInstallMarker(marker)
	if err := deps.FS.AtomicWrite(ctx, markerPath, markerJSON, 0o644); err != nil {
		return Result{}, fmt.Errorf("install: write marker: %w", err)
	}
	manifest = append(manifest, markerPath)

	binDir := filepath.Dir(binPath)
	return Result{
		Marker:             marker,
		AlreadyInstalled:   false,
		PathEnsured:        deps.Paths.InPATH(binDir),
		CompletionsWritten: completionsWritten,
		AutostartInstalled: autostartInstalled,
		Manifest:           manifest,
	}, nil
}

// shouldSkipBash32 checks whether the running bash on darwin is version 3.x.
// If so, it writes a warning to w mentioning "brew install bash" and returns true.
// The check reads BASH_VERSION from the environment via EnvPort.
func shouldSkipBash32(env ports.EnvPort, w io.Writer) bool {
	if env.DetectOS() != "darwin" {
		return false
	}
	bashVer := env.Get("BASH_VERSION")
	if strings.HasPrefix(bashVer, "3.") {
		fmt.Fprintf(w, "warning: skipping bash completions - darwin ships Bash %s (requires >= 4)\n", bashVer)
		fmt.Fprintf(w, "  to get a modern bash: brew install bash\n")
		return true
	}
	return false
}

// readMarker reads and parses the marker file at path via the filesystem port.
// Returns an error if the file does not exist or is not valid JSON.
func readMarker(ctx context.Context, deps Deps, path string) (InstallMarker, error) {
	raw, err := deps.FS.ReadFile(ctx, path)
	if err != nil {
		return InstallMarker{}, err
	}
	var m InstallMarker
	if err := json.Unmarshal(raw, &m); err != nil {
		return InstallMarker{}, err
	}
	return m, nil
}

// marshalInstallMarker serializes m to indented JSON. InstallMarker contains
// only primitive JSON-serialisable types (strings, bool, []ShellKind) so the
// error return from json.MarshalIndent is structurally impossible; it is
// discarded rather than propagated.
func marshalInstallMarker(m InstallMarker) []byte {
	b, _ := json.MarshalIndent(m, "", "  ")
	return b
}

// shellRcPath returns the canonical RC file path for the given shell and home
// directory. Returns an error for unsupported shells.
//
// shell  | RC file
// bash   | $HOME/.bashrc
// zsh    | $HOME/.zshrc
func shellRcPath(shell ports.ShellKind, home string) (string, error) {
	switch shell {
	case ports.ShellBash:
		return filepath.Join(home, ".bashrc"), nil
	case ports.ShellZsh:
		return filepath.Join(home, ".zshrc"), nil
	default:
		return "", fmt.Errorf("shellRcPath: unsupported shell %q", shell)
	}
}

// fpathBlock returns the content string for the fpath shell RC block, pointing
// to the directory containing the completion file. When completionPath is empty,
// a placeholder comment is returned.
func fpathBlock(appName, completionPath string) string {
	if completionPath == "" {
		return fmt.Sprintf("# %s completions (path not resolved)", appName)
	}
	dir := filepath.Dir(completionPath)
	return fmt.Sprintf("fpath=(%s $fpath)\nautoload -Uz compinit && compinit", dir)
}
