//go:build darwin

package platform

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// System probes machine-level storage state by running read-only macOS
// command-line tools. It is the production implementation of the probe
// the service layer's Preflight takes, so tests can substitute a fake.
type System struct{}

// Volume returns capacity information for the volume containing path.
func (System) Volume(path string) (VolumeStats, error) { return VolumeStatsForPath(path) }

var (
	containerFreeRE = regexp.MustCompile(`Container Free Space:.*\((\d+) Bytes\)`)
	volumeFreeRE    = regexp.MustCompile(`Volume Free Space:.*\((\d+) Bytes\)`)
)

// ContainerFreeBytes returns the free bytes of the APFS container
// holding path (shared by every volume in it), falling back to the
// volume's free space on non-APFS filesystems. It parses `diskutil
// info`, which has no stable machine-readable form for this field.
func (System) ContainerFreeBytes(ctx context.Context, path string) (int64, error) {
	//nolint:gosec // G204: path is passed as a single argv element, never through a shell
	out, err := exec.CommandContext(ctx, "diskutil", "info", path).Output()
	if err != nil {
		return 0, fmt.Errorf("platform: diskutil info %s: %w", path, err)
	}
	return parseFreeSpace(string(out))
}

// parseFreeSpace extracts the free-byte count from `diskutil info` output.
func parseFreeSpace(out string) (int64, error) {
	for _, re := range []*regexp.Regexp{containerFreeRE, volumeFreeRE} {
		m := re.FindStringSubmatch(out)
		if m == nil {
			continue
		}
		for _, g := range m[1:] {
			if g == "" {
				continue
			}
			n, err := strconv.ParseInt(g, 10, 64)
			if err != nil {
				return 0, fmt.Errorf("platform: parse free space %q: %w", g, err)
			}
			return n, nil
		}
	}
	return 0, fmt.Errorf("platform: no free-space line in diskutil output")
}

// LocalSnapshots lists Time Machine local snapshots on the boot volume
// (`tmutil listlocalsnapshots /`). macOS may reclaim these on its own
// when space is needed, so they are context for an upgrade check, not
// something DiskWise ever removes.
func (System) LocalSnapshots(ctx context.Context) ([]string, error) {
	out, err := exec.CommandContext(ctx, "tmutil", "listlocalsnapshots", "/").Output()
	if err != nil {
		return nil, fmt.Errorf("platform: tmutil listlocalsnapshots: %w", err)
	}
	return parseSnapshots(string(out)), nil
}

// parseSnapshots keeps the snapshot-name lines of tmutil output,
// dropping its header.
func parseSnapshots(out string) []string {
	var snaps []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "com.apple.") {
			snaps = append(snaps, line)
		}
	}
	return snaps
}
