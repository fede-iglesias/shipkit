package install

import (
	"strings"

	"github.com/fede-iglesias/shipkit/ports"
	"github.com/spf13/cobra"
)

// NewCommand returns a *cobra.Command that wraps the install verb.
//
// The command wires the following flags:
//
//   - --force: re-run install even when already installed
//   - --autostart: register a platform autostart unit
//   - --completions: comma-separated list of shells (bash,zsh,fish)
//   - --print: dry-run; print plan without writing anything
//   - -y / --yes: skip interactive confirmation prompts
//
// deps must be fully populated before calling NewCommand; it is captured in the
// command RunE closure and reused on each invocation.
func NewCommand(deps Deps) (*cobra.Command, error) {
	var (
		flagForce        bool
		flagAutostart    bool
		flagCompletions  string
		flagPrint        bool
		flagYes          bool
	)

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Set up " + deps.Cfg.AppName + " data dirs, completions, and optional autostart",
		Long: `Install sets up user-scope state for ` + deps.Cfg.AppName + `:

  - Creates XDG data, config, and cache directories
  - Installs shell completion scripts for the detected shell
  - Injects guarded fpath blocks into the shell RC file
  - Optionally registers an autostart service unit (--autostart)
  - Writes a JSON marker recording what was installed

Install is idempotent: re-running without --force is a no-op when the
marker already exists for the current version.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := Options{
				Force:     flagForce,
				Autostart: flagAutostart,
				Print:     flagPrint,
				Yes:       flagYes,
				Stderr:    cmd.ErrOrStderr(),
			}
			if flagCompletions != "" {
				for _, s := range strings.Split(flagCompletions, ",") {
					shell := ports.ShellKind(strings.TrimSpace(s))
					opts.Completions = append(opts.Completions, shell)
				}
			}
			result, err := Run(cmd.Context(), deps, opts, cmd.Root())
			if err != nil {
				return err
			}
			if result.AlreadyInstalled {
				cmd.Printf("%s is already installed at %s (installed %s)\n",
					deps.Cfg.AppName, result.Marker.VersionInstalled, result.Marker.InstalledAt)
				return nil
			}
			if !flagPrint {
				cmd.Printf("%s installed successfully.\n", deps.Cfg.AppName)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&flagForce, "force", false, "re-run install even when already installed")
	cmd.Flags().BoolVar(&flagAutostart, "autostart", false, "register a platform autostart unit")
	cmd.Flags().StringVar(&flagCompletions, "completions", "", "comma-separated shells to install completions for (bash,zsh,fish)")
	cmd.Flags().BoolVar(&flagPrint, "print", false, "dry-run: print plan without writing anything")
	cmd.Flags().BoolVarP(&flagYes, "yes", "y", false, "skip interactive confirmation prompts")

	return cmd, nil
}
