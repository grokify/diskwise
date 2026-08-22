package main

import (
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/grokify/diskwise/policy"
	"github.com/grokify/diskwise/service"
)

// tierDisplayOrder is most-actionable first, matching how a user
// would triage: what's free to reclaim right now, down to what needs
// the most caution.
var tierDisplayOrder = []policy.ActionClass{
	policy.SafeDelete,
	policy.LikelySafe,
	policy.BackupThenDelete,
	policy.Review,
	policy.Keep,
	policy.Unknown,
}

func newSavingsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "savings [path]",
		Short: "Show potential disk savings by action-class tier",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			raw := "."
			if len(args) == 1 {
				raw = args[0]
			}
			path, err := resolvePath(raw)
			if err != nil {
				return err
			}

			dbFlag, _ := cmd.Flags().GetString("db")
			db, err := openIndexDB(dbFlag)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()

			savings, err := service.New(db).Savings(cmd.Context(), path)
			if err != nil {
				return err
			}

			asJSON, _ := cmd.Flags().GetBool("json")
			if asJSON {
				return printJSON(cmd.OutOrStdout(), savings)
			}

			w := cmd.OutOrStdout()
			var total int64
			for _, v := range savings.Tiers {
				total += v
			}
			fprintf(w, "Potential savings under %s: %s\n\n", savings.Path, humanBytes(total))

			seen := make(map[policy.ActionClass]bool, len(tierDisplayOrder))
			for _, tier := range tierDisplayOrder {
				seen[tier] = true
				if v, ok := savings.Tiers[tier]; ok {
					fprintf(w, "  %-20s %s\n", displayTier(tier), humanBytes(v))
				}
			}
			// Any tier not in the known display order (shouldn't
			// normally happen, but don't silently drop data).
			var extra []string
			for tier := range savings.Tiers {
				if !seen[tier] {
					extra = append(extra, string(tier))
				}
			}
			sort.Strings(extra)
			for _, tier := range extra {
				fprintf(w, "  %-20s %s\n", displayTier(policy.ActionClass(tier)), humanBytes(savings.Tiers[policy.ActionClass(tier)]))
			}
			return nil
		},
	}
	return cmd
}

func displayTier(tier policy.ActionClass) string {
	return strings.ToUpper(strings.ReplaceAll(string(tier), "_", " "))
}
