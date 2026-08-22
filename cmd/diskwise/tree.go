package main

import (
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/grokify/diskwise/service"
)

func newTreeCmd() *cobra.Command {
	var depth int
	var minSizeFlag string

	cmd := &cobra.Command{
		Use:   "tree [path]",
		Short: "Show subtree totals, sorted by size",
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
			minSize, err := parseSize(minSizeFlag)
			if err != nil {
				return err
			}

			dbFlag, _ := cmd.Flags().GetString("db")
			db, err := openIndexDB(dbFlag)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()

			node, err := service.New(db).Tree(cmd.Context(), service.TreeQuery{Path: path, Depth: depth, MinSize: minSize})
			if err != nil {
				return err
			}

			asJSON, _ := cmd.Flags().GetBool("json")
			if asJSON {
				return printJSON(cmd.OutOrStdout(), node)
			}
			printTree(cmd.OutOrStdout(), *node, 0)
			return nil
		},
	}
	cmd.Flags().IntVar(&depth, "depth", 2, "levels of children to display")
	cmd.Flags().StringVar(&minSizeFlag, "min-size", "", "hide entries below this size (e.g. 100mb, 1gb)")
	return cmd
}

func printTree(w io.Writer, node service.TreeNode, indent int) {
	prefix := strings.Repeat("  ", indent)
	fprintf(w, "%s%-10s %s\n", prefix, humanBytes(node.AllocatedSize), node.Name)
	for _, child := range node.Children {
		printTree(w, child, indent+1)
	}
}
