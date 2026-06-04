package adapters

import (
	"bufio"
	"strings"
	"testing"

	"github.com/fede-iglesias/shipkit/ports"
)

// TestNewPromptTerm verifies the constructor returns a non-nil adapter.
func TestNewPromptTerm(t *testing.T) {
	a := NewPromptTerm()
	if a == nil {
		t.Fatal("NewPromptTerm returned nil")
	}
}

// TestPromptTermAdapter_IsInteractive_False verifies non-interactive mode when
// IsTerminalFn returns false.
func TestPromptTermAdapter_IsInteractive_False(t *testing.T) {
	a := &PromptTermAdapter{
		IsTerminalFn: func(int) bool { return false },
		StdinFd:      0,
		Reader:       bufio.NewReader(strings.NewReader("")),
	}
	if a.IsInteractive() {
		t.Error("IsInteractive = true; want false")
	}
}

// TestPromptTermAdapter_IsInteractive_True verifies interactive mode.
func TestPromptTermAdapter_IsInteractive_True(t *testing.T) {
	a := &PromptTermAdapter{
		IsTerminalFn: func(int) bool { return true },
		StdinFd:      0,
		Reader:       bufio.NewReader(strings.NewReader("y\n")),
	}
	if !a.IsInteractive() {
		t.Error("IsInteractive = false; want true")
	}
}

// TestPromptTermAdapter_Confirm_NonInteractiveDefaultTrue verifies that
// non-interactive returns defaultYes=true without reading input.
func TestPromptTermAdapter_Confirm_NonInteractiveDefaultTrue(t *testing.T) {
	a := &PromptTermAdapter{
		IsTerminalFn: func(int) bool { return false },
		StdinFd:      0,
		Reader:       bufio.NewReader(strings.NewReader("n\n")),
	}
	got, err := a.Confirm("delete?", true)
	if err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	if !got {
		t.Error("Confirm non-interactive defaultYes=true: got false; want true")
	}
}

// TestPromptTermAdapter_Confirm_NonInteractiveDefaultFalse verifies that
// non-interactive returns defaultYes=false.
func TestPromptTermAdapter_Confirm_NonInteractiveDefaultFalse(t *testing.T) {
	a := &PromptTermAdapter{
		IsTerminalFn: func(int) bool { return false },
		StdinFd:      0,
		Reader:       bufio.NewReader(strings.NewReader("y\n")),
	}
	got, err := a.Confirm("delete?", false)
	if err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	if got {
		t.Error("Confirm non-interactive defaultYes=false: got true; want false")
	}
}

// TestPromptTermAdapter_Confirm_Yes verifies "y" input returns true.
func TestPromptTermAdapter_Confirm_Yes(t *testing.T) {
	a := &PromptTermAdapter{
		IsTerminalFn: func(int) bool { return true },
		StdinFd:      0,
		Reader:       bufio.NewReader(strings.NewReader("y\n")),
	}
	got, err := a.Confirm("continue?", false)
	if err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	if !got {
		t.Error("Confirm with y input: got false; want true")
	}
}

// TestPromptTermAdapter_Confirm_No verifies "n" input returns false.
func TestPromptTermAdapter_Confirm_No(t *testing.T) {
	a := &PromptTermAdapter{
		IsTerminalFn: func(int) bool { return true },
		StdinFd:      0,
		Reader:       bufio.NewReader(strings.NewReader("n\n")),
	}
	got, err := a.Confirm("continue?", true)
	if err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	if got {
		t.Error("Confirm with n input: got true; want false")
	}
}

// TestPromptTermAdapter_Confirm_Empty returns the default when empty input.
func TestPromptTermAdapter_Confirm_Empty(t *testing.T) {
	a := &PromptTermAdapter{
		IsTerminalFn: func(int) bool { return true },
		StdinFd:      0,
		Reader:       bufio.NewReader(strings.NewReader("\n")),
	}
	got, err := a.Confirm("continue?", true)
	if err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	if !got {
		t.Error("Confirm empty input defaultYes=true: got false; want true")
	}
}

// TestPromptTermAdapter_Confirm_Yes_LongForm verifies "yes" is accepted.
func TestPromptTermAdapter_Confirm_Yes_LongForm(t *testing.T) {
	a := &PromptTermAdapter{
		IsTerminalFn: func(int) bool { return true },
		StdinFd:      0,
		Reader:       bufio.NewReader(strings.NewReader("yes\n")),
	}
	got, err := a.Confirm("continue?", false)
	if err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	if !got {
		t.Error("Confirm with 'yes' input: got false; want true")
	}
}

// TestPromptTermAdapter_Confirm_EOF returns defaultYes on EOF.
func TestPromptTermAdapter_Confirm_EOF(t *testing.T) {
	a := &PromptTermAdapter{
		IsTerminalFn: func(int) bool { return true },
		StdinFd:      0,
		Reader:       bufio.NewReader(strings.NewReader("")), // EOF immediately
	}
	got, err := a.Confirm("continue?", true)
	if err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	if !got {
		t.Error("Confirm on EOF defaultYes=true: got false; want true")
	}
}

// TestPromptPort_Compliance verifies interface satisfaction at compile time.
func TestPromptPort_Compliance(t *testing.T) {
	var _ ports.PromptPort = NewPromptTerm()
}
