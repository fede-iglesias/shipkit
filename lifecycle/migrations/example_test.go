package migrations_test

import (
	"context"
	"fmt"

	"github.com/fede-iglesias/shipkit/lifecycle/migrations"
)

// noopMigration is a migration that does nothing, used in examples.
type noopMigration struct {
	version string
	desc    string
}

func (m *noopMigration) Version() string                            { return m.version }
func (m *noopMigration) Description() string                        { return m.desc }
func (m *noopMigration) Apply(_ context.Context, _ string) error    { return nil }
func (m *noopMigration) Revert(_ context.Context, _ string) error   { return nil }

// ExampleRegistry_Register shows how to create a registry and register
// migrations in any order. The registry maintains sorted order internally.
func ExampleRegistry_Register() {
	r := migrations.New()
	r.Register(&noopMigration{version: "0.2.0", desc: "add search index"})
	r.Register(&noopMigration{version: "0.1.0", desc: "rename config field"})

	// Pending returns them sorted ascending regardless of registration order.
	pending, _ := r.Pending("", "0.2.0")
	for _, m := range pending {
		fmt.Printf("%s: %s\n", m.Version(), m.Description())
	}
	// Output:
	// 0.1.0: rename config field
	// 0.2.0: add search index
}

// ExampleRegistry_ApplyPending shows how to apply all pending migrations for
// an upgrade from one version to another.
func ExampleRegistry_ApplyPending() {
	r := migrations.New()
	r.Register(&noopMigration{version: "0.1.0", desc: "rename config field"})
	r.Register(&noopMigration{version: "0.2.0", desc: "add search index"})

	count, err := r.ApplyPending(context.Background(), "/tmp/data", "0.0.0", "0.2.0")
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Printf("applied %d migration(s)\n", count)
	// Output:
	// applied 2 migration(s)
}

// ExampleRegistry_Revert shows how to roll back migrations from a target
// version down to (but not including) a previous version.
func ExampleRegistry_Revert() {
	r := migrations.New()
	r.Register(&noopMigration{version: "0.1.0", desc: "rename config field"})
	r.Register(&noopMigration{version: "0.2.0", desc: "add search index"})

	// Revert from 0.2.0 back to 0.0.0: undoes 0.2.0, then 0.1.0.
	err := r.Revert(context.Background(), "/tmp/data", "0.2.0", "0.0.0")
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println("reverted ok")
	// Output:
	// reverted ok
}
