// Package migrations provides an ordered, versioned migration registry for
// shipkit consumers. Each migration declares a target semver version, a human
// description, and idempotent Apply and Revert operations. The registry
// applies a contiguous range from->to using the minimal set of pending
// migrations, and reverts that same range in reverse order.
//
// # Design
//
// The registry is a pure data store; sequencing logic is deterministic by
// semver comparison (MAJOR.MINOR.PATCH). Migrations are kept sorted on every
// [Registry.Register] call, so callers can register in any order and still
// obtain a stable sequence.
//
// Pending migrations for a range [current, target] are those whose version v
// satisfies: current < v <= target. An empty current is treated as "below
// everything", so all migrations up to target are included.
//
// # Usage
//
//	import "github.com/fede-iglesias/shipkit/lifecycle/migrations"
//
//	// Define concrete migrations.
//	type addIndexMigration struct{}
//	func (m *addIndexMigration) Version() string     { return "0.2.0" }
//	func (m *addIndexMigration) Description() string { return "add search index" }
//	func (m *addIndexMigration) Apply(_ context.Context, root string) error {
//	    // create index file under root
//	    return nil
//	}
//	func (m *addIndexMigration) Revert(_ context.Context, root string) error {
//	    // remove index file from root
//	    return nil
//	}
//
//	// Build registry and apply.
//	r := migrations.New()
//	r.Register(&addIndexMigration{})
//	count, err := r.ApplyPending(ctx, "/data/root", "0.1.0", "0.2.0")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println("applied", count, "migration(s)")
//
// # See also
//
// - [shipkit/lifecycle/update] for the update lifecycle verb that drives migrations.
// - [shipkit/frontmatter] for reading and writing the YAML front-matter that
//   migration data often targets.
package migrations
