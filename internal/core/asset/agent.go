package asset

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// AgentMeta holds agent-specific metadata parsed from frontmatter.
type AgentMeta struct{}

// AssetKind implements Meta.
func (m AgentMeta) AssetKind() Kind { return KindAgent }

// AgentData is the raw parsed agent data: frontmatter map + Markdown body.
// It is attached to Asset.Meta via AgentDataMeta for transport through the
// handler/system pipeline. The frontmatter is opaque — duckrow does not
// interpret field semantics.
type AgentData struct {
	Frontmatter map[string]any // all YAML frontmatter fields
	Body        string         // Markdown body (system prompt)
}

// AgentDataMeta wraps AgentData to satisfy the Meta interface while
// carrying the full parsed content through the install pipeline.
type AgentDataMeta struct {
	AgentMeta
	Data *AgentData
}

// excludedAgentFiles are filenames that should never be treated as agents.
var excludedAgentFiles = map[string]bool{
	"SKILL.md":  true,
	"AGENTS.md": true,
	"README.md": true,
	"CLAUDE.md": true,
	"GEMINI.md": true,
	"codex.md":  true,
}

// AgentHandler discovers and validates agent assets via Markdown files
// with YAML frontmatter containing `name` and `description` fields.
type AgentHandler struct{}

func (h *AgentHandler) Kind() Kind          { return KindAgent }
func (h *AgentHandler) DisplayName() string { return "Agent" }

// Discover walks basePath looking for .md files with agent frontmatter
// (name + description fields) and returns an Asset for each one found.
func (h *AgentHandler) Discover(basePath string, opts DiscoverOptions) ([]Asset, error) {
	searchPath := basePath
	if opts.SubPath != "" {
		searchPath = filepath.Join(basePath, opts.SubPath)
	}

	var assets []Asset
	seen := make(map[string]bool)

	err := filepath.WalkDir(searchPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}

		// Skip hidden directories (except known agent locations).
		if d.IsDir() && path != searchPath {
			name := d.Name()
			if strings.HasPrefix(name, ".") {
				switch name {
				case ".agents", ".claude", ".opencode", ".github", ".gemini":
					// Allow traversal into these directories.
				default:
					return filepath.SkipDir
				}
			}
			switch name {
			case "node_modules", "vendor", "__pycache__":
				return filepath.SkipDir
			}
		}

		// Only process .md files.
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}

		// Skip excluded filenames.
		if excludedAgentFiles[d.Name()] {
			return nil
		}

		data, err := ParseAgentFile(path)
		if err != nil {
			return nil // skip unparseable files
		}

		name, _ := data.Frontmatter["name"].(string)
		description, _ := data.Frontmatter["description"].(string)

		// Must have both name and description to qualify as an agent.
		if name == "" || description == "" {
			return nil
		}

		if seen[name] {
			return nil
		}
		seen[name] = true

		// Apply name filter.
		if opts.NameFilter != "" && name != opts.NameFilter {
			return nil
		}

		assets = append(assets, Asset{
			Kind:         KindAgent,
			Name:         name,
			Description:  description,
			PreparedPath: path, // path to the .md file itself
			Meta:         AgentDataMeta{Data: data},
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking %s: %w", searchPath, err)
	}

	return assets, nil
}

// Parse reads agent frontmatter from a .md file at the given path.
func (h *AgentHandler) Parse(path string) (Meta, error) {
	// If path is a directory, we can't parse it as an agent file.
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		// Look for any .md file in the directory — agents are single files.
		return nil, fmt.Errorf("agents are single .md files, not directories")
	}

	data, err := ParseAgentFile(path)
	if err != nil {
		return nil, err
	}

	return AgentDataMeta{Data: data}, nil
}

// Validate checks that an agent asset is well-formed for installation.
func (h *AgentHandler) Validate(a Asset) error {
	if a.Name == "" {
		return fmt.Errorf("agent name is required")
	}

	meta, ok := a.Meta.(AgentDataMeta)
	if !ok {
		// Allow plain AgentMeta (e.g., from registry entries without data).
		if _, ok2 := a.Meta.(AgentMeta); ok2 {
			return nil
		}
		return fmt.Errorf("expected AgentDataMeta or AgentMeta, got %T", a.Meta)
	}

	desc, _ := meta.Data.Frontmatter["description"].(string)
	if desc == "" {
		return fmt.Errorf("agent %q missing description in frontmatter", a.Name)
	}

	if strings.TrimSpace(meta.Data.Body) == "" {
		return fmt.Errorf("agent %q has empty body (no system prompt)", a.Name)
	}

	return nil
}

// agentManifestEntry mirrors the JSON structure for an agent in a registry manifest.
type agentManifestEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Source      string `json:"source"`
	Commit      string `json:"commit,omitempty"`
}

// ParseManifestEntries unmarshals agent entries from a registry manifest.
func (h *AgentHandler) ParseManifestEntries(raw json.RawMessage) ([]RegistryEntry, error) {
	var entries []agentManifestEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("unmarshaling agent entries: %w", err)
	}
	result := make([]RegistryEntry, len(entries))
	for i, e := range entries {
		result[i] = RegistryEntry{
			Name:        e.Name,
			Description: e.Description,
			Source:      e.Source,
			Commit:      e.Commit,
			Meta:        AgentMeta{},
		}
	}
	return result, nil
}

// LockData produces a LockedAsset from an agent installation.
// Agents use the same thin format as skills: source + commit only.
func (h *AgentHandler) LockData(a Asset, info InstallInfo) LockedAsset {
	return LockedAsset{
		Kind:   KindAgent,
		Name:   a.Name,
		Source: a.Source,
		Commit: info.Commit,
		Ref:    info.Ref,
	}
}

func init() { Register(&AgentHandler{}) }
