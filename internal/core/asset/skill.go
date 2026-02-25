package asset

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const skillFileName = "SKILL.md"

// SkillMeta holds skill-specific metadata parsed from SKILL.md frontmatter.
type SkillMeta struct {
	Author   string `yaml:"author,omitempty"`
	Version  string `yaml:"version,omitempty"`
	Internal bool   `yaml:"internal,omitempty"`
	ArgHint  string `yaml:"argument-hint,omitempty"`
	License  string `yaml:"license,omitempty"`
}

// AssetKind implements Meta.
func (m SkillMeta) AssetKind() Kind { return KindSkill }

// skillFrontmatter is the raw YAML structure in a SKILL.md file.
// It mirrors the existing core.SkillMetadata layout for parsing compatibility.
type skillFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	License     string `yaml:"license,omitempty"`
	Metadata    struct {
		Author   string `yaml:"author,omitempty"`
		Version  string `yaml:"version,omitempty"`
		Internal bool   `yaml:"internal,omitempty"`
		ArgHint  string `yaml:"argument-hint,omitempty"`
	} `yaml:"metadata,omitempty"`
}

// SkillHandler discovers and validates skill assets via SKILL.md files.
type SkillHandler struct{}

func (h *SkillHandler) Kind() Kind          { return KindSkill }
func (h *SkillHandler) DisplayName() string { return "Skill" }

// Discover walks basePath looking for SKILL.md files and returns an Asset for
// each one found, applying the options filters.
func (h *SkillHandler) Discover(basePath string, opts DiscoverOptions) ([]Asset, error) {
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

		// Skip hidden directories (except .agents which is a known skills location).
		if d.IsDir() && path != searchPath {
			name := d.Name()
			if strings.HasPrefix(name, ".") && name != ".agents" {
				return filepath.SkipDir
			}
			switch name {
			case "node_modules", "vendor", "__pycache__":
				return filepath.SkipDir
			}
		}

		if d.IsDir() || d.Name() != skillFileName {
			return nil
		}

		skillDir := filepath.Dir(path)
		if seen[skillDir] {
			return nil
		}
		seen[skillDir] = true

		fm, err := parseSkillFrontmatter(path)
		if err != nil {
			return nil // skip unparseable skills
		}

		meta := SkillMeta{
			Author:   fm.Metadata.Author,
			Version:  fm.Metadata.Version,
			Internal: fm.Metadata.Internal,
			ArgHint:  fm.Metadata.ArgHint,
			License:  fm.License,
		}

		// Apply internal filter.
		if !opts.IncludeInternal && meta.Internal {
			return nil
		}

		// Apply name filter.
		if opts.NameFilter != "" && fm.Name != opts.NameFilter {
			return nil
		}

		assets = append(assets, Asset{
			Kind:         KindSkill,
			Name:         fm.Name,
			Description:  fm.Description,
			PreparedPath: skillDir,
			Meta:         meta,
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking %s: %w", searchPath, err)
	}

	return assets, nil
}

// Parse reads SKILL.md frontmatter at the given directory path.
func (h *SkillHandler) Parse(path string) (Meta, error) {
	skillMdPath := filepath.Join(path, skillFileName)
	fm, err := parseSkillFrontmatter(skillMdPath)
	if err != nil {
		return nil, err
	}
	return SkillMeta{
		Author:   fm.Metadata.Author,
		Version:  fm.Metadata.Version,
		Internal: fm.Metadata.Internal,
		ArgHint:  fm.Metadata.ArgHint,
		License:  fm.License,
	}, nil
}

// Validate checks that an asset is well-formed for installation.
func (h *SkillHandler) Validate(a Asset) error {
	if a.Name == "" {
		return fmt.Errorf("skill name is required")
	}
	if _, ok := a.Meta.(SkillMeta); !ok {
		return fmt.Errorf("expected SkillMeta, got %T", a.Meta)
	}
	return nil
}

// skillManifestEntry mirrors the JSON structure for a skill in a v2 registry manifest.
type skillManifestEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Source      string `json:"source"`
	Commit      string `json:"commit,omitempty"`
}

// ParseManifestEntries unmarshals skill entries from a registry manifest.
func (h *SkillHandler) ParseManifestEntries(raw json.RawMessage) ([]RegistryEntry, error) {
	var entries []skillManifestEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("unmarshaling skill entries: %w", err)
	}
	result := make([]RegistryEntry, len(entries))
	for i, e := range entries {
		result[i] = RegistryEntry{
			Name:        e.Name,
			Description: e.Description,
			Source:      e.Source,
			Commit:      e.Commit,
			Meta:        SkillMeta{},
		}
	}
	return result, nil
}

// LockData produces a LockedAsset from a skill installation.
func (h *SkillHandler) LockData(a Asset, info InstallInfo) LockedAsset {
	return LockedAsset{
		Kind:   KindSkill,
		Name:   a.Name,
		Source: a.Source,
		Commit: info.Commit,
		Ref:    info.Ref,
	}
}

// parseSkillFrontmatter reads YAML frontmatter from a SKILL.md file.
func parseSkillFrontmatter(path string) (*skillFrontmatter, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)

	// Look for opening ---
	if !scanner.Scan() {
		return nil, fmt.Errorf("empty file: %s", path)
	}
	if strings.TrimSpace(scanner.Text()) != "---" {
		return nil, fmt.Errorf("no frontmatter in %s", path)
	}

	// Collect frontmatter lines until closing ---
	var frontmatter strings.Builder
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "---" {
			break
		}
		frontmatter.WriteString(line)
		frontmatter.WriteString("\n")
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var fm skillFrontmatter
	if err := yaml.Unmarshal([]byte(frontmatter.String()), &fm); err != nil {
		return nil, fmt.Errorf("parsing frontmatter in %s: %w", path, err)
	}

	if fm.Name == "" {
		return nil, fmt.Errorf("SKILL.md missing name field: %s", path)
	}

	return &fm, nil
}

func init() { Register(&SkillHandler{}) }
