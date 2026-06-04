package install_test

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/fede-iglesias/shipkit/lifecycle/install"
	"github.com/fede-iglesias/shipkit/ports"
	"github.com/spf13/cobra"
)

// ExampleRun demonstrates a complete install using mock ports. This is the
// pattern consumers use in their own integration tests or documentation.
func ExampleRun() {
	// Fixed time for deterministic output.
	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)

	// Create a fresh temp directory so the example is idempotent across runs.
	dataDir, err := os.MkdirTemp("", "shipkit-install-example-*")
	if err != nil {
		fmt.Printf("error creating temp dir: %v\n", err)
		return
	}
	defer os.RemoveAll(dataDir)

	paths := ports.NewMockPathsPort()
	paths.DataDirFunc = func(app string) (string, error) { return dataDir, nil }
	paths.ExecutableResult = "/usr/local/bin/myapp"
	paths.InPATHResult = true

	env := ports.NewMockEnvPort()
	env.ShellResult = ports.ShellUnknown // skip completions for a minimal example

	cfg := install.Config{
		AppName: "myapp",
		Version: "v0.1.0",
	}

	deps := install.Deps{
		Cfg:        cfg,
		FS:         ports.NewMockFsPort(),
		Paths:      paths,
		Env:        env,
		ShellRc:    ports.NewMockShellRcPort(),
		Completion: ports.NewMockCompletionPort(),
		Autostart:  ports.NewMockAutostartPort(),
		Prompt:     ports.NewMockPromptPort(),
		Clock:      ports.NewMockClockPort(now),
	}

	root := &cobra.Command{Use: "myapp"}

	result, err := install.Run(context.Background(), deps, install.Options{}, root)
	if err != nil {
		// In tests, errors are surfaced via t.Fatal. Here we print for the example.
		fmt.Printf("error: %v\n", err)
		return
	}

	fmt.Printf("installed: %v\n", !result.AlreadyInstalled)
	fmt.Printf("version: %s\n", result.Marker.VersionInstalled)
	fmt.Printf("bin_path: %s\n", result.Marker.BinPath)
	fmt.Printf("path_ensured: %v\n", result.PathEnsured)
	fmt.Printf("manifest_items: %d\n", len(result.Manifest))
	// Output:
	// installed: true
	// version: v0.1.0
	// bin_path: /usr/local/bin/myapp
	// path_ensured: true
	// manifest_items: 2
}
