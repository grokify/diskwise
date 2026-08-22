package policy

import "github.com/grokify/diskwise/entity"

// MinConfidenceForSafeDelete is the minimum detection confidence a
// finding must carry to keep a SafeDelete recommendation; below it,
// Evaluate downgrades to LikelySafe. This is the "only regeneratable
// caches plus corroborating evidence may reach safe_delete" rule from
// docs/specs/TRD.md §6.2.
const MinConfidenceForSafeDelete = 0.8

// managedKinds are entity kinds structurally barred from SafeDelete
// and LikelySafe in V1 — no detector may promote them past
// BackupThenDelete, because their bytes are live-managed data (a
// database, a running container, a VM disk, installed software), not
// disposable cache. There is no override.
var managedKinds = map[entity.Kind]bool{
	entity.KindDatabase:    true,
	entity.KindContainer:   true,
	entity.KindVM:          true,
	entity.KindApplication: true,
}

// rank orders action classes from most to least aggressive, so a
// ceiling can be enforced: "don't recommend anything past this tier."
var rank = map[ActionClass]int{
	SafeDelete:       4,
	LikelySafe:       3,
	BackupThenDelete: 2,
	Review:           1,
	Keep:             0,
	Unknown:          0,
}

// IsManagedData reports whether kind represents live-managed data
// that must never be recommended for outright deletion without
// review — a database, container, VM, or installed application.
func IsManagedData(kind entity.Kind) bool {
	return managedKinds[kind]
}

// Evaluate applies DiskWise's fail-closed policy rules to a
// detector's proposed action class, returning the class a finding
// should actually carry. A detector's proposal can only be made more
// conservative here, never less — the scanner discovers bytes, a
// detector proposes what they mean, and this is the one place that
// decides whether that proposal is safe to surface.
func Evaluate(proposed ActionClass, kind entity.Kind, confidence float64) ActionClass {
	if IsManagedData(kind) {
		proposed = capAt(proposed, BackupThenDelete)
	}
	if proposed == SafeDelete && confidence < MinConfidenceForSafeDelete {
		proposed = LikelySafe
	}
	return proposed
}

// capAt returns whichever of proposed or ceiling is less aggressive.
func capAt(proposed, ceiling ActionClass) ActionClass {
	if rank[proposed] > rank[ceiling] {
		return ceiling
	}
	return proposed
}
