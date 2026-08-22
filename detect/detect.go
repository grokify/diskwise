package detect

import (
	"context"
	"strings"

	"github.com/grokify/diskwise/entity"
	"github.com/grokify/diskwise/index"
	"github.com/grokify/diskwise/policy"
)

// Finding is one semantic storage observation: an entity, anchored to
// one or more indexed nodes, with a size and a reclaimability
// recommendation. ActionClass is always the output of policy.Evaluate
// — a detector proposes, the policy engine decides what's safe to
// surface.
type Finding struct {
	Entity entity.Entity
	// Path is the finding's primary/anchor path — the single node for
	// single-path findings, or the containing directory for
	// multi-path findings like an artifact family.
	Path string
	// Paths lists every indexed node this finding covers; rollup uses
	// this as the authoritative "what does this finding touch."
	Paths         []string
	LogicalSize   int64
	AllocatedSize int64
	Confidence    float64
	ActionClass   policy.ActionClass
	Reason        string
	Scenarios     []Scenario `json:",omitempty"`
}

// Scenario is one alternative cleanup outcome for a finding — e.g. an
// artifact family's "keep newest" versus "remove all" choice.
type Scenario struct {
	Name             string
	Description      string
	ReclaimableBytes int64
	Paths            []string
}

// Detector produces findings from the current index state under root.
// The scanner discovers bytes; a Detector determines what those bytes
// represent — it never decides what is safe to delete.
type Detector interface {
	ID() string
	Detect(ctx context.Context, db *index.DB, root string) ([]Finding, error)
}

// underRoot reports whether path is root itself or a descendant of it.
func underRoot(path, root string) bool {
	return path == root || strings.HasPrefix(path, root+"/")
}
