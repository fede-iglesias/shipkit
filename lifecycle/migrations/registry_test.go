package migrations

import (
	"context"
	"testing"
)

func TestCompareSemver_Equal(t *testing.T) {
	got := compareSemver("1.2.3", "1.2.3")
	if got != 0 {
		t.Errorf("expected 0 for equal versions, got %d", got)
	}
}

func TestCompareSemver_AGreater(t *testing.T) {
	got := compareSemver("1.2.4", "1.2.3")
	if got != 1 {
		t.Errorf("expected 1 when a > b, got %d", got)
	}
	got = compareSemver("2.0.0", "1.9.9")
	if got != 1 {
		t.Errorf("expected 1 when major a > b, got %d", got)
	}
	got = compareSemver("1.3.0", "1.2.9")
	if got != 1 {
		t.Errorf("expected 1 when minor a > b, got %d", got)
	}
}

func TestCompareSemver_BGreater(t *testing.T) {
	got := compareSemver("1.2.3", "1.2.4")
	if got != -1 {
		t.Errorf("expected -1 when b > a, got %d", got)
	}
	got = compareSemver("0.9.9", "1.0.0")
	if got != -1 {
		t.Errorf("expected -1 when major b > a, got %d", got)
	}
}

func TestCompareSemver_InvalidReturnsZero(t *testing.T) {
	// Unparseable segments are treated as 0 - two identical invalid strings equal.
	got := compareSemver("not-semver", "not-semver")
	if got != 0 {
		t.Errorf("expected 0 for two identical invalid strings, got %d", got)
	}
}

func TestSortMigrations_RandomOrderSorted(t *testing.T) {
	r := &Registry{}
	r.migrations = []Migration{
		&fakeMigration{ver: "0.0.3"},
		&fakeMigration{ver: "0.0.1"},
		&fakeMigration{ver: "0.0.5"},
		&fakeMigration{ver: "0.0.2"},
		&fakeMigration{ver: "0.0.4"},
	}
	sortMigrations(r.migrations)

	expected := []string{"0.0.1", "0.0.2", "0.0.3", "0.0.4", "0.0.5"}
	for i, m := range r.migrations {
		if m.Version() != expected[i] {
			t.Errorf("position %d: expected %s, got %s", i, expected[i], m.Version())
		}
	}
}

// fakeMigration is a minimal stub used only in internal registry tests.
type fakeMigration struct{ ver string }

func (f *fakeMigration) Version() string                         { return f.ver }
func (f *fakeMigration) Description() string                     { return "" }
func (f *fakeMigration) Apply(_ context.Context, _ string) error { panic("not used in registry tests") }
func (f *fakeMigration) Revert(_ context.Context, _ string) error {
	panic("not used in registry tests")
}
