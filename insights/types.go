// Package insights defines a JSON intermediate representation for a
// narrative storage-savings report: a ranked list of investigation
// opportunities with directories, evidence, and reclaim actions.
//
// DiskWise's detectors and policy engine produce deterministic,
// conservative findings (see detect/ and policy/) — they never guess.
// The gap this package fills is the other half of the workflow
// described in docs/specs/PRD.md: an LLM agent reads DiskWise's raw
// output (opportunities/largest/hotspots), recognizes patterns the
// detectors can't yet (duplicate migration backups, redundant
// archive+extracted-folder pairs, stale model weights) and writes a
// Report as JSON matching these types. Report itself is then a
// stable, re-renderable artifact: Markdown and HTML are generated
// deterministically from it (see markdown.go, html.go), so producing
// the human-readable write-up again later never requires re-running
// the LLM analysis — only re-running the renderer.
package insights

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

// SchemaVersion identifies the shape of Report so a future renderer
// can detect and reject a document written against an incompatible
// version instead of silently mis-rendering it.
const SchemaVersion = "1"

// Report is the top-level document. Root/GeneratedAt/UsedBytes/
// CapacityBytes are objective facts a caller can populate directly
// from DiskWise's own output (service.Savings, platform.VolumeStats);
// Summary and Opportunities are the narrative analysis an LLM agent
// contributes.
type Report struct {
	SchemaVersion string        `json:"schemaVersion" jsonschema:"enum=1,description=Must be insights.SchemaVersion; a renderer rejects any other value."`
	Root          string        `json:"root" jsonschema:"description=Absolute path the analysis was scoped to."`
	GeneratedAt   time.Time     `json:"generatedAt"`
	UsedBytes     int64         `json:"usedBytes,omitempty" jsonschema:"description=Total bytes used on the volume containing Root, if known."`
	CapacityBytes int64         `json:"capacityBytes,omitempty" jsonschema:"description=Total volume capacity in bytes, if known."`
	Summary       string        `json:"summary" jsonschema:"description=Two to four sentence narrative overview: the headline finding and why it matters."`
	Opportunities []Opportunity `json:"opportunities"`
}

// Opportunity is one ranked investigation lead: a cluster of
// directories worth a human's attention, with the evidence that made
// it worth surfacing and concrete next steps. It intentionally
// carries its own Confidence — a qualitative judgment call by the
// authoring agent — distinct from policy.ActionClass, which is
// DiskWise's own deterministic, conservative safety tier. The two are
// not meant to be reconciled: an Opportunity here can point at a
// pattern (e.g. a duplicate migration backup) that spans many
// individual detect.Finding rows, several tiers, or none at all (an
// unclassified "unknown" directory an agent has now explained).
type Opportunity struct {
	Rank        int      `json:"rank" jsonschema:"description=1-based priority order; lower is more valuable/confident to investigate first."`
	Title       string   `json:"title" jsonschema:"description=Short human-readable headline, e.g. 'Duplicate 2022 migration backup in ~/Data'."`
	Category    Category `json:"category" jsonschema:"enum=duplicate_backup,enum=duplicate_download,enum=regeneratable_cache,enum=managed_app_data,enum=unexplained_large,enum=other"`
	Directories []string `json:"directories" jsonschema:"description=Absolute paths this opportunity covers. The primary 'where to look' signal."`
	// EstimatedBytes is allocated (on-disk) bytes, matching DiskWise's
	// own convention (see detect.Finding.AllocatedSize) — the number
	// that actually reflects potential reclaim, not apparent size.
	EstimatedBytes    int64      `json:"estimatedBytes" jsonschema:"description=Estimated reclaimable allocated bytes, matching DiskWise's allocated-size convention."`
	Confidence        Confidence `json:"confidence" jsonschema:"enum=high,enum=medium,enum=low"`
	Description       string     `json:"description" jsonschema:"description=Why this looks reclaimable: the reasoning, in prose."`
	Evidence          []string   `json:"evidence,omitempty" jsonschema:"description=Concrete observations backing Description, e.g. 'sibling file X is byte-identical in size to Y'."`
	VerificationSteps []string   `json:"verificationSteps,omitempty" jsonschema:"description=What a human should check before acting — this package never recommends unverified deletion."`
	Actions           []Action   `json:"actions,omitempty" jsonschema:"description=How to reclaim once verified, one entry per distinct method."`
}

// Category buckets an Opportunity for grouping/styling in a rendered
// report. It is intentionally coarser and more narrative than
// entity.Kind — e.g. CategoryDuplicateBackup has no equivalent
// detect/entity type today.
type Category string

const (
	CategoryDuplicateBackup    Category = "duplicate_backup"    // an old machine-migration or manual backup duplicating live data
	CategoryDuplicateDownload  Category = "duplicate_download"  // repeated downloads of the same artifact (see detect.ArtifactFamilyDetector)
	CategoryRegeneratableCache Category = "regeneratable_cache" // safe to clear; the owning tool/app rebuilds it on demand
	CategoryManagedAppData     Category = "managed_app_data"    // live-managed data an app owns (VM disks, model weights, containers)
	CategoryUnexplainedLarge   Category = "unexplained_large"   // large and unclassified; investigate before assuming anything
	CategoryOther              Category = "other"
)

// validCategories backs Validate; kept alongside the constants so the
// two can't silently drift.
var validCategories = map[Category]bool{
	CategoryDuplicateBackup:    true,
	CategoryDuplicateDownload:  true,
	CategoryRegeneratableCache: true,
	CategoryManagedAppData:     true,
	CategoryUnexplainedLarge:   true,
	CategoryOther:              true,
}

// Confidence is a qualitative judgment call by the authoring agent —
// see the Opportunity doc comment for why this is deliberately kept
// separate from policy.ActionClass.
type Confidence string

const (
	ConfidenceHigh   Confidence = "high"
	ConfidenceMedium Confidence = "medium"
	ConfidenceLow    Confidence = "low"
)

var validConfidences = map[Confidence]bool{
	ConfidenceHigh:   true,
	ConfidenceMedium: true,
	ConfidenceLow:    true,
}

// Action describes one concrete way to reclaim an Opportunity's
// space. Method is the discriminator: Directories is the primary
// payload for "filesystem", Steps for "app_ui" (e.g. Chrome's own
// clear-browsing-data UI, safer than deleting its cache directory by
// hand while it may be running), and Command for "cli" (e.g. `ollama
// rm <model>`, `docker system prune`) where the owning tool manages
// its own storage and a raw filesystem delete would be unsafe.
// Pairs is orthogonal to the Method discriminator: use it (alongside
// Directories, still method=filesystem) whenever the evidence itself
// is a redundancy relationship — a kept path and the specific
// redundant path it makes disposable — rather than a flat list where
// that relationship would otherwise be lost.
type Action struct {
	Method Method `json:"method" jsonschema:"enum=filesystem,enum=app_ui,enum=cli"`
	// Target is a human label for what this action affects, e.g.
	// "Google Chrome" or "Ollama" — most useful for app_ui/cli
	// actions where Directories may be empty or informational only.
	Target      string   `json:"target,omitempty"`
	Directories []string `json:"directories,omitempty" jsonschema:"description=Filesystem paths this action clears; primary payload for method=filesystem when there is no kept/redundant pairing to preserve."`
	Pairs       []Pair   `json:"pairs,omitempty" jsonschema:"description=Kept-vs-redundant path pairs, for method=filesystem findings where a specific file is disposable because of a specific other file (e.g. an extracted folder and the tar archive of that same folder)."`
	Command     string   `json:"command,omitempty" jsonschema:"description=A single shell command; primary payload for method=cli."`
	Steps       []string `json:"steps,omitempty" jsonschema:"description=Ordered instructions; primary payload for method=app_ui, e.g. UI navigation."`
	Notes       string   `json:"notes,omitempty"`
}

// Pair names a specific kept-vs-redundant relationship — see the
// Action.Pairs doc comment.
type Pair struct {
	Kept      string `json:"kept" jsonschema:"description=The path worth keeping."`
	Redundant string `json:"redundant" jsonschema:"description=The path this pairing makes disposable."`
	Notes     string `json:"notes,omitempty" jsonschema:"description=Why they're believed to be redundant, e.g. matching size or byte-identical checksum."`
}

// Method discriminates Action's payload — see the Action doc comment.
type Method string

const (
	MethodFilesystem Method = "filesystem"
	MethodAppUI      Method = "app_ui"
	MethodCLI        Method = "cli"
)

var validMethods = map[Method]bool{
	MethodFilesystem: true,
	MethodAppUI:      true,
	MethodCLI:        true,
}

// Validate reports whether r is well-formed enough to render: valid
// enum values, non-empty required fields, and ranks that are unique
// and gap-free from 1. It does not — and cannot — verify that the
// paths it names exist or that the analysis is correct; that's the
// verification step a human still owns.
func (r Report) Validate() error {
	if r.SchemaVersion != SchemaVersion {
		return fmt.Errorf("insights: schemaVersion %q, want %q", r.SchemaVersion, SchemaVersion)
	}
	if r.Root == "" {
		return fmt.Errorf("insights: root is required")
	}
	if r.Summary == "" {
		return fmt.Errorf("insights: summary is required")
	}

	seenRank := make(map[int]bool, len(r.Opportunities))
	for i, o := range r.Opportunities {
		if err := o.validate(); err != nil {
			return fmt.Errorf("insights: opportunities[%d]: %w", i, err)
		}
		if seenRank[o.Rank] {
			return fmt.Errorf("insights: opportunities[%d]: duplicate rank %d", i, o.Rank)
		}
		seenRank[o.Rank] = true
	}
	for rank := 1; rank <= len(r.Opportunities); rank++ {
		if !seenRank[rank] {
			return fmt.Errorf("insights: rank %d is missing; ranks must be 1..N with no gaps", rank)
		}
	}
	return nil
}

func (o Opportunity) validate() error {
	if o.Title == "" {
		return fmt.Errorf("title is required")
	}
	if !validCategories[o.Category] {
		return fmt.Errorf("unknown category %q", o.Category)
	}
	if !validConfidences[o.Confidence] {
		return fmt.Errorf("unknown confidence %q", o.Confidence)
	}
	if len(o.Directories) == 0 {
		return fmt.Errorf("directories must not be empty")
	}
	if o.Description == "" {
		return fmt.Errorf("description is required")
	}
	for i, a := range o.Actions {
		if !validMethods[a.Method] {
			return fmt.Errorf("actions[%d]: unknown method %q", i, a.Method)
		}
		for j, p := range a.Pairs {
			if p.Kept == "" || p.Redundant == "" {
				return fmt.Errorf("actions[%d]: pairs[%d]: kept and redundant are both required", i, j)
			}
		}
	}
	return nil
}

// Parse strictly decodes JSON into a Report and validates it,
// rejecting unknown fields so a typo or schema drift in an
// LLM-authored document fails loudly instead of silently dropping
// data.
func Parse(data []byte) (Report, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var r Report
	if err := dec.Decode(&r); err != nil {
		return Report{}, fmt.Errorf("insights: parse: %w", err)
	}
	if err := r.Validate(); err != nil {
		return Report{}, err
	}
	return r, nil
}
