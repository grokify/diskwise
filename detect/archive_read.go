package detect

import (
	"archive/tar"
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
)

// archiveSummary is what reading an archive's member list yields.
type archiveSummary struct {
	Files int64
	Bytes int64
	// OriginalFiles/OriginalBytes cover only a Photos library's original
	// media (Masters/ in the legacy layout, originals/ in the modern one).
	OriginalFiles int64
	OriginalBytes int64
	HasLibrary    bool
}

// libraryOriginalMarkers mark a Photos library's original media in the
// legacy and modern layouts respectively.
var libraryOriginalMarkers = []string{".photoslibrary/Masters/", ".photoslibrary/originals/"}

func isLibraryOriginal(name string) bool {
	for _, m := range libraryOriginalMarkers {
		if strings.Contains(name, m) {
			return true
		}
	}
	return false
}

// errUnsupportedArchive marks a format DiskWise cannot list.
var errUnsupportedArchive = errors.New("unsupported archive format")

// errCompressedTooLarge marks a compressed archive over the read cap.
var errCompressedTooLarge = errors.New("compressed archive exceeds the read limit")

// summarizeArchive lists the members of a .tar, .tar.gz/.tgz,
// .tar.bz2/.tbz2 or .zip archive without extracting it. Plain tar and
// zip are read by header/central directory (fast, even for huge files);
// gzip and bzip2 must be decompressed end to end, so they are refused
// beyond maxCompressed bytes (<= 0 means no cap). Directories and macOS
// AppleDouble "._*" sidecars are not counted: extraction merges those
// into extended attributes, so counting them would report phantom
// differences against the extracted copy.
func summarizeArchive(file string, size, maxCompressed int64) (archiveSummary, error) {
	lower := strings.ToLower(file)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return summarizeZip(file)
	case strings.HasSuffix(lower, ".tar"):
		return summarizeTar(file, nil)
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		if maxCompressed > 0 && size > maxCompressed {
			return archiveSummary{}, errCompressedTooLarge
		}
		return summarizeTar(file, func(r io.Reader) (io.Reader, error) { return gzip.NewReader(r) })
	case strings.HasSuffix(lower, ".tar.bz2"), strings.HasSuffix(lower, ".tbz2"):
		if maxCompressed > 0 && size > maxCompressed {
			return archiveSummary{}, errCompressedTooLarge
		}
		return summarizeTar(file, func(r io.Reader) (io.Reader, error) { return bzip2.NewReader(r), nil })
	}
	return archiveSummary{}, errUnsupportedArchive
}

func (s *archiveSummary) add(name string, size int64) {
	if strings.HasPrefix(path.Base(name), "._") {
		return
	}
	s.Files++
	s.Bytes += size
	if strings.Contains(name, ".photoslibrary/") {
		s.HasLibrary = true
	}
	if isLibraryOriginal(name) {
		s.OriginalFiles++
		s.OriginalBytes += size
	}
}

func summarizeTar(file string, wrap func(io.Reader) (io.Reader, error)) (_ archiveSummary, err error) {
	f, err := os.Open(file) //nolint:gosec // G304: the path is an indexed archive the user asked to compare
	if err != nil {
		return archiveSummary{}, fmt.Errorf("open %s: %w", file, err)
	}
	defer func() {
		if cerr := f.Close(); err == nil && cerr != nil {
			err = fmt.Errorf("close %s: %w", file, cerr)
		}
	}()

	var r io.Reader = f // an *os.File lets archive/tar seek past member data
	if wrap != nil {
		if r, err = wrap(f); err != nil {
			return archiveSummary{}, fmt.Errorf("decompress %s: %w", file, err)
		}
	}

	var sum archiveSummary
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return sum, nil
		}
		if err != nil {
			return archiveSummary{}, fmt.Errorf("read %s: %w", file, err)
		}
		if hdr.Typeflag == tar.TypeReg {
			sum.add(hdr.Name, hdr.Size)
		}
	}
}

func summarizeZip(file string) (_ archiveSummary, err error) {
	zr, err := zip.OpenReader(file)
	if err != nil {
		return archiveSummary{}, fmt.Errorf("open %s: %w", file, err)
	}
	defer func() {
		if cerr := zr.Close(); err == nil && cerr != nil {
			err = fmt.Errorf("close %s: %w", file, cerr)
		}
	}()

	var sum archiveSummary
	for _, m := range zr.File {
		if m.FileInfo().IsDir() {
			continue
		}
		sum.add(m.Name, int64(m.UncompressedSize64)) //nolint:gosec // G115: a member would need to exceed 8 EiB to overflow
	}
	return sum, nil
}
