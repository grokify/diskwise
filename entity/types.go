package entity

// Kind classifies a semantic storage entity — what a scanned path
// actually represents, as distinct from raw filesystem structure.
type Kind string

const (
	KindApplication    Kind = "application"
	KindCache          Kind = "cache"
	KindArchive        Kind = "archive"
	KindInstaller      Kind = "installer"
	KindArtifactFamily Kind = "artifact_family"
	KindDatabase       Kind = "database"
	KindContainer      Kind = "container"
	KindVM             Kind = "vm"
	KindModel          Kind = "model"
	KindProject        Kind = "project"
	KindBackup         Kind = "backup"
	// KindManagedBundle is an app-managed package directory (a Photos
	// library, a Logic project): live user data whose internals belong
	// to the app that owns it, not to a cleanup tool.
	KindManagedBundle Kind = "managed_bundle"
	KindUnknown       Kind = "unknown"
)
