package main

import (
	"github.com/spf13/cobra"

	"github.com/grokify/diskwise/service"
)

func newPairsCmd() *cobra.Command {
	var minSizeFlag, maxCompressedFlag string

	cmd := &cobra.Command{
		Use:   "pairs [path]",
		Short: "Compare archives with the extracted directories beside them",
		Long: `Find archives (.tar, .tar.gz, .tgz, .tar.bz2, .zip) that sit beside a
directory of the same name and compare the two by file count and bytes,
so you can see which of an archive and its extracted copy is redundant.

Read-only: archive member lists are read but nothing is extracted. File
contents are not hashed, so "same" is strong evidence, not proof. For a
Photos library only original media is compared, because the library's
internal layout changes between app versions.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := pathArg(cmd, args)
			if err != nil {
				return err
			}
			minSize, err := parseSize(minSizeFlag)
			if err != nil {
				return err
			}
			maxCompressed, err := parseSize(maxCompressedFlag)
			if err != nil {
				return err
			}
			if maxCompressedFlag == "0" {
				maxCompressed = -1 // explicit "no cap"
			}

			dbFlag, _ := cmd.Flags().GetString("db")
			db, err := openIndexDB(dbFlag)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()

			pairs, err := service.New(db).ArchivePairs(cmd.Context(), service.ArchivePairQuery{
				Path: path, MinSize: minSize, MaxCompressedBytes: maxCompressed, Progress: archiveProgress(cmd),
			})
			if err != nil {
				return err
			}
			rd, err := redactorFor(cmd)
			if err != nil {
				return err
			}
			if rd != nil {
				pairs = rd.Pairs(pairs)
			}

			if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
				return printJSON(cmd.OutOrStdout(), pairs)
			}
			w := cmd.OutOrStdout()
			if len(pairs) == 0 {
				fprintf(w, "no archive/directory pairs found under %s\n", path)
				return nil
			}
			for _, p := range pairs {
				fprintf(w, "%-10s %-17s %s\n", humanBytes(p.ArchiveAlloc), p.Verdict, p.Archive)
				fprintf(w, "           beside directory %s (%s)\n", p.Dir, humanBytes(p.DirAlloc))
				switch p.Verdict {
				case "skipped", "unreadable":
					fprintf(w, "           %s\n", p.Detail)
				default:
					fprintf(w, "           compared %s: archive %d files (%s), directory %d files (%s)\n",
						p.Compared, p.ArchiveFiles, humanBytes(p.ArchiveBytes), p.DirFiles, humanBytes(p.DirBytes))
					if p.ArchiveExtra > 0 {
						fprintf(w, "           the archive lists %d more file(s) than the directory holds; check before removing it\n", p.ArchiveExtra)
					}
					if p.Detail != "" {
						fprintf(w, "           %s\n", p.Detail)
					}
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&minSizeFlag, "min-size", "10mb", "skip archives smaller than this")
	cmd.Flags().StringVar(&maxCompressedFlag, "max-compressed", "8gib", "skip gzip/bzip2 archives larger than this, which must be decompressed to read (0 = no cap)")
	addRedactFlags(cmd)
	return cmd
}
