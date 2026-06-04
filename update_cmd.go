package shipkit

import (
	"context"
	"fmt"

	"github.com/fede-iglesias/shipkit/lifecycle/update"
	"github.com/spf13/cobra"
)

// updateRunner is the interface that newUpdateCommand accepts. It matches the
// method set of [update.Orchestrator.Run], enabling test injection of a fake
// runner without touching the real orchestrator.
type updateRunner interface {
	Run(ctx context.Context, opts update.RunOpts) (update.Result, error)
}

// newUpdateCommand builds the "update" subcommand. It is called by [UpdateCmd]
// with the fully wired production orchestrator and by tests with a fake.
func newUpdateCommand(cfg Config, runner updateRunner) *cobra.Command {
	var dryRun bool
	var checkOnly bool
	var version string
	var allowDowngrade bool

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update " + cfg.AppName + " to the latest release",
		Long: "Download and install the latest release from " + cfg.Repo + ".\n" +
			"The update is cosign-verified and supports automatic rollback on failure.",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := update.RunOpts{
				DryRun:         dryRun,
				CheckOnly:      checkOnly,
				Version:        version,
				AllowDowngrade: allowDowngrade,
			}
			result, err := runner.Run(cmd.Context(), opts)
			if err != nil {
				return fmt.Errorf("update: %w", err)
			}
			switch result.Kind {
			case update.KindOK:
				fmt.Fprintf(cmd.OutOrStdout(), "updated %s -> %s\n", result.From, result.To)
			case update.KindNoOp:
				fmt.Fprintf(cmd.OutOrStdout(), "%s is already at %s\n", cfg.AppName, result.From)
			case update.KindCheckOnly:
				fmt.Fprintf(cmd.OutOrStdout(), "latest: %s (current: %s)\n", result.Latest, result.From)
			case update.KindDryRun:
				fmt.Fprintf(cmd.OutOrStdout(), "dry-run: would update %s -> %s\n", result.From, result.To)
			case update.KindCancelled:
				fmt.Fprintf(cmd.OutOrStdout(), "update cancelled\n")
			case update.KindRolledBack:
				fmt.Fprintf(cmd.OutOrStdout(), "update failed and was rolled back to %s\n", result.From)
			default:
				fmt.Fprintf(cmd.OutOrStdout(), "update result: %s\n", result.Kind)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "plan the update without applying any changes")
	cmd.Flags().BoolVar(&checkOnly, "check", false, "report the latest available version without downloading")
	cmd.Flags().StringVar(&version, "version", "", "pin the update to a specific version (e.g. v0.1.3)")
	cmd.Flags().BoolVar(&allowDowngrade, "allow-downgrade", false, "allow installing an older version than the current one")

	return cmd
}
