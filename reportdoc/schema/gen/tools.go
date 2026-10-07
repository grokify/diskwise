//go:build tools

// This file exists only so `go mod tidy` keeps invopop/jsonschema in
// go.mod — main.go in this directory is //go:build ignore, so without
// this blank import the generator's own dependency would be silently
// stripped the next time tidy runs.
package gen

import _ "github.com/invopop/jsonschema"
