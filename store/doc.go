// Package store provides universal storage primitives for shipkit consumers:
// atomic file writes, content-addressable checksums, advisory locks, and safe
// path resolution for the knowledge card layout.
//
// # Design
//
// store is stdlib-only - it has no external dependencies. Each primitive is
// independent and can be used without the others:
//
//   - AtomicWrite: temp-file + rename idiom for crash-safe writes. The temp
//     file and destination must be on the same filesystem for the rename to
//     be atomic. Parent directories are created automatically.
//
//   - BodyChecksum: SHA-256 of normalized content. Normalization strips
//     trailing whitespace and appends a single newline, so two documents that
//     differ only in trailing whitespace produce the same digest.
//
//   - Acquire/Lock: file-backed advisory flock(2) with configurable timeout.
//     "Advisory" means only processes that call Acquire participate in mutual
//     exclusion. Other readers/writers are not blocked by the lock.
//
//   - PathFor / KindFromPath: map card kinds and slugs to deterministic file
//     paths under the knowledge/ subdirectory. KindFromPath is the inverse.
//
// All injectable globals (flockFn, walkDirFn, etc.) are unexported and exist
// solely to allow error-branch coverage in tests without OS-level tricks.
//
// # Usage
//
//	// Write a file atomically.
//	data := []byte("# My Card\n\nHello world.\n")
//	if err := store.AtomicWrite("/project/knowledge/plans/q3.md", data, 0o644); err != nil {
//	    // handle err
//	}
//
//	// Compute a content checksum.
//	digest := store.BodyChecksum(data)
//	fmt.Println(digest) // sha256 hex of normalized body
//
//	// Acquire an exclusive advisory lock before a critical section.
//	lk, err := store.Acquire("/project/.locks/plans.lock", 5*time.Second)
//	if err != nil {
//	    // handle err (ErrLockTimeout if no holder released within 5s)
//	}
//	defer lk.Release()
//
// # See also
//
// [github.com/fede-iglesias/shipkit/frontmatter] for YAML frontmatter
// marshal/unmarshal used alongside AtomicWrite.
//
// [github.com/fede-iglesias/shipkit/lifecycle/update] for the full upgrade
// state machine that uses AtomicWrite and Acquire internally.
package store
