package main

import (
	"github.com/spf13/cobra"

	"github.com/grokify/diskwise/entity"
	"github.com/grokify/diskwise/policy"
	"github.com/grokify/diskwise/service"
)

func newOpportunitiesCmd() *cobra.Command {
	var actionFlag, typeFlag string
	var minConfidence float64
	var pathsOnly bool

	cmd := &cobra.Command{
		Use:   "opportunities [path]",
		Short: "List reclaimable findings, grouped by action-class tier",
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

			opps, err := service.New(db).Opportunities(cmd.Context(), service.OpportunityQuery{
				Path:          path,
				ActionClass:   policy.ActionClass(actionFlag),
				MinConfidence: minConfidence,
				Kind:          entity.Kind(typeFlag),
			})
			if err != nil {
				return err
			}

			asJSON, _ := cmd.Flags().GetBool("json")
			if asJSON {
				return printJSON(cmd.OutOrStdout(), opps)
			}

			w := cmd.OutOrStdout()
			if pathsOnly {
				for _, o := range opps {
					for _, p := range o.Paths {
						fprintf(w, "%s\n", p)
					}
				}
				return nil
			}

			byTier := make(map[policy.ActionClass][]service.Opportunity)
			var order []policy.ActionClass
			for _, o := range opps {
				if _, ok := byTier[o.Finding.ActionClass]; !ok {
					order = append(order, o.Finding.ActionClass)
				}
				byTier[o.Finding.ActionClass] = append(byTier[o.Finding.ActionClass], o)
			}
			for _, tier := range order {
				fprintf(w, "%s\n", tier)
				for _, o := range byTier[tier] {
					fprintf(w, "  %-10s %-6s %s\n", humanBytes(o.Finding.AllocatedSize), o.Finding.Entity.Kind, o.Finding.Path)
					fprintf(w, "             %s\n", o.Finding.Reason)
					for _, sc := range o.Finding.Scenarios {
						fprintf(w, "             [%s] %s — %s\n", sc.Name, humanBytes(sc.ReclaimableBytes), sc.Description)
					}
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&actionFlag, "action", "", "filter to one action class (safe_delete, likely_safe, backup_then_delete, review, keep, unknown)")
	cmd.Flags().Float64Var(&minConfidence, "min-confidence", 0, "hide findings below this detection confidence (0-1)")
	cmd.Flags().StringVar(&typeFlag, "type", "", "filter to one entity kind (cache, archive, artifact_family, ...)")
	cmd.Flags().BoolVar(&pathsOnly, "paths", false, "print one actionable path per line instead of a table")
	return cmd
}
