# Known Locations

DiskWise ships a registry of storage locations it recognizes by path. Each
entry is only used when it applies to your machine (for example, the Xcode
entries need Xcode installed). A recognized location is reported with its
full path, size, and a default tier.

| Location | Paths | Default tier |
|---|---|---|
| Docker Desktop VM disk and container storage | `~/Library/Containers/com.docker.docker` | `review` |
| Xcode DerivedData | `~/Library/Developer/Xcode/DerivedData` | `safe_delete` |
| Xcode Archives | `~/Library/Developer/Xcode/Archives` | `review` |
| iOS / watchOS / tvOS simulators | `~/Library/Developer/CoreSimulator` | `review` |
| Homebrew downloaded archives | `~/Library/Caches/Homebrew` | `safe_delete` |
| Homebrew installed packages | `/opt/homebrew/Cellar`, `/usr/local/Cellar` | `keep` |
| Go module and build caches | `~/go/pkg/mod`, `~/Library/Caches/go-build` | `safe_delete` |
| Node package manager caches (npm, pnpm, Yarn) | `~/.npm`, `~/Library/pnpm`, `~/Library/Caches/Yarn` | `safe_delete` |
| Python package caches (pip, uv) | `~/Library/Caches/pip`, `~/.cache/uv` | `safe_delete` |
| Local AI models (Hugging Face, Ollama, LM Studio) | `~/.cache/huggingface`, `~/.ollama/models`, `~/.lmstudio` | `review` |
| iOS / iPadOS device backups | `~/Library/Application Support/MobileSync/Backup` | `review` |
| Google Chrome cache | `~/Library/Caches/Google/Chrome` | `safe_delete` |
| Firefox cache | `~/Library/Caches/Firefox` | `safe_delete` |
| Safari cache | `~/Library/Caches/com.apple.Safari` | `safe_delete` |
| Trash | `~/.Trash` | `keep` |

Default tiers are only proposals. The policy step can make them more
conservative but never less, and `safe_delete` additionally requires a
confidence of at least 0.8.

## Why some of these are `review`

- **Docker, simulators, AI models, iOS backups** are large and usually
  regenerable, but removing them costs real time or loses state (images you
  built, models you downloaded, a backup that may be your only copy). A
  person should decide.
- **Xcode Archives** hold built `.xcarchive` bundles you may need for
  distribution or symbolication.

## Managed bundles

Separately from the registry, DiskWise recognizes app-managed packages by name
and reports them as `keep`: `.photoslibrary`, `.migratedphotolibrary`,
`.aplibrary`, `.musiclibrary`, `.tvlibrary`, `.imovielibrary`, `.fcpbundle`,
`.logicx`, `.band`, and `.sparsebundle`. A bundle inside one of the locations
above is reported as part of that location instead.

## Adding a location

Entries live in `knowledge/default.go`. Each declares its probes (is the app
installed?), data paths, entity kind, semantics, and default tier. Add a test
using an injected registry, never the real machine state. See the
[Technical Requirements](../specs/TRD.md) for the design.
