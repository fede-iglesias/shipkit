package ports_test

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/fede-iglesias/shipkit/ports"
	"github.com/spf13/cobra"
)

// Compile-time proof that MockCompletionPort satisfies CompletionPort.
var _ ports.CompletionPort = (*ports.MockCompletionPort)(nil)

func TestMockCompletionPort_EmitCompletion_default(t *testing.T) {
	m := ports.NewMockCompletionPort()
	root := &cobra.Command{Use: "app"}
	var buf bytes.Buffer
	if err := m.EmitCompletion(ports.ShellZsh, root, &buf); err != nil {
		t.Fatal(err)
	}
	if len(m.EmitCompletionCalls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(m.EmitCompletionCalls))
	}
	if m.EmitCompletionCalls[0] != ports.ShellZsh {
		t.Errorf("expected shell recorded")
	}
}

func TestMockCompletionPort_EmitCompletion_error(t *testing.T) {
	m := ports.NewMockCompletionPort()
	sentinel := errors.New("unsupported shell")
	m.EmitCompletionFunc = func(_ ports.ShellKind, _ *cobra.Command, _ io.Writer) error {
		return sentinel
	}
	err := m.EmitCompletion(ports.ShellUnknown, &cobra.Command{}, &bytes.Buffer{})
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel, got %v", err)
	}
}

func TestMockCompletionPort_CompletionPath_default(t *testing.T) {
	m := ports.NewMockCompletionPort()
	path, err := m.CompletionPath(ports.ShellZsh, "myapp", "/home/user")
	if err != nil {
		t.Fatal(err)
	}
	if path == "" {
		t.Error("expected non-empty path")
	}
	if len(m.CompletionPathCalls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(m.CompletionPathCalls))
	}
}

func TestMockCompletionPort_CompletionPath_func(t *testing.T) {
	m := ports.NewMockCompletionPort()
	m.CompletionPathFunc = func(shell ports.ShellKind, app, home string) (string, error) {
		return home + "/.zsh_completions/_" + app, nil
	}
	path, err := m.CompletionPath(ports.ShellZsh, "kt", "/home/alice")
	if err != nil {
		t.Fatal(err)
	}
	want := "/home/alice/.zsh_completions/_kt"
	if path != want {
		t.Errorf("want %q, got %q", want, path)
	}
}

// Verify cobra import is intentional: CompletionPort.EmitCompletion accepts *cobra.Command.
func TestCompletionPort_cobraCoupling(t *testing.T) {
	// This test confirms the cobra coupling per OQ A2 decision:
	// shipkit/ports imports spf13/cobra directly (intentional).
	root := &cobra.Command{Use: "myapp"}
	m := ports.NewMockCompletionPort()
	if err := m.EmitCompletion(ports.ShellBash, root, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
}
