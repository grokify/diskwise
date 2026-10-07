package detect

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/grokify/diskwise/index"
)

// PairVerdict is the outcome of comparing an archive with its sibling
// extracted directory.
type PairVerdict string

const (
	// PairSame means the archive and directory hold the same number of
	// files and the same total bytes. Strong (not conclusive) evidence
	// that one is redundant; file contents are not hashed.
	PairSame PairVerdict = "same"
	// PairSameCount means equal file counts but different total bytes.
	PairSameCount PairVerdict = "same_count"
	// PairArchiveHasMore means the archive lists more files than the
	// directory holds: removing the archive could lose files.
	PairArchiveHasMore PairVerdict = "archive_has_more"
	// PairDirHasMore means the directory holds more files than the
	// archive lists (it has grown since, or the archive is partial).
	PairDirHasMore PairVerdict = "dir_has_more"
	// PairSkipped means the archive was not read (see Detail).
	PairSkipped PairVerdict = "skipped"
	// PairUnreadable means reading the archive failed (see Detail).
	PairUnreadable PairVerdict = "unreadable"
)

// Comparison basis for an ArchivePair.
const (
	CompareAllFiles         = "all files"
	CompareLibraryOriginals = "photos library originals only"
)

// ArchivePair relates an archive to the directory of the same name
// beside it ("Downloads.tar" and "Downloads/"), with a file-count and
// byte comparison. It is evidence for a human deciding which of the two
// to keep; DiskWise never acts on it.
type ArchivePair struct {
	Archive      string
	ArchiveAlloc int64
	Dir          string
	DirAlloc     int64
	Compared     string
	ArchiveFiles int64
	ArchiveBytes int64
	DirFiles     int64
	DirBytes     int64
	Verdict      PairVerdict
	Detail       string `json:",omitempty"`
	ArchiveExtra int64  `json:",omitempty"` // how many more files the archive lists than the directory holds
}

// ArchivePairOptions tunes FindArchivePairs.
type ArchivePairOptions struct {
	// MinSize skips archives smaller than this (allocated bytes).
	MinSize int64
	// MaxCompressedBytes caps gzip/bzip2 archives that are read (they
	// must be decompressed in full); <= 0 means no cap. Plain tar and
	// zip are never capped: they are read by header only.
	MaxCompressedBytes int64
}

// FindArchivePairs finds readable archives under root that have a
// sibling directory named like the archive minus its extension, and
// compares each with that directory's indexed contents. Archives inside
// managed bundles are ignored. Results are largest-archive first.
//
// When either side is (or contains) a Photos library, only original
// media is compared: a library migrated between app versions keeps the
// same originals under a different internal layout, so comparing every
// file would report thousands of phantom differences.
func FindArchivePairs(ctx context.Context, db *index.DB, root string, opts ArchivePairOptions) ([]ArchivePair, error) {
	bundles := map[string]bool{}
	if err := withBundleRoots(ctx, db, root, bundles); err != nil {
		return nil, fmt.Errorf("detect: archive-pairs: %w", err)
	}
	rows, err := db.FilesByExtensions(ctx, root, archiveExtCandidates)
	if err != nil {
		return nil, fmt.Errorf("detect: archive-pairs: %w", err)
	}

	var pairs []ArchivePair
	for _, row := range rows {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if row.AllocatedSizeFor() < opts.MinSize || isClaimed(row.Path, bundles) {
			continue
		}
		base, ok := stripArchiveSuffix(row.Name)
		if !ok {
			continue
		}
		dir := filepath.Join(filepath.Dir(row.Path), base)
		dirRow, ok, err := db.NodeByPath(ctx, dir)
		if err != nil {
			return nil, fmt.Errorf("detect: archive-pairs: %w", err)
		}
		if !ok || dirRow.Kind != index.KindDir {
			continue
		}

		pair := ArchivePair{
			Archive: row.Path, ArchiveAlloc: row.AllocatedSizeFor(),
			Dir: dir, DirAlloc: dirRow.AllocatedSizeFor(),
			Compared: CompareAllFiles,
		}
		sum, err := summarizeArchive(row.Path, row.LogicalSizeFor(), opts.MaxCompressedBytes)
		switch {
		case errors.Is(err, errUnsupportedArchive):
			continue // e.g. .gz of a single file, .7z: nothing to compare
		case errors.Is(err, errCompressedTooLarge):
			pair.Verdict, pair.Detail = PairSkipped, "compressed archive exceeds the read limit; raise --max-compressed to compare it"
			pairs = append(pairs, pair)
			continue
		case err != nil:
			pair.Verdict, pair.Detail = PairUnreadable, err.Error()
			pairs = append(pairs, pair)
			continue
		}

		infixes := []string(nil)
		pair.ArchiveFiles, pair.ArchiveBytes = sum.Files, sum.Bytes
		if sum.HasLibrary {
			pair.Compared = CompareLibraryOriginals
			pair.ArchiveFiles, pair.ArchiveBytes = sum.OriginalFiles, sum.OriginalBytes
			infixes = libraryOriginalMarkers
			pair.Detail = "compared original media only: a Photos library keeps the same originals under a different internal layout across app versions"
		}
		pair.DirFiles, pair.DirBytes, err = db.FileTotals(ctx, dir, infixes)
		if err != nil {
			return nil, fmt.Errorf("detect: archive-pairs: %w", err)
		}
		pair.Verdict = comparePair(pair)
		if pair.ArchiveFiles > pair.DirFiles {
			pair.ArchiveExtra = pair.ArchiveFiles - pair.DirFiles
		}
		pairs = append(pairs, pair)
	}

	sort.SliceStable(pairs, func(i, j int) bool { return pairs[i].ArchiveAlloc > pairs[j].ArchiveAlloc })
	return pairs, nil
}

func comparePair(p ArchivePair) PairVerdict {
	switch {
	case p.ArchiveFiles > p.DirFiles:
		return PairArchiveHasMore
	case p.ArchiveFiles < p.DirFiles:
		return PairDirHasMore
	case p.ArchiveBytes == p.DirBytes:
		return PairSame
	default:
		return PairSameCount
	}
}
