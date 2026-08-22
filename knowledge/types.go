package knowledge

import (
	"fmt"

	"github.com/grokify/diskwise/entity"
	"github.com/grokify/diskwise/policy"
)

// Semantics describes how a known location's bytes should be
// interpreted, for explanation purposes — e.g. a sparse VM disk whose
// allocated size (not logical size) reflects real consumption.
type Semantics string

const (
	SemanticsRegeneratableCache Semantics = "regeneratable_cache"
	SemanticsSparseVMDisk       Semantics = "sparse_vm_disk"
	SemanticsInstalledSoftware  Semantics = "installed_software"
	SemanticsUserData           Semantics = "user_data"
	SemanticsAlreadyDeleted     Semantics = "already_deleted"
)

// KnownLocation is one entry in the registry of known heavy macOS
// storage locations (TRD §4.1): an app/tool adapter that knows where
// its data lives and what those bytes mean, checked before the
// generic scanner reaches that subtree.
type KnownLocation struct {
	ID          string
	Description string
	// Probes gate relevance: the location applies if any probe
	// succeeds, or unconditionally if there are none.
	Probes []Probe
	// DataPaths are where the heavy bytes live; a leading "~" is
	// expanded to the current user's home directory.
	DataPaths          []string
	Entity             entity.Kind
	Semantics          Semantics
	DefaultActionClass policy.ActionClass
}

// Relevant reports whether loc applies to this machine.
func (loc KnownLocation) Relevant() (bool, error) {
	if len(loc.Probes) == 0 {
		return true, nil
	}
	for _, p := range loc.Probes {
		ok, err := p()
		if err != nil {
			return false, fmt.Errorf("knowledge: probe %s: %w", loc.ID, err)
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}

// ExpandedDataPaths returns loc.DataPaths with a leading "~" resolved
// to the current user's home directory.
func (loc KnownLocation) ExpandedDataPaths() ([]string, error) {
	out := make([]string, len(loc.DataPaths))
	for i, p := range loc.DataPaths {
		expanded, err := expandPath(p)
		if err != nil {
			return nil, fmt.Errorf("knowledge: %s: %w", loc.ID, err)
		}
		out[i] = expanded
	}
	return out, nil
}

// Registry is an ordered set of known locations.
type Registry []KnownLocation

// Relevant returns the subset of the registry applicable to this
// machine, in registry order.
func (r Registry) Relevant() (Registry, error) {
	var out Registry
	for _, loc := range r {
		ok, err := loc.Relevant()
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, loc)
		}
	}
	return out, nil
}
