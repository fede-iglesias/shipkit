// Package doctor implements the doctor lifecycle verb for shipkit-powered CLIs.
//
// # Design
//
// Doctor performs a read-only health inspection of a shipkit-managed CLI
// installation. It runs a fixed inventory of checks, each returning one of four
// outcomes: pass, warn, fail, or skipped. The checks are aggregated into a
// Report and the process exits with code 0 (no failures) or 1 (at least one
// failure). Warnings do not affect the exit code.
//
// All external I/O is injected through the Deps struct using function fields that
// follow the sigstoreRealVerify pattern from shipkit/lifecycle/update: each
// function is nil by default and must be wired by the consumer's cmd layer.
// When nil, the check produces StatusWarn with a hint rather than panicking.
// This design keeps 100% statement coverage achievable in unit tests.
//
// Network checks (network.github, network.cosign-tuf, network.update-feed) are
// gated behind the --network flag and return StatusSkipped by default, keeping
// doctor fast and usable in offline environments.
//
// # Check inventory
//
// The following checks are always run:
//
//   - binary.in-path: binary's parent directory is listed in $PATH
//   - binary.executable: binary has executable permission bits set
//   - binary.version: binary's reported version matches the built-in version
//   - xdg.data-dir: XDG data directory exists on disk
//   - xdg.config-dir: XDG config directory exists on disk
//   - xdg.cache-dir: XDG cache directory exists on disk
//   - marker: .shipkit.installed marker is present and version matches
//   - completion.<shell>: completion file exists for the detected shell
//   - autostart: autostart service is installed and running (if configured)
//   - recovery.manifest: no pending .shipkit.recovery-manifest.json file
//
// The following checks require --network:
//
//   - network.github: GitHub Releases API is reachable
//   - network.cosign-tuf: Sigstore TUF mirror is reachable
//   - network.update-feed: latest release tag is fetchable
//
// # Usage
//
//	deps := doctor.Deps{
//	    AppName:  "myapp",
//	    BinPath:  "/usr/local/bin/myapp",
//	    Version:  "0.1.0",
//	    DataRoot: "/home/user/.local/share/myapp",
//	    ConfigRoot: "/home/user/.config/myapp",
//	    CacheRoot:  "/home/user/.cache/myapp",
//	    Spawn:    adapters.NewRealSpawnPort(),
//	    Paths:    adapters.NewXDGPathsPort(),
//	    Env:      adapters.NewOSEnvPort(),
//	    Autostart: adapters.NewRealAutostartPort(),
//	    Completion: adapters.NewCobraCompletionPort(),
//	    Clock:    adapters.NewRealClockPort(),
//	    // Wire filesystem stat functions in the consumer cmd layer:
//	    StatExecutableFunc: func(path string) (bool, error) {
//	        info, err := os.Stat(path)
//	        if err != nil { return false, err }
//	        return info.Mode()&0111 != 0, nil
//	    },
//	    StatDirFunc: func(path string) (bool, error) {
//	        _, err := os.Stat(path)
//	        if os.IsNotExist(err) { return false, nil }
//	        return err == nil, err
//	    },
//	    StatFileFunc: func(path string) (bool, error) {
//	        _, err := os.Stat(path)
//	        if os.IsNotExist(err) { return false, nil }
//	        return err == nil, err
//	    },
//	    ReadMarkerFunc: func(path string) (string, error) {
//	        data, err := os.ReadFile(path)
//	        return string(data), err
//	    },
//	}
//	report, err := doctor.Run(ctx, deps, doctor.Options{})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Print(doctor.FormatText(report, false))
//	os.Exit(doctor.ExitCode(report))
//
// # Cobra integration
//
// NewCommand returns a pre-wired *cobra.Command ready to add to any cobra root:
//
//	root.AddCommand(doctor.NewCommand(deps))
//
// The command registers --network, --json, and --verbose flags.
//
// # See also
//
// [shipkit/lifecycle/install] for the install verb that sets up what doctor checks.
// [shipkit/lifecycle/update] for the update state machine and recovery manifest.
package doctor
