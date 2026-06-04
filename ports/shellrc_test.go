package ports_test

import (
	"errors"
	"testing"

	"github.com/fede-iglesias/shipkit/ports"
)

// Compile-time proof that MockShellRcPort satisfies ShellRcPort.
var _ ports.ShellRcPort = (*ports.MockShellRcPort)(nil)

func TestMockShellRcPort_EnsureBlock_default(t *testing.T) {
	m := ports.NewMockShellRcPort()
	res, err := m.EnsureBlock("/home/user/.zshrc", "kt:fpath", "fpath+=~/.local/share/zsh/site-functions")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Written {
		t.Error("expected Written=true by default")
	}
	if len(m.EnsureBlockCalls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(m.EnsureBlockCalls))
	}
	if m.EnsureBlockCalls[0].RcPath != "/home/user/.zshrc" {
		t.Errorf("expected rcPath recorded")
	}
	if m.EnsureBlockCalls[0].BlockID != "kt:fpath" {
		t.Errorf("expected blockID recorded")
	}
}

// EnsureBlock idempotency contract: calling with same content returns Unchanged.
func TestMockShellRcPort_EnsureBlock_idempotent(t *testing.T) {
	m := ports.NewMockShellRcPort()
	// Override to simulate idempotent behaviour.
	called := 0
	m.EnsureBlockFunc = func(_, _, _ string) (ports.EnsureResult, error) {
		called++
		if called == 1 {
			return ports.EnsureResult{Written: true}, nil
		}
		return ports.EnsureResult{Unchanged: true}, nil
	}
	r1, _ := m.EnsureBlock("rc", "id", "content")
	r2, _ := m.EnsureBlock("rc", "id", "content")
	if !r1.Written {
		t.Error("first call should be Written")
	}
	if !r2.Unchanged {
		t.Error("second call should be Unchanged (idempotent)")
	}
}

func TestMockShellRcPort_EnsureBlock_updated(t *testing.T) {
	m := ports.NewMockShellRcPort()
	m.EnsureBlockFunc = func(_, _, _ string) (ports.EnsureResult, error) {
		return ports.EnsureResult{Updated: true}, nil
	}
	res, err := m.EnsureBlock("rc", "id", "new-content")
	if err != nil || !res.Updated {
		t.Errorf("expected Updated=true, err=%v", err)
	}
}

func TestMockShellRcPort_RemoveBlock_default(t *testing.T) {
	m := ports.NewMockShellRcPort()
	res, err := m.RemoveBlock("/home/user/.zshrc", "kt:fpath")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Removed {
		t.Error("expected Removed=true by default")
	}
	if len(m.RemoveBlockCalls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(m.RemoveBlockCalls))
	}
}

func TestMockShellRcPort_RemoveBlock_notFound(t *testing.T) {
	m := ports.NewMockShellRcPort()
	m.RemoveBlockFunc = func(_, _ string) (ports.RemoveResult, error) {
		return ports.RemoveResult{NotFound: true}, nil
	}
	res, err := m.RemoveBlock("rc", "missing-block")
	if err != nil || !res.NotFound {
		t.Errorf("expected NotFound=true, err=%v", err)
	}
}

func TestMockShellRcPort_EnsureBlock_error(t *testing.T) {
	m := ports.NewMockShellRcPort()
	sentinel := errors.New("read-only filesystem")
	m.EnsureBlockFunc = func(_, _, _ string) (ports.EnsureResult, error) {
		return ports.EnsureResult{}, sentinel
	}
	_, err := m.EnsureBlock("rc", "id", "content")
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel, got %v", err)
	}
}
