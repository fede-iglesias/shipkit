package ports

// EnsureResult describes the outcome of a ShellRcPort.EnsureBlock call.
type EnsureResult struct {
	// Written is true when the block was newly inserted into the file.
	Written bool

	// Updated is true when an existing block was found and its content replaced
	// because it differed from the requested content.
	Updated bool

	// Unchanged is true when an existing block was found and its content was
	// identical to the requested content; no write was performed.
	Unchanged bool
}

// RemoveResult describes the outcome of a ShellRcPort.RemoveBlock call.
type RemoveResult struct {
	// Removed is true when the block was found and deleted from the file.
	Removed bool

	// NotFound is true when no matching block markers were found; the file
	// was not modified.
	NotFound bool
}

// ShellRcPort abstracts guarded-block management in shell RC files (e.g.
// ~/.zshrc, ~/.bashrc).
//
// The install verb uses EnsureBlock to inject fpath entries and other shell
// hooks; the uninstall verb uses RemoveBlock to cleanly remove them. Blocks
// are demarcated with sentinel comments so that re-runs are idempotent:
//
//	# >>> shipkit:<app>:<blockID> >>>
//	<content>
//	# <<< shipkit:<app>:<blockID> <<<
//
// A file may contain multiple blocks with different blockIDs. EnsureBlock is
// idempotent: calling it twice with the same content leaves the file unchanged
// on the second call (returns EnsureResult.Unchanged = true). RemoveBlock is
// also idempotent: calling it on a file that does not contain the block returns
// RemoveResult.NotFound = true without error.
//
// Implementations must use atomic writes (write to a tmp file then rename) to
// avoid leaving a corrupted shellrc on partial write.
type ShellRcPort interface {
	// EnsureBlock ensures that rcPath contains a guarded block identified by
	// blockID with the given content. If no block exists, it is appended. If
	// a block exists with different content, it is replaced. If the content
	// matches, no write is performed.
	//
	// Returns an error if rcPath cannot be read or written.
	EnsureBlock(rcPath, blockID, content string) (EnsureResult, error)

	// RemoveBlock removes the guarded block identified by blockID from rcPath.
	// If the block does not exist, RemoveResult.NotFound is true and no error
	// is returned (idempotent).
	//
	// Returns an error if rcPath cannot be read or written.
	RemoveBlock(rcPath, blockID string) (RemoveResult, error)
}

// MockShellRcPort is a test double for ShellRcPort. It records calls and
// returns the values set on its Func fields. Use NewMockShellRcPort for safe
// defaults.
type MockShellRcPort struct {
	// EnsureBlockFunc overrides EnsureBlock when non-nil.
	EnsureBlockFunc func(rcPath, blockID, content string) (EnsureResult, error)
	// RemoveBlockFunc overrides RemoveBlock when non-nil.
	RemoveBlockFunc func(rcPath, blockID string) (RemoveResult, error)

	// EnsureBlockCalls records each call to EnsureBlock.
	EnsureBlockCalls []struct{ RcPath, BlockID, Content string }
	// RemoveBlockCalls records each call to RemoveBlock.
	RemoveBlockCalls []struct{ RcPath, BlockID string }
}

// NewMockShellRcPort returns a MockShellRcPort whose EnsureBlock returns
// EnsureResult{Written: true} and RemoveBlock returns RemoveResult{Removed: true}
// unless Func fields are set.
func NewMockShellRcPort() *MockShellRcPort { return &MockShellRcPort{} }

// EnsureBlock implements ShellRcPort.
func (m *MockShellRcPort) EnsureBlock(rcPath, blockID, content string) (EnsureResult, error) {
	m.EnsureBlockCalls = append(m.EnsureBlockCalls, struct{ RcPath, BlockID, Content string }{rcPath, blockID, content})
	if m.EnsureBlockFunc != nil {
		return m.EnsureBlockFunc(rcPath, blockID, content)
	}
	return EnsureResult{Written: true}, nil
}

// RemoveBlock implements ShellRcPort.
func (m *MockShellRcPort) RemoveBlock(rcPath, blockID string) (RemoveResult, error) {
	m.RemoveBlockCalls = append(m.RemoveBlockCalls, struct{ RcPath, BlockID string }{rcPath, blockID})
	if m.RemoveBlockFunc != nil {
		return m.RemoveBlockFunc(rcPath, blockID)
	}
	return RemoveResult{Removed: true}, nil
}
