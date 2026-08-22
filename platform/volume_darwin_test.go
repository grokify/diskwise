//go:build darwin

package platform

import "testing"

func TestVolumeStatsForPath(t *testing.T) {
	dir := t.TempDir()

	vs, err := VolumeStatsForPath(dir)
	if err != nil {
		t.Fatal(err)
	}
	if vs.CapacityBytes <= 0 {
		t.Errorf("CapacityBytes = %d, want > 0", vs.CapacityBytes)
	}
	if vs.AvailableBytes < 0 || vs.AvailableBytes > vs.CapacityBytes {
		t.Errorf("AvailableBytes = %d, out of range for CapacityBytes = %d", vs.AvailableBytes, vs.CapacityBytes)
	}
	if vs.UsedBytes < 0 || vs.UsedBytes > vs.CapacityBytes {
		t.Errorf("UsedBytes = %d, out of range for CapacityBytes = %d", vs.UsedBytes, vs.CapacityBytes)
	}
	if vs.MountPoint == "" {
		t.Error("MountPoint is empty")
	}
	if vs.FSType == "" {
		t.Error("FSType is empty")
	}
}

func TestVolumeStatsForPath_NotFound(t *testing.T) {
	_, err := VolumeStatsForPath("/nonexistent-path-diskwise-test-xyz")
	if err == nil {
		t.Fatal("expected error for nonexistent path, got nil")
	}
}
