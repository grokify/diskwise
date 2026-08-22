//go:build darwin

package platform

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestStat_RegularFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.bin")
	data := bytes.Repeat([]byte{0xAB}, 128<<10) // 128 KiB non-zero data

	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	st, err := Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.LogicalSize != int64(len(data)) {
		t.Errorf("LogicalSize = %d, want %d", st.LogicalSize, len(data))
	}
	if st.AllocatedSize <= 0 {
		t.Errorf("AllocatedSize = %d, want > 0", st.AllocatedSize)
	}
	if st.Inode == 0 {
		t.Error("Inode = 0, want non-zero")
	}
	if st.IsDir {
		t.Error("IsDir = true for a regular file")
	}
	if st.IsSymlink {
		t.Error("IsSymlink = true for a regular file")
	}
}

// TestStat_SparseFile verifies the exact distinction DiskWise depends
// on throughout the product: a sparse file's allocated size can be far
// smaller than its logical size (as with Docker.raw), so callers must
// never treat LogicalSize as real disk consumption.
func TestStat_SparseFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sparse.bin")
	const logicalSize = 64 << 20 // 64 MiB, no data written

	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(logicalSize); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	st, err := Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.LogicalSize != logicalSize {
		t.Errorf("LogicalSize = %d, want %d", st.LogicalSize, logicalSize)
	}
	if st.AllocatedSize >= logicalSize {
		t.Errorf("AllocatedSize = %d, want < LogicalSize (%d) for a sparse file", st.AllocatedSize, logicalSize)
	}
}

func TestStat_HardLink(t *testing.T) {
	dir := t.TempDir()
	original := filepath.Join(dir, "original.bin")
	linked := filepath.Join(dir, "linked.bin")

	if err := os.WriteFile(original, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(original, linked); err != nil {
		t.Fatal(err)
	}

	a, err := Stat(original)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Stat(linked)
	if err != nil {
		t.Fatal(err)
	}

	if a.Device != b.Device || a.Inode != b.Inode {
		t.Errorf("hard-linked files should share device+inode, got (%d,%d) vs (%d,%d)",
			a.Device, a.Inode, b.Device, b.Inode)
	}
	if a.Nlink != 2 {
		t.Errorf("Nlink = %d, want 2 after hard link", a.Nlink)
	}
}

func TestStat_Symlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	link := filepath.Join(dir, "link.txt")

	if err := os.WriteFile(target, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	st, err := Stat(link)
	if err != nil {
		t.Fatal(err)
	}
	if !st.IsSymlink {
		t.Error("IsSymlink = false, want true")
	}
}

func TestStat_Directory(t *testing.T) {
	dir := t.TempDir()

	st, err := Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !st.IsDir {
		t.Error("IsDir = false, want true")
	}
}

func TestStat_NotFound(t *testing.T) {
	_, err := Stat(filepath.Join(t.TempDir(), "missing"))
	if err == nil {
		t.Fatal("expected error for missing path, got nil")
	}
}
