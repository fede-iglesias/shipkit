package clean

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// NewCommand constructs the cobra.Command for the "clean" subcommand.
//
// The returned command is ready to be added to a root cobra.Command:
//
//	root.AddCommand(clean.NewCommand(deps))
//
// Flags registered:
//
//	--snapshots     Clean snapshot directories under DataDir/snapshots/.
//	--tmp           Clean work directories under DataDir/tmp/.
//	--cache         Clean cache entries under DataDir/cache/.
//	--logs          Clean log files under DataDir/logs/.
//	--all           Equivalent to --snapshots --tmp --cache --logs.
//	--older-than    Only remove entries older than DUR (default 720h = 30d).
//	--keep N        Always keep the newest N snapshots (overrides --older-than).
//	--print         Dry-run: print candidates without removing anything.
//	-y, --yes       Skip the confirmation prompt.
//
// When no scope flag is set, the command prints its usage and exits 1.
func NewCommand(deps Deps) *cobra.Command {
	opts := Options{}
	var olderThanStr string

	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Remove stale artifacts from " + deps.AppName + "'s data directory",
		Long: `Clean removes stale artifacts from ` + deps.AppName + `'s data directory.

Scope flags select what to clean (at least one is required):

  --snapshots   Remove old snapshot directories (filtered by --older-than / --keep)
  --tmp         Remove work directories left over from failed updates
  --cache       Remove cache entries
  --logs        Remove log files
  --all         Clean everything (equivalent to all scope flags)

Safety defaults:
  - No flags = print help and exit 1 (no accidental deletion)
  - --print shows candidates without removing anything
  - Prompts for confirmation unless -y is passed
  - Never deletes a snapshot referenced by .shipkit.recovery-manifest.json
  - Never follows symlinks that escape the data directory`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Parse --older-than string into time.Duration.
			if olderThanStr != "" {
				d, err := time.ParseDuration(olderThanStr)
				if err != nil {
					return fmt.Errorf("--older-than: %w", err)
				}
				opts.OlderThan = d
			}

			result, err := Run(cmd.Context(), deps, opts)
			if err != nil {
				if errors.Is(err, ErrNoScope) {
					// Print usage and return an error so cobra exits 1.
					_ = cmd.Usage()
					return err
				}
				return err
			}

			if opts.Print {
				if len(result.Items) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "Dry-run: no candidates found.")
					return nil
				}
				fmt.Fprintln(cmd.OutOrStdout(), "Dry-run: would remove:")
				for _, item := range result.Items {
					fmt.Fprintf(cmd.OutOrStdout(), "  [%s] %s (%s)\n",
						item.Category, item.Path, formatBytes(item.Size))
				}
				totalBytes := int64(0)
				for _, item := range result.Items {
					totalBytes += item.Size
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Total to reclaim: %s (%d items)\n",
					formatBytes(totalBytes), len(result.Items))
				return nil
			}

			if len(result.Items) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Nothing to clean.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed %d item(s). Reclaimed %s.\n",
				len(result.Items), formatBytes(result.Reclaimed))
			return nil
		},
	}

	cmd.Flags().BoolVar(&opts.Snapshots, "snapshots", false,
		"Clean snapshot directories under DataDir/snapshots/")
	cmd.Flags().BoolVar(&opts.Tmp, "tmp", false,
		"Clean work directories under DataDir/tmp/")
	cmd.Flags().BoolVar(&opts.Cache, "cache", false,
		"Clean cache entries under DataDir/cache/")
	cmd.Flags().BoolVar(&opts.Logs, "logs", false,
		"Clean log files under DataDir/logs/")
	cmd.Flags().BoolVar(&opts.All, "all", false,
		"Clean everything (equivalent to --snapshots --tmp --cache --logs)")
	cmd.Flags().StringVar(&olderThanStr, "older-than", "",
		`Only remove snapshot entries older than DUR (e.g. "168h", "720h"). Default: 720h`)
	cmd.Flags().IntVar(&opts.Keep, "keep", 0,
		"Always keep the newest N snapshots regardless of --older-than")
	cmd.Flags().BoolVar(&opts.Print, "print", false,
		"Dry-run: print candidates without removing anything")
	cmd.Flags().BoolVarP(&opts.Yes, "yes", "y", false,
		"Skip the confirmation prompt")

	return cmd
}
