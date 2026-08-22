package main

import (
	"github.com/spf13/cobra"

	"github.com/grokify/diskwise/scan"
	"github.com/grokify/diskwise/service"
)

func newRescanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rescan <path>",
		Short: "Re-walk one subtree and update the index, propagating the byte delta to its ancestors",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := resolvePath(args[0])
			if err != nil {
				return err
			}

			dbFlag, _ := cmd.Flags().GetString("db")
			db, err := openIndexDB(dbFlag)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()

			stats := &scan.Stats{}
			stop := startProgress(cmd.ErrOrStderr(), stats)
			result, err := service.New(db).Rescan(cmd.Context(), service.RescanRequest{Path: path, Stats: stats})
			stop()
			if err != nil {
				return err
			}

			asJSON, _ := cmd.Flags().GetBool("json")
			if asJSON {
				return printJSON(cmd.OutOrStdout(), result)
			}
			printScanResult(cmd.OutOrStdout(), result)
			return nil
		},
	}
	return cmd
}
