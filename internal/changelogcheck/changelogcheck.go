// Package changelogcheck verifies that every commit hash CHANGELOG.json
// references is reachable from a given ref. A changelog link to a commit
// that is not on the release branch is dead, and nothing else notices it:
// CI is green and the tag is already pushed. Run the check before tagging.
package changelogcheck

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
)

var hashRE = regexp.MustCompile(`^[0-9a-f]{7,40}$`)

// CommitRefs returns the distinct commit hashes referenced by a
// structured changelog: every "commit" field at any depth (release
// commits and per-entry commits alike), sorted.
func CommitRefs(changelogJSON []byte) ([]string, error) {
	var doc any
	if err := json.Unmarshal(changelogJSON, &doc); err != nil {
		return nil, fmt.Errorf("changelogcheck: parse changelog: %w", err)
	}
	seen := map[string]bool{}
	var walk func(v any)
	walk = func(v any) {
		switch t := v.(type) {
		case map[string]any:
			for k, val := range t {
				if s, ok := val.(string); ok && k == "commit" {
					seen[s] = true
					continue
				}
				walk(val)
			}
		case []any:
			for _, e := range t {
				walk(e)
			}
		}
	}
	walk(doc)

	out := make([]string, 0, len(seen))
	for h := range seen {
		out = append(out, h)
	}
	sort.Strings(out)
	return out, nil
}

// Unreachable returns the hashes in changelogJSON that are not
// ancestors of ref (or ref itself) in the git repository at repoDir.
// A malformed hash, or one git does not know at all, counts as
// unreachable. It returns an error only if git itself cannot be run.
func Unreachable(repoDir, ref string, changelogJSON []byte) ([]string, error) {
	hashes, err := CommitRefs(changelogJSON)
	if err != nil {
		return nil, err
	}
	var bad []string
	for _, h := range hashes {
		if !hashRE.MatchString(h) {
			bad = append(bad, h)
			continue
		}
		//nolint:gosec // G204: h matched ^[0-9a-f]{7,40}$ and ref is chosen by the caller, never a shell
		cmd := exec.Command("git", "-C", repoDir, "merge-base", "--is-ancestor", h, ref)
		err := cmd.Run()
		var exitErr *exec.ExitError
		switch {
		case err == nil:
		case errors.As(err, &exitErr):
			bad = append(bad, h) // exit 1: not an ancestor; 128: unknown object
		default:
			return nil, fmt.Errorf("changelogcheck: run git: %w", err)
		}
	}
	return bad, nil
}
