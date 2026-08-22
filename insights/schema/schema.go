// Package schema embeds the JSON Schema generated from
// insights.Report, so a caller (CLI command, MCP tool) can hand an
// LLM agent the exact contract it must fill out without shelling out
// to regenerate it. Regenerate via
// `go run insights/schema/gen/main.go` after changing insights/types.go.
package schema

import _ "embed"

//go:embed insights.schema.json
var JSON []byte
