package knowledge

import (
	"github.com/grokify/diskwise/entity"
	"github.com/grokify/diskwise/policy"
)

// Default is the built-in registry of known heavy macOS storage
// locations (TRD §4.1). Entries with no Probes are always considered
// relevant; the detector that consumes them (KnownLocationDetector)
// only reports a finding if the data path is actually indexed, so an
// absent/never-used location costs nothing beyond the probe check.
var Default = Registry{
	{
		ID:                 "docker-desktop",
		Description:        "Docker Desktop VM disk and container storage",
		Probes:             []Probe{AppBundleExists("Docker.app")},
		DataPaths:          []string{"~/Library/Containers/com.docker.docker"},
		Entity:             entity.KindContainer,
		Semantics:          SemanticsSparseVMDisk,
		DefaultActionClass: policy.Review,
	},
	{
		ID:                 "xcode-derived-data",
		Description:        "Xcode DerivedData (build artifacts, regenerated on next build)",
		Probes:             []Probe{AppBundleExists("Xcode.app")},
		DataPaths:          []string{"~/Library/Developer/Xcode/DerivedData"},
		Entity:             entity.KindCache,
		Semantics:          SemanticsRegeneratableCache,
		DefaultActionClass: policy.SafeDelete,
	},
	{
		ID:                 "xcode-archives",
		Description:        "Xcode Archives (built .xcarchive bundles for distribution)",
		Probes:             []Probe{AppBundleExists("Xcode.app")},
		DataPaths:          []string{"~/Library/Developer/Xcode/Archives"},
		Entity:             entity.KindApplication,
		Semantics:          SemanticsUserData,
		DefaultActionClass: policy.Review,
	},
	{
		ID:                 "xcode-simulators",
		Description:        "iOS/watchOS/tvOS Simulator runtimes and device data",
		Probes:             []Probe{AppBundleExists("Xcode.app")},
		DataPaths:          []string{"~/Library/Developer/CoreSimulator"},
		Entity:             entity.KindCache,
		Semantics:          SemanticsUserData,
		DefaultActionClass: policy.Review,
	},
	{
		ID:                 "homebrew-cache",
		Description:        "Homebrew downloaded package archives",
		Probes:             []Probe{PathExists("/opt/homebrew/bin/brew"), PathExists("/usr/local/bin/brew")},
		DataPaths:          []string{"~/Library/Caches/Homebrew"},
		Entity:             entity.KindCache,
		Semantics:          SemanticsRegeneratableCache,
		DefaultActionClass: policy.SafeDelete,
	},
	{
		ID:                 "homebrew-cellar",
		Description:        "Homebrew installed package versions",
		Probes:             []Probe{PathExists("/opt/homebrew/bin/brew"), PathExists("/usr/local/bin/brew")},
		DataPaths:          []string{"/opt/homebrew/Cellar", "/usr/local/Cellar"},
		Entity:             entity.KindApplication,
		Semantics:          SemanticsInstalledSoftware,
		DefaultActionClass: policy.Keep,
	},
	{
		ID:                 "go-caches",
		Description:        "Go module cache and build cache",
		DataPaths:          []string{"~/go/pkg/mod", "~/Library/Caches/go-build"},
		Entity:             entity.KindCache,
		Semantics:          SemanticsRegeneratableCache,
		DefaultActionClass: policy.SafeDelete,
	},
	{
		ID:                 "node-caches",
		Description:        "Node.js package manager caches (npm/pnpm/Yarn)",
		DataPaths:          []string{"~/.npm", "~/Library/pnpm", "~/Library/Caches/Yarn"},
		Entity:             entity.KindCache,
		Semantics:          SemanticsRegeneratableCache,
		DefaultActionClass: policy.SafeDelete,
	},
	{
		ID:                 "python-caches",
		Description:        "Python package manager caches (pip/uv)",
		DataPaths:          []string{"~/Library/Caches/pip", "~/.cache/uv"},
		Entity:             entity.KindCache,
		Semantics:          SemanticsRegeneratableCache,
		DefaultActionClass: policy.SafeDelete,
	},
	{
		ID:                 "ai-models",
		Description:        "Local AI models (HuggingFace/Ollama/LM Studio)",
		DataPaths:          []string{"~/.cache/huggingface", "~/.ollama/models", "~/.lmstudio"},
		Entity:             entity.KindModel,
		Semantics:          SemanticsUserData,
		DefaultActionClass: policy.Review,
	},
	{
		ID:                 "ios-backups",
		Description:        "iOS/iPadOS device backups",
		DataPaths:          []string{"~/Library/Application Support/MobileSync/Backup"},
		Entity:             entity.KindBackup,
		Semantics:          SemanticsUserData,
		DefaultActionClass: policy.Review,
	},
	{
		ID:                 "chrome-cache",
		Description:        "Google Chrome browser cache",
		Probes:             []Probe{AppBundleExists("Google Chrome.app")},
		DataPaths:          []string{"~/Library/Caches/Google/Chrome"},
		Entity:             entity.KindCache,
		Semantics:          SemanticsRegeneratableCache,
		DefaultActionClass: policy.SafeDelete,
	},
	{
		ID:                 "firefox-cache",
		Description:        "Firefox browser cache",
		Probes:             []Probe{AppBundleExists("Firefox.app")},
		DataPaths:          []string{"~/Library/Caches/Firefox"},
		Entity:             entity.KindCache,
		Semantics:          SemanticsRegeneratableCache,
		DefaultActionClass: policy.SafeDelete,
	},
	{
		ID:                 "safari-cache",
		Description:        "Safari browser cache",
		DataPaths:          []string{"~/Library/Caches/com.apple.Safari"},
		Entity:             entity.KindCache,
		Semantics:          SemanticsRegeneratableCache,
		DefaultActionClass: policy.SafeDelete,
	},
	{
		ID:                 "trash",
		Description:        "Items already moved to Trash",
		DataPaths:          []string{"~/.Trash"},
		Entity:             entity.KindCache,
		Semantics:          SemanticsAlreadyDeleted,
		DefaultActionClass: policy.Keep,
	},
}
