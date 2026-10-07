// Package reportdoc defines the report document: one versioned JSON
// file holding everything a DiskWise report shows, from which HTML,
// XLSX and Markdown are rendered deterministically.
//
// It replaces juggling separate savings, hotspots and opportunities
// output. Those overlap (savings is the per-tier total of the findings;
// hotspots is a selection of them), and each repeated the root and scan
// time. Here they are stated once: the tier totals (computed before any
// size cut), the freshness of the data, an account of what was filtered
// out or has vanished from disk, the findings, and optionally the
// archive/extracted-directory comparisons.
//
// The Go structs are the source of truth for the JSON Schema in the
// schema subpackage. The types are deliberately separate from the
// service package's, so the file format can be versioned on its own and
// internal refactors do not change what is on disk.
package reportdoc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

// SchemaVersion identifies the shape of Document so a renderer can
// reject a file written against an incompatible version instead of
// silently mis-rendering it.
const SchemaVersion = "1"

// Document is the top-level report document.
type Document struct {
	SchemaVersion string `json:"schemaVersion" jsonschema:"enum=1,description=Must be reportdoc.SchemaVersion; a renderer rejects any other value."`
	Root          string `json:"root" jsonschema:"description=Absolute path the report covers."`
	// MeasuredAt is when the scan covering Root finished (UTC). It comes
	// from the index and never from the clock, so rendering the same
	// document always gives the same output.
	MeasuredAt time.Time `json:"measuredAt"`
	ScanStatus string    `json:"scanStatus" jsonschema:"enum=complete,enum=partial,description=partial means some directories were denied or skipped."`
	Stale      bool      `json:"stale" jsonschema:"description=True when the scan is more than a week old."`

	// Tiers totals allocated bytes per action tier across ALL findings,
	// including any omitted by the size filter.
	Tiers map[string]int64 `json:"tiers" jsonschema:"description=Allocated bytes per action tier before any size filter. Keys are the tier names."`
	// ReclaimableBytes sums the tiers a user could actually reclaim:
	// everything except keep and unknown.
	ReclaimableBytes int64 `json:"reclaimableBytes"`

	Missing  Missing   `json:"missing"`
	Filter   Filter    `json:"filter"`
	Findings []Finding `json:"findings"`
	Pairs    []Pair    `json:"pairs,omitempty" jsonschema:"description=Archive and extracted-directory comparisons. Present only when requested."`
}

// Missing accounts for findings whose paths no longer exist on disk:
// the index predates their removal. Their bytes are still in Tiers.
type Missing struct {
	Count int   `json:"count"`
	Bytes int64 `json:"bytes"`
}

// Filter records the size cut applied to Findings, so a filtered list
// never hides how much it left out.
type Filter struct {
	MinSizeBytes int64 `json:"minSizeBytes" jsonschema:"description=Findings smaller than this many allocated bytes were omitted. 0 means no cut."`
	OmittedCount int   `json:"omittedCount"`
	OmittedBytes int64 `json:"omittedBytes"`
}

// Finding is one reclaimable (or explained) item.
type Finding struct {
	Tier            string     `json:"tier" jsonschema:"enum=safe_delete,enum=likely_safe,enum=backup_then_delete,enum=review,enum=keep,enum=unknown"`
	Kind            string     `json:"kind" jsonschema:"description=What the finding is: cache or archive or artifact_family or model or container or managed_bundle or unknown and so on."`
	Name            string     `json:"name,omitempty"`
	Detector        string     `json:"detector,omitempty"`
	Path            string     `json:"path" jsonschema:"description=Anchor path: the single node or the containing directory for a multi-path finding."`
	Paths           []string   `json:"paths,omitempty" jsonschema:"description=Every indexed node the finding covers."`
	ActionablePaths []string   `json:"actionablePaths,omitempty" jsonschema:"description=Paths rolled up to the highest fully covered directory: what to act on."`
	AllocatedBytes  int64      `json:"allocatedBytes"`
	LogicalBytes    int64      `json:"logicalBytes"`
	Confidence      float64    `json:"confidence" jsonschema:"minimum=0,maximum=1"`
	Reason          string     `json:"reason,omitempty"`
	Missing         bool       `json:"missing,omitempty" jsonschema:"description=True when a path of the finding no longer exists on disk."`
	Scenarios       []Scenario `json:"scenarios,omitempty"`
}

// Scenario is an alternative outcome for a finding, such as keeping
// the newest of several downloaded versions.
type Scenario struct {
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	ReclaimableBytes int64    `json:"reclaimableBytes"`
	Paths            []string `json:"paths,omitempty"`
}

// Pair compares an archive with the directory of the same name beside
// it. Contents are never hashed, so the verdict is evidence, not proof.
type Pair struct {
	Archive      string `json:"archive"`
	ArchiveAlloc int64  `json:"archiveAllocatedBytes"`
	Dir          string `json:"dir"`
	DirAlloc     int64  `json:"dirAllocatedBytes"`
	Compared     string `json:"compared" jsonschema:"description=What was compared: all files or photos library originals only."`
	ArchiveFiles int64  `json:"archiveFiles"`
	ArchiveBytes int64  `json:"archiveBytes"`
	DirFiles     int64  `json:"dirFiles"`
	DirBytes     int64  `json:"dirBytes"`
	Verdict      string `json:"verdict" jsonschema:"enum=same,enum=same_count,enum=archive_has_more,enum=dir_has_more,enum=skipped,enum=unreadable"`
	Detail       string `json:"detail,omitempty"`
	ArchiveExtra int64  `json:"archiveExtraFiles,omitempty" jsonschema:"description=How many more files the archive lists than the directory holds."`
}

var validTiers = map[string]bool{
	"safe_delete": true, "likely_safe": true, "backup_then_delete": true,
	"review": true, "keep": true, "unknown": true,
}

var validVerdicts = map[string]bool{
	"same": true, "same_count": true, "archive_has_more": true,
	"dir_has_more": true, "skipped": true, "unreadable": true,
}

// Validate reports whether d is well-formed enough to render: the right
// schema version, a root, known tiers and verdicts, and findings that
// name a path. It cannot check that paths exist or that the analysis is
// right.
func (d Document) Validate() error {
	if d.SchemaVersion != SchemaVersion {
		return fmt.Errorf("reportdoc: schemaVersion %q, want %q", d.SchemaVersion, SchemaVersion)
	}
	if d.Root == "" {
		return fmt.Errorf("reportdoc: root is required")
	}
	for tier := range d.Tiers {
		if !validTiers[tier] {
			return fmt.Errorf("reportdoc: tiers: unknown tier %q", tier)
		}
	}
	for i, f := range d.Findings {
		if f.Path == "" {
			return fmt.Errorf("reportdoc: findings[%d]: path is required", i)
		}
		if !validTiers[f.Tier] {
			return fmt.Errorf("reportdoc: findings[%d]: unknown tier %q", i, f.Tier)
		}
		if f.Confidence < 0 || f.Confidence > 1 {
			return fmt.Errorf("reportdoc: findings[%d]: confidence %v outside 0..1", i, f.Confidence)
		}
	}
	for i, p := range d.Pairs {
		if p.Archive == "" || p.Dir == "" {
			return fmt.Errorf("reportdoc: pairs[%d]: archive and dir are required", i)
		}
		if !validVerdicts[p.Verdict] {
			return fmt.Errorf("reportdoc: pairs[%d]: unknown verdict %q", i, p.Verdict)
		}
	}
	return nil
}

// Parse strictly decodes JSON into a Document and validates it,
// rejecting unknown fields so a typo or schema drift fails loudly
// instead of silently dropping data.
func Parse(data []byte) (Document, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var d Document
	if err := dec.Decode(&d); err != nil {
		return Document{}, fmt.Errorf("reportdoc: parse: %w", err)
	}
	if err := d.Validate(); err != nil {
		return Document{}, err
	}
	return d, nil
}

// Marshal renders d as indented JSON with a trailing newline, the form
// written to disk.
func Marshal(d Document) ([]byte, error) {
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("reportdoc: marshal: %w", err)
	}
	return append(data, '\n'), nil
}
