package policy

// ActionClass is the reclaimability recommendation tier assigned to a
// finding. Phase 2 detectors assign a location's static default;
// Phase 3's policy engine (see docs/specs/TRD.md §6.2) adds the
// fail-closed, confidence- and risk-based rules that can refine it —
// this package currently defines only the vocabulary, not that logic.
type ActionClass string

const (
	SafeDelete       ActionClass = "safe_delete"
	LikelySafe       ActionClass = "likely_safe"
	BackupThenDelete ActionClass = "backup_then_delete"
	Review           ActionClass = "review"
	Keep             ActionClass = "keep"
	Unknown          ActionClass = "unknown"
)
