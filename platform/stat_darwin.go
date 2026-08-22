//go:build darwin

package platform

import (
	"fmt"
	"os"
	"syscall"
)

// statBlockSize is the fixed unit that st_blocks counts in, per POSIX —
// independent of the filesystem's actual block size.
const statBlockSize = 512

// Stat returns filesystem metadata for path without following a
// trailing symlink, including the allocated (on-disk) size APFS
// reports via st_blocks.
func Stat(path string) (FileStat, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return FileStat{}, fmt.Errorf("platform: stat %s: %w", path, err)
	}
	raw, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return FileStat{}, fmt.Errorf("platform: stat %s: unsupported Sys() type %T", path, info.Sys())
	}
	return FileStat{
		Path:          path,
		LogicalSize:   info.Size(),
		AllocatedSize: raw.Blocks * statBlockSize,
		Device:        raw.Dev,
		Inode:         raw.Ino,
		Nlink:         raw.Nlink,
		IsDir:         info.IsDir(),
		IsSymlink:     info.Mode()&os.ModeSymlink != 0,
		ModTime:       info.ModTime(),
	}, nil
}
