package adapters

import (
	"fmt"
	"os"
	"strings"

	"github.com/fede-iglesias/shipkit/ports"
)

// ShellRcRealAdapter is the production implementation of
// [github.com/fede-iglesias/shipkit/ports.ShellRcPort]. It manages guarded
// content blocks in shell RC files using sentinel comment markers:
//
//	# >>> shipkit:<blockID> >>>
//	<content>
//	# <<< shipkit:<blockID> <<<
//
// Operations are idempotent: EnsureBlock is a no-op when the block already
// exists with identical content; RemoveBlock is a no-op when the block is
// absent. Both operations use atomic writes (tmp file then rename) so a
// partial failure cannot corrupt the shell RC file.
//
// # Injectable seams
//
// ReadFileFn, WriteFileFn, MkdirAllFn, TempFileFn, and RenameFn are all
// injectable for failure-path tests. Use NewShellRcReal in production.
type ShellRcRealAdapter struct {
	// ReadFileFn reads the contents of a file. Defaults to os.ReadFile.
	ReadFileFn func(name string) ([]byte, error)

	// WriteFileFn writes data to a file with the given permissions.
	// Defaults to os.WriteFile.
	WriteFileFn func(name string, data []byte, perm os.FileMode) error

	// RenameFn renames (moves) a file atomically. Defaults to os.Rename.
	RenameFn func(oldpath, newpath string) error
}

// NewShellRcReal returns a ShellRcRealAdapter with all seams wired to real os
// functions. Use this constructor in production wiring.
func NewShellRcReal() *ShellRcRealAdapter {
	return &ShellRcRealAdapter{
		ReadFileFn:  os.ReadFile,
		WriteFileFn: os.WriteFile,
		RenameFn:    os.Rename,
	}
}

// openMarker returns the opening sentinel line for blockID.
func openMarker(blockID string) string {
	return "# >>> shipkit:" + blockID + " >>>"
}

// closeMarker returns the closing sentinel line for blockID.
func closeMarker(blockID string) string {
	return "# <<< shipkit:" + blockID + " <<<"
}

// EnsureBlock ensures rcPath contains a guarded block with blockID and the
// given content. If no block exists it is appended. If a block exists with
// different content it is replaced. If content matches no write is performed.
//
// Returns an EnsureResult describing whether the file was written, updated, or
// left unchanged. Returns an error if rcPath cannot be read or atomically
// written.
func (a *ShellRcRealAdapter) EnsureBlock(rcPath, blockID, content string) (ports.EnsureResult, error) {
	open := openMarker(blockID)
	close := closeMarker(blockID)

	// Read existing file; treat a missing file as an empty string (first install).
	raw, err := a.ReadFileFn(rcPath)
	if err != nil && !os.IsNotExist(err) {
		return ports.EnsureResult{}, fmt.Errorf("shellrc ensure: read %s: %w", rcPath, err)
	}
	existing := string(raw)

	startIdx := strings.Index(existing, open)
	endIdx := strings.Index(existing, close)

	if startIdx != -1 && endIdx != -1 && endIdx > startIdx {
		// Block already present; extract current content between markers.
		// +1 to skip the trailing newline after the open marker line.
		currentContent := existing[startIdx+len(open)+1 : endIdx]
		// Remove a trailing newline that was added when the block was written.
		currentContent = strings.TrimRight(currentContent, "\n")
		if currentContent == strings.TrimRight(content, "\n") {
			return ports.EnsureResult{Unchanged: true}, nil
		}

		// Content differs: replace the block in-place.
		newBlock := open + "\n" + content + "\n" + close
		// The slice includes the close marker but not what follows it.
		replaced := existing[:startIdx] + newBlock + existing[endIdx+len(close):]
		if err := a.atomicWrite(rcPath, []byte(replaced)); err != nil {
			return ports.EnsureResult{}, err
		}
		return ports.EnsureResult{Updated: true}, nil
	}

	// Block absent: append it.
	newBlock := "\n" + open + "\n" + content + "\n" + close + "\n"
	updated := existing + newBlock
	if err := a.atomicWrite(rcPath, []byte(updated)); err != nil {
		return ports.EnsureResult{}, err
	}
	return ports.EnsureResult{Written: true}, nil
}

// RemoveBlock removes the guarded block identified by blockID from rcPath.
// Returns RemoveResult{NotFound: true} without error when no matching block
// exists (idempotent). Returns an error if rcPath cannot be read or written.
func (a *ShellRcRealAdapter) RemoveBlock(rcPath, blockID string) (ports.RemoveResult, error) {
	raw, err := a.ReadFileFn(rcPath)
	if err != nil {
		if os.IsNotExist(err) {
			return ports.RemoveResult{NotFound: true}, nil
		}
		return ports.RemoveResult{}, fmt.Errorf("shellrc remove: read %s: %w", rcPath, err)
	}
	existing := string(raw)

	open := openMarker(blockID)
	close := closeMarker(blockID)

	startIdx := strings.Index(existing, open)
	endIdx := strings.Index(existing, close)

	if startIdx == -1 || endIdx == -1 || endIdx <= startIdx {
		return ports.RemoveResult{NotFound: true}, nil
	}

	// Remove the block including trailing newline after the close marker.
	afterClose := endIdx + len(close)
	// Consume optional leading newline before the open marker (appended by EnsureBlock).
	blockStart := startIdx
	if blockStart > 0 && existing[blockStart-1] == '\n' {
		blockStart--
	}

	var removed string
	if afterClose < len(existing) && existing[afterClose] == '\n' {
		removed = existing[:blockStart] + existing[afterClose+1:]
	} else {
		removed = existing[:blockStart] + existing[afterClose:]
	}

	if err := a.atomicWrite(rcPath, []byte(removed)); err != nil {
		return ports.RemoveResult{}, err
	}
	return ports.RemoveResult{Removed: true}, nil
}

// atomicWrite writes data to path via a sibling .tmp file then renames it
// atomically. This prevents a partially written shellrc on crash.
func (a *ShellRcRealAdapter) atomicWrite(path string, data []byte) error {
	tmpPath := path + ".shipkit.tmp"
	if err := a.WriteFileFn(tmpPath, data, 0o644); err != nil {
		return fmt.Errorf("shellrc: write tmp %s: %w", tmpPath, err)
	}
	if err := a.RenameFn(tmpPath, path); err != nil {
		// Best-effort cleanup of the tmp file.
		_ = a.WriteFileFn(tmpPath, nil, 0o644)
		return fmt.Errorf("shellrc: rename %s -> %s: %w", tmpPath, path, err)
	}
	return nil
}
