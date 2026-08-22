package platform

// VolumeStats holds capacity information for the volume containing a
// given path.
type VolumeStats struct {
	MountPoint     string
	FSType         string
	CapacityBytes  int64
	AvailableBytes int64
	UsedBytes      int64
}
