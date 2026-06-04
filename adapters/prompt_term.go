package adapters

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// PromptTermAdapter is the production implementation of
// [github.com/fede-iglesias/shipkit/ports.PromptPort]. It uses
// golang.org/x/term.IsTerminal to decide whether os.Stdin is an interactive
// tty, and bufio.Reader for line-buffered input.
//
// When IsInteractive returns false, Confirm immediately returns defaultYes
// without displaying anything and without blocking. This is the correct
// behaviour for CI environments, piped scripts, and any invocation where no
// human is watching the terminal.
//
// # Injectable seams
//
// IsTerminalFn, StdinFd, Reader, and StderrWriter are injectable so every
// code path (interactive, non-interactive, EOF on read, prompt-write failure)
// is covered by unit tests.
type PromptTermAdapter struct {
	// IsTerminalFn reports whether the file descriptor is an interactive tty.
	// Defaults to term.IsTerminal. Injectable to simulate interactive/non-interactive.
	IsTerminalFn func(fd int) bool

	// StdinFd is the file descriptor checked by IsTerminalFn.
	// Defaults to int(os.Stdin.Fd()).
	StdinFd int

	// Reader is the source for reading user input. Defaults to a
	// bufio.NewReader wrapping os.Stdin. Injectable in tests to avoid
	// reading from a real terminal.
	Reader *bufio.Reader

	// StderrWriter is the sink for the confirmation prompt text. Defaults to
	// os.Stderr. Injectable so tests can simulate a write failure without
	// closing the real stderr.
	StderrWriter io.Writer
}

// NewPromptTerm returns a PromptTermAdapter wired to the real terminal.
// IsTerminalFn uses golang.org/x/term, Reader wraps os.Stdin, StderrWriter
// wraps os.Stderr.
func NewPromptTerm() *PromptTermAdapter {
	return &PromptTermAdapter{
		IsTerminalFn: term.IsTerminal,
		StdinFd:      int(os.Stdin.Fd()),
		Reader:       bufio.NewReader(os.Stdin),
		StderrWriter: os.Stderr,
	}
}

// IsInteractive reports whether os.Stdin is connected to an interactive tty.
// Returns false in CI environments, when stdin is piped, or when running as a
// subprocess without a terminal allocation.
func (a *PromptTermAdapter) IsInteractive() bool {
	return a.IsTerminalFn(a.StdinFd)
}

// Confirm displays question to the user and waits for a y/n response. When
// IsInteractive returns false, returns defaultYes immediately without
// displaying anything. The prompt suffix "[y/N]" or "[Y/n]" reflects
// defaultYes.
//
// Returns an error only if reading from stdin fails unexpectedly (e.g. the
// reader was closed mid-session). EOF is treated as the default value (no
// error).
func (a *PromptTermAdapter) Confirm(question string, defaultYes bool) (bool, error) {
	if !a.IsInteractive() {
		return defaultYes, nil
	}

	suffix := "[y/N]"
	if defaultYes {
		suffix = "[Y/n]"
	}

	w := a.StderrWriter
	if w == nil {
		w = os.Stderr
	}
	if _, err := fmt.Fprintf(w, "%s %s ", question, suffix); err != nil {
		// Best-effort: if we cannot write the prompt, return the default.
		return defaultYes, nil
	}

	line, err := a.Reader.ReadString('\n')
	if err != nil {
		if err == io.EOF {
			// EOF without input: honour the default.
			return defaultYes, nil
		}
		return defaultYes, fmt.Errorf("prompt: read input: %w", err)
	}

	line = strings.TrimSpace(strings.ToLower(line))
	switch line {
	case "y", "yes":
		return true, nil
	case "n", "no":
		return false, nil
	default:
		// Empty input or anything else: return the default.
		return defaultYes, nil
	}
}
