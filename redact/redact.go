// Package redact rewrites paths (and the names and descriptions derived
// from them) in findings so reports and JSON can be shared without
// leaking directory layout or sensitive project names.
//
// Two layers apply. The home directory is always abbreviated to "~"
// (this hides the username). Paths under a deny prefix are replaced
// entirely by a stable token, "<redacted:HASH>", where HASH is derived
// from the original path: the same path yields the same token across
// outputs, so redacted files can still be correlated, but the original
// cannot be read back. A finding with any path under a deny prefix is
// redacted as a whole, including its entity name and any filename in
// its reason or scenario text.
package redact

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"sort"
	"strings"

	"github.com/grokify/diskwise/detect"
	"github.com/grokify/diskwise/service"
)

// Redactor rewrites findings. The zero value redacts nothing; use New.
type Redactor struct {
	home string
	deny []string
}

// New returns a Redactor that abbreviates home to "~" and fully
// redacts anything under a deny prefix. A deny prefix starting with
// "~" is expanded against home; prefixes are matched on path-segment
// boundaries.
func New(home string, deny []string) *Redactor {
	r := &Redactor{home: filepath.Clean(home)}
	if home == "" {
		r.home = ""
	}
	for _, d := range deny {
		d = strings.TrimSpace(d)
		if d == "" {
			continue
		}
		if d == "~" || strings.HasPrefix(d, "~/") {
			d = filepath.Join(home, strings.TrimPrefix(d, "~"))
		}
		r.deny = append(r.deny, filepath.Clean(d))
	}
	return r
}

func under(path, root string) bool {
	return path == root || strings.HasPrefix(path, root+"/")
}

func (r *Redactor) denied(path string) bool {
	for _, d := range r.deny {
		if under(path, d) {
			return true
		}
	}
	return false
}

func token(path string) string {
	sum := sha256.Sum256([]byte(path))
	return "<redacted:" + hex.EncodeToString(sum[:4]) + ">"
}

// Path returns the redacted form of one path.
func (r *Redactor) Path(path string) string {
	if r.denied(path) {
		return token(path)
	}
	if r.home != "" && under(path, r.home) {
		return "~" + strings.TrimPrefix(path, r.home)
	}
	return path
}

// text scrubs free text: any deny prefix, then fragments of the
// finding's own redacted paths (full paths and base names), and finally
// abbreviates the home directory. Deny prefixes are scrubbed everywhere
// because a prefix is itself the sensitive name.
func (r *Redactor) text(s string, scrub []string) string {
	for _, d := range r.deny {
		s = strings.ReplaceAll(s, d, "<redacted>")
	}
	for _, f := range scrub {
		s = strings.ReplaceAll(s, f, "<redacted>")
	}
	if r.home != "" {
		s = strings.ReplaceAll(s, r.home, "~")
	}
	return s
}

// Finding returns a redacted copy of f.
func (r *Redactor) Finding(f detect.Finding) detect.Finding {
	var redacted bool
	for _, p := range append([]string{f.Path}, f.Paths...) {
		if r.denied(p) {
			redacted = true
		}
	}

	// Fragments whose appearance in free text would leak a redacted
	// path: each full path and its (non-trivial) base name. Longest
	// first so a full path is replaced before its own base name.
	var scrub []string
	if redacted {
		for _, p := range append([]string{f.Path}, f.Paths...) {
			if !r.denied(p) {
				continue
			}
			scrub = append(scrub, p)
			if b := filepath.Base(p); len(b) >= 3 {
				scrub = append(scrub, b)
			}
		}
		sort.Slice(scrub, func(i, j int) bool { return len(scrub[i]) > len(scrub[j]) })
	}

	out := f
	out.Path = r.Path(f.Path)
	out.Paths = r.paths(f.Paths)
	out.Reason = r.text(f.Reason, scrub)

	out.Entity = f.Entity
	if strings.HasPrefix(f.Entity.ID, "/") {
		out.Entity.ID = r.Path(f.Entity.ID)
	}
	if redacted {
		out.Entity.Name = token(f.Path)
		if !strings.HasPrefix(out.Entity.ID, "<redacted:") && strings.HasPrefix(f.Entity.ID, "/") {
			out.Entity.ID = token(f.Entity.ID)
		}
	}

	if f.Scenarios != nil {
		out.Scenarios = make([]detect.Scenario, len(f.Scenarios))
		for i, sc := range f.Scenarios {
			sc.Description = r.text(sc.Description, scrub)
			sc.Paths = r.paths(sc.Paths)
			out.Scenarios[i] = sc
		}
	}
	return out
}

func (r *Redactor) paths(in []string) []string {
	if in == nil {
		return nil
	}
	out := make([]string, len(in))
	for i, p := range in {
		out[i] = r.Path(p)
	}
	return out
}

// Opportunities returns redacted copies of opps.
func (r *Redactor) Opportunities(opps []service.Opportunity) []service.Opportunity {
	out := make([]service.Opportunity, len(opps))
	for i, o := range opps {
		out[i] = service.Opportunity{Finding: r.Finding(o.Finding), Paths: r.paths(o.Paths)}
	}
	return out
}

// Hotspots returns a redacted copy of rep.
func (r *Redactor) Hotspots(rep *service.HotspotsReport) *service.HotspotsReport {
	out := &service.HotspotsReport{Root: r.Path(rep.Root)}
	for _, f := range rep.Known {
		out.Known = append(out.Known, r.Finding(f))
	}
	for _, f := range rep.Unexplained {
		out.Unexplained = append(out.Unexplained, r.Finding(f))
	}
	return out
}

// Savings returns a redacted copy of s (only its root path is a path).
func (r *Redactor) Savings(s *service.SavingsByTier) *service.SavingsByTier {
	return &service.SavingsByTier{Path: r.Path(s.Path), Tiers: s.Tiers}
}

// Pairs returns redacted copies of pairs. A pair is redacted as a
// whole when its archive or directory is under a deny prefix.
func (r *Redactor) Pairs(pairs []detect.ArchivePair) []detect.ArchivePair {
	out := make([]detect.ArchivePair, len(pairs))
	for i, p := range pairs {
		scrub := []string(nil)
		if r.denied(p.Archive) || r.denied(p.Dir) {
			for _, x := range []string{p.Archive, p.Dir} {
				scrub = append(scrub, x)
				if b := filepath.Base(x); len(b) >= 3 {
					scrub = append(scrub, b)
				}
			}
			sort.Slice(scrub, func(i, j int) bool { return len(scrub[i]) > len(scrub[j]) })
		}
		p.Detail = r.text(p.Detail, scrub)
		p.Archive, p.Dir = r.Path(p.Archive), r.Path(p.Dir)
		out[i] = p
	}
	return out
}
