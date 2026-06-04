package ports_test

import (
	"context"
	"fmt"

	"github.com/fede-iglesias/shipkit/ports"
)

// ExamplePathsPort demonstrates how a consumer (e.g. the install verb) declares
// its dependencies as port interfaces and injects a fake in tests.
//
// In production code, inject a real adapter from shipkit/adapters instead of
// the mock.
func ExamplePathsPort() {
	// 1. Declare dependency on the port (done in Deps struct in practice).
	var paths ports.PathsPort

	// 2. Inject a test double (in production, inject adapters.NewXDGPathsPort()).
	mock := ports.NewMockPathsPort()
	mock.DataDirFunc = func(app string) (string, error) {
		return "/home/alice/.local/share/" + app, nil
	}
	paths = mock

	// 3. Use the port without knowing the implementation.
	dir, err := paths.DataDir("myapp")
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(dir)
	// Output: /home/alice/.local/share/myapp
}

// ExampleEnvPort_DetectShell shows how the install verb checks the shell before
// writing shell completion scripts, without accessing os.Getenv directly.
func ExampleEnvPort_DetectShell() {
	env := ports.NewMockEnvPort()
	env.ShellResult = ports.ShellZsh

	switch env.DetectShell() {
	case ports.ShellZsh:
		fmt.Println("detected: zsh")
	case ports.ShellBash:
		fmt.Println("detected: bash")
	case ports.ShellFish:
		fmt.Println("detected: fish")
	default:
		fmt.Println("detected: unknown")
	}
	// Output: detected: zsh
}

// ExampleShellRcPort_EnsureBlock illustrates the idempotent guarded-block
// pattern used by the install verb to add fpath entries to ~/.zshrc.
func ExampleShellRcPort_EnsureBlock() {
	rc := ports.NewMockShellRcPort()
	rc.EnsureBlockFunc = func(rcPath, blockID, content string) (ports.EnsureResult, error) {
		// Simulate first run: block did not exist.
		return ports.EnsureResult{Written: true}, nil
	}

	res, err := rc.EnsureBlock("/home/alice/.zshrc", "kt:fpath", "fpath+=(~/.local/share/zsh/site-functions)")
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	if res.Written {
		fmt.Println("block written")
	}
	// Output: block written
}

// ExamplePromptPort_Confirm shows how the uninstall verb confirms a destructive
// action and how tests bypass it without blocking.
func ExamplePromptPort_Confirm() {
	// In tests: inject a mock that auto-confirms without blocking.
	prompt := ports.NewMockPromptPort() // ConfirmResult = true, non-interactive

	ok, err := prompt.Confirm("Remove all app data? [y/N]", false)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	if ok {
		fmt.Println("confirmed")
	} else {
		fmt.Println("cancelled")
	}
	// Output: confirmed
}

// ExampleMockFsPort_CopyFile demonstrates injecting a FsPort fake for the
// install verb's CopyFile call and asserting the recorded arguments.
func ExampleMockFsPort_CopyFile() {
	fs := ports.NewMockFsPort()

	// Call CopyFile as the install verb would.
	_ = fs.CopyFile(context.Background(), "/tmp/app.tmp", "/usr/local/bin/app", 0o755)

	if len(fs.CopyFileCalls) == 1 {
		c := fs.CopyFileCalls[0]
		fmt.Printf("src=%s dst=%s mode=%o\n", c.Src, c.Dst, c.Mode)
	}
	// Output: src=/tmp/app.tmp dst=/usr/local/bin/app mode=755
}
