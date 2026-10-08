package main

import (
	"runtime/debug"
	"strings"
	"testing"
)

func info(mainVersion string, settings ...string) func() (*debug.BuildInfo, bool) {
	return func() (*debug.BuildInfo, bool) {
		bi := &debug.BuildInfo{Main: debug.Module{Version: mainVersion}}
		for i := 0; i+1 < len(settings); i += 2 {
			bi.Settings = append(bi.Settings, debug.BuildSetting{Key: settings[i], Value: settings[i+1]})
		}
		return bi, true
	}
}

func TestResolveVersion(t *testing.T) {
	const full = "6bf178024036464ba03adc6bbc6416583b1cd9b7"
	tests := []struct {
		name      string
		linkerSet string
		read      func() (*debug.BuildInfo, bool)
		want      string
	}{
		{"linker value wins over build info", "v9.9.9", info("v0.3.0"), "v9.9.9"},
		{"tagged install", devVersion, info("v0.3.0"), "v0.3.0"},
		{"clean build past a tag", devVersion, info("v0.3.1-0.20261008011256-6bf178024036"), "v0.3.1-0.20261008011256-6bf178024036"},
		{"dirty build keeps Go's suffix", devVersion, info("v0.3.0+dirty"), "v0.3.0+dirty"},
		{"devel with vcs info", devVersion, info("(devel)", "vcs.revision", full, "vcs.modified", "false"), "dev-6bf1780"},
		{"devel with vcs info and changes", devVersion, info("(devel)", "vcs.revision", full, "vcs.modified", "true"), "dev-6bf1780+dirty"},
		{"devel without vcs info", devVersion, info("(devel)"), "dev"},
		{"empty version without vcs info", devVersion, info(""), "dev"},
		{"short revision is kept whole", devVersion, info("(devel)", "vcs.revision", "abc12"), "dev-abc12"},
		{"no build info", devVersion, func() (*debug.BuildInfo, bool) { return nil, false }, "dev"},
		{"build info reported ok but nil", devVersion, func() (*debug.BuildInfo, bool) { return nil, true }, "dev"},
	}
	for _, tt := range tests {
		if got := resolveVersion(tt.linkerSet, tt.read); got != tt.want {
			t.Errorf("%s: resolveVersion = %q, want %q", tt.name, got, tt.want)
		}
	}
}

// `diskwise --version` is wired to the resolver: whatever the build
// reports, the output is "diskwise version <non-empty>".
func TestRootCommand_PrintsAVersion(t *testing.T) {
	out := runCLI(t, "--version")
	out = strings.TrimSpace(out)
	if !strings.HasPrefix(out, "diskwise version ") || strings.TrimPrefix(out, "diskwise version ") == "" {
		t.Errorf("--version output = %q, want %q followed by a version", out, "diskwise version ")
	}
}
