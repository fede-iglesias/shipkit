// Package install provides the install lifecycle verb for shipkit-based CLIs.
//
// # Design
//
// The install verb sets up the user-scope state for a CLI application after
// the binary has been placed on disk by install.sh. It performs no network
// calls and requires no elevated privileges.
//
// The state machine is linear and marker-gated:
//
//	Plan -> CreateDirs -> EmitCompletions -> EnsureShellHooks ->
//	  InstallAutostart? -> WriteMarker -> Done
//
// The JSON marker at {DataRoot}/.shipkit.installed is the idempotency gate:
// its presence means install completed successfully. A missing marker means
// install is incomplete and safe to re-run without --force.
//
// All external I/O is injected through the port interfaces in Deps:
//   - FsPort for filesystem operations
//   - PathsPort for XDG directory resolution
//   - EnvPort for shell and OS detection
//   - ShellRcPort for guarded RC-file block management
//   - CompletionPort for shell completion script generation
//   - AutostartPort for platform service unit management
//   - PromptPort for interactive confirmation (not used by install directly)
//   - ClockPort for deterministic timestamps
//
// This makes every code path testable without a real filesystem, tty, or
// init system.
//
// # Key invariants
//
//   - No network calls. install.sh handles the download; the install verb
//     handles only user-scope state.
//   - No sudo. All writes go to user XDG directories and $HOME.
//   - Atomic writes. Files are written via a temp-then-rename pattern so a
//     partial write never leaves a corrupted file.
//   - Bash 3.2 skip. macOS ships Bash 3.2 which does not support programmable
//     completion. The install verb detects this via EnvPort and skips bash
//     completion on darwin, emitting a "brew install bash" warning to stderr.
//
// # Usage
//
//	deps := install.Deps{
//	    Cfg: install.Config{
//	        AppName: "myapp",
//	        Version: "v0.1.0",
//	    },
//	    FS:         fsAdapter,
//	    Paths:      pathsAdapter,
//	    Env:        envAdapter,
//	    ShellRc:    shellrcAdapter,
//	    Completion: completionAdapter,
//	    Autostart:  autostartAdapter,
//	    Prompt:     promptAdapter,
//	    Clock:      clockAdapter,
//	}
//	result, err := install.Run(ctx, deps, install.Options{}, rootCmd)
//
// # See also
//
// [shipkit/ports] for the port interfaces used in Deps.
package install
