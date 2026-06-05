package install

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/fede-iglesias/shipkit/ports"
)

// Plan describes what the install verb would do without executing any mutations.
// It is built by BuildPlan and rendered by Plan.Print.
type Plan struct {
	// AppName is the application name, used as the plan header.
	AppName string

	// DataDir is the XDG data directory path, e.g. ~/.local/share/<app>.
	DataDir string

	// ConfigDir is the XDG config directory path, e.g. ~/.config/<app>.
	ConfigDir string

	// CacheDir is the XDG cache directory path, e.g. ~/.cache/<app>.
	CacheDir string

	// BinaryPath is the absolute path of the currently running binary.
	BinaryPath string

	// MarkerPath is the absolute path of the install marker JSON file.
	MarkerPath string

	// CompletionPaths maps each shell to the user-scoped completion file path.
	// Only populated for the shells that would be installed.
	CompletionPaths map[string]string

	// ShellRCBlocks is the list of shell RC file injections that would be made.
	ShellRCBlocks []ShellRCBlock

	// AutostartUnit is non-nil when autostart would be installed.
	AutostartUnit *PlanAutostartUnit
}

// ShellRCBlock describes a guarded block that would be injected into a shell RC file.
type ShellRCBlock struct {
	// File is the absolute path of the RC file, e.g. /home/user/.zshrc.
	File string

	// BlockPreview is the first 3 lines (or fewer) of the block content that
	// would be injected.
	BlockPreview string
}

// PlanAutostartUnit describes the autostart service unit that would be installed.
type PlanAutostartUnit struct {
	// Label is the platform service label, e.g. "com.fede-iglesias.myapp".
	Label string

	// Path is the user-scoped path where the unit file would be written.
	// On darwin: ~/Library/LaunchAgents/<label>.plist
	// On linux:  ~/.config/systemd/user/<label>.service
	Path string
}

// BuildPlan resolves all paths and constructs a Plan from the given Deps and
// Options. It performs no filesystem mutations; all port calls are read-only.
//
// BuildPlan is the single source of truth for path resolution shared between
// the dry-run (Print) and the live install (Run) code paths.
func BuildPlan(deps Deps, opts Options) (Plan, error) {
	// Resolve binary path.
	binPath, err := deps.Paths.Executable()
	if err != nil {
		return Plan{}, fmt.Errorf("install plan: resolve binary path: %w", err)
	}

	// Resolve data dir.
	dataDir, err := deps.Paths.DataDir(deps.Cfg.AppName)
	if err != nil {
		return Plan{}, fmt.Errorf("install plan: resolve data dir: %w", err)
	}

	// Resolve config dir (best-effort; non-fatal).
	configDir, _ := deps.Paths.ConfigDir(deps.Cfg.AppName)

	// Resolve cache dir (best-effort; non-fatal).
	cacheDir, _ := deps.Paths.CacheDir(deps.Cfg.AppName)

	// Resolve marker path.
	markerPath := filepath.Join(dataDir, markerFileName)

	// Resolve home dir (best-effort; used for completions and RC paths).
	home, _ := deps.Paths.UserHome()

	// Resolve completion paths for each requested shell.
	completionShells := opts.Completions
	if completionShells == nil {
		detected := deps.Env.DetectShell()
		if detected != ports.ShellUnknown {
			completionShells = []ports.ShellKind{detected}
		}
	}

	completionPaths := map[string]string{}
	for _, shell := range completionShells {
		path, err := deps.Completion.CompletionPath(shell, deps.Cfg.AppName, home)
		if err != nil {
			// Skip shells that cannot resolve a path; the live run will error
			// if they are actually needed.
			continue
		}
		completionPaths[string(shell)] = path
	}

	// Resolve shell RC blocks for non-fish, non-unknown shells.
	var shellRCBlocks []ShellRCBlock
	detectedShell := deps.Env.DetectShell()
	if detectedShell != ports.ShellFish && detectedShell != ports.ShellUnknown {
		rcPath, err := shellRcPath(detectedShell, home)
		if err == nil && rcPath != "" {
			compPath := completionPaths[string(detectedShell)]
			block := fpathBlock(deps.Cfg.AppName, compPath)
			preview := firstNLines(block, 3)
			shellRCBlocks = append(shellRCBlocks, ShellRCBlock{
				File:         rcPath,
				BlockPreview: preview,
			})
		}
	}

	// Resolve autostart unit when requested and enabled.
	var autostartUnit *PlanAutostartUnit
	if opts.Autostart && deps.Cfg.EnableAutostart {
		label := deps.Cfg.autostartLabel()
		unitPath := autostartUnitPath(home, label, deps.Env.DetectOS())
		autostartUnit = &PlanAutostartUnit{
			Label: label,
			Path:  unitPath,
		}
	}

	return Plan{
		AppName:         deps.Cfg.AppName,
		DataDir:         dataDir,
		ConfigDir:       configDir,
		CacheDir:        cacheDir,
		BinaryPath:      binPath,
		MarkerPath:      markerPath,
		CompletionPaths: completionPaths,
		ShellRCBlocks:   shellRCBlocks,
		AutostartUnit:   autostartUnit,
	}, nil
}

// Print renders the plan to w in a human-readable tabular format.
// It never writes to the filesystem. Returns the first write error, if any.
func (p Plan) Print(w io.Writer) error {
	pw := &planWriter{w: w}
	pw.printf("install plan for %s:\n", p.AppName)
	pw.printf("  %-16s %s\n", "data dir:", p.DataDir)
	pw.printf("  %-16s %s\n", "config dir:", p.ConfigDir)
	pw.printf("  %-16s %s\n", "cache dir:", p.CacheDir)
	pw.printf("  %-16s %s\n", "binary:", p.BinaryPath)
	pw.printf("  %-16s %s\n", "marker:", p.MarkerPath)

	if len(p.CompletionPaths) == 0 {
		pw.printf("  %-16s (none detected)\n", "completions:")
	} else {
		pw.printf("  completions:\n")
		for shell, path := range p.CompletionPaths {
			pw.printf("    %-12s %s\n", shell+":", path)
		}
	}

	if len(p.ShellRCBlocks) == 0 {
		pw.printf("  %-16s (none)\n", "shell RC blocks:")
	} else {
		pw.printf("  shell RC blocks:\n")
		for _, block := range p.ShellRCBlocks {
			pw.printf("    %s\n", block.File)
			if block.BlockPreview != "" {
				for _, line := range strings.Split(block.BlockPreview, "\n") {
					if line != "" {
						pw.printf("      %s\n", line)
					}
				}
			}
		}
	}

	if p.AutostartUnit == nil {
		pw.printf("  %-16s (not requested)\n", "autostart:")
	} else {
		pw.printf("  %-16s %s\n", "autostart:", p.AutostartUnit.Label)
		pw.printf("    %-14s %s\n", "unit path:", p.AutostartUnit.Path)
	}

	return pw.err
}

// planWriter wraps an io.Writer and accumulates the first write error,
// short-circuiting subsequent writes once an error is recorded.
type planWriter struct {
	w   io.Writer
	err error
}

func (pw *planWriter) printf(format string, args ...any) {
	if pw.err != nil {
		return
	}
	_, pw.err = fmt.Fprintf(pw.w, format, args...)
}

// firstNLines returns the first n lines of s, joined with newlines.
// If s has fewer than n lines, it returns all of s.
func firstNLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[:n], "\n")
}

// autostartUnitPath returns the user-scoped path where the autostart unit file
// would be written. This is informational only (dry-run); the actual write is
// done by AutostartPort.Install in the live path.
//
//   - darwin: ~/Library/LaunchAgents/<label>.plist
//   - linux:  ~/.config/systemd/user/<label>.service
func autostartUnitPath(home, label, goos string) string {
	if goos == "darwin" {
		return filepath.Join(home, "Library", "LaunchAgents", label+".plist")
	}
	return filepath.Join(home, ".config", "systemd", "user", label+".service")
}
