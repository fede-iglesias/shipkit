// Package doctor implements the doctor lifecycle verb for shipkit-powered CLIs.
//
// # Design
//
// Doctor performs a read-only health check of a shipkit-managed CLI installation.
// It runs a fixed set of checks, each returning pass/warn/fail/skipped, aggregates
// them into a Report, and exits with code 0 (warn or better) or 1 (any failure).
//
// All external I/O is injected via the Deps struct: filesystem stat operations,
// binary health checks, autostart status queries, and optional network checks.
// The functions that perform real OS stat calls (StatExecutableFunc, StatDirFunc,
// StatFileFunc, ReadMarkerFunc) follow the sigstoreRealVerify pattern: they are
// nil by default and must be wired by the consumer's cmd layer. When nil, doctor
// uses safe fallback behaviour documented per check.
//
// Network checks (network.github, network.cosign-tuf, network.update-feed) are
// gated behind Options.Network (--network flag) and skipped by default. Their
// check functions (CheckNetworkGitHubFunc etc.) also follow the injection pattern.
//
// # Check inventory
//
//   - binary.in-path: binary directory is in $PATH
//   - binary.executable: binary has executable permission bits
//   - binary.version: binary reports version matching Deps.Version
//   - xdg.data-dir: XDG data directory exists
//   - xdg.config-dir: XDG config directory exists
//   - xdg.cache-dir: XDG cache directory exists
//   - marker: .shipkit.installed marker file present and version matches
//   - completion.<shell>: completion file exists for detected shell
//   - autostart: autostart service state (skipped if AutostartLabel empty)
//   - recovery.manifest: no pending recovery manifest (fail if present)
//   - network.github: GitHub API reachable (--network only)
//   - network.cosign-tuf: TUF/Sigstore reachable (--network only)
//   - network.update-feed: latest release tag reachable (--network only)
//
// # Usage
//
//	deps := doctor.Deps{
//	    AppName:  "myapp",
//	    BinPath:  "/usr/local/bin/myapp",
//	    Version:  "0.1.0",
//	    DataRoot: "/home/user/.local/share/myapp",
//	    ...
//	}
//	report, err := doctor.Run(ctx, deps, doctor.Options{})
//	os.Exit(doctor.ExitCode(report))
//
// # See also
//
// [shipkit/lifecycle/install] for the install verb that sets up what doctor checks.
package doctor

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/fede-iglesias/shipkit/ports"
)

// CheckID is the stable identifier for a doctor check.
// It is used as the JSON "id" key and in stdout output.
type CheckID string

// Status describes the outcome of a single doctor check.
type Status string

const (
	// StatusPass means the check completed and everything looks healthy.
	StatusPass Status = "pass"

	// StatusWarn means the check completed but found a non-critical issue.
	// The exit code is still 0.
	StatusWarn Status = "warn"

	// StatusFail means the check found a critical problem. The exit code is 1.
	StatusFail Status = "fail"

	// StatusSkipped means the check was not run (e.g. network checks without
	// --network, or autostart when not configured).
	StatusSkipped Status = "skipped"
)

// Check is the result of a single doctor check.
type Check struct {
	// ID is the stable machine-readable identifier (e.g. "binary.in-path").
	ID CheckID `json:"id"`

	// Title is the human-readable label shown in the [PASS]/[WARN]/[FAIL] line.
	Title string `json:"title"`

	// Status is the outcome: pass, warn, fail, or skipped.
	Status Status `json:"status"`

	// Details is the human-readable explanation of the check result.
	Details string `json:"details"`

	// Hint is an optional suggestion for fixing a warn or fail. Empty on pass.
	Hint string `json:"hint,omitempty"`
}

// Summary aggregates the check counts for the full report.
type Summary struct {
	// Pass is the number of checks that passed.
	Pass int `json:"pass"`

	// Warn is the number of checks that produced a warning.
	Warn int `json:"warn"`

	// Fail is the number of checks that failed.
	Fail int `json:"fail"`

	// Skipped is the number of checks that were skipped.
	Skipped int `json:"skipped"`

	// OK is true when Fail == 0. Exit code 0.
	OK bool `json:"ok"`
}

// Report is the full result of a doctor run, ready for JSON serialisation.
type Report struct {
	// Checks is the ordered list of check results.
	Checks []Check `json:"checks"`

	// Summary aggregates the check counts.
	Summary Summary `json:"summary"`
}

// Options controls which checks are enabled.
type Options struct {
	// Network enables the three network checks (network.github, network.cosign-tuf,
	// network.update-feed). Off by default to keep doctor fast and offline-safe.
	Network bool

	// JSON enables machine-readable JSON output instead of human text.
	JSON bool

	// Verbose includes additional detail for passing checks. Without Verbose,
	// passing checks show a one-line summary; with Verbose they include full details.
	Verbose bool
}

// Deps holds the injected ports and configuration that Run needs.
// All port fields are required unless otherwise noted.
//
// The Func fields follow the sigstoreRealVerify pattern: they are nil by default
// and must be wired by the consumer's cmd layer. When nil, doctor falls back to
// safe behaviour (typically StatusWarn or StatusSkipped with an explanatory detail).
type Deps struct {
	// AppName is the application name used for check labels and hints. Required.
	AppName string

	// BinPath is the absolute path of the binary being checked. Required.
	BinPath string

	// Version is the expected version string (from -ldflags at build time). Required.
	Version string

	// DataRoot is the expected XDG data directory for the app. Required.
	DataRoot string

	// ConfigRoot is the expected XDG config directory. Required.
	ConfigRoot string

	// CacheRoot is the expected XDG cache directory. Required.
	CacheRoot string

	// AutostartLabel is the platform service label (e.g. "com.fede-iglesias.myapp").
	// When empty, the autostart check reports "not enabled" with StatusPass.
	AutostartLabel string

	// Repo is the GitHub repository slug used for network.update-feed check
	// (e.g. "fede-iglesias/tools"). May be empty when Network is false.
	Repo string

	// TagPrefix is the release tag prefix used for network.update-feed
	// (e.g. "myapp-"). May be empty when Network is false.
	TagPrefix string

	// HTTP provides HTTP operations for network checks. Required when Network is true.
	HTTP ports.HTTPPort

	// FS is not used directly by doctor but is included for Deps consistency
	// across lifecycle verbs.
	FS ports.FsPort

	// Spawn is used for binary.version health check. Required.
	Spawn ports.SpawnPort

	// Paths provides PATH inspection (InPATH, PATHList). Required.
	Paths ports.PathsPort

	// Env provides shell detection. Required.
	Env ports.EnvPort

	// Autostart provides service status query. Required.
	Autostart ports.AutostartPort

	// Clock provides deterministic time. Required.
	Clock ports.ClockPort

	// Completion provides completion path resolution. Required.
	Completion ports.CompletionPort

	// StatExecutableFunc reports whether the file at path has executable permission
	// bits set (mode & 0111 != 0). When nil, binary.executable is StatusWarn with
	// a hint to wire this function.
	//
	// Consumer wiring:
	//   deps.StatExecutableFunc = func(path string) (bool, error) {
	//       info, err := os.Stat(path)
	//       if err != nil { return false, err }
	//       return info.Mode()&0111 != 0, nil
	//   }
	StatExecutableFunc func(path string) (bool, error)

	// StatDirFunc reports whether the directory at path exists. When nil,
	// xdg.* checks are StatusWarn with a hint to wire this function.
	//
	// Consumer wiring:
	//   deps.StatDirFunc = func(path string) (bool, error) {
	//       _, err := os.Stat(path)
	//       if os.IsNotExist(err) { return false, nil }
	//       return err == nil, err
	//   }
	StatDirFunc func(path string) (bool, error)

	// StatFileFunc reports whether a file at path exists. Used for completion
	// file presence and recovery manifest detection. When nil:
	//   - completion.* uses StatusWarn.
	//   - recovery.manifest uses StatusPass (safe: assumes no manifest).
	StatFileFunc func(path string) (bool, error)

	// ReadMarkerFunc reads the content of the .shipkit.installed marker file.
	// Returns the raw JSON string and any error. When nil, marker check uses
	// StatusWarn with a hint.
	ReadMarkerFunc func(path string) (string, error)

	// CheckNetworkGitHubFunc performs the network.github check. When nil and
	// Network is true, the check is StatusWarn.
	CheckNetworkGitHubFunc func(ctx context.Context) error

	// CheckNetworkCosignTUFFunc performs the network.cosign-tuf check. When nil
	// and Network is true, the check is StatusWarn.
	CheckNetworkCosignTUFFunc func(ctx context.Context) error

	// CheckNetworkUpdateFeedFunc queries the latest release tag. Returns the tag
	// string and any error. When nil and Network is true, the check is StatusWarn.
	CheckNetworkUpdateFeedFunc func(ctx context.Context) (string, error)

	// RunFunc overrides the Run function when non-nil. This follows the
	// sigstoreRealVerify injection pattern: it is nil by default (production
	// uses the real Run) and injectable in tests to trigger error paths in the
	// cobra RunE that are structurally unreachable via the real Run.
	//
	// This field lives on Deps (not passed separately) so NewCommand can access
	// it without changing the NewCommand signature.
	RunFunc func(ctx context.Context, opts Options) (Report, error)
}

// markerJSON is the JSON shape of the .shipkit.installed marker file.
type markerJSON struct {
	VersionInstalled string `json:"version_installed"`
	InstalledAt      string `json:"installed_at"`
}

// Run executes all doctor checks, builds a Report, and returns it.
// Run never returns a non-nil error for check-level failures; errors in Run
// itself represent programming mistakes (nil required Deps fields).
func Run(ctx context.Context, deps Deps, opts Options) (Report, error) {
	var checks []Check

	// binary.in-path
	checks = append(checks, checkBinaryInPath(deps))

	// binary.executable
	checks = append(checks, checkBinaryExecutable(deps))

	// binary.version
	checks = append(checks, checkBinaryVersion(ctx, deps))

	// xdg.data-dir
	checks = append(checks, checkDir("xdg.data-dir", "XDG data directory", deps.DataRoot, deps.AppName, deps))

	// xdg.config-dir
	checks = append(checks, checkDir("xdg.config-dir", "XDG config directory", deps.ConfigRoot, deps.AppName, deps))

	// xdg.cache-dir
	checks = append(checks, checkDir("xdg.cache-dir", "XDG cache directory", deps.CacheRoot, deps.AppName, deps))

	// marker
	checks = append(checks, checkMarker(deps))

	// completion.<shell>
	checks = append(checks, checkCompletion(deps)...)

	// autostart
	checks = append(checks, checkAutostart(deps))

	// recovery.manifest
	checks = append(checks, checkRecoveryManifest(deps))

	// network checks (skipped unless --network)
	checks = append(checks, checkNetwork(ctx, deps, opts)...)

	report := Report{
		Checks:  checks,
		Summary: ComputeSummary(checks),
	}
	return report, nil
}

// checkBinaryInPath returns the binary.in-path check.
// Pass: parent directory of BinPath is listed in $PATH.
// Warn: not in PATH (non-critical; binary still works if called by full path).
func checkBinaryInPath(deps Deps) Check {
	dir := filepath.Dir(deps.BinPath)
	if deps.Paths.InPATH(dir) {
		return Check{
			ID:      "binary.in-path",
			Title:   "binary in $PATH",
			Status:  StatusPass,
			Details: fmt.Sprintf("%s is in $PATH", deps.BinPath),
		}
	}
	return Check{
		ID:      "binary.in-path",
		Title:   "binary in $PATH",
		Status:  StatusWarn,
		Details: fmt.Sprintf("%s directory (%s) is not in $PATH", deps.BinPath, dir),
		Hint:    fmt.Sprintf("add %s to your PATH, or move %s to a directory already in PATH", dir, deps.AppName),
	}
}

// checkBinaryExecutable returns the binary.executable check.
// Uses StatExecutableFunc; if nil, returns warn with a hint.
func checkBinaryExecutable(deps Deps) Check {
	if deps.StatExecutableFunc == nil {
		return Check{
			ID:      "binary.executable",
			Title:   "binary is executable",
			Status:  StatusWarn,
			Details: "StatExecutableFunc not wired; cannot verify permissions",
			Hint:    "wire StatExecutableFunc in the consumer cmd layer",
		}
	}
	ok, err := deps.StatExecutableFunc(deps.BinPath)
	if err != nil {
		return Check{
			ID:      "binary.executable",
			Title:   "binary is executable",
			Status:  StatusFail,
			Details: fmt.Sprintf("stat %s: %v", deps.BinPath, err),
		}
	}
	if !ok {
		return Check{
			ID:      "binary.executable",
			Title:   "binary is executable",
			Status:  StatusFail,
			Details: fmt.Sprintf("%s exists but has no execute permission bits", deps.BinPath),
			Hint:    fmt.Sprintf("chmod +x %s", deps.BinPath),
		}
	}
	return Check{
		ID:      "binary.executable",
		Title:   "binary is executable",
		Status:  StatusPass,
		Details: fmt.Sprintf("%s is executable", deps.BinPath),
	}
}

// checkBinaryVersion returns the binary.version check.
// Uses SpawnPort.HealthCheck to run the binary with --version.
// Pass: reported version matches deps.Version.
// Fail: version mismatch or binary failed to run.
func checkBinaryVersion(ctx context.Context, deps Deps) Check {
	result, err := deps.Spawn.HealthCheck(ctx, deps.BinPath, 5*time.Second)
	if err != nil {
		return Check{
			ID:      "binary.version",
			Title:   "binary version",
			Status:  StatusFail,
			Details: fmt.Sprintf("health check failed: %v", err),
		}
	}
	if !result.Ok {
		return Check{
			ID:      "binary.version",
			Title:   "binary version",
			Status:  StatusFail,
			Details: fmt.Sprintf("binary did not report a version: %s", result.Reason),
		}
	}
	reported := strings.TrimPrefix(result.Version, "v")
	expected := strings.TrimPrefix(deps.Version, "v")
	if reported != expected {
		return Check{
			ID:      "binary.version",
			Title:   "binary version",
			Status:  StatusFail,
			Details: fmt.Sprintf("reports %s but built-in version is %s", result.Version, deps.Version),
			Hint:    fmt.Sprintf("run `%s update` to sync, or reinstall via install.sh", deps.AppName),
		}
	}
	return Check{
		ID:      "binary.version",
		Title:   "binary version",
		Status:  StatusPass,
		Details: fmt.Sprintf("reports %s matches built-in %s", result.Version, deps.Version),
	}
}

// checkDir returns a single xdg.* check for the given directory path.
// Uses StatDirFunc; if nil, returns warn with a hint.
func checkDir(id, title, dir, appName string, deps Deps) Check {
	if deps.StatDirFunc == nil {
		return Check{
			ID:      CheckID(id),
			Title:   title,
			Status:  StatusWarn,
			Details: "StatDirFunc not wired; cannot verify directory existence",
			Hint:    fmt.Sprintf("run `%s install` to create directories", appName),
		}
	}
	ok, err := deps.StatDirFunc(dir)
	if err != nil {
		return Check{
			ID:      CheckID(id),
			Title:   title,
			Status:  StatusFail,
			Details: fmt.Sprintf("stat %s: %v", dir, err),
		}
	}
	if !ok {
		return Check{
			ID:      CheckID(id),
			Title:   title,
			Status:  StatusWarn,
			Details: fmt.Sprintf("%s does not exist", dir),
			Hint:    fmt.Sprintf("run `%s install` to create it", appName),
		}
	}
	return Check{
		ID:      CheckID(id),
		Title:   title,
		Status:  StatusPass,
		Details: fmt.Sprintf("%s exists", dir),
	}
}

// checkMarker returns the marker check.
// Uses ReadMarkerFunc; if nil or file not found, returns warn.
// Version mismatch returns warn.
func checkMarker(deps Deps) Check {
	markerPath := filepath.Join(deps.DataRoot, ".shipkit.installed")

	if deps.ReadMarkerFunc == nil {
		return Check{
			ID:      "marker",
			Title:   "install marker",
			Status:  StatusWarn,
			Details: "ReadMarkerFunc not wired; cannot verify install marker",
			Hint:    fmt.Sprintf("run `%s install` to create the marker", deps.AppName),
		}
	}

	content, err := deps.ReadMarkerFunc(markerPath)
	if err != nil {
		return Check{
			ID:      "marker",
			Title:   "install marker",
			Status:  StatusWarn,
			Details: fmt.Sprintf("marker not found at %s", markerPath),
			Hint:    fmt.Sprintf("run `%s install` to initialise", deps.AppName),
		}
	}

	var m markerJSON
	if jsonErr := json.Unmarshal([]byte(content), &m); jsonErr != nil {
		return Check{
			ID:      "marker",
			Title:   "install marker",
			Status:  StatusWarn,
			Details: fmt.Sprintf("marker at %s is not valid JSON: %v", markerPath, jsonErr),
			Hint:    fmt.Sprintf("run `%s install --force` to recreate it", deps.AppName),
		}
	}

	installed := strings.TrimPrefix(m.VersionInstalled, "v")
	expected := strings.TrimPrefix(deps.Version, "v")
	if installed != expected {
		return Check{
			ID:      "marker",
			Title:   "install marker",
			Status:  StatusWarn,
			Details: fmt.Sprintf("marker records %s but running version is %s", m.VersionInstalled, deps.Version),
			Hint:    fmt.Sprintf("run `%s install --force` to update the marker", deps.AppName),
		}
	}

	return Check{
		ID:      "marker",
		Title:   "install marker",
		Status:  StatusPass,
		Details: fmt.Sprintf("installed %s at %s", m.VersionInstalled, m.InstalledAt),
	}
}

// checkCompletion returns zero or one check for the detected shell's completion file.
// When the shell is unknown, returns an empty slice (no check added).
// Uses StatFileFunc; if nil, returns warn.
func checkCompletion(deps Deps) []Check {
	shell := deps.Env.DetectShell()
	if shell == ports.ShellUnknown {
		// Cannot determine shell; skip completion check.
		return nil
	}

	id := CheckID("completion." + string(shell))
	title := fmt.Sprintf("shell completion (%s)", shell)

	home, _ := deps.Paths.UserHome()
	completionPath, err := deps.Completion.CompletionPath(shell, deps.AppName, home)
	if err != nil {
		return []Check{{
			ID:      id,
			Title:   title,
			Status:  StatusWarn,
			Details: fmt.Sprintf("cannot resolve completion path: %v", err),
			Hint:    fmt.Sprintf("run `%s install` to install completions", deps.AppName),
		}}
	}

	if deps.StatFileFunc == nil {
		return []Check{{
			ID:      id,
			Title:   title,
			Status:  StatusWarn,
			Details: "StatFileFunc not wired; cannot verify completion file",
			Hint:    fmt.Sprintf("run `%s install` to install completions", deps.AppName),
		}}
	}

	ok, err := deps.StatFileFunc(completionPath)
	if err != nil {
		return []Check{{
			ID:      id,
			Title:   title,
			Status:  StatusWarn,
			Details: fmt.Sprintf("stat %s: %v", completionPath, err),
			Hint:    fmt.Sprintf("run `%s install --force` to reinstall completions", deps.AppName),
		}}
	}
	if !ok {
		return []Check{{
			ID:      id,
			Title:   title,
			Status:  StatusWarn,
			Details: fmt.Sprintf("completion file %s not found", completionPath),
			Hint:    fmt.Sprintf("run `%s install` to install completions", deps.AppName),
		}}
	}
	return []Check{{
		ID:      id,
		Title:   title,
		Status:  StatusPass,
		Details: fmt.Sprintf("%s exists", completionPath),
	}}
}

// checkAutostart returns the autostart check.
// When AutostartLabel is empty, returns pass with "not enabled in this app".
// When set: pass if installed+running, warn otherwise.
func checkAutostart(deps Deps) Check {
	if deps.AutostartLabel == "" {
		return Check{
			ID:      "autostart",
			Title:   "autostart service",
			Status:  StatusPass,
			Details: "not enabled in this app",
		}
	}

	status, err := deps.Autostart.Status(deps.AutostartLabel)
	if err != nil {
		return Check{
			ID:      "autostart",
			Title:   "autostart service",
			Status:  StatusWarn,
			Details: fmt.Sprintf("status query failed for %s: %v", deps.AutostartLabel, err),
			Hint:    fmt.Sprintf("run `%s install --autostart` to reinstall the service", deps.AppName),
		}
	}
	if !status.Installed {
		return Check{
			ID:      "autostart",
			Title:   "autostart service",
			Status:  StatusWarn,
			Details: fmt.Sprintf("service %s is configured but not installed", deps.AutostartLabel),
			Hint:    fmt.Sprintf("run `%s install --autostart` to install it", deps.AppName),
		}
	}
	if !status.Running {
		return Check{
			ID:      "autostart",
			Title:   "autostart service",
			Status:  StatusWarn,
			Details: fmt.Sprintf("service %s is installed but not running", deps.AutostartLabel),
			Hint:    fmt.Sprintf("check system logs or run `%s install --autostart --force`", deps.AppName),
		}
	}
	return Check{
		ID:      "autostart",
		Title:   "autostart service",
		Status:  StatusPass,
		Details: fmt.Sprintf("service %s is installed and running (PID %d)", deps.AutostartLabel, status.PID),
	}
}

// checkRecoveryManifest returns the recovery.manifest check.
// Fail: the manifest file exists (a previous update failed unrecoverably).
// Pass: no manifest.
// Uses StatFileFunc; when nil, returns pass (safe assumption: no manifest).
func checkRecoveryManifest(deps Deps) Check {
	manifestPath := filepath.Join(deps.DataRoot, ".shipkit.recovery-manifest.json")

	if deps.StatFileFunc == nil {
		return Check{
			ID:      "recovery.manifest",
			Title:   "recovery manifest",
			Status:  StatusPass,
			Details: "no recovery manifest pending",
		}
	}

	exists, err := deps.StatFileFunc(manifestPath)
	if err != nil {
		return Check{
			ID:      "recovery.manifest",
			Title:   "recovery manifest",
			Status:  StatusWarn,
			Details: fmt.Sprintf("cannot check manifest at %s: %v", manifestPath, err),
		}
	}
	if exists {
		return Check{
			ID:      "recovery.manifest",
			Title:   "recovery manifest",
			Status:  StatusFail,
			Details: fmt.Sprintf("recovery manifest found at %s: previous update failed", manifestPath),
			Hint:    fmt.Sprintf("run `%s update` to retry, or `%s install --force` to recover", deps.AppName, deps.AppName),
		}
	}
	return Check{
		ID:      "recovery.manifest",
		Title:   "recovery manifest",
		Status:  StatusPass,
		Details: "no recovery manifest pending",
	}
}

// checkNetwork returns the three network checks.
// When opts.Network is false, all three are StatusSkipped.
func checkNetwork(ctx context.Context, deps Deps, opts Options) []Check {
	if !opts.Network {
		return []Check{
			{
				ID:      "network.github",
				Title:   "network: GitHub API",
				Status:  StatusSkipped,
				Details: "use --network to run",
			},
			{
				ID:      "network.cosign-tuf",
				Title:   "network: Sigstore TUF",
				Status:  StatusSkipped,
				Details: "use --network to run",
			},
			{
				ID:      "network.update-feed",
				Title:   "network: update feed",
				Status:  StatusSkipped,
				Details: "use --network to run",
			},
		}
	}

	return []Check{
		checkNetworkGitHub(ctx, deps),
		checkNetworkCosignTUF(ctx, deps),
		checkNetworkUpdateFeed(ctx, deps),
	}
}

// checkNetworkGitHub returns the network.github check.
func checkNetworkGitHub(ctx context.Context, deps Deps) Check {
	if deps.CheckNetworkGitHubFunc == nil {
		return Check{
			ID:      "network.github",
			Title:   "network: GitHub API",
			Status:  StatusWarn,
			Details: "CheckNetworkGitHubFunc not wired",
			Hint:    "wire CheckNetworkGitHubFunc in the consumer cmd layer",
		}
	}
	if err := deps.CheckNetworkGitHubFunc(ctx); err != nil {
		return Check{
			ID:      "network.github",
			Title:   "network: GitHub API",
			Status:  StatusFail,
			Details: fmt.Sprintf("GitHub API unreachable: %v", err),
			Hint:    "check your internet connection or GitHub status",
		}
	}
	return Check{
		ID:      "network.github",
		Title:   "network: GitHub API",
		Status:  StatusPass,
		Details: "GitHub API reachable",
	}
}

// checkNetworkCosignTUF returns the network.cosign-tuf check.
func checkNetworkCosignTUF(ctx context.Context, deps Deps) Check {
	if deps.CheckNetworkCosignTUFFunc == nil {
		return Check{
			ID:      "network.cosign-tuf",
			Title:   "network: Sigstore TUF",
			Status:  StatusWarn,
			Details: "CheckNetworkCosignTUFFunc not wired",
			Hint:    "wire CheckNetworkCosignTUFFunc in the consumer cmd layer",
		}
	}
	if err := deps.CheckNetworkCosignTUFFunc(ctx); err != nil {
		return Check{
			ID:      "network.cosign-tuf",
			Title:   "network: Sigstore TUF",
			Status:  StatusFail,
			Details: fmt.Sprintf("Sigstore TUF unreachable: %v", err),
			Hint:    "check your internet connection",
		}
	}
	return Check{
		ID:      "network.cosign-tuf",
		Title:   "network: Sigstore TUF",
		Status:  StatusPass,
		Details: "Sigstore TUF reachable",
	}
}

// checkNetworkUpdateFeed returns the network.update-feed check.
func checkNetworkUpdateFeed(ctx context.Context, deps Deps) Check {
	if deps.CheckNetworkUpdateFeedFunc == nil {
		return Check{
			ID:      "network.update-feed",
			Title:   "network: update feed",
			Status:  StatusWarn,
			Details: "CheckNetworkUpdateFeedFunc not wired",
			Hint:    "wire CheckNetworkUpdateFeedFunc in the consumer cmd layer",
		}
	}
	latest, err := deps.CheckNetworkUpdateFeedFunc(ctx)
	if err != nil {
		return Check{
			ID:      "network.update-feed",
			Title:   "network: update feed",
			Status:  StatusFail,
			Details: fmt.Sprintf("update feed unreachable: %v", err),
			Hint:    "check your internet connection",
		}
	}

	current := strings.TrimPrefix(deps.Version, "v")
	latestClean := strings.TrimPrefix(latest, "v")
	detail := fmt.Sprintf("latest: %s, current: %s", latestClean, current)
	if latestClean != current {
		detail += fmt.Sprintf(" (run `%s update` to upgrade)", deps.AppName)
	}
	return Check{
		ID:      "network.update-feed",
		Title:   "network: update feed",
		Status:  StatusPass,
		Details: detail,
	}
}

// ComputeSummary aggregates check counts from the given slice.
// OK is true when Fail == 0.
func ComputeSummary(checks []Check) Summary {
	var s Summary
	for _, c := range checks {
		switch c.Status {
		case StatusPass:
			s.Pass++
		case StatusWarn:
			s.Warn++
		case StatusFail:
			s.Fail++
		case StatusSkipped:
			s.Skipped++
		}
	}
	s.OK = s.Fail == 0
	return s
}

// ExitCode returns 1 when the report contains at least one failing check,
// and 0 otherwise (including warn-only runs). Use this to set the process exit
// code after printing the report.
func ExitCode(report Report) int {
	if !report.Summary.OK {
		return 1
	}
	return 0
}

// FormatText formats a Report as human-readable text with [PASS]/[WARN]/[FAIL]/[SKIP]
// prefix per check and a Summary line at the end.
// When verbose is false, skipped and passing checks include only the Details line.
// When verbose is true, all fields are shown.
func FormatText(report Report, verbose bool) string {
	var sb strings.Builder
	for _, c := range report.Checks {
		tag := statusTag(c.Status)
		sb.WriteString(fmt.Sprintf("  %s %-28s %s\n", tag, c.ID, c.Details))
		if c.Hint != "" && (verbose || c.Status == StatusWarn || c.Status == StatusFail) {
			sb.WriteString(fmt.Sprintf("  %28s hint: %s\n", "", c.Hint))
		}
	}

	s := report.Summary
	sb.WriteString(fmt.Sprintf("\nSummary: %d pass, %d warn, %d fail, %d skipped.\n",
		s.Pass, s.Warn, s.Fail, s.Skipped))
	if s.OK {
		sb.WriteString("OK\n")
	} else {
		sb.WriteString("FAIL\n")
	}
	return sb.String()
}

// statusTag returns the bracket-prefix for the given status in text output.
func statusTag(s Status) string {
	switch s {
	case StatusPass:
		return "[PASS]"
	case StatusWarn:
		return "[WARN]"
	case StatusFail:
		return "[FAIL]"
	default:
		return "[SKIP]"
	}
}
