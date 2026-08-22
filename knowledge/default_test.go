package knowledge

import "testing"

// TestDefault_WellFormed guards the registry's basic invariants so a
// future entry can't silently ship broken: every ID unique, every
// entry has at least one data path, and every data path expands
// without error.
func TestDefault_WellFormed(t *testing.T) {
	seen := make(map[string]bool)
	for _, loc := range Default {
		if loc.ID == "" {
			t.Errorf("entry with empty ID (Description=%q)", loc.Description)
			continue
		}
		if seen[loc.ID] {
			t.Errorf("duplicate ID %q", loc.ID)
		}
		seen[loc.ID] = true

		if loc.Description == "" {
			t.Errorf("%s: empty Description", loc.ID)
		}
		if len(loc.DataPaths) == 0 {
			t.Errorf("%s: no DataPaths", loc.ID)
		}
		if loc.DefaultActionClass == "" {
			t.Errorf("%s: no DefaultActionClass", loc.ID)
		}

		paths, err := loc.ExpandedDataPaths()
		if err != nil {
			t.Errorf("%s: ExpandedDataPaths: %v", loc.ID, err)
			continue
		}
		for _, p := range paths {
			if p == "" {
				t.Errorf("%s: expanded to an empty path", loc.ID)
			}
		}
	}
}

func TestDefault_RelevantDoesNotError(t *testing.T) {
	if _, err := Default.Relevant(); err != nil {
		t.Fatal(err)
	}
}
