//go:build ignore

// Command gen writes insights/schema/insights.schema.json from
// insights.Report by reflection. insights.Report (Go structs) is the
// source of truth — this schema is generated, never hand-edited. Run
// via `go run insights/schema/gen/main.go` from the module root.
package main

import (
	"encoding/json"
	"log"
	"os"

	"github.com/invopop/jsonschema"

	"github.com/grokify/diskwise/insights"
)

func main() {
	r := &jsonschema.Reflector{
		DoNotReference: false,
	}
	if err := r.AddGoComments("github.com/grokify/diskwise/insights", "."); err != nil {
		log.Fatalf("gen: add comments: %v", err)
	}

	schema := r.Reflect(&insights.Report{})
	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		log.Fatalf("gen: marshal: %v", err)
	}
	data = append(data, '\n')

	if err := os.WriteFile("insights/schema/insights.schema.json", data, 0o644); err != nil {
		log.Fatalf("gen: write: %v", err)
	}
}
