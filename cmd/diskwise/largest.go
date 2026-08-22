package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/grokify/diskwise/index"
	"github.com/grokify/diskwise/service"
)

func newLargestCmd() *cobra.Command {
	var limit int
	var dirsOnly, filesOnly bool

	cmd := &cobra.Command{
		Use:   "largest [path]",
		Short: "List the largest files and directories under a path",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dirsOnly && filesOnly {
				return fmt.Errorf("--dirs and --files are mutually exclusive")
			}
			raw := "."
			if len(args) == 1 {
				raw = args[0]
			}
			path, err := resolvePath(raw)
			if err != nil {
				return err
			}

			var kind index.Kind
			switch {
			case dirsOnly:
				kind = index.KindDir
			case filesOnly:
				kind = index.KindFile
			}

			dbFlag, _ := cmd.Flags().GetString("db")
			db, err := openIndexDB(dbFlag)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()

			items, err := service.New(db).Largest(cmd.Context(), service.LargestQuery{Path: path, Limit: limit, Kind: kind})
			if err != nil {
				return err
			}

			asJSON, _ := cmd.Flags().GetBool("json")
			if asJSON {
				return printJSON(cmd.OutOrStdout(), items)
			}

			w := cmd.OutOrStdout()
			for _, item := range items {
				fprintf(w, "%-10s %-6s %s\n", humanBytes(item.AllocatedSize), item.Kind, item.Path)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "maximum number of items to show")
	cmd.Flags().BoolVar(&dirsOnly, "dirs", false, "show only directories")
	cmd.Flags().BoolVar(&filesOnly, "files", false, "show only files")
	return cmd
}
