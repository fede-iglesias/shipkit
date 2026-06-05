package install_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/fede-iglesias/shipkit/lifecycle/install"
	"github.com/fede-iglesias/shipkit/ports"
)

// TestPlan_Print_IncludesAllPaths asserts that Plan.Print outputs every path:
// data dir, config dir, cache dir, binary, marker, completions, shell RC blocks.
func TestPlan_Print_IncludesAllPaths(t *testing.T) {
	deps, _ := newDeps(t)

	// Wire completion paths for zsh and bash.
	env := deps.Env.(*ports.MockEnvPort)
	env.ShellResult = ports.ShellZsh

	mockCP := ports.NewMockCompletionPort()
	mockCP.CompletionPathFunc = func(shell ports.ShellKind, app, home string) (string, error) {
		switch shell {
		case ports.ShellZsh:
			return "/home/u/.local/share/zsh/site-functions/_" + app, nil
		case ports.ShellBash:
			return "/home/u/.local/share/bash-completion/completions/" + app, nil
		}
		return "/tmp/completions/" + app, nil
	}
	deps.Completion = mockCP

	// Request both zsh and bash completions explicitly.
	opts := install.Options{
		Completions: []ports.ShellKind{ports.ShellZsh, ports.ShellBash},
	}

	plan, err := install.BuildPlan(deps, opts)
	if err != nil {
		t.Fatalf("BuildPlan error: %v", err)
	}

	var buf bytes.Buffer
	if err := plan.Print(&buf); err != nil {
		t.Fatalf("Plan.Print error: %v", err)
	}

	out := buf.String()

	checks := []struct {
		label   string
		keyword string
	}{
		{"data dir label", "data dir"},
		{"config dir label", "config dir"},
		{"cache dir label", "cache dir"},
		{"binary label", "binary"},
		{"marker label", "marker"},
		{"completions label", "completions"},
		{"zsh completion path", "_testapp"},
		{"shell RC blocks label", "shell RC"},
	}

	for _, c := range checks {
		if !strings.Contains(out, c.keyword) {
			t.Errorf("Print output missing %q\ngot:\n%s", c.keyword, out)
		}
	}
}

// TestPlan_Print_WithAutostart asserts that with autostart=true the print output
// includes autostart unit info (label and path), not "(not requested)".
func TestPlan_Print_WithAutostart(t *testing.T) {
	deps, _ := newDeps(t)
	deps.Cfg.EnableAutostart = true
	deps.Cfg.AutostartLabel = "com.example.testapp"

	opts := install.Options{
		Autostart: true,
	}

	plan, err := install.BuildPlan(deps, opts)
	if err != nil {
		t.Fatalf("BuildPlan error: %v", err)
	}
	if plan.AutostartUnit == nil {
		t.Fatal("BuildPlan should set AutostartUnit when autostart is enabled and requested")
	}

	var buf bytes.Buffer
	if err := plan.Print(&buf); err != nil {
		t.Fatalf("Plan.Print error: %v", err)
	}

	out := buf.String()

	if strings.Contains(out, "not requested") {
		t.Errorf("autostart was requested but output says 'not requested':\n%s", out)
	}
	if !strings.Contains(out, "com.example.testapp") {
		t.Errorf("output should contain autostart label 'com.example.testapp':\n%s", out)
	}
}

// TestPlan_Print_NoAutostart asserts that without autostart the output shows
// "(not requested)".
func TestPlan_Print_NoAutostart(t *testing.T) {
	deps, _ := newDeps(t)

	opts := install.Options{Autostart: false}

	plan, err := install.BuildPlan(deps, opts)
	if err != nil {
		t.Fatalf("BuildPlan error: %v", err)
	}

	var buf bytes.Buffer
	if err := plan.Print(&buf); err != nil {
		t.Fatalf("Plan.Print error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "not requested") {
		t.Errorf("output should contain 'not requested' when autostart is false:\n%s", out)
	}
}

// TestPlan_Print_NoCompletions asserts that when no completions are resolved
// the Print output contains "(none detected)".
func TestPlan_Print_NoCompletions(t *testing.T) {
	deps, _ := newDeps(t)
	// Unknown shell -> no completions autodetected, empty explicit list.
	env := deps.Env.(*ports.MockEnvPort)
	env.ShellResult = ports.ShellUnknown

	opts := install.Options{
		Completions: []ports.ShellKind{}, // explicit empty = skip
	}

	plan, err := install.BuildPlan(deps, opts)
	if err != nil {
		t.Fatalf("BuildPlan error: %v", err)
	}

	var buf bytes.Buffer
	if err := plan.Print(&buf); err != nil {
		t.Fatalf("Plan.Print error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "none detected") {
		t.Errorf("expected 'none detected' in output when no completions:\n%s", out)
	}
}

// TestBuildPlan_DataDir_Error asserts that BuildPlan returns an error when
// PathsPort.DataDir fails.
func TestBuildPlan_DataDir_Error(t *testing.T) {
	deps, _ := newDeps(t)
	paths := deps.Paths.(*ports.MockPathsPort)
	paths.DataDirFunc = func(app string) (string, error) {
		return "", fmt.Errorf("data dir unavailable")
	}

	_, err := install.BuildPlan(deps, install.Options{})
	if err == nil {
		t.Error("BuildPlan should return error when DataDir fails")
	}
}

// TestBuildPlan_CompletionPath_Error_SkipsShell asserts that when CompletionPath
// returns an error for a shell, BuildPlan skips that shell rather than failing.
func TestBuildPlan_CompletionPath_Error_SkipsShell(t *testing.T) {
	deps, _ := newDeps(t)

	mockCP := ports.NewMockCompletionPort()
	mockCP.CompletionPathFunc = func(_ ports.ShellKind, _, _ string) (string, error) {
		return "", fmt.Errorf("path resolution unsupported")
	}
	deps.Completion = mockCP

	opts := install.Options{
		Completions: []ports.ShellKind{ports.ShellZsh},
	}

	plan, err := install.BuildPlan(deps, opts)
	if err != nil {
		t.Fatalf("BuildPlan should not fail on CompletionPath error; got: %v", err)
	}
	if len(plan.CompletionPaths) != 0 {
		t.Errorf("expected empty CompletionPaths when path resolution fails; got %v", plan.CompletionPaths)
	}
}

// TestRun_PrintMode_BuildPlanError asserts that when BuildPlan fails during a
// dry-run, Run returns a non-nil error.
func TestRun_PrintMode_BuildPlanError(t *testing.T) {
	deps, _ := newDeps(t)
	paths := deps.Paths.(*ports.MockPathsPort)
	paths.ExecutableErr = fmt.Errorf("executable resolution failed")
	paths.ExecutableResult = ""

	opts := install.Options{Print: true, Stderr: &bytes.Buffer{}}

	_, err := install.Run(t.Context(), deps, opts, newRootCmd())
	if err == nil {
		t.Error("Run with Print=true should return error when BuildPlan fails")
	}
}

// TestRun_PrintMode_WriterError asserts that when the stderr writer fails,
// Run returns a non-nil error.
func TestRun_PrintMode_WriterError(t *testing.T) {
	deps, _ := newDeps(t)

	opts := install.Options{
		Print:  true,
		Stderr: &failWriter{err: fmt.Errorf("write: disk full")},
	}

	_, err := install.Run(t.Context(), deps, opts, newRootCmd())
	if err == nil {
		t.Error("Run with Print=true should return error when stderr writer fails")
	}
}

// failWriter is an io.Writer that always returns an error.
type failWriter struct {
	err error
}

func (f *failWriter) Write([]byte) (int, error) {
	return 0, f.err
}

// TestInstall_PrintMode_UsesPlan is a smoke test: invoking Run with Print=true
// routes through BuildPlan+Print and the output must contain the expected fields.
func TestInstall_PrintMode_UsesPlan(t *testing.T) {
	deps, _ := newDeps(t)
	env := deps.Env.(*ports.MockEnvPort)
	env.ShellResult = ports.ShellZsh

	var buf bytes.Buffer
	stderr := &buf
	opts := install.Options{
		Print:  true,
		Stderr: stderr,
	}

	_, err := install.Run(t.Context(), deps, opts, newRootCmd())
	if err != nil {
		t.Fatalf("Run with Print=true error: %v", err)
	}

	out := buf.String()

	for _, want := range []string{"data dir", "binary", "marker", "completions", "shell RC"} {
		if !strings.Contains(out, want) {
			t.Errorf("Print output missing %q\ngot:\n%s", want, out)
		}
	}
}
