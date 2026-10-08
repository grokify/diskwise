package main

import "runtime/debug"

// devVersion is what a build reports when nothing better is known.
const devVersion = "dev"

// version is set at build time with -ldflags "-X main.version=..." for
// builds that want to pin it. When left at the default, resolveVersion
// derives it from the build info Go embeds.
var version = devVersion

// resolveVersion returns the version to report. An explicit linker-set
// value always wins. Otherwise it reads the module version Go embeds in
// every binary: "v0.3.0" for `go install ...@v0.3.0` and for a clean
// local build exactly on that tag, a pseudo-version such as
// "v0.3.1-0.20261008011256-6bf178024036" for a build past it, and a
// "+dirty" suffix for uncommitted changes. A "(devel)" build that still
// carries VCS information reports the short commit, and with no build
// info at all it stays "dev".
func resolveVersion(linkerSet string, read func() (*debug.BuildInfo, bool)) string {
	if linkerSet != devVersion {
		return linkerSet
	}
	info, ok := read()
	if !ok || info == nil {
		return devVersion
	}
	if v := info.Main.Version; v != "" && v != "(devel)" {
		return v
	}

	var rev string
	var modified bool
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			modified = s.Value == "true"
		}
	}
	if rev == "" {
		return devVersion
	}
	if len(rev) > 7 {
		rev = rev[:7]
	}
	v := devVersion + "-" + rev
	if modified {
		v += "+dirty"
	}
	return v
}
