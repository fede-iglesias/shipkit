package uninstall

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/fede-iglesias/shipkit/ports"
)

// TestPlan_Print_IncludesAllPaths verifies that Print writes every resolved
// path: data dir, config dir, cache dir, binary, marker, completion paths for
// each shell, shell RC entries, and autostart info.
func TestPlan_Print_IncludesAllPaths(t *testing.T) {
	plan := Plan{
		AppName:   "kt",
		DataDir:   "/home/u/.local/share/kt",
		ConfigDir: "/home/u/.config/kt",
		CacheDir:  "/home/u/.cache/kt",
		BinaryPath: "/usr/local/bin/kt",
		MarkerPath: "/home/u/.local/share/kt/.shipkit.installed",
		CompletionPaths: map[string]string{
			"zsh":  "/home/u/.local/share/zsh/site-functions/_kt",
			"bash": "/home/u/.local/share/bash-completion/completions/kt",
		},
		ShellRCFiles: []ShellRCEntry{
			{File: "~/.zshrc", BlockSummary: "fpath line + autoload compinit"},
		},
		AutostartUnit: &AutostartInfo{
			Label:    "com.example.kt",
			UnitPath: "/home/u/Library/LaunchAgents/com.example.kt.plist",
		},
		KeepData:   false,
		KeepConfig: false,
		KeepBinary: false,
	}

	var buf bytes.Buffer
	if err := plan.Print(&buf); err != nil {
		t.Fatalf("Print returned error: %v", err)
	}
	out := buf.String()

	checks := []string{
		"kt",
		"/home/u/.local/share/kt",
		"/home/u/.config/kt",
		"/home/u/.cache/kt",
		"/usr/local/bin/kt",
		"/home/u/.local/share/kt/.shipkit.installed",
		"/home/u/.local/share/zsh/site-functions/_kt",
		"/home/u/.local/share/bash-completion/completions/kt",
		"~/.zshrc",
		"fpath line + autoload compinit",
		"com.example.kt",
	}
	for _, want := range checks {
		if !strings.Contains(out, want) {
			t.Errorf("Print output missing %q\n--- output ---\n%s", want, out)
		}
	}
}

// TestPlan_Print_RespectsKeepFlags verifies that when KeepData=true, the
// output contains the "(kept)" annotation next to the data dir.
func TestPlan_Print_RespectsKeepFlags(t *testing.T) {
	plan := Plan{
		AppName:   "kt",
		DataDir:   "/home/u/.local/share/kt",
		ConfigDir: "/home/u/.config/kt",
		CacheDir:  "/home/u/.cache/kt",
		BinaryPath: "/usr/local/bin/kt",
		KeepData:   true,
		KeepConfig: false,
		KeepBinary: false,
	}

	var buf bytes.Buffer
	if err := plan.Print(&buf); err != nil {
		t.Fatalf("Print returned error: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "(kept)") {
		t.Errorf("expected '(kept)' annotation when KeepData=true\n--- output ---\n%s", out)
	}
}

// TestPlan_Print_NoAutostart verifies that when AutostartUnit is nil, the
// output contains "(not installed)" for the autostart line.
func TestPlan_Print_NoAutostart(t *testing.T) {
	plan := Plan{
		AppName:    "kt",
		DataDir:    "/home/u/.local/share/kt",
		ConfigDir:  "/home/u/.config/kt",
		CacheDir:   "/home/u/.cache/kt",
		BinaryPath: "/usr/local/bin/kt",
	}

	var buf bytes.Buffer
	if err := plan.Print(&buf); err != nil {
		t.Fatalf("Print returned error: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "(not installed)") {
		t.Errorf("expected '(not installed)' when AutostartUnit is nil\n--- output ---\n%s", out)
	}
}

// TestBuildPlan_ResolvesAllPaths verifies that BuildPlan populates all Plan
// fields by calling the injected ports without performing any mutations.
func TestBuildPlan_ResolvesAllPaths(t *testing.T) {
	completion := ports.NewMockCompletionPort()
	completion.CompletionPathFunc = func(shell ports.ShellKind, app, home string) (string, error) {
		switch shell {
		case ports.ShellZsh:
			return "/data/zsh/site-functions/_" + app, nil
		case ports.ShellBash:
			return "/data/bash-completion/completions/" + app, nil
		default:
			return "/data/fish/" + app + ".fish", nil
		}
	}

	deps := Deps{
		AppName:        "kt",
		BinPath:        "/usr/local/bin/kt",
		AutostartLabel: "com.example.kt",
		ShellKinds:     []ports.ShellKind{ports.ShellZsh, ports.ShellBash},
		FS:             ports.NewMockFsPort(),
		Paths: &ports.MockPathsPort{
			UserHomeResult: "/home/u",
			DataDirFunc:    func(app string) (string, error) { return "/home/u/.local/share/" + app, nil },
			ConfigDirFunc:  func(app string) (string, error) { return "/home/u/.config/" + app, nil },
			CacheDirFunc:   func(app string) (string, error) { return "/home/u/.cache/" + app, nil },
		},
		ShellRc:    ports.NewMockShellRcPort(),
		Completion: completion,
		Autostart:  ports.NewMockAutostartPort(),
		Prompt:     ports.NewMockPromptPort(),
	}
	opts := Options{KeepData: false, KeepConfig: false, KeepBinary: false}

	plan, err := BuildPlan(deps, opts)
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}

	if plan.AppName != "kt" {
		t.Errorf("AppName: got %q, want %q", plan.AppName, "kt")
	}
	if plan.DataDir == "" {
		t.Error("DataDir must be non-empty")
	}
	if plan.ConfigDir == "" {
		t.Error("ConfigDir must be non-empty")
	}
	if plan.CacheDir == "" {
		t.Error("CacheDir must be non-empty")
	}
	if plan.BinaryPath != "/usr/local/bin/kt" {
		t.Errorf("BinaryPath: got %q, want %q", plan.BinaryPath, "/usr/local/bin/kt")
	}
	if plan.MarkerPath == "" {
		t.Error("MarkerPath must be non-empty (resolved from DataDir)")
	}
	if len(plan.CompletionPaths) != 2 {
		t.Errorf("CompletionPaths: got %d entries, want 2", len(plan.CompletionPaths))
	}
	if plan.KeepData != opts.KeepData {
		t.Errorf("KeepData: got %v, want %v", plan.KeepData, opts.KeepData)
	}
}

// TestBuildPlan_EmptyAppName verifies that BuildPlan returns an error when
// AppName is empty, exercising the validation guard.
func TestBuildPlan_EmptyAppName(t *testing.T) {
	deps := Deps{
		AppName:    "",
		FS:         ports.NewMockFsPort(),
		Paths:      ports.NewMockPathsPort(),
		ShellRc:    ports.NewMockShellRcPort(),
		Completion: ports.NewMockCompletionPort(),
		Autostart:  ports.NewMockAutostartPort(),
		Prompt:     ports.NewMockPromptPort(),
	}
	_, err := BuildPlan(deps, Options{})
	if err == nil {
		t.Fatal("expected error when AppName is empty, got nil")
	}
}

// TestBuildPlan_CompletionPathError verifies that when CompletionPath returns
// an error, BuildPlan skips that shell but continues and returns without error.
func TestBuildPlan_CompletionPathError(t *testing.T) {
	completion := ports.NewMockCompletionPort()
	completion.CompletionPathFunc = func(shell ports.ShellKind, app, home string) (string, error) {
		return "", errors.New("unsupported shell")
	}

	deps := Deps{
		AppName:    "kt",
		BinPath:    "/usr/local/bin/kt",
		ShellKinds: []ports.ShellKind{ports.ShellZsh},
		FS:         ports.NewMockFsPort(),
		Paths: &ports.MockPathsPort{
			UserHomeResult: "/home/u",
			DataDirFunc:    func(app string) (string, error) { return "/home/u/.local/share/" + app, nil },
			ConfigDirFunc:  func(app string) (string, error) { return "/home/u/.config/" + app, nil },
			CacheDirFunc:   func(app string) (string, error) { return "/home/u/.cache/" + app, nil },
		},
		ShellRc:    ports.NewMockShellRcPort(),
		Completion: completion,
		Autostart:  ports.NewMockAutostartPort(),
		Prompt:     ports.NewMockPromptPort(),
	}

	plan, err := BuildPlan(deps, Options{})
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}
	if len(plan.CompletionPaths) != 0 {
		t.Errorf("expected empty CompletionPaths when all shells error, got %v", plan.CompletionPaths)
	}
}

// TestBuildPlan_EmptyDataDir verifies that when DataDir returns empty string,
// MarkerPath is also empty.
func TestBuildPlan_EmptyDataDir(t *testing.T) {
	deps := Deps{
		AppName:    "kt",
		BinPath:    "/usr/local/bin/kt",
		ShellKinds: []ports.ShellKind{ports.ShellZsh},
		FS:         ports.NewMockFsPort(),
		Paths: &ports.MockPathsPort{
			UserHomeResult: "/home/u",
			DataDirFunc:    func(app string) (string, error) { return "", errors.New("no xdg") },
			ConfigDirFunc:  func(app string) (string, error) { return "/home/u/.config/" + app, nil },
			CacheDirFunc:   func(app string) (string, error) { return "/home/u/.cache/" + app, nil },
		},
		ShellRc:    ports.NewMockShellRcPort(),
		Completion: ports.NewMockCompletionPort(),
		Autostart:  ports.NewMockAutostartPort(),
		Prompt:     ports.NewMockPromptPort(),
	}

	plan, err := BuildPlan(deps, Options{})
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}
	if plan.MarkerPath != "" {
		t.Errorf("MarkerPath must be empty when DataDir is empty, got %q", plan.MarkerPath)
	}
}

// TestBuildPlan_NoMutations verifies that BuildPlan never calls RemoveDir,
// RemoveBlock, Stop, or Uninstall (dry-run semantics).
func TestBuildPlan_NoMutations(t *testing.T) {
	fs := ports.NewMockFsPort()
	shellrc := ports.NewMockShellRcPort()
	autostart := ports.NewMockAutostartPort()

	deps := Deps{
		AppName:    "kt",
		BinPath:    "/usr/local/bin/kt",
		ShellKinds: []ports.ShellKind{ports.ShellZsh},
		FS:         fs,
		Paths: &ports.MockPathsPort{
			UserHomeResult: "/home/u",
			DataDirFunc:    func(app string) (string, error) { return "/home/u/.local/share/" + app, nil },
			ConfigDirFunc:  func(app string) (string, error) { return "/home/u/.config/" + app, nil },
			CacheDirFunc:   func(app string) (string, error) { return "/home/u/.cache/" + app, nil },
		},
		ShellRc:    shellrc,
		Completion: ports.NewMockCompletionPort(),
		Autostart:  autostart,
		Prompt:     ports.NewMockPromptPort(),
	}

	if _, err := BuildPlan(deps, Options{}); err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}

	if len(fs.RemoveDirCalls) != 0 {
		t.Errorf("BuildPlan called RemoveDir %d times, want 0", len(fs.RemoveDirCalls))
	}
	if len(shellrc.RemoveBlockCalls) != 0 {
		t.Errorf("BuildPlan called RemoveBlock %d times, want 0", len(shellrc.RemoveBlockCalls))
	}
	if len(autostart.StopCalls) != 0 {
		t.Errorf("BuildPlan called autostart.Stop %d times, want 0", len(autostart.StopCalls))
	}
	if len(autostart.UninstallCalls) != 0 {
		t.Errorf("BuildPlan called autostart.Uninstall %d times, want 0", len(autostart.UninstallCalls))
	}
}
