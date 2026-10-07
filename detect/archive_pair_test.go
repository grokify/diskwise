package detect

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"testing"
)

type member struct {
	name string
	size int
}

func writeTar(t *testing.T, path string, gz bool, members []member) {
	t.Helper()
	var buf bytes.Buffer
	var tw *tar.Writer
	var gw *gzip.Writer
	if gz {
		gw = gzip.NewWriter(&buf)
		tw = tar.NewWriter(gw)
	} else {
		tw = tar.NewWriter(&buf)
	}
	for _, m := range members {
		hdr := &tar.Header{Name: m.name, Mode: 0o600, Size: int64(m.size), Typeflag: tar.TypeReg}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(make([]byte, m.size)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if gw != nil {
		if err := gw.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeZip(t *testing.T, path string, members []member) {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, m := range members {
		w, err := zw.Create(m.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(make([]byte, m.size)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeTree(t *testing.T, root string, members []member) {
	t.Helper()
	for _, m := range members {
		p := filepath.Join(root, m.name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		writeFile(t, p, m.size)
	}
}

func pairFor(t *testing.T, pairs []ArchivePair, archive string) ArchivePair {
	t.Helper()
	for _, p := range pairs {
		if filepath.Base(p.Archive) == archive {
			return p
		}
	}
	t.Fatalf("no pair for %s in %+v", archive, pairs)
	return ArchivePair{}
}

func TestFindArchivePairs_Verdicts(t *testing.T) {
	root := t.TempDir()
	base := []member{{"Downloads/a.bin", 1000}, {"Downloads/sub/b.bin", 2000}}

	// Same: tar and directory identical.
	writeTree(t, root, base)
	writeTar(t, filepath.Join(root, "Downloads.tar"), false, base)

	// Same via gzip, plus AppleDouble sidecars and a directory-ish entry
	// that must not be counted.
	gzMembers := append([]member{{"Docs/._a.txt", 4096}}, []member{{"Docs/a.txt", 100}}...)
	writeTree(t, root, []member{{"Docs/a.txt", 100}})
	writeTar(t, filepath.Join(root, "Docs.tar.gz"), true, gzMembers)

	// Archive has more: the directory is missing one file.
	writeTree(t, root, []member{{"Photos2/x.bin", 10}})
	writeTar(t, filepath.Join(root, "Photos2.tar"), false, []member{{"Photos2/x.bin", 10}, {"Photos2/y.bin", 10}})

	// Directory has more.
	writeTree(t, root, []member{{"Proj/a", 5}, {"Proj/b", 5}})
	writeZip(t, filepath.Join(root, "Proj.zip"), []member{{"Proj/a", 5}})

	// Same count, different bytes.
	writeTree(t, root, []member{{"Edit/a", 5}})
	writeZip(t, filepath.Join(root, "Edit.zip"), []member{{"Edit/a", 6}})

	// No sibling directory: not a pair.
	writeTar(t, filepath.Join(root, "Lonely.tar"), false, base)

	db := ingestFixture(t, root)
	pairs, err := FindArchivePairs(context.Background(), db, root, ArchivePairOptions{})
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]PairVerdict{
		"Downloads.tar": PairSame,
		"Docs.tar.gz":   PairSame,
		"Photos2.tar":   PairArchiveHasMore,
		"Proj.zip":      PairDirHasMore,
		"Edit.zip":      PairSameCount,
	}
	if len(pairs) != len(want) {
		t.Fatalf("got %d pairs, want %d: %+v", len(pairs), len(want), pairs)
	}
	for name, v := range want {
		if got := pairFor(t, pairs, name).Verdict; got != v {
			t.Errorf("%s verdict = %s, want %s", name, got, v)
		}
	}
	if p := pairFor(t, pairs, "Photos2.tar"); p.ArchiveExtra != 1 {
		t.Errorf("ArchiveExtra = %d, want 1", p.ArchiveExtra)
	}
}

// A migrated Photos library keeps the same originals under a different
// layout: only originals may be compared, never the whole tree.
func TestFindArchivePairs_PhotosLibraryComparesOriginalsOnly(t *testing.T) {
	root := t.TempDir()
	// Directory: modern layout.
	writeTree(t, root, []member{
		{"Pics/Lib.photoslibrary/originals/1/a.jpeg", 100},
		{"Pics/Lib.photoslibrary/originals/1/b.jpeg", 200},
		{"Pics/Lib.photoslibrary/resources/derivatives/t1.jpeg", 5},
		{"Pics/Lib.photoslibrary/resources/derivatives/t2.jpeg", 5},
		{"Pics/Lib.photoslibrary/resources/derivatives/t3.jpeg", 5},
	})
	// Archive: legacy layout, same two originals plus different derivatives.
	writeTar(t, filepath.Join(root, "Pics.tar"), false, []member{
		{"Pics/Lib.photoslibrary/Masters/2017/a.jpg", 100},
		{"Pics/Lib.photoslibrary/Masters/2017/b.jpg", 200},
		{"Pics/Lib.photoslibrary/Thumbnails/x.jpg", 1},
	})
	db := ingestFixture(t, root)

	pairs, err := FindArchivePairs(context.Background(), db, root, ArchivePairOptions{})
	if err != nil {
		t.Fatal(err)
	}
	p := pairFor(t, pairs, "Pics.tar")
	if p.Compared != CompareLibraryOriginals {
		t.Errorf("Compared = %q, want originals only", p.Compared)
	}
	if p.Verdict != PairSame || p.ArchiveFiles != 2 || p.DirFiles != 2 {
		t.Errorf("verdict=%s archive=%d dir=%d, want same/2/2 (layout differences ignored)", p.Verdict, p.ArchiveFiles, p.DirFiles)
	}
}

func TestFindArchivePairs_CompressedLimitAndMinSize(t *testing.T) {
	root := t.TempDir()
	m := []member{{"Big/a", 50_000}}
	writeTree(t, root, m)
	writeTar(t, filepath.Join(root, "Big.tar.gz"), true, m)
	db := ingestFixture(t, root)
	ctx := context.Background()

	pairs, err := FindArchivePairs(ctx, db, root, ArchivePairOptions{MaxCompressedBytes: 1})
	if err != nil {
		t.Fatal(err)
	}
	if got := pairFor(t, pairs, "Big.tar.gz").Verdict; got != PairSkipped {
		t.Errorf("over the compressed cap: verdict = %s, want skipped", got)
	}

	pairs, err = FindArchivePairs(ctx, db, root, ArchivePairOptions{MinSize: 1 << 30})
	if err != nil {
		t.Fatal(err)
	}
	if len(pairs) != 0 {
		t.Errorf("MinSize should drop small archives, got %+v", pairs)
	}
}

func TestFindArchivePairs_UnreadableArchive(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, []member{{"Broken/a", 10}})
	if err := os.WriteFile(filepath.Join(root, "Broken.zip"), []byte("not a zip"), 0o600); err != nil {
		t.Fatal(err)
	}
	db := ingestFixture(t, root)
	pairs, err := FindArchivePairs(context.Background(), db, root, ArchivePairOptions{})
	if err != nil {
		t.Fatal(err)
	}
	p := pairFor(t, pairs, "Broken.zip")
	if p.Verdict != PairUnreadable || p.Detail == "" {
		t.Errorf("verdict=%s detail=%q, want unreadable with a reason", p.Verdict, p.Detail)
	}
}
