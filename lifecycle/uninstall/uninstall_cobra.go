package uninstall

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

// NewCommand constructs the cobra.Command for the "uninstall" subcommand.
//
// The returned command is ready to be added to a root cobra.Command:
//
//	root.AddCommand(uninstall.NewCommand(deps, root))
//
// Flags registered:
//
//	--keep-data    Do not remove the application data directory.
//	--keep-config  Do not remove the application config directory.
//	--keep-binary  Do not remove the binary; report BinaryKept.
//	-y, --yes      Skip confirmation prompt (safe for scripted use).
//	--print        Dry-run: print the teardown plan without making changes.
//
// root is the parent command passed to Run for completions generation; it may
// be nil when the caller does not need completion context.
func NewCommand(deps Deps, root *cobra.Command) *cobra.Command {
	opts := Options{}

	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove " + deps.AppName + " and its user-state from this machine",
		Long: `Uninstall removes ` + deps.AppName + ` and all of its user-state:

  - Stops and removes the autostart service (if any)
  - Removes shell completions
  - Removes guarded blocks from shell RC files
  - Removes XDG data, config, and cache directories
  - Removes the binary

Use --keep-data, --keep-config, or --keep-binary to selectively preserve
directories or the binary. Use --print to preview the plan without making
any changes.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := Run(context.Background(), deps, opts, root)
			if err != nil {
				return err
			}
			// Print a summary of what was done (or would be done with --print).
			if opts.Print {
				fmt.Fprintf(cmd.OutOrStdout(), "Dry-run: would remove:\n")
				for _, r := range result.Removed {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", r)
				}
				if len(result.Skipped) > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "Would keep:\n")
					for _, s := range result.Skipped {
						fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", s)
					}
				}
				return nil
			}
			// Real run: report next steps if any.
			if len(result.NextSteps) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Manual steps required:\n")
				for _, step := range result.NextSteps {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", step)
				}
			}
			if result.BinaryAction != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Binary: %s\n", result.BinaryAction)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&opts.KeepData, "keep-data", false, "Do not remove the application data directory")
	cmd.Flags().BoolVar(&opts.KeepConfig, "keep-config", false, "Do not remove the application config directory")
	cmd.Flags().BoolVar(&opts.KeepBinary, "keep-binary", false, "Do not remove the binary")
	cmd.Flags().BoolVarP(&opts.Yes, "yes", "y", false, "Skip the confirmation prompt")
	cmd.Flags().BoolVar(&opts.Print, "print", false, "Dry-run: print the teardown plan without making changes")

	return cmd
}
