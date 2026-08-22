package service

import (
	"context"
	"fmt"

	"github.com/grokify/diskwise/index"
)

// TreeQuery configures a Tree query.
type TreeQuery struct {
	Path string
	// Depth is how many levels of children to expand below Path; 0
	// returns just Path's own node with no children.
	Depth int
	// MinSize filters out entries below this many allocated bytes
	// (subtree total for directories, own size for files). 0 = no filter.
	MinSize int64
}

// TreeNode is one entry in a Tree result, with children populated up
// to the requested depth.
type TreeNode struct {
	Path          string
	Name          string
	Kind          index.Kind
	LogicalSize   int64
	AllocatedSize int64
	State         index.SubtreeState
	Children      []TreeNode `json:",omitempty"`
}

// Tree returns q.Path and its descendants down to q.Depth levels,
// each level sorted by size descending.
func (s *Service) Tree(ctx context.Context, q TreeQuery) (*TreeNode, error) {
	row, ok, err := s.db.NodeByPath(ctx, q.Path)
	if err != nil {
		return nil, fmt.Errorf("service: tree %s: %w", q.Path, err)
	}
	if !ok {
		return nil, errNotIndexed(q.Path)
	}

	node := toTreeNode(row)
	if q.Depth > 0 && row.Kind == index.KindDir {
		children, err := s.expandChildren(ctx, q.Path, q.Depth, q.MinSize)
		if err != nil {
			return nil, err
		}
		node.Children = children
	}
	return &node, nil
}

func (s *Service) expandChildren(ctx context.Context, path string, depth int, minSize int64) ([]TreeNode, error) {
	rows, err := s.db.Children(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("service: children of %s: %w", path, err)
	}

	var out []TreeNode
	for _, row := range rows {
		if row.AllocatedSizeFor() < minSize {
			continue
		}
		node := toTreeNode(row)
		if depth > 1 && row.Kind == index.KindDir {
			children, err := s.expandChildren(ctx, row.Path, depth-1, minSize)
			if err != nil {
				return nil, err
			}
			node.Children = children
		}
		out = append(out, node)
	}
	return out, nil
}

func toTreeNode(row index.NodeRow) TreeNode {
	return TreeNode{
		Path:          row.Path,
		Name:          row.Name,
		Kind:          row.Kind,
		LogicalSize:   row.LogicalSizeFor(),
		AllocatedSize: row.AllocatedSizeFor(),
		State:         row.State,
	}
}
