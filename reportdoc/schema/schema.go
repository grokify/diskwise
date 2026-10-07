// Package schema embeds the JSON Schema generated from
// reportdoc.Document, so a caller can validate a report document or
// hand its exact contract to another tool without regenerating it.
// Regenerate via `go run reportdoc/schema/gen/main.go` after changing
// reportdoc/types.go.
package schema

import _ "embed"

//go:embed reportdoc.schema.json
var JSON []byte
