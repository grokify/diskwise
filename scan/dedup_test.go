package scan

import "testing"

func TestDedupSet_ChecksAndMarks(t *testing.T) {
	d := newDedupSet()

	if d.checkAndMark(1, 100) {
		t.Fatal("first sighting should not be reported as a duplicate")
	}
	if !d.checkAndMark(1, 100) {
		t.Fatal("second sighting of the same (device, inode) should be a duplicate")
	}
	if d.checkAndMark(1, 200) {
		t.Fatal("different inode on the same device should not be a duplicate")
	}
	if d.checkAndMark(2, 100) {
		t.Fatal("same inode number on a different device should not be a duplicate")
	}
}
