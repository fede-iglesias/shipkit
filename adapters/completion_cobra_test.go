package adapters

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/fede-iglesias/shipkit/ports"
	"github.com/spf13/cobra"
)

// testCompletionAdapter returns a CompletionCobraAdapter with injectable
// generators. Each generator writes the shell name to w for assertions.
func testCompletionAdapter(envMap map[string]string) *CompletionCobraAdapter {
	return &CompletionCobraAdapter{
		GenBashFn: func(_ *cobra.Command, w io.Writer) error {
			_, err := w.Write([]byte("bash-completion"))
			return err
		},
		GenZshFn: func(_ *cobra.Command, w io.Writer) error {
			_, err := w.Write([]byte("zsh-completion"))
			return err
		},
		GenFishFn: func(_ *cobra.Command, w io.Writer, _ bool) error {
			_, err := w.Write([]byte("fish-completion"))
			return err
		},
		GetenvFn: func(key string) string { return envMap[key] },
	}
}

// TestNewCompletionCobra verifies the constructor returns a non-nil adapter.
func TestNewCompletionCobra(t *testing.T) {
	a := NewCompletionCobra()
	if a == nil {
		t.Fatal("NewCompletionCobra returned nil")
	}
}

// TestCompletionCobraAdapter_EmitCompletion_Bash verifies bash output.
func TestCompletionCobraAdapter_EmitCompletion_Bash(t *testing.T) {
	a := testCompletionAdapter(nil)
	var buf bytes.Buffer
	root := &cobra.Command{Use: "app"}
	if err := a.EmitCompletion(ports.ShellBash, root, &buf); err != nil {
		t.Fatalf("EmitCompletion bash: %v", err)
	}
	if buf.String() != "bash-completion" {
		t.Errorf("output = %q; want bash-completion", buf.String())
	}
}

// TestCompletionCobraAdapter_EmitCompletion_Zsh verifies zsh output.
func TestCompletionCobraAdapter_EmitCompletion_Zsh(t *testing.T) {
	a := testCompletionAdapter(nil)
	var buf bytes.Buffer
	root := &cobra.Command{Use: "app"}
	if err := a.EmitCompletion(ports.ShellZsh, root, &buf); err != nil {
		t.Fatalf("EmitCompletion zsh: %v", err)
	}
	if buf.String() != "zsh-completion" {
		t.Errorf("output = %q; want zsh-completion", buf.String())
	}
}

// TestCompletionCobraAdapter_EmitCompletion_Fish verifies fish output.
func TestCompletionCobraAdapter_EmitCompletion_Fish(t *testing.T) {
	a := testCompletionAdapter(nil)
	var buf bytes.Buffer
	root := &cobra.Command{Use: "app"}
	if err := a.EmitCompletion(ports.ShellFish, root, &buf); err != nil {
		t.Fatalf("EmitCompletion fish: %v", err)
	}
	if buf.String() != "fish-completion" {
		t.Errorf("output = %q; want fish-completion", buf.String())
	}
}

// TestCompletionCobraAdapter_EmitCompletion_Unknown verifies that
// ShellUnknown returns an error.
func TestCompletionCobraAdapter_EmitCompletion_Unknown(t *testing.T) {
	a := testCompletionAdapter(nil)
	var buf bytes.Buffer
	root := &cobra.Command{Use: "app"}
	err := a.EmitCompletion(ports.ShellUnknown, root, &buf)
	if err == nil {
		t.Fatal("want error for ShellUnknown; got nil")
	}
}

// TestCompletionCobraAdapter_EmitCompletion_GenError propagates generator error.
func TestCompletionCobraAdapter_EmitCompletion_GenError(t *testing.T) {
	sentinel := errors.New("gen fail")
	a := &CompletionCobraAdapter{
		GenBashFn: func(_ *cobra.Command, _ io.Writer) error { return sentinel },
		GenZshFn:  func(_ *cobra.Command, _ io.Writer) error { return nil },
		GenFishFn: func(_ *cobra.Command, _ io.Writer, _ bool) error { return nil },
		GetenvFn:  func(string) string { return "" },
	}
	err := a.EmitCompletion(ports.ShellBash, &cobra.Command{Use: "app"}, &bytes.Buffer{})
	if !errors.Is(err, sentinel) {
		t.Fatalf("want sentinel; got %v", err)
	}
}

// TestCompletionCobraAdapter_CompletionPath_Bash verifies bash path with XDG default.
func TestCompletionCobraAdapter_CompletionPath_Bash(t *testing.T) {
	a := testCompletionAdapter(nil)
	got, err := a.CompletionPath(ports.ShellBash, "myapp", "/home/user")
	if err != nil {
		t.Fatalf("CompletionPath bash: %v", err)
	}
	want := "/home/user/.local/share/bash-completion/completions/myapp"
	if got != want {
		t.Errorf("CompletionPath = %q; want %q", got, want)
	}
}

// TestCompletionCobraAdapter_CompletionPath_Zsh verifies zsh path.
func TestCompletionCobraAdapter_CompletionPath_Zsh(t *testing.T) {
	a := testCompletionAdapter(nil)
	got, err := a.CompletionPath(ports.ShellZsh, "myapp", "/home/user")
	if err != nil {
		t.Fatalf("CompletionPath zsh: %v", err)
	}
	want := "/home/user/.local/share/zsh/site-functions/_myapp"
	if got != want {
		t.Errorf("CompletionPath = %q; want %q", got, want)
	}
}

// TestCompletionCobraAdapter_CompletionPath_Fish verifies fish path.
func TestCompletionCobraAdapter_CompletionPath_Fish(t *testing.T) {
	a := testCompletionAdapter(nil)
	got, err := a.CompletionPath(ports.ShellFish, "myapp", "/home/user")
	if err != nil {
		t.Fatalf("CompletionPath fish: %v", err)
	}
	want := "/home/user/.config/fish/completions/myapp.fish"
	if got != want {
		t.Errorf("CompletionPath = %q; want %q", got, want)
	}
}

// TestCompletionCobraAdapter_CompletionPath_XDGOverride verifies that
// XDG_DATA_HOME overrides the default ~/.local/share path.
func TestCompletionCobraAdapter_CompletionPath_XDGOverride(t *testing.T) {
	a := testCompletionAdapter(map[string]string{
		"XDG_DATA_HOME":   "/mydata",
		"XDG_CONFIG_HOME": "/mycfg",
	})
	got, err := a.CompletionPath(ports.ShellBash, "app", "/home/user")
	if err != nil {
		t.Fatalf("CompletionPath: %v", err)
	}
	want := "/mydata/bash-completion/completions/app"
	if got != want {
		t.Errorf("CompletionPath = %q; want %q", got, want)
	}
}

// TestCompletionCobraAdapter_CompletionPath_Unknown returns error for ShellUnknown.
func TestCompletionCobraAdapter_CompletionPath_Unknown(t *testing.T) {
	a := testCompletionAdapter(nil)
	_, err := a.CompletionPath(ports.ShellUnknown, "app", "/home/user")
	if err == nil {
		t.Fatal("want error for ShellUnknown; got nil")
	}
}

// TestCompletionPort_Compliance verifies interface satisfaction at compile time.
func TestCompletionPort_Compliance(t *testing.T) {
	var _ ports.CompletionPort = NewCompletionCobra()
}
