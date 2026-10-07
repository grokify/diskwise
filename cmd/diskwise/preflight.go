package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/grokify/diskwise/service"
)

func newPreflightCmd() *cobra.Command {
	var needFlag string

	cmd := &cobra.Command{
		Use:   "preflight [path]",
		Short: "Check whether a volume has enough free space (e.g. for an OS upgrade)",
		Long: `Check whether the volume holding path (default /) has at least --need bytes
free, reporting free space, the APFS container's free space, and Time
Machine local snapshots. Read-only: it never touches the index or any file.
Exits non-zero when the space is insufficient, so it can gate a script.

Purgeable space (caches and snapshots macOS evicts on demand) is not
measurable from the command line and is not counted.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			raw := "/"
			if len(args) == 1 {
				raw = args[0]
			}
			path, err := resolvePath(raw)
			if err != nil {
				return err
			}
			need, err := parseSize(needFlag)
			if err != nil {
				return err
			}

			// Preflight reads live system state, not the index, so the
			// database is irrelevant; a throwaway handle satisfies New.
			svc := service.New(nil)
			res, err := svc.Preflight(cmd.Context(), service.PreflightRequest{Path: path, NeedBytes: need})
			if err != nil {
				return err
			}

			if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
				if err := printJSON(cmd.OutOrStdout(), res); err != nil {
					return err
				}
			} else {
				printPreflight(cmd, res)
			}
			if !res.Sufficient {
				return fmt.Errorf("insufficient free space: short by %s", humanBytes(res.ShortfallBytes))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&needFlag, "need", "", "space required, e.g. 50gib (omit to just report free space)")
	return cmd
}

func printPreflight(cmd *cobra.Command, res *service.PreflightResult) {
	w := cmd.OutOrStdout()
	fprintf(w, "%s (%s, mounted at %s)\n", res.Path, res.FSType, res.MountPoint)
	fprintf(w, "  capacity:   %s\n", humanBytes(res.CapacityBytes))
	fprintf(w, "  available:  %s\n", humanBytes(res.AvailableBytes))
	if res.ContainerFreeBytes > 0 {
		fprintf(w, "  container:  %s free (shared by all volumes in the APFS container)\n", humanBytes(res.ContainerFreeBytes))
	}
	fprintf(w, "  snapshots:  %d local\n", len(res.LocalSnapshots))
	if res.NeedBytes > 0 {
		if res.Sufficient {
			fprintf(w, "\nOK: %s available, %s needed\n", humanBytes(res.AvailableBytes), humanBytes(res.NeedBytes))
		} else {
			fprintf(w, "\nINSUFFICIENT: %s available, %s needed (short by %s)\n",
				humanBytes(res.AvailableBytes), humanBytes(res.NeedBytes), humanBytes(res.ShortfallBytes))
		}
	}
	for _, n := range res.Notes {
		fprintf(w, "  note: %s\n", n)
	}
}
