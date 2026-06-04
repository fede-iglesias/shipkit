package adapters

import "testing"

// TestNewRealSpawn verifies that the constructor returns a non-nil adapter
// with CommandFn wired.
func TestNewRealSpawn(t *testing.T) {
	a := NewRealSpawn()
	if a == nil {
		t.Fatal("NewRealSpawn returned nil")
	}
	if a.CommandFn == nil {
		t.Error("CommandFn is nil; want exec.CommandContext")
	}
}
