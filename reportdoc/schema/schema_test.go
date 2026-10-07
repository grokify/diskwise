package schema

import (
	"encoding/json"
	"sort"
	"testing"

	"github.com/invopop/jsonschema"

	"github.com/grokify/diskwise/reportdoc"
)

type defs map[string]struct {
	Properties map[string]json.RawMessage `json:"properties"`
	Required   []string                   `json:"required"`
}

type doc struct {
	Ref  string `json:"$ref"`
	Defs defs   `json:"$defs"`
}

func load(t *testing.T, data []byte) doc {
	t.Helper()
	var d doc
	if err := json.Unmarshal(data, &d); err != nil {
		t.Fatalf("not valid JSON: %v", err)
	}
	return d
}

func TestJSON_IsValidAndRootedAtDocument(t *testing.T) {
	if len(JSON) == 0 {
		t.Fatal("embedded schema is empty")
	}
	d := load(t, JSON)
	if d.Ref != "#/$defs/Document" {
		t.Errorf("schema $ref = %q, want #/$defs/Document", d.Ref)
	}
	for _, key := range []string{"schemaVersion", "root", "measuredAt", "tiers", "findings"} {
		if _, ok := d.Defs["Document"].Properties[key]; !ok {
			t.Errorf("Document is missing property %q", key)
		}
	}
}

// The embedded file is generated from the Go types. If someone edits
// reportdoc/types.go and forgets `go run reportdoc/schema/gen/main.go`,
// the property names (or which are required) drift. This regenerates the
// structure in memory and compares it with what is embedded.
func TestJSON_MatchesTheGoTypes(t *testing.T) {
	r := &jsonschema.Reflector{}
	raw, err := json.Marshal(r.Reflect(&reportdoc.Document{}))
	if err != nil {
		t.Fatal(err)
	}
	want, got := load(t, raw), load(t, JSON)

	names := func(d defs) []string {
		var out []string
		for def, v := range d {
			for p := range v.Properties {
				out = append(out, def+"."+p)
			}
			for _, req := range v.Required {
				out = append(out, def+" requires "+req)
			}
		}
		sort.Strings(out)
		return out
	}
	w, g := names(want.Defs), names(got.Defs)
	if len(w) != len(g) {
		t.Fatalf("embedded schema is stale: %d entries vs %d from the types. Run `go run reportdoc/schema/gen/main.go`.", len(g), len(w))
	}
	for i := range w {
		if w[i] != g[i] {
			t.Fatalf("embedded schema is stale at %q vs %q. Run `go run reportdoc/schema/gen/main.go`.", g[i], w[i])
		}
	}
}
