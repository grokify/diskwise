package main

import "testing"

func TestParseSize(t *testing.T) {
	tests := []struct {
		in   string
		want int64
	}{
		{"", 0},
		{"1024", 1024},
		{"1k", 1 << 10},
		{"1kb", 1 << 10},
		{"1KiB", 1 << 10},
		{"50gib", 50 << 30},
		{"50GB", 50 << 30},
		{"1.5mib", 3 << 19},
		{"2tib", 2 << 40},
	}
	for _, tt := range tests {
		got, err := parseSize(tt.in)
		if err != nil || got != tt.want {
			t.Errorf("parseSize(%q) = %d, %v; want %d", tt.in, got, err, tt.want)
		}
	}
	if _, err := parseSize("lots"); err == nil {
		t.Error("parseSize(\"lots\") should fail")
	}
}
