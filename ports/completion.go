package ports

import (
	"io"

	"github.com/spf13/cobra"
)

// CompletionPort abstracts shell completion script generation and path
// resolution for a given application and shell.
//
// The install verb calls EmitCompletion to write the generated completion
// script to the path returned by CompletionPath. The uninstall verb removes
// that file. The dependency on *cobra.Command is intentional: shipkit's
// premise is a cobra-app builder toolkit, so coupling this port to cobra is
// acceptable (OQ A2 resolution).
//
// Completion paths are always user-scoped (XDG data/config dirs) and never
// written to system directories (/etc, /usr/share). Caller is responsible for
// creating the parent directory before writing.
//
// Bash 3.2 on macOS (the system default): the install verb detects this via
// EnvPort and warns the user rather than silently installing a completion that
// requires bash >= 4. CompletionPort itself does not enforce this constraint;
// the decision lives in the install verb logic.
type CompletionPort interface {
	// EmitCompletion writes the shell completion script for the given shell
	// and the given cobra root command to dst. Returns an error if the shell
	// is unsupported or if the write fails.
	EmitCompletion(shell ShellKind, root *cobra.Command, dst io.Writer) error

	// CompletionPath returns the conventional user-scoped path where the
	// completion script for app should be installed for the given shell and
	// home directory. The path is NOT created; the caller must mkdir -p.
	//
	// shell | path pattern
	// bash  | $XDG_DATA_HOME/bash-completion/completions/<app>
	// zsh   | $XDG_DATA_HOME/zsh/site-functions/_<app>
	// fish  | $XDG_CONFIG_HOME/fish/completions/<app>.fish
	//
	// Returns an error when shell is ShellUnknown.
	CompletionPath(shell ShellKind, app, home string) (string, error)
}

// MockCompletionPort is a test double for CompletionPort. It records calls and
// returns the values set on its Func fields. Use NewMockCompletionPort for safe
// defaults.
type MockCompletionPort struct {
	// EmitCompletionFunc overrides EmitCompletion when non-nil.
	EmitCompletionFunc func(shell ShellKind, root *cobra.Command, dst io.Writer) error
	// CompletionPathFunc overrides CompletionPath when non-nil.
	CompletionPathFunc func(shell ShellKind, app, home string) (string, error)

	// EmitCompletionCalls records each shell passed to EmitCompletion.
	EmitCompletionCalls []ShellKind
	// CompletionPathCalls records each (shell, app, home) triple.
	CompletionPathCalls []struct{ Shell ShellKind; App, Home string }
}

// NewMockCompletionPort returns a MockCompletionPort whose EmitCompletion
// returns nil and CompletionPath returns "/tmp/completions/<app>" unless Func
// fields are set.
func NewMockCompletionPort() *MockCompletionPort { return &MockCompletionPort{} }

// EmitCompletion implements CompletionPort.
func (m *MockCompletionPort) EmitCompletion(shell ShellKind, root *cobra.Command, dst io.Writer) error {
	m.EmitCompletionCalls = append(m.EmitCompletionCalls, shell)
	if m.EmitCompletionFunc != nil {
		return m.EmitCompletionFunc(shell, root, dst)
	}
	return nil
}

// CompletionPath implements CompletionPort.
func (m *MockCompletionPort) CompletionPath(shell ShellKind, app, home string) (string, error) {
	m.CompletionPathCalls = append(m.CompletionPathCalls, struct {
		Shell      ShellKind
		App, Home string
	}{shell, app, home})
	if m.CompletionPathFunc != nil {
		return m.CompletionPathFunc(shell, app, home)
	}
	return "/tmp/completions/" + app, nil
}
