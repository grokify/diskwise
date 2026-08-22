package schema

import (
	"encoding/json"
	"testing"
)

func TestJSON_IsValid(t *testing.T) {
	if len(JSON) == 0 {
		t.Fatal("embedded schema is empty")
	}
	var v map[string]any
	if err := json.Unmarshal(JSON, &v); err != nil {
		t.Fatalf("embedded schema is not valid JSON: %v", err)
	}
	if v["$ref"] != "#/$defs/Report" {
		t.Errorf(`schema $ref = %v, want "#/$defs/Report"`, v["$ref"])
	}
}
