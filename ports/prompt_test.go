package ports_test

import (
	"errors"
	"testing"

	"github.com/fede-iglesias/shipkit/ports"
)

// Compile-time proof that MockPromptPort satisfies PromptPort.
var _ ports.PromptPort = (*ports.MockPromptPort)(nil)

func TestMockPromptPort_Confirm_default(t *testing.T) {
	m := ports.NewMockPromptPort()
	ok, err := m.Confirm("Proceed with uninstall?", false)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected Confirm=true by default (safe for tests)")
	}
	if len(m.ConfirmCalls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(m.ConfirmCalls))
	}
	if m.ConfirmCalls[0].Question != "Proceed with uninstall?" {
		t.Errorf("expected question recorded, got %q", m.ConfirmCalls[0].Question)
	}
	if m.ConfirmCalls[0].DefaultYes != false {
		t.Error("expected defaultYes=false recorded")
	}
}

func TestMockPromptPort_Confirm_false(t *testing.T) {
	m := ports.NewMockPromptPort()
	m.ConfirmResult = false
	ok, err := m.Confirm("Really?", false)
	if err != nil || ok {
		t.Errorf("expected false, nil; got %v, %v", ok, err)
	}
}

func TestMockPromptPort_Confirm_func(t *testing.T) {
	m := ports.NewMockPromptPort()
	m.ConfirmFunc = func(q string, d bool) (bool, error) {
		return d, nil // returns the defaultYes value
	}
	ok, err := m.Confirm("Clean?", true)
	if err != nil || !ok {
		t.Errorf("expected true (defaultYes), got %v %v", ok, err)
	}
}

func TestMockPromptPort_Confirm_error(t *testing.T) {
	m := ports.NewMockPromptPort()
	sentinel := errors.New("stdin closed")
	m.ConfirmErr = sentinel
	_, err := m.Confirm("?", false)
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel, got %v", err)
	}
}

func TestMockPromptPort_IsInteractive_default(t *testing.T) {
	m := ports.NewMockPromptPort()
	// Default is non-interactive (safe for CI/tests).
	if m.IsInteractive() {
		t.Error("expected IsInteractive=false by default")
	}
}

func TestMockPromptPort_IsInteractive_true(t *testing.T) {
	m := ports.NewMockPromptPort()
	m.Interactive = true
	if !m.IsInteractive() {
		t.Error("expected IsInteractive=true when set")
	}
}
