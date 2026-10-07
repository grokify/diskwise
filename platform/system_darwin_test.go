//go:build darwin

package platform

import "testing"

func TestParseFreeSpace(t *testing.T) {
	apfs := "   Volume Name:  Macintosh HD\n   Container Total Space:  2.0 TB (1995218165760 Bytes) (exactly 3896910480 512-Byte-Units)\n   Container Free Space:   176.2 GB (176233668608 Bytes) (exactly 344206384 512-Byte-Units)\n"
	got, err := parseFreeSpace(apfs)
	if err != nil || got != 176233668608 {
		t.Errorf("apfs: got %d, %v; want 176233668608", got, err)
	}

	hfs := "   Volume Free Space:  12.0 GB (12000000000 Bytes) (exactly 23437500 512-Byte-Units)\n"
	got, err = parseFreeSpace(hfs)
	if err != nil || got != 12000000000 {
		t.Errorf("non-apfs fallback: got %d, %v; want 12000000000", got, err)
	}

	if _, err := parseFreeSpace("nothing useful"); err == nil {
		t.Error("expected an error when no free-space line is present")
	}
}

func TestParseSnapshots(t *testing.T) {
	out := "Snapshots for volume group containing disk /:\ncom.apple.TimeMachine.2026-01-01-000000.local\ncom.apple.os.update-ABC\n"
	got := parseSnapshots(out)
	if len(got) != 2 || got[0] != "com.apple.TimeMachine.2026-01-01-000000.local" {
		t.Errorf("parseSnapshots = %v", got)
	}
	if len(parseSnapshots("Snapshots for disk /:\n")) != 0 {
		t.Error("header-only output should yield no snapshots")
	}
}
