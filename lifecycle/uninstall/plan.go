package uninstall

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fede-iglesias/shipkit/ports"
)

// Plan describes all paths and actions that would be taken by an uninstall run.
// It is built by BuildPlan and formatted by Print. No mutations are performed
// by either function.
type Plan struct {
	// AppName is the application name (e.g. "kt").
	AppName string

	// DataDir is the XDG data directory for the app.
	DataDir string

	// ConfigDir is the XDG config directory for the app.
	ConfigDir string

	// CacheDir is the XDG cache directory for the app.
	CacheDir string

	// BinaryPath is the absolute path of the binary.
	BinaryPath string

	// MarkerPath is the path of the install marker inside DataDir.
	MarkerPath string

	// CompletionPaths maps shell name (e.g. "zsh", "bash") to the path of the
	// completion script that would be removed.
	CompletionPaths map[string]string

	// ShellRCFiles lists entries describing which RC files contain guarded
	// blocks that would be cleaned.
	ShellRCFiles []ShellRCEntry

	// AutostartUnit describes the autostart service to be removed, or nil when
	// no autostart service is installed.
	AutostartUnit *AutostartInfo

	// KeepData mirrors Options.KeepData for formatting.
	KeepData bool

	// KeepConfig mirrors Options.KeepConfig for formatting.
	KeepConfig bool

	// KeepBinary mirrors Options.KeepBinary for formatting.
	KeepBinary bool
}

// ShellRCEntry describes one shell RC file and the block that would be removed.
type ShellRCEntry struct {
	// File is the path to the RC file (e.g. "~/.zshrc").
	File string

	// BlockSummary is a one-line human-readable description of what gets removed.
	BlockSummary string
}

// AutostartInfo describes the autostart service unit that would be removed.
type AutostartInfo struct {
	// Label is the service label (e.g. "com.example.kt").
	Label string

	// UnitPath is the path of the unit file on disk (informational only).
	UnitPath string
}

// BuildPlan resolves all paths that the uninstall verb would touch and returns
// a Plan struct. No filesystem mutations are performed.
func BuildPlan(deps Deps, opts Options) (Plan, error) {
	if deps.AppName == "" {
		return Plan{}, fmt.Errorf("BuildPlan: deps.AppName is required")
	}
	home, _ := deps.Paths.UserHome()

	dataDir, _ := deps.Paths.DataDir(deps.AppName)
	configDir, _ := deps.Paths.ConfigDir(deps.AppName)
	cacheDir, _ := deps.Paths.CacheDir(deps.AppName)

	// Marker lives at the root of the data directory.
	markerPath := ""
	if dataDir != "" {
		markerPath = filepath.Join(dataDir, ".shipkit.installed")
	}

	// Resolve completion paths for each configured shell.
	shells := deps.ShellKinds
	if len(shells) == 0 {
		shells = []ports.ShellKind{ports.ShellBash, ports.ShellZsh, ports.ShellFish}
	}
	completionPaths := make(map[string]string, len(shells))
	for _, shell := range shells {
		path, err := deps.Completion.CompletionPath(shell, deps.AppName, home)
		if err != nil {
			continue
		}
		completionPaths[string(shell)] = path
	}

	// Build shell RC entries: we look at the same RC paths that runTeardown
	// would clean.
	rcPaths := buildRcPaths(home)
	shellRCFiles := make([]ShellRCEntry, 0, len(rcPaths))
	for _, rc := range rcPaths {
		// Humanise the path: replace home prefix with ~.
		display := rc
		if home != "" && len(rc) > len(home) && rc[:len(home)] == home {
			display = "~" + rc[len(home):]
		}
		shellRCFiles = append(shellRCFiles, ShellRCEntry{
			File:         display,
			BlockSummary: "fpath line + autoload compinit",
		})
	}

	// Autostart info when a label is configured.
	var autostartInfo *AutostartInfo
	if deps.AutostartLabel != "" {
		autostartInfo = &AutostartInfo{
			Label: deps.AutostartLabel,
		}
	}

	return Plan{
		AppName:         deps.AppName,
		DataDir:         dataDir,
		ConfigDir:       configDir,
		CacheDir:        cacheDir,
		BinaryPath:      deps.BinPath,
		MarkerPath:      markerPath,
		CompletionPaths: completionPaths,
		ShellRCFiles:    shellRCFiles,
		AutostartUnit:   autostartInfo,
		KeepData:        opts.KeepData,
		KeepConfig:      opts.KeepConfig,
		KeepBinary:      opts.KeepBinary,
	}, nil
}

// Print writes a human-readable summary of the plan to w. The format mirrors
// the spec example output. It collects all output in a buffer and flushes once
// to keep the io.Writer error surface minimal and fully testable.
func (p Plan) Print(w io.Writer) error {
	kept := func(keep bool) string {
		if keep {
			return " (kept)"
		}
		return ""
	}

	var b strings.Builder

	fmt.Fprintf(&b, "uninstall plan for %s:\n", p.AppName)
	fmt.Fprintf(&b, "  data dir:        %s%s\n", p.DataDir, kept(p.KeepData))
	fmt.Fprintf(&b, "  config dir:      %s%s\n", p.ConfigDir, kept(p.KeepConfig))
	fmt.Fprintf(&b, "  cache dir:       %s\n", p.CacheDir)
	fmt.Fprintf(&b, "  binary:          %s%s\n", p.BinaryPath, kept(p.KeepBinary))
	if p.MarkerPath != "" {
		fmt.Fprintf(&b, "  marker:          %s\n", p.MarkerPath)
	}

	// Completions: print in sorted shell order for deterministic output.
	if len(p.CompletionPaths) > 0 {
		fmt.Fprintf(&b, "  completions:\n")
		shells := make([]string, 0, len(p.CompletionPaths))
		for s := range p.CompletionPaths {
			shells = append(shells, s)
		}
		sort.Strings(shells)
		for _, shell := range shells {
			fmt.Fprintf(&b, "    %s:%-12s%s\n", shell, "", p.CompletionPaths[shell])
		}
	}

	// Shell RC blocks.
	if len(p.ShellRCFiles) > 0 {
		fmt.Fprintf(&b, "  shell RC blocks:\n")
		for _, entry := range p.ShellRCFiles {
			fmt.Fprintf(&b, "    %s:      %s\n", entry.File, entry.BlockSummary)
		}
	}

	// Autostart.
	if p.AutostartUnit != nil {
		fmt.Fprintf(&b, "  autostart:       %s\n", p.AutostartUnit.Label)
	} else {
		fmt.Fprintf(&b, "  autostart:       (not installed)\n")
	}

	_, err := io.WriteString(w, b.String())
	return err
}
