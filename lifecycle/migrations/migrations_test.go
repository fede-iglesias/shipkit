package migrations_test

import (
	"context"
	"errors"
	"testing"

	"github.com/fede-iglesias/shipkit/lifecycle/migrations"
)

// mockMigration is a test double for Migration.
type mockMigration struct {
	version     string
	description string
	applyFn     func(ctx context.Context, root string) error
	revertFn    func(ctx context.Context, root string) error
	// call tracking
	applyCalls  []string
	revertCalls []string
}

func (m *mockMigration) Version() string     { return m.version }
func (m *mockMigration) Description() string { return m.description }
func (m *mockMigration) Apply(ctx context.Context, root string) error {
	m.applyCalls = append(m.applyCalls, root)
	if m.applyFn != nil {
		return m.applyFn(ctx, root)
	}
	return nil
}
func (m *mockMigration) Revert(ctx context.Context, root string) error {
	m.revertCalls = append(m.revertCalls, root)
	if m.revertFn != nil {
		return m.revertFn(ctx, root)
	}
	return nil
}

// compile-time interface check
var _ migrations.Migration = (*mockMigration)(nil)

func TestRegistry_NewIsEmpty(t *testing.T) {
	r := migrations.New()
	pending, err := r.Pending("0.0.1", "0.0.2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("expected empty pending, got %d", len(pending))
	}
}

func TestRegistry_RegisterAppendsSorted(t *testing.T) {
	r := migrations.New()
	// Register out-of-order
	r.Register(&mockMigration{version: "0.0.3"})
	r.Register(&mockMigration{version: "0.0.1"})
	r.Register(&mockMigration{version: "0.0.2"})

	// Pending from nothing -> 0.0.3 should include all three in ascending order
	pending, err := r.Pending("", "0.0.3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pending) != 3 {
		t.Fatalf("expected 3 pending, got %d", len(pending))
	}
	if pending[0].Version() != "0.0.1" {
		t.Errorf("expected first 0.0.1, got %s", pending[0].Version())
	}
	if pending[1].Version() != "0.0.2" {
		t.Errorf("expected second 0.0.2, got %s", pending[1].Version())
	}
	if pending[2].Version() != "0.0.3" {
		t.Errorf("expected third 0.0.3, got %s", pending[2].Version())
	}
}

func TestRegistry_Pending_FromTo(t *testing.T) {
	r := migrations.New()
	r.Register(&mockMigration{version: "0.0.1"})
	r.Register(&mockMigration{version: "0.0.2"})
	r.Register(&mockMigration{version: "0.0.3"})
	r.Register(&mockMigration{version: "0.0.4"})

	// current=0.0.1, target=0.0.3 -> should return 0.0.2 and 0.0.3
	pending, err := r.Pending("0.0.1", "0.0.3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pending) != 2 {
		t.Fatalf("expected 2 pending, got %d", len(pending))
	}
	if pending[0].Version() != "0.0.2" {
		t.Errorf("expected 0.0.2, got %s", pending[0].Version())
	}
	if pending[1].Version() != "0.0.3" {
		t.Errorf("expected 0.0.3, got %s", pending[1].Version())
	}
}

func TestRegistry_Pending_NoNeed(t *testing.T) {
	r := migrations.New()
	r.Register(&mockMigration{version: "0.0.1"})
	r.Register(&mockMigration{version: "0.0.2"})

	// current >= target -> nothing to apply
	pending, err := r.Pending("0.0.2", "0.0.2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("expected 0 pending, got %d", len(pending))
	}

	pending, err = r.Pending("0.0.3", "0.0.2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("expected 0 pending when current > target, got %d", len(pending))
	}
}

func TestRegistry_ApplyPending_RunsInOrder(t *testing.T) {
	r := migrations.New()
	var order []string
	m1 := &mockMigration{version: "0.0.2", applyFn: func(_ context.Context, _ string) error {
		order = append(order, "0.0.2")
		return nil
	}}
	m2 := &mockMigration{version: "0.0.3", applyFn: func(_ context.Context, _ string) error {
		order = append(order, "0.0.3")
		return nil
	}}
	// Register out-of-order to verify sorting
	r.Register(m2)
	r.Register(m1)

	root := t.TempDir()
	count, err := r.ApplyPending(context.Background(), root, "0.0.1", "0.0.3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 2 {
		t.Errorf("expected count 2, got %d", count)
	}
	if len(order) != 2 || order[0] != "0.0.2" || order[1] != "0.0.3" {
		t.Errorf("unexpected apply order: %v", order)
	}
}

func TestRegistry_ApplyPending_StopsOnError(t *testing.T) {
	r := migrations.New()
	var order []string
	sentinel := errors.New("apply failed")
	m1 := &mockMigration{version: "0.0.2", applyFn: func(_ context.Context, _ string) error {
		order = append(order, "0.0.2")
		return nil
	}}
	m2 := &mockMigration{version: "0.0.3", applyFn: func(_ context.Context, _ string) error {
		order = append(order, "0.0.3")
		return sentinel
	}}
	m3 := &mockMigration{version: "0.0.4", applyFn: func(_ context.Context, _ string) error {
		order = append(order, "0.0.4")
		return nil
	}}
	r.Register(m1)
	r.Register(m2)
	r.Register(m3)

	root := t.TempDir()
	count, err := r.ApplyPending(context.Background(), root, "0.0.1", "0.0.4")
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel error, got %v", err)
	}
	// Should have applied 0.0.2 and attempted 0.0.3, NOT applied 0.0.4
	if count != 1 {
		t.Errorf("expected partial count 1 (only 0.0.2 succeeded), got %d", count)
	}
	if len(order) != 2 || order[0] != "0.0.2" || order[1] != "0.0.3" {
		t.Errorf("unexpected order: %v", order)
	}
}

func TestRegistry_Revert_RunsInReverseOrder(t *testing.T) {
	r := migrations.New()
	var order []string
	m1 := &mockMigration{version: "0.0.2", revertFn: func(_ context.Context, _ string) error {
		order = append(order, "0.0.2")
		return nil
	}}
	m2 := &mockMigration{version: "0.0.3", revertFn: func(_ context.Context, _ string) error {
		order = append(order, "0.0.3")
		return nil
	}}
	// Register in natural order
	r.Register(m1)
	r.Register(m2)

	root := t.TempDir()
	// Revert from target=0.0.3 back to current=0.0.1 means undo 0.0.3, then 0.0.2
	err := r.Revert(context.Background(), root, "0.0.3", "0.0.1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 2 || order[0] != "0.0.3" || order[1] != "0.0.2" {
		t.Errorf("expected reverse order [0.0.3, 0.0.2], got %v", order)
	}
}

func TestRegistry_Revert_StopsOnError(t *testing.T) {
	r := migrations.New()
	sentinel := errors.New("revert failed")
	var order []string
	m1 := &mockMigration{version: "0.0.2", revertFn: func(_ context.Context, _ string) error {
		order = append(order, "0.0.2")
		return nil
	}}
	m2 := &mockMigration{version: "0.0.3", revertFn: func(_ context.Context, _ string) error {
		order = append(order, "0.0.3")
		return sentinel
	}}
	r.Register(m1)
	r.Register(m2)

	root := t.TempDir()
	// Revert from 0.0.3 -> 0.0.1: should try 0.0.3 first, fail, and stop.
	err := r.Revert(context.Background(), root, "0.0.3", "0.0.1")
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel error, got %v", err)
	}
	// Only 0.0.3 revert was attempted (failed) - 0.0.2 not reached.
	if len(order) != 1 || order[0] != "0.0.3" {
		t.Errorf("expected only 0.0.3 attempted, got %v", order)
	}
}

// TestMigration_InterfaceCompliance is a compile-time check via var _ Migration = ...
// (already present above as var _ migrations.Migration = (*mockMigration)(nil))
func TestMigration_InterfaceCompliance(t *testing.T) {
	// Runtime sanity: methods are callable through interface
	var m migrations.Migration = &mockMigration{version: "1.0.0", description: "test"}
	if m.Version() != "1.0.0" {
		t.Errorf("Version() mismatch")
	}
	if m.Description() != "test" {
		t.Errorf("Description() mismatch")
	}
	if err := m.Apply(context.Background(), t.TempDir()); err != nil {
		t.Errorf("Apply() unexpected error: %v", err)
	}
	if err := m.Revert(context.Background(), t.TempDir()); err != nil {
		t.Errorf("Revert() unexpected error: %v", err)
	}
}
