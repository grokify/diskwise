package service

import (
	"context"
	"fmt"

	"github.com/grokify/diskwise/index"
)

// LargestQuery configures a Largest query.
type LargestQuery struct {
	Path  string
	Limit int
	// Kind restricts results to index.KindDir or index.KindFile; empty
	// returns both.
	Kind index.Kind
}

// Item is one entry in a Largest result.
type Item struct {
	Path          string
	Name          string
	Kind          index.Kind
	LogicalSize   int64
	AllocatedSize int64
}

// Largest returns the biggest items (by allocated size) under q.Path,
// excluding q.Path itself.
func (s *Service) Largest(ctx context.Context, q LargestQuery) ([]Item, error) {
	if _, ok, err := s.db.NodeByPath(ctx, q.Path); err != nil {
		return nil, fmt.Errorf("service: largest under %s: %w", q.Path, err)
	} else if !ok {
		return nil, errNotIndexed(q.Path)
	}

	limit := q.Limit
	if limit <= 0 {
		limit = 20
	}

	rows, err := s.db.Largest(ctx, q.Path, limit, q.Kind)
	if err != nil {
		return nil, fmt.Errorf("service: largest under %s: %w", q.Path, err)
	}

	items := make([]Item, len(rows))
	for i, row := range rows {
		items[i] = Item{
			Path:          row.Path,
			Name:          row.Name,
			Kind:          row.Kind,
			LogicalSize:   row.LogicalSizeFor(),
			AllocatedSize: row.AllocatedSizeFor(),
		}
	}
	return items, nil
}
