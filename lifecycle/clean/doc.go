// Package clean implements the clean lifecycle verb for shipkit-powered CLIs.
//
// # Design
//
// Clean enumerates stale artifacts under a CLI's data directory and removes
// them after an explicit user confirmation (or with --yes for scripted use).
// The verb is scoped: the caller must pass at least one of --snapshots, --tmp,
// --cache, --logs, or --all to select what to clean. Calling clean with no
// scope flags prints help and exits 1, preventing accidental mass deletion.
//
// Key invariants:
//
//   - Recovery manifest protection: if a snapshot is referenced by the active
//     .shipkit.recovery-manifest.json file, it is never deleted regardless of
//     age or --keep.
//
//   - Symlink escape prevention: any entry whose resolved path escapes DataDir
//     is refused and left in place.
//
//   - --keep N: the newest N snapshots are always preserved regardless of
//     --older-than.
//
//   - --print: dry-run mode; computes the candidate list and reports bytes
//     that would be reclaimed without touching anything.
//
//   - --older-than DUR: default 720h (30 days). Accepts any duration accepted
//     by time.ParseDuration (e.g. "168h" for 7 days).
//
// All external I/O is injected through the Deps struct, making the verb fully
// testable without a real filesystem.
//
// # Usage
//
//	deps := clean.Deps{
//	    AppName: "myapp",
//	    FS:      adapters.NewRealFsPort(),
//	    Paths:   adapters.NewXDGPathsPort(),
//	    Clock:   adapters.NewRealClockPort(),
//	    Prompt:  adapters.NewTermPromptPort(),
//	    ListSnapshotsFunc: clean.DefaultListSnapshots,
//	    ListTmpFunc:       clean.DefaultListTmp,
//	    ListCacheFunc:     clean.DefaultListCache,
//	    ListLogsFunc:      clean.DefaultListLogs,
//	    ReadManifestFunc:  clean.DefaultReadManifest,
//	}
//	result, err := clean.Run(ctx, deps, clean.Options{
//	    Snapshots: true,
//	    Keep:      3,
//	    Yes:       true,
//	})
//
// # See also
//
// [shipkit/lifecycle/update] for the update verb that creates snapshots.
// [shipkit/lifecycle/uninstall] for the uninstall verb.
// [shipkit/ports] for the port interfaces injected into Deps.
package clean
