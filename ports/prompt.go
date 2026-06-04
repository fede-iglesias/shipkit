package ports

// PromptPort abstracts interactive confirmation prompts for destructive
// lifecycle operations (uninstall, clean).
//
// The production adapter (shipkit/adapters) uses golang.org/x/term IsTerminal
// to decide whether the process has an interactive tty. When IsInteractive
// returns false, Confirm must NOT block for input; it must return the default
// value immediately.
//
// Callers that run with --yes flags should inject a MockPromptPort configured
// to always return true, rather than invoking the real adapter in non-interactive
// environments (e.g. CI, piped install scripts).
type PromptPort interface {
	// Confirm displays question to the user and waits for a y/n response.
	// defaultYes controls what is returned when the user presses Enter without
	// typing y or n. When IsInteractive returns false, Confirm returns
	// defaultYes immediately without displaying anything.
	//
	// Returns an error only if reading from stdin fails unexpectedly.
	Confirm(question string, defaultYes bool) (bool, error)

	// IsInteractive reports whether the process is connected to an interactive
	// terminal (i.e. os.Stdin is a tty). When false, Confirm returns the
	// default value without blocking.
	IsInteractive() bool
}

// MockPromptPort is a test double for PromptPort. It returns ConfirmResult
// for all Confirm calls and Interactive for IsInteractive. Use
// NewMockPromptPort for safe defaults.
type MockPromptPort struct {
	// ConfirmResult is returned by Confirm when ConfirmFunc is nil.
	ConfirmResult bool
	// ConfirmErr is returned as the error from Confirm when non-nil.
	ConfirmErr error
	// ConfirmFunc overrides Confirm when non-nil.
	ConfirmFunc func(question string, defaultYes bool) (bool, error)

	// Interactive is returned by IsInteractive. Defaults to false (non-interactive).
	Interactive bool

	// ConfirmCalls records each question and defaultYes passed to Confirm.
	ConfirmCalls []struct{ Question string; DefaultYes bool }
}

// NewMockPromptPort returns a MockPromptPort whose Confirm always returns true
// (confirmed) and IsInteractive returns false (non-interactive). This is the
// safe default for test code that wants to skip prompts.
func NewMockPromptPort() *MockPromptPort {
	return &MockPromptPort{ConfirmResult: true}
}

// Confirm implements PromptPort.
func (m *MockPromptPort) Confirm(question string, defaultYes bool) (bool, error) {
	m.ConfirmCalls = append(m.ConfirmCalls, struct {
		Question   string
		DefaultYes bool
	}{question, defaultYes})
	if m.ConfirmFunc != nil {
		return m.ConfirmFunc(question, defaultYes)
	}
	return m.ConfirmResult, m.ConfirmErr
}

// IsInteractive implements PromptPort.
func (m *MockPromptPort) IsInteractive() bool { return m.Interactive }
