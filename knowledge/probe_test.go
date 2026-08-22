package knowledge

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPathExists(t *testing.T) {
	dir := t.TempDir()
	present := filepath.Join(dir, "here.txt")
	if err := os.WriteFile(present, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	ok, err := PathExists(present)()
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected PathExists to report true for an existing path")
	}

	ok, err = PathExists(filepath.Join(dir, "missing.txt"))()
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected PathExists to report false for a missing path")
	}
}

func TestExpandPath_TildeExpansion(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	got, err := expandPath("~/Downloads")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, "Downloads")
	if got != want {
		t.Errorf("expandPath(~/Downloads) = %s, want %s", got, want)
	}

	got, err = expandPath("/absolute/path")
	if err != nil {
		t.Fatal(err)
	}
	if got != "/absolute/path" {
		t.Errorf("expandPath should leave absolute paths unchanged, got %s", got)
	}
}
