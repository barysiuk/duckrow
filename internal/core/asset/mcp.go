package asset

import (
	"encoding/json"
	"fmt"
	"sort"
)

// MCPMeta holds MCP-specific metadata.
type MCPMeta struct {
	Command   string   `json:"command,omitempty"`
	Args      []string `json:"args,omitempty"`
	Env       []string `json:"env,omitempty"`
	URL       string   `json:"url,omitempty"`
	Transport string   `json:"type,omitempty"` // "http", "sse", "streamable-http"
}

// AssetKind implements Meta.
func (m MCPMeta) AssetKind() Kind { return KindMCP }

// IsStdio returns true if this MCP uses stdio transport (has a command).
func (m MCPMeta) IsStdio() bool { return m.Command != "" }

// IsRemote returns true if this MCP uses a remote transport (has a URL).
func (m MCPMeta) IsRemote() bool { return m.URL != "" }

// MCPHandler handles MCP server configuration assets.
// MCPs are config-only — they have no on-disk discovery (no equivalent of SKILL.md).
// They come exclusively from registry manifests.
type MCPHandler struct{}

func (h *MCPHandler) Kind() Kind          { return KindMCP }
func (h *MCPHandler) DisplayName() string { return "MCP Server" }

// Discover returns nil — MCPs are not discoverable on disk.
func (h *MCPHandler) Discover(_ string, _ DiscoverOptions) ([]Asset, error) {
	return nil, nil
}

// Parse returns an error — MCPs are config-only, not file-based.
func (h *MCPHandler) Parse(_ string) (Meta, error) {
	return nil, fmt.Errorf("MCP assets are config-only, not file-based")
}

// Validate checks that an MCP asset is well-formed.
func (h *MCPHandler) Validate(a Asset) error {
	meta, ok := a.Meta.(MCPMeta)
	if !ok {
		return fmt.Errorf("expected MCPMeta, got %T", a.Meta)
	}
	if a.Name == "" {
		return fmt.Errorf("MCP name is required")
	}
	if meta.Command == "" && meta.URL == "" {
		return fmt.Errorf("MCP must have either command (stdio) or url (remote)")
	}
	if meta.Command != "" && meta.URL != "" {
		return fmt.Errorf("MCP cannot have both command and url")
	}
	if meta.URL != "" && meta.Transport == "" {
		return fmt.Errorf("MCP with url must specify transport type")
	}
	return nil
}

// mcpManifestEntry mirrors the JSON structure for an MCP in a v2 registry manifest.
type mcpManifestEntry struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Command     string   `json:"command,omitempty"`
	Args        []string `json:"args,omitempty"`
	Env         []string `json:"env,omitempty"`
	URL         string   `json:"url,omitempty"`
	Type        string   `json:"type,omitempty"`
}

// ParseManifestEntries unmarshals MCP entries from a registry manifest.
func (h *MCPHandler) ParseManifestEntries(raw json.RawMessage) ([]RegistryEntry, error) {
	var entries []mcpManifestEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("unmarshaling MCP entries: %w", err)
	}
	result := make([]RegistryEntry, len(entries))
	for i, e := range entries {
		result[i] = RegistryEntry{
			Name:        e.Name,
			Description: e.Description,
			Meta: MCPMeta{
				Command:   e.Command,
				Args:      e.Args,
				Env:       e.Env,
				URL:       e.URL,
				Transport: e.Type,
			},
		}
	}
	return result, nil
}

// LockData produces a LockedAsset from an MCP installation.
func (h *MCPHandler) LockData(a Asset, info InstallInfo) LockedAsset {
	meta, _ := a.Meta.(MCPMeta)
	data := map[string]any{
		"registry":   info.Registry,
		"configHash": computeConfigHash(meta),
	}
	if envKeys := extractRequiredEnv(meta.Env); len(envKeys) > 0 {
		data["requiredEnv"] = envKeys
	}
	return LockedAsset{
		Kind: KindMCP,
		Name: a.Name,
		Data: data,
	}
}

// computeConfigHash produces a deterministic hash of the MCP config for
// change detection in the lock file.
func computeConfigHash(meta MCPMeta) string {
	// Use a canonical JSON encoding of the config fields.
	canonical := struct {
		Command   string   `json:"command,omitempty"`
		Args      []string `json:"args,omitempty"`
		Env       []string `json:"env,omitempty"`
		URL       string   `json:"url,omitempty"`
		Transport string   `json:"type,omitempty"`
	}{
		Command:   meta.Command,
		Args:      meta.Args,
		Env:       meta.Env,
		URL:       meta.URL,
		Transport: meta.Transport,
	}
	data, _ := json.Marshal(canonical)

	// Simple hash using crypto/sha256.
	// Import inline to avoid polluting the package imports when not needed.
	// We use a package-level import instead.
	return hashBytes(data)
}

// extractRequiredEnv returns the env var names from the Env list.
// Returns a sorted, deduplicated copy.
func extractRequiredEnv(env []string) []string {
	if len(env) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(env))
	var keys []string
	for _, k := range env {
		if !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys
}

func init() { Register(&MCPHandler{}) }
