//go:build darwin

package platform

import (
	"fmt"
	"syscall"
)

// VolumeStatsForPath returns capacity information for the volume
// containing path.
func VolumeStatsForPath(path string) (VolumeStats, error) {
	var raw syscall.Statfs_t
	if err := syscall.Statfs(path, &raw); err != nil {
		return VolumeStats{}, fmt.Errorf("platform: statfs %s: %w", path, err)
	}
	capacity := int64(raw.Blocks) * int64(raw.Bsize)
	used := capacity - int64(raw.Bfree)*int64(raw.Bsize)
	available := int64(raw.Bavail) * int64(raw.Bsize)
	return VolumeStats{
		MountPoint:     cString(raw.Mntonname[:]),
		FSType:         cString(raw.Fstypename[:]),
		CapacityBytes:  capacity,
		AvailableBytes: available,
		UsedBytes:      used,
	}, nil
}

// cString converts a NUL-terminated int8 byte array, as returned by
// Statfs_t's fixed-size string fields, into a Go string.
func cString(b []int8) string {
	n := 0
	for n < len(b) && b[n] != 0 {
		n++
	}
	buf := make([]byte, n)
	for i, c := range b[:n] {
		buf[i] = byte(c)
	}
	return string(buf)
}
