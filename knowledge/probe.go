package knowledge

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Probe reports whether a KnownLocation is relevant on this machine —
// e.g. whether the owning application is installed — so entries whose
// probe fails cost effectively nothing to check.
type Probe func() (bool, error)

// AppBundleExists probes for an installed .app bundle in /Applications.
func AppBundleExists(bundleName string) Probe {
	return PathExists(filepath.Join("/Applications", bundleName))
}

// PathExists probes for the existence of an arbitrary path; a leading
// "~" is expanded to the current user's home directory.
func PathExists(path string) Probe {
	return func() (bool, error) {
		expanded, err := expandPath(path)
		if err != nil {
			return false, err
		}
		_, err = os.Stat(expanded)
		switch {
		case err == nil:
			return true, nil
		case os.IsNotExist(err):
			return false, nil
		default:
			return false, err
		}
	}
}

func expandPath(path string) (string, error) {
	if !strings.HasPrefix(path, "~") {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("expand %s: %w", path, err)
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~")), nil
}
