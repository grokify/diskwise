package changelogcheck

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// git runs a git command in dir and returns trimmed stdout.
func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	//nolint:gosec // G204: test helper; arguments come from literals in this file
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestCommitRefs_FindsEveryCommitFieldAtAnyDepth(t *testing.T) {
	doc := `{
	  "releases": [
	    {"version": "v1", "commit": "aaaaaaa",
	     "added": [{"description": "x", "commit": "bbbbbbb"}, {"description": "no commit"}],
	     "fixed": [{"description": "y", "commit": "bbbbbbb"}]},
	    {"version": "v0", "commit": "ccccccc"}
	  ]}`
	got, err := CommitRefs([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, ",") != "aaaaaaa,bbbbbbb,ccccccc" {
		t.Errorf("CommitRefs = %v, want the three distinct hashes sorted", got)
	}
	if _, err := CommitRefs([]byte("{not json")); err == nil {
		t.Error("invalid JSON should be an error")
	}
}

// The failure this package exists for: a hash that was once on the
// branch but is no longer reachable from it (history was rewritten, or
// the commit lived on a deleted branch).
func TestUnreachable_FlagsHashesOffTheBranch(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "-q", "-b", "main")
	write := func(name string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("a")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-q", "-m", "one")
	onBranch := git(t, dir, "rev-parse", "--short=7", "HEAD")

	git(t, dir, "checkout", "-q", "-b", "gone")
	write("b")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-q", "-m", "on a branch we delete")
	orphan := git(t, dir, "rev-parse", "--short=7", "HEAD")
	git(t, dir, "checkout", "-q", "main")
	git(t, dir, "branch", "-q", "-D", "gone")

	changelog := `{"releases":[{"version":"v1","commit":"` + onBranch + `",
	  "added":[{"description":"d","commit":"` + orphan + `"},
	           {"description":"e","commit":"deadbee"},
	           {"description":"f","commit":"not-a-hash"}]}]}`
	bad, err := Unreachable(dir, "main", []byte(changelog))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{orphan: true, "deadbee": true, "not-a-hash": true}
	if len(bad) != len(want) {
		t.Fatalf("Unreachable = %v, want exactly %v", bad, want)
	}
	for _, h := range bad {
		if !want[h] {
			t.Errorf("unexpected unreachable hash %q", h)
		}
	}

	ok, err := Unreachable(dir, "main", []byte(`{"releases":[{"version":"v1","commit":"`+onBranch+`"}]}`))
	if err != nil || len(ok) != 0 {
		t.Errorf("a reachable hash must pass: got %v, %v", ok, err)
	}
}

// TestRepoChangelog is the real check: run it before tagging a release.
// It needs full history, so it skips on shallow checkouts (the dedicated
// changelog workflow fetches everything).
func TestRepoChangelog(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := git(t, ".", "rev-parse", "--show-toplevel")
	if git(t, root, "rev-parse", "--is-shallow-repository") == "true" {
		t.Skip("shallow clone: older commits are absent, so reachability cannot be judged")
	}
	data, err := os.ReadFile(filepath.Join(root, "CHANGELOG.json")) //nolint:gosec // G304: fixed file name under the repo root
	if err != nil {
		t.Fatal(err)
	}
	bad, err := Unreachable(root, "HEAD", data)
	if err != nil {
		t.Fatal(err)
	}
	if len(bad) > 0 {
		t.Errorf("CHANGELOG.json references %d commit(s) not reachable from HEAD, so their links are dead: %v", len(bad), bad)
	}
}
