package store

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"
)

// ErrLockTimeout is returned when a lock cannot be acquired within the timeout.
var ErrLockTimeout = errors.New("store: lock acquisition timed out")

// Lock represents an acquired exclusive lock on a file.
// Call Release when done to release the lock and close the file.
//
// Lock is NOT safe for concurrent use from multiple goroutines. Each goroutine
// that needs mutual exclusion must Acquire its own Lock on the same path.
// The zero value is invalid; use Acquire to obtain a Lock.
type Lock struct {
	f *os.File
}

// flockFn is the flock system call used by Acquire. Replaceable in tests to
// inject errors without OS-level tricks. Production code always delegates to
// syscall.Flock.
var flockFn = func(fd int, how int) error {
	return syscall.Flock(fd, how)
}

// unlockFn is the flock LOCK_UN call used by Release. A separate injectable
// is needed because flockFn is guarded by the EWOULDBLOCK retry loop in
// Acquire and cannot be reused for unlock without semantic changes.
var unlockFn = func(fd int) error {
	return syscall.Flock(fd, syscall.LOCK_UN)
}

// Acquire obtains an exclusive flock on path, retrying until timeout elapses.
// Creates the lock file (and parent directories) if they do not exist.
// Returns ErrLockTimeout if the lock is not obtained within the given duration.
//
// The returned Lock must be released via Release when the critical section ends.
//
// Returns:
//   - *Lock on success.
//   - ErrLockTimeout when timeout elapses without acquiring the lock.
//   - A wrapped error when the lock file cannot be opened or the flock call
//     returns an unexpected (non-EWOULDBLOCK) error.
func Acquire(path string, timeout time.Duration) (*Lock, error) {
	// Ensure parent directory exists.
	if err := EnsureParent(path); err != nil {
		return nil, err
	}

	// Open (or create) the lock file.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("store: open lock file %q: %w", path, err)
	}

	// Retry loop: attempt LOCK_EX|LOCK_NB, backoff on EWOULDBLOCK.
	deadline := time.Now().Add(timeout)
	const backoff = 10 * time.Millisecond

	for {
		err := flockFn(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return &Lock{f: f}, nil
		}

		// EWOULDBLOCK means another holder exists; retry if within timeout.
		if errors.Is(err, syscall.EWOULDBLOCK) {
			if time.Now().After(deadline) {
				_ = f.Close()
				return nil, ErrLockTimeout
			}
			time.Sleep(backoff)
			continue
		}

		// Any other error is fatal.
		_ = f.Close()
		return nil, fmt.Errorf("store: flock %q: %w", path, err)
	}
}

// Release unlocks and closes the lock file. Calling Release on a nil Lock
// is a no-op.
//
// Returns a wrapped error if the underlying flock unlock or file close fails.
// Callers should still treat the lock as released even on error.
func (l *Lock) Release() error {
	if l == nil || l.f == nil {
		return nil
	}
	if err := unlockFn(int(l.f.Fd())); err != nil {
		_ = l.f.Close()
		return fmt.Errorf("store: unlock %q: %w", l.f.Name(), err)
	}
	return l.f.Close()
}
