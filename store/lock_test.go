package store_test

import (
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/fede-iglesias/shipkit/store"
)

func TestLockAcquireRelease(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.lock")

	lk, err := store.Acquire(lockPath, time.Second)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if err := lk.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
}

func TestLockAcquireTwiceSequential(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.lock")

	// Acquire and release; then acquire again - must succeed.
	lk, err := store.Acquire(lockPath, time.Second)
	if err != nil {
		t.Fatalf("first Acquire: %v", err)
	}
	if err := lk.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}

	lk2, err := store.Acquire(lockPath, time.Second)
	if err != nil {
		t.Fatalf("second Acquire: %v", err)
	}
	if err := lk2.Release(); err != nil {
		t.Fatalf("second Release: %v", err)
	}
}

// TestLockConcurrentGoroutines exercises mutual exclusion between two
// goroutines competing on the same lock file.
//
// LIMITATION: goroutine-only test. It does NOT validate cross-process flock
// semantics because Go's flock on the same file descriptor (within one process)
// may behave differently from two separate processes. A proper cross-process
// test is tracked in backlog.
func TestLockConcurrentGoroutines(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	lockPath := filepath.Join(dir, "concurrent.lock")

	var mu sync.Mutex
	counter := 0
	const goroutines = 4

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			lk, err := store.Acquire(lockPath, 5*time.Second)
			if err != nil {
				t.Errorf("goroutine Acquire: %v", err)
				return
			}
			defer func() {
				if err := lk.Release(); err != nil {
					t.Errorf("goroutine Release: %v", err)
				}
			}()

			// Critical section: increment and check no overlap.
			mu.Lock()
			counter++
			v := counter
			mu.Unlock()
			// Brief pause to increase chance of concurrent goroutine interfering.
			time.Sleep(2 * time.Millisecond)
			mu.Lock()
			if counter != v {
				t.Errorf("lock not exclusive: counter changed from %d to %d during critical section", v, counter)
			}
			mu.Unlock()
		}()
	}
	wg.Wait()
}

func TestLockTimeout(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	lockPath := filepath.Join(dir, "timeout.lock")

	// Acquire the lock on the main goroutine.
	lk, err := store.Acquire(lockPath, time.Second)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	defer func() { _ = lk.Release() }()

	// Try to acquire from a second goroutine with a short timeout.
	done := make(chan error, 1)
	go func() {
		_, err := store.Acquire(lockPath, 50*time.Millisecond)
		done <- err
	}()

	select {
	case err := <-done:
		if !errors.Is(err, store.ErrLockTimeout) {
			t.Errorf("expected ErrLockTimeout, got: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("Acquire did not return within 2s")
	}
}

func TestLockCreatesFileIfMissing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Use a path in a sub-dir that doesn't exist yet.
	lockPath := filepath.Join(dir, "subdir", "mylock.lock")

	lk, err := store.Acquire(lockPath, time.Second)
	if err != nil {
		t.Fatalf("Acquire to missing path: %v", err)
	}
	if err := lk.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
}
