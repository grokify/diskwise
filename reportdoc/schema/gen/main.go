//go:build ignore

// Command gen writes reportdoc/schema/reportdoc.schema.json from
// reportdoc.Document by reflection. reportdoc.Document (Go structs) is
// the source of truth — this schema is generated, never hand-edited.
// Run via `go run reportdoc/schema/gen/main.go` from the module root.
package main

import (
	"encoding/json"
	"log"
	"os"

	"github.com/invopop/jsonschema"

	"github.com/grokify/diskwise/reportdoc"
)

func main() {
	r := &jsonschema.Reflector{
		DoNotReference: false,
	}
	if err := r.AddGoComments("github.com/grokify/diskwise/reportdoc", "./reportdoc"); err != nil {
		log.Fatalf("gen: add comments: %v", err)
	}

	schema := r.Reflect(&reportdoc.Document{})
	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		log.Fatalf("gen: marshal: %v", err)
	}
	data = append(data, '\n')

	if err := os.WriteFile("reportdoc/schema/reportdoc.schema.json", data, 0o644); err != nil {
		log.Fatalf("gen: write: %v", err)
	}
}
