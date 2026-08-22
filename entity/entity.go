package entity

// Entity is a semantic storage entity that one or more findings
// (detect package) attach to — what a group of scanned paths actually
// represents.
type Entity struct {
	ID         string
	Kind       Kind
	Name       string
	Detector   string // which detector produced this entity, e.g. "known-location:docker-desktop"
	Confidence float64
}
