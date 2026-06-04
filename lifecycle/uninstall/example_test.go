package uninstall_test

import (
	"context"
	"fmt"

	"github.com/fede-iglesias/shipkit/lifecycle/uninstall"
	"github.com/fede-iglesias/shipkit/ports"
)

// ExampleRun demonstrates the standard usage of Run using Mock ports.
// The prompt is wired to return true (user confirms) and RemoveBinaryFunc
// is wired to succeed, producing BinaryDeletedNow.
func ExampleRun() {
	deps := uninstall.Deps{
		AppName: "myapp",
		BinPath: "/usr/local/bin/myapp",
		FS:      ports.NewMockFsPort(),
		Paths: &ports.MockPathsPort{
			DataDirFunc:   func(app string) (string, error) { return "/home/user/.local/share/" + app, nil },
			ConfigDirFunc: func(app string) (string, error) { return "/home/user/.config/" + app, nil },
			CacheDirFunc:  func(app string) (string, error) { return "/home/user/.cache/" + app, nil },
		},
		ShellRc:          ports.NewMockShellRcPort(),
		Completion:       ports.NewMockCompletionPort(),
		Autostart:        ports.NewMockAutostartPort(),
		Prompt:           &ports.MockPromptPort{ConfirmResult: true},
		RemoveBinaryFunc: func(path string) error { return nil },
	}

	result, err := uninstall.Run(context.Background(), deps, uninstall.Options{}, nil)
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Println("binary action:", result.BinaryAction)
	// Output: binary action: deleted-now
}

// ExampleRun_keepData demonstrates --keep-data: the data directory is
// preserved while everything else is removed.
func ExampleRun_keepData() {
	deps := uninstall.Deps{
		AppName: "myapp",
		BinPath: "/usr/local/bin/myapp",
		FS:      ports.NewMockFsPort(),
		Paths: &ports.MockPathsPort{
			DataDirFunc:   func(app string) (string, error) { return "/home/user/.local/share/" + app, nil },
			ConfigDirFunc: func(app string) (string, error) { return "/home/user/.config/" + app, nil },
			CacheDirFunc:  func(app string) (string, error) { return "/home/user/.cache/" + app, nil },
		},
		ShellRc:    ports.NewMockShellRcPort(),
		Completion: ports.NewMockCompletionPort(),
		Autostart:  ports.NewMockAutostartPort(),
		Prompt:     &ports.MockPromptPort{ConfirmResult: true},
	}

	result, err := uninstall.Run(context.Background(), deps, uninstall.Options{
		KeepData: true,
	}, nil)
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	// Data dir is skipped; binary action is kept (no RemoveBinaryFunc wired).
	fmt.Println("binary action:", result.BinaryAction)
	// Output: binary action: kept
}

// ExampleRun_print demonstrates --print dry-run mode: the plan is computed
// but no changes are made.
func ExampleRun_print() {
	deps := uninstall.Deps{
		AppName: "myapp",
		BinPath: "/usr/local/bin/myapp",
		FS:      ports.NewMockFsPort(),
		Paths: &ports.MockPathsPort{
			DataDirFunc:   func(app string) (string, error) { return "/home/user/.local/share/" + app, nil },
			ConfigDirFunc: func(app string) (string, error) { return "/home/user/.config/" + app, nil },
			CacheDirFunc:  func(app string) (string, error) { return "/home/user/.cache/" + app, nil },
		},
		ShellRc:    ports.NewMockShellRcPort(),
		Completion: ports.NewMockCompletionPort(),
		Autostart:  ports.NewMockAutostartPort(),
		Prompt:     ports.NewMockPromptPort(),
	}

	result, err := uninstall.Run(context.Background(), deps, uninstall.Options{
		Print: true,
	}, nil)
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	// BinaryAction is empty in dry-run (no mutation occurred).
	fmt.Println("dry-run removed count:", len(result.Removed))
	fmt.Println("binary action:", result.BinaryAction)
	// Output:
	// dry-run removed count: 4
	// binary action:
}
