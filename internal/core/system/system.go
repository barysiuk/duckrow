// Package system defines the System abstraction for duckrow.
//
// A System represents an AI coding tool (OpenCode, Cursor, Claude Code, etc.).
// Each system knows its own paths, detection logic, config formats, and how to
// accept assets. Systems are self-contained Go structs — no JSON definition file.
package system

import (
	"errors"
	"fmt"
	"strings"

	"github.com/barysiuk/duckrow/internal/core/asset"
)

// ErrAlreadyExists is returned by Install when an asset is already present
// and Force is not set. Consumers should treat this as a skip, not a failure.
var ErrAlreadyExists = errors.New("already exists")

// System defines how an AI coding tool integrates with duckrow.
// Each system is a self-contained unit that knows its own paths,
// detection logic, config formats, and how to accept assets.
type System interface {
	// Identity
	Name() string        // machine name: "opencode", "cursor"
	DisplayName() string // human name: "OpenCode", "Cursor"

	// Detection
	IsInstalled() bool                       // globally installed on this machine
	IsActiveInFolder(folderPath string) bool // has config artifacts in this folder
	DetectionSignals() []string              // config files/dirs indicating active use

	// Asset support
	Supports(kind asset.Kind) bool
	SupportedKinds() []asset.Kind

	// Asset lifecycle — the system owns installation and removal
	Install(a asset.Asset, projectDir string, opts InstallOptions) error
	Remove(kind asset.Kind, name string, projectDir string) error

	// Scanning — find installed assets of a given kind in a project
	Scan(kind asset.Kind, projectDir string) ([]asset.InstalledAsset, error)

	// Paths
	AssetDir(kind asset.Kind, projectDir string) string

	// Classification
	IsUniversal() bool // shares .agents/skills/ directly
}

// InstallOptions for system-level installation.
type InstallOptions struct {
	Force bool
}

// --- Registry ---

var systems []System

// Register adds a system to the global registry.
func Register(s System) { systems = append(systems, s) }

// All returns all registered systems.
func All() []System { return systems }

// ByName returns the system with the given machine name, if registered.
func ByName(name string) (System, bool) {
	for _, s := range systems {
		if s.Name() == name {
			return s, true
		}
	}
	return nil, false
}

// Detect returns all globally installed systems.
func Detect() []System {
	var detected []System
	for _, s := range systems {
		if s.IsInstalled() {
			detected = append(detected, s)
		}
	}
	return detected
}

// DetectInFolder returns systems that are either globally installed or
// active in the given project folder.
func DetectInFolder(path string) []System {
	var detected []System
	for _, s := range systems {
		if s.IsActiveInFolder(path) || s.IsInstalled() {
			detected = append(detected, s)
		}
	}
	return detected
}

// ByNames resolves a list of system names to System values.
// Returns an error if any name is unknown.
func ByNames(names []string) ([]System, error) {
	result := make([]System, 0, len(names))
	for _, name := range names {
		s, ok := ByName(name)
		if !ok {
			var valid []string
			for _, sys := range systems {
				valid = append(valid, sys.Name())
			}
			return nil, fmt.Errorf("unknown system %q; available: %s",
				name, strings.Join(valid, ", "))
		}
		result = append(result, s)
	}
	return result, nil
}

// Supporting returns all systems that support the given asset kind.
func Supporting(kind asset.Kind) []System {
	var result []System
	for _, s := range systems {
		if s.Supports(kind) {
			result = append(result, s)
		}
	}
	return result
}

// Universal returns all universal systems (those sharing .agents/skills/).
func Universal() []System {
	var result []System
	for _, s := range systems {
		if s.IsUniversal() {
			result = append(result, s)
		}
	}
	return result
}

// NonUniversal returns all non-universal systems (those with their own skill dirs).
func NonUniversal() []System {
	var result []System
	for _, s := range systems {
		if !s.IsUniversal() {
			result = append(result, s)
		}
	}
	return result
}

// Names returns the machine names of the given systems.
func Names(systems []System) []string {
	names := make([]string, len(systems))
	for i, s := range systems {
		names[i] = s.Name()
	}
	return names
}

// DisplayNames returns the display names of the given systems.
func DisplayNames(systems []System) []string {
	names := make([]string, len(systems))
	for i, s := range systems {
		names[i] = s.DisplayName()
	}
	return names
}
