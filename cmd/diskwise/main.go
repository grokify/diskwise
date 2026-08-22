// Command diskwise is the DiskWise CLI: a macOS storage-intelligence
// tool that discovers what is consuming disk space and what can be
// safely reclaimed. Subcommands are added as the underlying service
// layer is implemented (see docs/specs/PLAN.md, Phase 1).
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "diskwise",
		Short:         "Know your disk. Use it wisely.",
		Long:          "DiskWise discovers what is consuming macOS disk space and what can be safely reclaimed.",
		Version:       version,
		SilenceUsage:  true, // a runtime error (e.g. "not indexed") isn't a usage mistake
		SilenceErrors: true, // main() prints the error once, itself
	}
	cmd.PersistentFlags().String("db", "", "path to the DiskWise index (default: ~/Library/Application Support/DiskWise/index.db)")
	cmd.PersistentFlags().Bool("json", false, "output JSON instead of a human-readable table")

	cmd.AddCommand(
		newScanCmd(),
		newRescanCmd(),
		newSummaryCmd(),
		newTreeCmd(),
		newLargestCmd(),
		newHotspotsCmd(),
		newOpportunitiesCmd(),
		newSavingsCmd(),
		newReportCmd(),
		newInsightsCmd(),
	)
	return cmd
}
