package adapters

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/fede-iglesias/shipkit/ports"
	"github.com/spf13/cobra"
)

// CompletionCobraAdapter is the production implementation of
// [github.com/fede-iglesias/shipkit/ports.CompletionPort]. It delegates to
// cobra's built-in completion generators (GenBashCompletion,
// GenZshCompletion, GenFishCompletion).
//
// # Bash 3.2 on macOS
//
// This adapter does NOT enforce the bash 3.2 restriction; that policy lives in
// the install verb, which queries EnvPort.DetectShell and either skips bash
// completion or emits a warning. CompletionCobraAdapter will generate bash
// completions for any bash variant when called.
//
// # XDG completion paths
//
// CompletionPath returns user-scoped paths under XDG_DATA_HOME (for bash and
// zsh) or XDG_CONFIG_HOME (for fish). System directories (/etc, /usr/share)
// are never returned; shipkit respects the no-sudo principle.
//
// # Injectable seams
//
// GenBashFn, GenZshFn, GenFishFn are injectable to avoid exercising cobra's
// actual template engine in unit tests.
type CompletionCobraAdapter struct {
	// GenBashFn generates bash completions. Defaults to root.GenBashCompletion.
	GenBashFn func(root *cobra.Command, w io.Writer) error

	// GenZshFn generates zsh completions. Defaults to root.GenZshCompletion.
	GenZshFn func(root *cobra.Command, w io.Writer) error

	// GenFishFn generates fish completions. Defaults to root.GenFishCompletion.
	GenFishFn func(root *cobra.Command, w io.Writer, includeDesc bool) error

	// GetenvFn reads environment variables for XDG_DATA_HOME / XDG_CONFIG_HOME
	// resolution. Defaults to os.Getenv (via closure over PathsXDGAdapter).
	// Injectable so tests can set XDG vars without touching the real env.
	GetenvFn func(string) string
}

// NewCompletionCobra returns a CompletionCobraAdapter with seams wired to
// real cobra generators. GetenvFn defaults to os.Getenv; override it for
// hermetic path tests.
func NewCompletionCobra() *CompletionCobraAdapter {
	return &CompletionCobraAdapter{
		GenBashFn: func(root *cobra.Command, w io.Writer) error {
			return root.GenBashCompletion(w)
		},
		GenZshFn: func(root *cobra.Command, w io.Writer) error {
			return root.GenZshCompletion(w)
		},
		GenFishFn: func(root *cobra.Command, w io.Writer, includeDesc bool) error {
			return root.GenFishCompletion(w, includeDesc)
		},
		GetenvFn: os.Getenv,
	}
}

// EmitCompletion writes the shell completion script for the given shell and
// cobra root command to dst. Returns an error if the shell is ShellUnknown
// or unsupported, or if the cobra generator fails.
func (a *CompletionCobraAdapter) EmitCompletion(shell ports.ShellKind, root *cobra.Command, dst io.Writer) error {
	switch shell {
	case ports.ShellBash:
		return a.GenBashFn(root, dst)
	case ports.ShellZsh:
		return a.GenZshFn(root, dst)
	case ports.ShellFish:
		return a.GenFishFn(root, dst, true)
	default:
		return fmt.Errorf("completion: unsupported shell %q", shell)
	}
}

// CompletionPath returns the user-scoped path where the completion script for
// app should be installed for the given shell and home directory.
//
// Path patterns:
//   - bash: $XDG_DATA_HOME/bash-completion/completions/<app>
//   - zsh:  $XDG_DATA_HOME/zsh/site-functions/_<app>
//   - fish: $XDG_CONFIG_HOME/fish/completions/<app>.fish
//
// Returns an error when shell is ShellUnknown.
func (a *CompletionCobraAdapter) CompletionPath(shell ports.ShellKind, app, home string) (string, error) {
	dataHome := a.GetenvFn("XDG_DATA_HOME")
	if dataHome == "" {
		dataHome = filepath.Join(home, ".local", "share")
	}
	configHome := a.GetenvFn("XDG_CONFIG_HOME")
	if configHome == "" {
		configHome = filepath.Join(home, ".config")
	}

	switch shell {
	case ports.ShellBash:
		return filepath.Join(dataHome, "bash-completion", "completions", app), nil
	case ports.ShellZsh:
		return filepath.Join(dataHome, "zsh", "site-functions", "_"+app), nil
	case ports.ShellFish:
		return filepath.Join(configHome, "fish", "completions", app+".fish"), nil
	default:
		return "", fmt.Errorf("completion path: unsupported shell %q", shell)
	}
}
