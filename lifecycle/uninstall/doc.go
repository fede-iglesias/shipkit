// Package uninstall implements the "uninstall" lifecycle verb for
// shipkit-powered CLI applications.
//
// # Purpose
//
// The uninstall verb tears down everything that the install verb created: XDG
// directories, shell completions, RC-file hooks, autostart service units, and
// the binary itself. The goal is to leave the machine in the same state it was
// in before the application was installed.
//
// # Design
//
// All external I/O is injected through the Deps struct (ports pattern). This
// makes every path deterministic and fully unit-testable without touching the
// real filesystem, shell RC files, or service management layer.
//
// The teardown sequence is strictly linear:
//
//  1. Confirm: prompt the user unless --yes or --print.
//  2. StopAutostart: stop the running service via AutostartPort.Stop.
//  3. RemoveAutostartUnit: remove the service unit via AutostartPort.Uninstall.
//  4. RemoveCompletions: delete per-shell completion files via FsPort.RemoveDir.
//  5. RemoveShellRcBlocks: remove guarded blocks via ShellRcPort.RemoveBlock.
//  6. RemoveDirs: remove XDG data, config, and cache directories.
//  7. BinaryAction: attempt binary self-deletion; report the outcome.
//
// # Binary self-deletion
//
// On Unix, calling os.Remove on a running binary releases the inode at process
// exit; the running process continues normally. Deps.RemoveBinaryFunc wraps
// this os.Remove call and is injected from the consumer's cmd layer (following
// the sigstoreRealVerify pattern from shipkit/lifecycle/update). When
// RemoveBinaryFunc succeeds and ScheduledExitFunc is wired, the caller can
// schedule a timed os.Exit after returning from Run to flush any final output.
//
// BinaryAction outcomes:
//   - BinaryDeletedNow: removal succeeded; binary gone after process exits.
//   - BinaryScheduledExit: removal succeeded and ScheduledExitFunc was called.
//   - BinaryKept: --keep-binary passed or RemoveBinaryFunc is nil.
//   - BinaryDeleteRequested: removal failed; Result.NextSteps has a sudo hint.
//
// # Usage
//
//	deps := uninstall.Deps{
//	    AppName:    "myapp",
//	    BinPath:    "/usr/local/bin/myapp",
//	    FS:         realFsPort,
//	    Paths:      xdgPathsPort,
//	    ShellRc:    realShellRcPort,
//	    Completion: cobraCompletionPort,
//	    Autostart:  realAutostartPort,
//	    Prompt:     termPromptPort,
//	    RemoveBinaryFunc: func(p string) error { return os.Remove(p) },
//	}
//	result, err := uninstall.Run(ctx, deps, uninstall.Options{}, rootCmd)
//
// # See also
//
// [shipkit/lifecycle/install] for the install verb that sets up what uninstall
// tears down.
//
// [shipkit/ports] for the port interfaces (FsPort, PathsPort, etc.) and their
// Mock test doubles.
package uninstall
