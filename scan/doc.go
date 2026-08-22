// Package scan implements the progressive, two-track filesystem walker:
// a breadth-first depth-limited pass for fast subtree totals, and a
// prioritized descent guided by the knowledge registry and running byte
// counts.
package scan
