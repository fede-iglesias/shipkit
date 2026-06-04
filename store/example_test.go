package store_test

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fede-iglesias/shipkit/store"
)

// ExampleAtomicWrite demonstrates writing a file atomically.
// The temp-file + rename idiom ensures readers never see a partial write.
func ExampleAtomicWrite() {
	dir, err := os.MkdirTemp("", "store-example-*")
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "hello.md")
	data := []byte("# Hello\n\nWorld.\n")

	if err := store.AtomicWrite(path, data, 0o644); err != nil {
		fmt.Println("error:", err)
		return
	}

	got, _ := os.ReadFile(path)
	fmt.Print(string(got))
	// Output:
	// # Hello
	//
	// World.
}

// ExampleBodyChecksum demonstrates content-addressable checksum computation.
// Two bodies that differ only in trailing whitespace produce the same digest.
func ExampleBodyChecksum() {
	a := store.BodyChecksum([]byte("hello\n"))
	b := store.BodyChecksum([]byte("hello\n\n\n"))
	c := store.BodyChecksum([]byte("hello   "))

	fmt.Println(a == b) // trailing newlines normalized
	fmt.Println(a == c) // trailing spaces normalized
	// Output:
	// true
	// true
}

// ExampleAcquire demonstrates acquiring and releasing an advisory lock.
// Only callers that use Acquire participate in mutual exclusion.
func ExampleAcquire() {
	dir, err := os.MkdirTemp("", "store-lock-example-*")
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	defer os.RemoveAll(dir)

	lockPath := filepath.Join(dir, "write.lock")

	lk, err := store.Acquire(lockPath, time.Second)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	// Critical section: only one goroutine/process holds the lock.
	fmt.Println("lock acquired")
	if err := lk.Release(); err != nil {
		fmt.Println("release error:", err)
		return
	}
	fmt.Println("lock released")
	// Output:
	// lock acquired
	// lock released
}

// ExamplePathFor demonstrates resolving card paths from kind and slug.
func ExamplePathFor() {
	// Person card: no extra options needed.
	p, err := store.PathFor("/project", "person", "alice")
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(filepath.ToSlash(p))

	// ADR card: requires WithADRID.
	p2, err := store.PathFor("/project", "adr", "use-postgres", store.WithADRID("ADR-0001"))
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(filepath.ToSlash(p2))
	// Output:
	// /project/knowledge/people/alice/index.md
	// /project/knowledge/decisions/0001-use-postgres.md
}
