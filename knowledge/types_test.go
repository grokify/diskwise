package knowledge

import "testing"

func TestKnownLocation_Relevant(t *testing.T) {
	always := KnownLocation{ID: "always"}
	ok, err := always.Relevant()
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("a location with no probes should always be relevant")
	}

	anyTrue := KnownLocation{
		ID: "any-true",
		Probes: []Probe{
			func() (bool, error) { return false, nil },
			func() (bool, error) { return true, nil },
		},
	}
	ok, err = anyTrue.Relevant()
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("relevance should be true if any probe succeeds")
	}

	allFalse := KnownLocation{
		ID: "all-false",
		Probes: []Probe{
			func() (bool, error) { return false, nil },
			func() (bool, error) { return false, nil },
		},
	}
	ok, err = allFalse.Relevant()
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("relevance should be false if every probe fails")
	}
}

func TestKnownLocation_ExpandedDataPaths(t *testing.T) {
	loc := KnownLocation{ID: "x", DataPaths: []string{"~/foo", "/bar"}}
	paths, err := loc.ExpandedDataPaths()
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 {
		t.Fatalf("got %d paths, want 2", len(paths))
	}
	if paths[1] != "/bar" {
		t.Errorf("paths[1] = %s, want /bar", paths[1])
	}
}

func TestRegistry_Relevant(t *testing.T) {
	reg := Registry{
		{ID: "a", Probes: []Probe{func() (bool, error) { return true, nil }}},
		{ID: "b", Probes: []Probe{func() (bool, error) { return false, nil }}},
		{ID: "c"}, // always relevant
	}
	relevant, err := reg.Relevant()
	if err != nil {
		t.Fatal(err)
	}
	if len(relevant) != 2 {
		t.Fatalf("got %d relevant locations, want 2", len(relevant))
	}
	if relevant[0].ID != "a" || relevant[1].ID != "c" {
		t.Errorf("relevant IDs = [%s, %s], want [a, c]", relevant[0].ID, relevant[1].ID)
	}
}
