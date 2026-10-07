package service

import (
	"context"
	"fmt"

	"github.com/grokify/diskwise/detect"
)

// ArchivePairQuery configures an ArchivePairs report.
type ArchivePairQuery struct {
	Path string
	// MinSize skips archives smaller than this many allocated bytes.
	MinSize int64
	// MaxCompressedBytes caps how large a gzip/bzip2 archive is read
	// (they must be decompressed in full); 0 uses
	// DefaultMaxCompressedBytes, negative means no cap.
	MaxCompressedBytes int64
	// Progress, if non-nil, is called with each archive before it is read.
	Progress func(archive string, size int64)
}

// DefaultMaxCompressedBytes is the compressed-archive size above which
// ArchivePairs skips reading by default (8 GiB).
const DefaultMaxCompressedBytes int64 = 8 << 30

// ArchivePairs finds archives under q.Path that sit beside a directory
// of the same name and compares the two by file count and bytes. It
// reads archive member lists from disk (not the index), but never
// extracts or modifies anything.
func (s *Service) ArchivePairs(ctx context.Context, q ArchivePairQuery) ([]detect.ArchivePair, error) {
	if _, ok, err := s.db.NodeByPath(ctx, q.Path); err != nil {
		return nil, fmt.Errorf("service: archive pairs %s: %w", q.Path, err)
	} else if !ok {
		return nil, errNotIndexed(q.Path)
	}

	maxCompressed := q.MaxCompressedBytes
	if maxCompressed == 0 {
		maxCompressed = DefaultMaxCompressedBytes
	}
	pairs, err := detect.FindArchivePairs(ctx, s.db, q.Path, detect.ArchivePairOptions{
		MinSize: q.MinSize, MaxCompressedBytes: maxCompressed, Progress: q.Progress,
	})
	if err != nil {
		return nil, fmt.Errorf("service: archive pairs %s: %w", q.Path, err)
	}
	return pairs, nil
}
