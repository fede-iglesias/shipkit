// Package recovery is the shared on-disk recovery manifest format for shipkit
// lifecycle modules.
//
// # Purpose
//
// When a self-update fails and the rollback path itself cannot fully restore
// the prior state, lifecycle/update writes a small JSON manifest at a
// well-known location under the application's XDG data root. The manifest tells
// downstream tooling:
//
//   - which app the partial recovery applies to (AppName),
//   - which snapshot directory must be preserved for a manual recovery
//     (SnapshotPath),
//   - which forward states were already completed before failure (Steps),
//   - what triggered the recovery (Cause), and
//   - when it was written (CreatedAt).
//
// The Version field tags the on-disk schema so future readers can detect and
// migrate older manifests.
//
// # Producers and consumers
//
// Producer: lifecycle/update calls Write from its rollback cascade when the
// rollback cannot complete cleanly.
//
// Consumers:
//
//   - lifecycle/clean reads the manifest to protect the referenced snapshot
//     from deletion (snapshot retention sweeps skip SnapshotPath).
//   - lifecycle/doctor reads the manifest to surface a pending recovery to the
//     operator (recovery.manifest check turns red until the operator
//     completes recovery and removes the file).
//
// # On-disk shape
//
// The manifest is a single JSON object indented with two spaces. The filename
// is Filename (".shipkit.recovery-manifest.json"), and the canonical full path
// under a data root is returned by Path.
//
// # Atomicity
//
// Write uses a temp-file + rename pattern inside the manifest's parent
// directory so that partial writes are never observable by concurrent readers.
// Readers that see the file at all see the fully serialised JSON.
//
// # Stdlib only
//
// The package has no external dependencies. It is safe to import from any
// lifecycle module without expanding the dependency closure.
package recovery
