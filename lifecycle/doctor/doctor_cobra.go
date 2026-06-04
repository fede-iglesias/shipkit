package doctor

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// NewCommand returns a *cobra.Command for the doctor verb, pre-wired with the
// given Deps. The command's RunE calls Run with the flags parsed from the command
// line, formats the report, and writes it to cmd.OutOrStdout().
//
// Exit code semantics: cobra itself does not set the process exit code for RunE
// returning nil. Callers should check ExitCode(report) and call os.Exit when
// building the final command tree. The common pattern is to use a PersistentPostRunE
// that reads a stored report value, or to wrap NewCommand.RunE in the consumer.
//
// Example consumer wiring:
//
//	cmd := doctor.NewCommand(deps)
//	root.AddCommand(cmd)
func NewCommand(deps Deps) *cobra.Command {
	var opts Options

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: fmt.Sprintf("check the health of your %s installation", deps.AppName),
		Long: fmt.Sprintf(`doctor inspects the %s installation and reports the health of each component.

Checks cover: binary path, executable permission, version match, XDG directories,
install marker, shell completions, autostart service, and recovery manifest.

Network checks (--network) additionally probe GitHub, Sigstore TUF, and the
update feed. These are off by default to keep doctor fast and offline-safe.

Exit code: 0 when all checks pass or warn; 1 when any check fails.`, deps.AppName),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if ctx == nil {
				// Fallback for commands called without an ExecuteContext parent.
				// In production cobra always sets a context; this protects against
				// direct RunE calls in tests and unusual consumer configurations.
				ctx = context.Background()
			}

			report, err := runFn(ctx, deps, opts)
			if err != nil {
				return fmt.Errorf("doctor: %w", err)
			}

			writeReport(cmd, report, opts)

			// Return an error to signal the non-zero exit code to cobra when
			// there are failures. We use a sentinel that the consumer can detect.
			if !report.Summary.OK {
				// Return a typed exit error so the caller can os.Exit(1).
				// cobra.Command.Execute() returns this error; consumer calls os.Exit.
				return &ExitError{Code: 1, Report: report}
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&opts.Network, "network", false, "include network health checks (GitHub, Sigstore TUF, update feed)")
	cmd.Flags().BoolVar(&opts.JSON, "json", false, "output results as JSON")
	cmd.Flags().BoolVar(&opts.Verbose, "verbose", false, "show additional detail for passing checks")

	return cmd
}

// runFn calls deps.RunFunc when set, otherwise delegates to Run.
// This follows the sigstoreRealVerify injection pattern: the function is
// nil by default (production) and injectable in tests to trigger error paths
// in the cobra RunE that are structurally unreachable via Run itself.
func runFn(ctx context.Context, deps Deps, opts Options) (Report, error) {
	if deps.RunFunc != nil {
		return deps.RunFunc(ctx, opts)
	}
	return Run(ctx, deps, opts)
}

// writeReport writes the report to cmd.OutOrStdout() in text or JSON format.
// JSON marshalling of Report never fails (no channels, funcs, or unsupported types),
// so the json.Marshal error branch is structurally unreachable but kept as a
// defensive fallback.
func writeReport(cmd *cobra.Command, report Report, opts Options) {
	if opts.JSON {
		// json.Marshal on Report is always safe (only strings, ints, bools).
		data, _ := json.MarshalIndent(report, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return
	}
	fmt.Fprint(cmd.OutOrStdout(), FormatText(report, opts.Verbose))
}

// ExitError is returned by the cobra command's RunE when the doctor report
// contains at least one failing check. It carries the exit code (always 1) and
// the full Report for callers that want to inspect it without re-running.
type ExitError struct {
	// Code is the process exit code to use (1 for failures).
	Code int

	// Report is the full doctor report that caused the failure.
	Report Report
}

// Error implements the error interface. Returns a short human-readable message.
func (e *ExitError) Error() string {
	return fmt.Sprintf("doctor: %d check(s) failed", e.Report.Summary.Fail)
}
