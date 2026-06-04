package clean_test

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/fede-iglesias/shipkit/lifecycle/clean"
	"github.com/fede-iglesias/shipkit/ports"
)

// ExampleRun demonstrates using the clean verb with mock ports.
// This is the canonical pattern for wiring the clean verb in tests and
// in consumer cmd layers that need custom injection.
func ExampleRun() {
	// Simulate a fixed clock so the example is deterministic.
	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	clock := ports.NewMockClockPort(now)
	fs := ports.NewMockFsPort()
	prompt := ports.NewMockPromptPort() // defaults: confirm=true, non-interactive
	paths := ports.NewMockPathsPort()

	// Two snapshots: one old enough to clean, one recent.
	age40 := now.Add(-40 * 24 * time.Hour)
	age5 := now.Add(-5 * 24 * time.Hour)

	deps := clean.Deps{
		AppName: "myapp",
		FS:      fs,
		Paths:   paths,
		Clock:   clock,
		Prompt:  prompt,
		ListSnapshotsFunc: func(snapshotDir string) ([]clean.SnapshotEntry, error) {
			return []clean.SnapshotEntry{
				{Path: filepath.Join(snapshotDir, "snap-old"), ModTime: age40, Size: 8 * 1024 * 1024},
				{Path: filepath.Join(snapshotDir, "snap-new"), ModTime: age5, Size: 8 * 1024 * 1024},
			}, nil
		},
	}

	opts := clean.Options{
		Snapshots: true,
		OlderThan: 30 * 24 * time.Hour, // 30 days
		Keep:      1,                    // always keep at least 1 snapshot
		Yes:       true,                 // skip prompt in example
	}

	result, err := clean.Run(context.Background(), deps, opts)
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Printf("Removed %d item(s)\n", len(result.Items))
	fmt.Printf("Reclaimed %d bytes\n", result.Reclaimed)

	// Output:
	// Removed 1 item(s)
	// Reclaimed 8388608 bytes
}

// ExampleRun_noScope demonstrates that calling Run without any scope flag
// returns ErrNoScope, which the cobra layer converts into a help print + exit 1.
func ExampleRun_noScope() {
	deps := clean.Deps{
		AppName: "myapp",
		FS:      ports.NewMockFsPort(),
		Paths:   ports.NewMockPathsPort(),
		Clock:   ports.NewMockClockPort(time.Now()),
		Prompt:  ports.NewMockPromptPort(),
	}
	_, err := clean.Run(context.Background(), deps, clean.Options{})
	if err != nil {
		fmt.Println("error:", err)
	}

	// Output:
	// error: clean: at least one scope flag (--snapshots, --tmp, --cache, --logs, --all) is required
}

// ExampleNewCommand demonstrates constructing and executing the cobra command
// in a real cobra hierarchy (no rootCmd.Execute() - just showing wiring).
func ExampleNewCommand() {
	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	deps := clean.Deps{
		AppName: "myapp",
		FS:      ports.NewMockFsPort(),
		Paths:   ports.NewMockPathsPort(),
		Clock:   ports.NewMockClockPort(now),
		Prompt:  ports.NewMockPromptPort(),
		ListSnapshotsFunc: func(_ string) ([]clean.SnapshotEntry, error) {
			return nil, nil // nothing to clean
		},
	}

	cmd := clean.NewCommand(deps)
	fmt.Println(cmd.Use)

	// Output:
	// clean
}
