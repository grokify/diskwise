package rollup

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/grokify/diskwise/index"
)

// Collapse returns the smallest set of paths that together cover
// exactly the same nodes as paths, replacing a directory's children
// with the directory itself wherever every child is covered. A
// directory that's only partially covered — a "mixed" directory —
// naturally stays decomposed into its individual covered child paths,
// since it's never collapsed. This is the directly-actionable form of
// a finding: one path to delete instead of a scattered file list.
func Collapse(ctx context.Context, db *index.DB, paths []string) ([]string, error) {
	if len(paths) <= 1 {
		return paths, nil
	}

	covered := make(map[string]bool, len(paths))
	for _, p := range paths {
		covered[p] = true
	}

	// Repeatedly try to collapse each covered path's parent: if every
	// child of the parent is covered, replace them all with the
	// parent — whose own parent might now also be fully covered, so
	// keep going until a full pass makes no further changes.
	for changed := true; changed; {
		changed = false
		for p := range covered {
			parent := filepath.Dir(p)
			if parent == p || covered[parent] {
				continue
			}

			parentRow, ok, err := db.NodeByPath(ctx, parent)
			if err != nil {
				return nil, fmt.Errorf("rollup: %w", err)
			}
			if !ok || parentRow.Kind != index.KindDir {
				continue // reached the scanned root's boundary
			}

			children, err := db.Children(ctx, parent)
			if err != nil {
				return nil, fmt.Errorf("rollup: %w", err)
			}
			if len(children) == 0 {
				continue
			}
			allCovered := true
			for _, c := range children {
				if !covered[c.Path] {
					allCovered = false
					break
				}
			}
			if !allCovered {
				continue
			}

			for _, c := range children {
				delete(covered, c.Path)
			}
			covered[parent] = true
			changed = true
		}
	}

	out := make([]string, 0, len(covered))
	for p := range covered {
		out = append(out, p)
	}
	sort.Strings(out)
	return out, nil
}
