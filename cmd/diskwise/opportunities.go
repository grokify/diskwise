package main

import (
	"github.com/spf13/cobra"

	"github.com/grokify/diskwise/entity"
	"github.com/grokify/diskwise/policy"
	"github.com/grokify/diskwise/service"
)

func newOpportunitiesCmd() *cobra.Command {
	var actionFlag, typeFlag, minSizeFlag string
	var minConfidence float64
	var pathsOnly bool

	cmd := &cobra.Command{
		Use:   "opportunities [path]",
		Short: "List reclaimable findings, grouped by action-class tier",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := pathArg(cmd, args)
			if err != nil {
				return err
			}

			dbFlag, _ := cmd.Flags().GetString("db")
			db, err := openIndexDB(dbFlag)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()

			minSize, err := parseSize(minSizeFlag)
			if err != nil {
				return err
			}
			rep, err := service.New(db).OpportunitiesReport(cmd.Context(), service.OpportunityQuery{
				Path:          path,
				ActionClass:   policy.ActionClass(actionFlag),
				MinConfidence: minConfidence,
				Kind:          entity.Kind(typeFlag),
				MinSize:       minSize,
			})
			if err != nil {
				return err
			}
			rd, err := redactorFor(cmd)
			if err != nil {
				return err
			}
			if rd != nil {
				rep = rd.OpportunitiesReport(rep)
			}
			opps := rep.Opportunities

			asJSON, _ := cmd.Flags().GetBool("json")
			warnStale(cmd, rep.Freshness, rep.MissingCount, 0, rep.Root)
			if asJSON {
				return printJSON(cmd.OutOrStdout(), rep)
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

			printMeasured(w, rep.Freshness)
			if rep.OmittedCount > 0 {
				fprintf(w, "%d smaller finding(s) (%s) omitted; --min-size 0 lists them\n", rep.OmittedCount, humanBytes(rep.OmittedBytes))
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
					gone := ""
					if o.Missing {
						gone = "  [missing]"
					}
					fprintf(w, "  %-10s %-6s %s%s\n", humanBytes(o.Finding.AllocatedSize), o.Finding.Entity.Kind, o.Finding.Path, gone)
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
	cmd.Flags().StringVar(&minSizeFlag, "min-size", "", "hide findings smaller than this (e.g. 1mb); the count hidden is reported")
	cmd.Flags().BoolVar(&pathsOnly, "paths", false, "print one actionable path per line instead of a table")
	addRedactFlags(cmd)
	return cmd
}
