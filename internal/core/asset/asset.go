// Package asset defines the asset kind abstraction for duckrow.
//
// An Asset is a system-agnostic managed unit (skill, MCP server config, rule,
// etc.). Each kind registers a Handler that knows how to discover, parse, and
// validate assets of that kind. Handlers do NOT know about systems.
package asset

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
)

// Kind identifies an asset type.
type Kind string

const (
	KindSkill Kind = "skill"
	KindMCP   Kind = "mcp"
)

// Asset is the system-agnostic envelope describing something to install.
// It is produced by an asset Handler and consumed by a System.
type Asset struct {
	Kind         Kind
	Name         string
	Description  string
	Source       string // origin URL/identifier
	PreparedPath string // local path to prepared content (e.g., cloned repo dir)
	Meta         Meta   // kind-specific typed metadata
}

// Meta is the interface for kind-specific metadata.
// Each asset kind defines its own struct implementing this.
type Meta interface {
	AssetKind() Kind
}

// Handler defines how a particular asset kind is discovered, parsed,
// and validated. Handlers do NOT know about systems.
type Handler interface {
	// Identity
	Kind() Kind
	DisplayName() string // Human-readable: "Skill", "MCP Server", "Rule"

	// Discovery: find assets of this kind in a cloned repository.
	Discover(basePath string, opts DiscoverOptions) ([]Asset, error)

	// Parsing: read metadata from an on-disk asset at the given path.
	Parse(path string) (Meta, error)

	// Validation: check that an asset is well-formed before installation.
	Validate(a Asset) error

	// Registry: unmarshal kind-specific entries from a manifest's raw JSON.
	ParseManifestEntries(raw json.RawMessage) ([]RegistryEntry, error)

	// Lock file: produce lock data from an installed asset.
	LockData(a Asset, info InstallInfo) LockedAsset
}

// DiscoverOptions controls discovery behavior.
type DiscoverOptions struct {
	SubPath         string
	IncludeInternal bool
	NameFilter      string // e.g., @skill-name syntax
}

// RegistryEntry is a parsed entry from a registry manifest for a given kind.
type RegistryEntry struct {
	Name        string
	Description string
	Source      string
	Commit      string // optional pinned commit
	Meta        Meta
}

// InstallInfo carries context from the installation process, used by
// the handler to produce lock file data.
type InstallInfo struct {
	Commit      string
	Ref         string
	Registry    string
	SystemNames []string
}

// LockedAsset is the kind-agnostic lock file representation of an installed asset.
type LockedAsset struct {
	Kind   Kind           `json:"kind"`
	Name   string         `json:"name"`
	Source string         `json:"source,omitempty"`
	Commit string         `json:"commit,omitempty"`
	Ref    string         `json:"ref,omitempty"`
	Data   map[string]any `json:"data,omitempty"` // kind-specific lock fields
}

// InstalledAsset represents an asset found on disk in a project folder.
type InstalledAsset struct {
	Kind        Kind
	Name        string
	Description string
	Author      string // only set for file-based assets with metadata
	Path        string // on-disk location
	Meta        Meta
	SystemName  string // which system owns this instance ("" = canonical/shared)
}

// --- Registry ---

var handlers = map[Kind]Handler{}

// Register adds a handler for the given asset kind.
func Register(h Handler) { handlers[h.Kind()] = h }

// Get returns the handler for the given kind, if registered.
func Get(k Kind) (Handler, bool) { h, ok := handlers[k]; return h, ok }

// All returns all registered handlers.
func All() map[Kind]Handler { return handlers }

// Kinds returns all registered asset kinds in a stable order.
func Kinds() []Kind {
	// Return in a deterministic order: skill first, then mcp, then others.
	var known, other []Kind
	for k := range handlers {
		switch k {
		case KindSkill, KindMCP:
			known = append(known, k)
		default:
			other = append(other, k)
		}
	}
	// Sort known: skill before mcp
	result := make([]Kind, 0, len(handlers))
	if _, ok := handlers[KindSkill]; ok {
		result = append(result, KindSkill)
	}
	if _, ok := handlers[KindMCP]; ok {
		result = append(result, KindMCP)
	}
	// Append any other kinds (future extensibility)
	_ = known // suppress unused
	result = append(result, other...)
	return result
}

// hashBytes returns a "sha256:<hex>" hash string for the given data.
func hashBytes(data []byte) string {
	h := sha256.Sum256(data)
	return fmt.Sprintf("sha256:%x", h)
}
