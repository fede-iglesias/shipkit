// Package shipkit is the public toolkit that gives any Go cobra CLI a complete
// self-contained lifecycle with a single [RegisterLifecycle] call. The five
// lifecycle verbs (install, update, uninstall, doctor, clean) wire themselves as
// cobra subcommands, handle XDG directories, shell completions, cosign keyless
// signature verification, and atomic self-replace with rollback.
//
// # Design
//
// shipkit is organised as a multi-module mono-repo. Each lifecycle verb and
// primitive lives in its own Go module so consumers can take the exact slice they
// need without pulling the entire tree:
//
//   - [shipkit/lifecycle/install]   - config dirs, completions, shell hooks, autostart
//   - [shipkit/lifecycle/update]    - cosign-verified atomic self-update with rollback
//   - [shipkit/lifecycle/uninstall] - clean removal of binary, dirs, shell hooks
//   - [shipkit/lifecycle/doctor]    - local and optional network health checks
//   - [shipkit/lifecycle/clean]     - snapshot and cache pruning with confirmation
//   - [shipkit/frontmatter]         - YAML frontmatter round-trip for structured files
//   - [shipkit/store]               - atomic write, file locking, checksum primitives
//   - [shipkit/lifecycle/migrations] - ordered migration registry for on-disk schema upgrades
//
// Dependencies always point inward: lifecycle verbs depend on ports and primitives;
// the root shipkit package depends on all verbs and wires them together. The ports
// package defines IO interfaces (HTTP, FS, Cosign, Spawn, Clock, Paths, Env, etc.)
// that every verb accepts, enabling full test coverage with fakes and zero network
// calls in unit tests.
//
// Network-call functions (cosign TUF verification, GitHub release queries) follow
// the sigstoreRealVerify pattern: the default implementation returns
// ErrNotConfigured, and the consumer CLI wires the real adapter in cmd/, keeping
// pkg/ at 100% statement coverage without test fakes for network paths.
//
// # Usage
//
// The 90% case: one call to wire all five verbs into an existing cobra root.
//
//	import (
//	    "github.com/fede-iglesias/shipkit"
//	    "github.com/spf13/cobra"
//	)
//
//	func main() {
//	    root := &cobra.Command{Use: "myapp"}
//	    cfg := shipkit.Config{
//	        AppName:    "myapp",
//	        BinaryName: "myapp",
//	        Repo:       "owner/tools",
//	        TagPrefix:  "myapp-",
//	        Version:    "0.1.0", // injected via -ldflags at build time
//	        BinaryPath: "/usr/local/bin/myapp",
//	    }.WithDefaults()
//	    if err := shipkit.RegisterLifecycle(root, cfg); err != nil {
//	        log.Fatal(err)
//	    }
//	    root.Execute()
//	}
//
// For selective wiring or injection of custom ports (useful in tests), use the
// per-verb getters [InstallCmd], [UpdateCmd], [UninstallCmd], [DoctorCmd],
// [CleanCmd] together with [Option] variants such as [WithoutInstall] or
// [WithHTTPPort].
//
// # See also
//
// [shipkit/lifecycle/install] for config dir setup, shell completions, and autostart.
// [shipkit/lifecycle/update] for cosign-verified atomic self-update.
// [shipkit/lifecycle/uninstall] for full clean removal.
// [shipkit/lifecycle/doctor] for health checks.
// [shipkit/lifecycle/clean] for snapshot and cache pruning.
// [shipkit/frontmatter] for YAML frontmatter round-trip.
// [shipkit/store] for atomic write, file lock, and checksum primitives.
// [shipkit/lifecycle/migrations] for ordered on-disk schema migration registry.
package shipkit
