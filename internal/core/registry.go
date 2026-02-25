package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/barysiuk/duckrow/internal/core/asset"
)

const (
	registryManifestFile = "duckrow.json"
	registryCloneTimeout = 60 * time.Second
	registryPullTimeout  = 30 * time.Second
)

// RegistryManager handles registry operations: add, remove, refresh, and list assets.
type RegistryManager struct {
	registriesDir string // ~/.duckrow/registries/
}

// NewRegistryManager creates a RegistryManager that stores clones in the given directory.
func NewRegistryManager(registriesDir string) *RegistryManager {
	return &RegistryManager{registriesDir: registriesDir}
}

// RegistryDirKey derives a unique, filesystem-safe directory name from a repo URL.
// This ensures that two registries with different repos but the same manifest name
// are stored separately on disk.
func RegistryDirKey(repoURL string) string {
	// Normalize: trim trailing .git, trailing slashes, lowercase
	normalized := strings.ToLower(strings.TrimSuffix(strings.TrimSuffix(repoURL, "/"), ".git"))

	// Extract a human-readable suffix from the URL
	// e.g. "git@github.com:org/repo.git" → "org-repo"
	// e.g. "https://github.com/org/repo" → "org-repo"
	readable := normalized
	// Strip SSH prefix (git@host:path)
	if idx := strings.LastIndex(readable, ":"); idx >= 0 && !strings.Contains(readable, "://") {
		readable = readable[idx+1:]
	}
	// Strip HTTPS prefix
	if idx := strings.LastIndex(readable, "://"); idx >= 0 {
		readable = readable[idx+3:]
		// Remove host part
		if slashIdx := strings.Index(readable, "/"); slashIdx >= 0 {
			readable = readable[slashIdx+1:]
		}
	}
	// Replace path separators with dashes
	readable = strings.ReplaceAll(readable, "/", "-")
	readable = strings.ReplaceAll(readable, string(filepath.Separator), "-")

	// Add a short hash for uniqueness
	h := sha256.Sum256([]byte(repoURL))
	shortHash := hex.EncodeToString(h[:4]) // 8 hex chars

	if readable == "" {
		return shortHash
	}
	return readable + "-" + shortHash
}

// --- Manifest types ---

// RegistryManifest is the parsed duckrow.json from a registry repo.
// It supports both v1 (Skills/MCPs arrays) and v2 (Assets map) formats.
// The Version field discriminates: 0 or 1 = v1, 2 = v2.
type RegistryManifest struct {
	Version     int                        `json:"version,omitempty"`
	Name        string                     `json:"name"`
	Description string                     `json:"description,omitempty"`
	Assets      map[string]json.RawMessage `json:"assets,omitempty"`
	// v1 legacy fields — populated when reading v1 manifests, converted internally.
	Skills   []json.RawMessage `json:"skills,omitempty"`
	MCPs     []json.RawMessage `json:"mcps,omitempty"`
	Warnings []string          `json:"-"` // validation warnings, not serialized
}

// ParsedManifest holds the fully resolved entries after handler parsing.
type ParsedManifest struct {
	Name        string
	Description string
	Entries     map[asset.Kind][]asset.RegistryEntry
	Warnings    []string
}

// ParseManifest loads a manifest and delegates each kind to its handler.
func ParseManifest(raw *RegistryManifest) (*ParsedManifest, error) {
	pm := &ParsedManifest{
		Name:        raw.Name,
		Description: raw.Description,
		Entries:     make(map[asset.Kind][]asset.RegistryEntry),
		Warnings:    raw.Warnings,
	}

	// Build the assets map — either from v2 Assets field or v1 legacy fields.
	assetsMap := raw.Assets
	if len(assetsMap) == 0 {
		// v1 format: convert Skills/MCPs arrays to the assets map.
		assetsMap = make(map[string]json.RawMessage)
		if len(raw.Skills) > 0 {
			skillsJSON, err := json.Marshal(raw.Skills)
			if err != nil {
				return nil, fmt.Errorf("marshaling v1 skills: %w", err)
			}
			assetsMap[string(asset.KindSkill)] = skillsJSON
		}
		if len(raw.MCPs) > 0 {
			mcpsJSON, err := json.Marshal(raw.MCPs)
			if err != nil {
				return nil, fmt.Errorf("marshaling v1 MCPs: %w", err)
			}
			assetsMap[string(asset.KindMCP)] = mcpsJSON
		}
	}

	for kindStr, data := range assetsMap {
		kind := asset.Kind(kindStr)
		handler, ok := asset.Get(kind)
		if !ok {
			pm.Warnings = append(pm.Warnings,
				fmt.Sprintf("unknown asset kind %q in manifest; skipping", kindStr))
			continue
		}
		entries, err := handler.ParseManifestEntries(data)
		if err != nil {
			return nil, fmt.Errorf("parsing %s entries: %w", kindStr, err)
		}
		pm.Entries[kind] = entries
	}

	// Validate entries and add warnings.
	if skills, ok := pm.Entries[asset.KindSkill]; ok {
		for _, s := range skills {
			if s.Source != "" && !isCanonicalSource(s.Source) {
				pm.Warnings = append(pm.Warnings,
					fmt.Sprintf("skill %q has non-canonical source %q (expected host/owner/repo/path format)",
						s.Name, s.Source))
			}
		}
	}
	if mcps, ok := pm.Entries[asset.KindMCP]; ok {
		for _, m := range mcps {
			if m.Name == "" {
				pm.Warnings = append(pm.Warnings,
					"MCP entry missing required 'name' field")
				continue
			}
			meta, ok := m.Meta.(asset.MCPMeta)
			if !ok {
				continue
			}
			if !meta.IsStdio() && !meta.IsRemote() {
				pm.Warnings = append(pm.Warnings,
					fmt.Sprintf("MCP %q missing both 'command' and 'url' (one is required)", m.Name))
			}
			if meta.IsStdio() && meta.IsRemote() {
				pm.Warnings = append(pm.Warnings,
					fmt.Sprintf("MCP %q has both 'command' and 'url' (only one allowed)", m.Name))
			}
		}
	}

	return pm, nil
}

// --- Core registry operations ---

// Add clones a registry repo and returns the parsed manifest.
// The clone is stored in a directory derived from the repo URL to avoid
// collisions when different repos share the same manifest name.
func (rm *RegistryManager) Add(repoURL string) (*RegistryManifest, error) {
	repoURL = strings.TrimSpace(repoURL)
	if repoURL == "" {
		return nil, fmt.Errorf("repository URL is required")
	}

	// Clone to a temp directory first to read the manifest
	tmpDir, err := os.MkdirTemp("", "duckrow-registry-*")
	if err != nil {
		return nil, fmt.Errorf("creating temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	if err := gitClone(repoURL, "", tmpDir, registryCloneTimeout); err != nil {
		return nil, fmt.Errorf("cloning registry: %w", err)
	}

	// Read manifest to get the name
	manifest, err := readManifest(tmpDir)
	if err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}

	if manifest.Name == "" {
		return nil, fmt.Errorf("registry manifest missing required 'name' field")
	}

	// Move to permanent location keyed by repo URL
	dirKey := RegistryDirKey(repoURL)
	destDir := filepath.Join(rm.registriesDir, dirKey)
	if dirExists(destDir) {
		// Remove existing clone to update
		if err := os.RemoveAll(destDir); err != nil {
			return nil, fmt.Errorf("removing existing registry clone: %w", err)
		}
	}

	if err := os.MkdirAll(filepath.Dir(destDir), 0o755); err != nil {
		return nil, fmt.Errorf("creating registries directory: %w", err)
	}

	// Clone directly to the final location (cleaner than moving)
	if err := gitClone(repoURL, "", destDir, registryCloneTimeout); err != nil {
		return nil, fmt.Errorf("cloning registry to final location: %w", err)
	}

	// Populate warnings by parsing through handlers.
	pm, parseErr := ParseManifest(manifest)
	if parseErr == nil {
		manifest.Warnings = pm.Warnings
	}

	return manifest, nil
}

// Remove deletes a registry clone from disk using the repo URL to locate it.
func (rm *RegistryManager) Remove(repoURL string) error {
	repoURL = strings.TrimSpace(repoURL)
	if repoURL == "" {
		return fmt.Errorf("registry repo URL is required")
	}

	dirKey := RegistryDirKey(repoURL)
	dir := filepath.Join(rm.registriesDir, dirKey)
	if !dirExists(dir) {
		return fmt.Errorf("registry clone for %q not found", repoURL)
	}

	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("removing registry %q: %w", repoURL, err)
	}

	return nil
}

// Refresh runs git pull on a registry clone to update it.
func (rm *RegistryManager) Refresh(repoURL string) (*RegistryManifest, error) {
	repoURL = strings.TrimSpace(repoURL)
	if repoURL == "" {
		return nil, fmt.Errorf("registry repo URL is required")
	}

	dirKey := RegistryDirKey(repoURL)
	dir := filepath.Join(rm.registriesDir, dirKey)
	if !dirExists(dir) {
		return nil, fmt.Errorf("registry clone for %q not found", repoURL)
	}

	if err := gitPull(dir, registryPullTimeout); err != nil {
		return nil, fmt.Errorf("refreshing registry %q: %w", repoURL, err)
	}

	manifest, err := readManifest(dir)
	if err != nil {
		return nil, fmt.Errorf("reading manifest after refresh: %w", err)
	}

	return manifest, nil
}

// RefreshAll refreshes all registered registries.
// The returned map is keyed by repo URL.
func (rm *RegistryManager) RefreshAll(registries []Registry) (map[string]*RegistryManifest, error) {
	results := make(map[string]*RegistryManifest)

	for _, reg := range registries {
		manifest, err := rm.Refresh(reg.Repo)
		if err != nil {
			// Continue with other registries but record the error
			continue
		}
		results[reg.Repo] = manifest
	}

	return results, nil
}

// LoadManifest reads and parses the manifest for a registry identified by repo URL.
func (rm *RegistryManager) LoadManifest(repoURL string) (*RegistryManifest, error) {
	dirKey := RegistryDirKey(repoURL)
	dir := filepath.Join(rm.registriesDir, dirKey)
	if !dirExists(dir) {
		return nil, fmt.Errorf("registry clone for %q not found", repoURL)
	}

	return readManifest(dir)
}

// LoadAllManifests loads manifests for all given registries.
// Registries that fail to load are silently skipped.
// The returned map is keyed by repo URL.
func (rm *RegistryManager) LoadAllManifests(registries []Registry) map[string]*RegistryManifest {
	results := make(map[string]*RegistryManifest)

	for _, reg := range registries {
		manifest, err := rm.LoadManifest(reg.Repo)
		if err != nil {
			continue
		}
		results[reg.Repo] = manifest
	}

	return results
}

// --- Generic asset lookup ---

// FindAsset searches all registries for an asset by kind and name.
// Returns the registry entry, the registry name, and any error.
// If the name is ambiguous across registries, an error is returned.
func (rm *RegistryManager) FindAsset(registries []Registry, kind asset.Kind, name string) (*asset.RegistryEntry, string, error) {
	if name == "" {
		return nil, "", fmt.Errorf("asset name is required")
	}

	handler, ok := asset.Get(kind)
	if !ok {
		return nil, "", fmt.Errorf("unknown asset kind: %s", kind)
	}

	type match struct {
		entry        asset.RegistryEntry
		registryName string
		registryRepo string
	}
	var matches []match

	for _, reg := range registries {
		manifest, err := rm.LoadManifest(reg.Repo)
		if err != nil {
			continue
		}

		parsed, err := ParseManifest(manifest)
		if err != nil {
			continue
		}

		for _, entry := range parsed.Entries[kind] {
			if entry.Name == name {
				matches = append(matches, match{
					entry:        entry,
					registryName: parsed.Name,
					registryRepo: reg.Repo,
				})
			}
		}
	}

	switch len(matches) {
	case 0:
		// List available assets of this kind to help the user.
		var allNames []string
		for _, reg := range registries {
			manifest, err := rm.LoadManifest(reg.Repo)
			if err != nil {
				continue
			}
			parsed, err := ParseManifest(manifest)
			if err != nil {
				continue
			}
			for _, entry := range parsed.Entries[kind] {
				allNames = append(allNames, entry.Name)
			}
		}
		if len(allNames) == 0 {
			return nil, "", fmt.Errorf("%s %q not found (no %ss available in configured registries)",
				handler.DisplayName(), name, strings.ToLower(handler.DisplayName()))
		}
		return nil, "", fmt.Errorf("%s %q not found in registries. Available: %s",
			handler.DisplayName(), name, strings.Join(allNames, ", "))
	case 1:
		return &matches[0].entry, matches[0].registryName, nil
	default:
		var registryNames []string
		for _, m := range matches {
			registryNames = append(registryNames, fmt.Sprintf("%s (%s)", m.registryName, m.registryRepo))
		}
		return nil, "", fmt.Errorf("%s %q found in multiple registries; use --registry to disambiguate:\n  %s",
			handler.DisplayName(), name, strings.Join(registryNames, "\n  "))
	}
}

// --- Unified registry asset info ---

// RegistryAssetInfo associates a registry entry with its registry and asset kind.
// This is the unified replacement for the legacy RegistrySkillInfo/RegistryMCPInfo types.
type RegistryAssetInfo struct {
	RegistryName string              // Display name from the manifest
	RegistryRepo string              // Repo URL (unique identifier)
	Kind         asset.Kind          // Asset kind (skill, mcp, etc.)
	Entry        asset.RegistryEntry // The registry entry
}

// ListAssets returns all assets of a given kind across all loaded registries.
func (rm *RegistryManager) ListAssets(registries []Registry, kind asset.Kind) []RegistryAssetInfo {
	var assets []RegistryAssetInfo

	for _, reg := range registries {
		manifest, err := rm.LoadManifest(reg.Repo)
		if err != nil {
			continue
		}

		parsed, err := ParseManifest(manifest)
		if err != nil {
			continue
		}

		for _, entry := range parsed.Entries[kind] {
			assets = append(assets, RegistryAssetInfo{
				RegistryName: parsed.Name,
				RegistryRepo: reg.Repo,
				Kind:         kind,
				Entry:        entry,
			})
		}
	}

	return assets
}

// ListAllAssets returns all assets across all kinds and all loaded registries.
func (rm *RegistryManager) ListAllAssets(registries []Registry) []RegistryAssetInfo {
	var all []RegistryAssetInfo
	for _, kind := range asset.Kinds() {
		all = append(all, rm.ListAssets(registries, kind)...)
	}
	return all
}

// --- Legacy compatibility methods ---
// These methods provide backward-compatible access to skills and MCPs
// using the old RegistrySkillInfo/RegistryMCPInfo types.

// RegistrySkillInfo associates a skill entry with its registry.
type RegistrySkillInfo struct {
	RegistryName string // Display name from the manifest
	RegistryRepo string // Repo URL (unique identifier)
	Skill        asset.RegistryEntry
}

// RegistryMCPInfo associates an MCP entry with its registry.
type RegistryMCPInfo struct {
	RegistryName string // Display name from the manifest
	RegistryRepo string // Repo URL (unique identifier)
	MCP          asset.RegistryEntry
}

// ListSkills returns all skills across all loaded registries.
func (rm *RegistryManager) ListSkills(registries []Registry) []RegistrySkillInfo {
	var skills []RegistrySkillInfo

	for _, reg := range registries {
		manifest, err := rm.LoadManifest(reg.Repo)
		if err != nil {
			continue
		}

		parsed, err := ParseManifest(manifest)
		if err != nil {
			continue
		}

		for _, entry := range parsed.Entries[asset.KindSkill] {
			skills = append(skills, RegistrySkillInfo{
				RegistryName: parsed.Name,
				RegistryRepo: reg.Repo,
				Skill:        entry,
			})
		}
	}

	return skills
}

// ListMCPs returns all MCPs across all loaded registries.
func (rm *RegistryManager) ListMCPs(registries []Registry) []RegistryMCPInfo {
	var mcps []RegistryMCPInfo

	for _, reg := range registries {
		manifest, err := rm.LoadManifest(reg.Repo)
		if err != nil {
			continue
		}

		parsed, err := ParseManifest(manifest)
		if err != nil {
			continue
		}

		for _, entry := range parsed.Entries[asset.KindMCP] {
			mcps = append(mcps, RegistryMCPInfo{
				RegistryName: parsed.Name,
				RegistryRepo: reg.Repo,
				MCP:          entry,
			})
		}
	}

	return mcps
}

// FindSkill searches all registries for a skill by name.
// If registryFilter is non-empty, only that registry (matched by name or repo URL) is searched.
// Returns an error if the skill is not found or if the name is ambiguous across registries.
func (rm *RegistryManager) FindSkill(registries []Registry, skillName, registryFilter string) (*RegistrySkillInfo, error) {
	if skillName == "" {
		return nil, fmt.Errorf("skill name is required")
	}

	searchRegistries := registries
	if registryFilter != "" {
		var filtered []Registry
		for _, r := range registries {
			if r.Name == registryFilter || r.Repo == registryFilter {
				filtered = append(filtered, r)
			}
		}
		if len(filtered) == 0 {
			return nil, fmt.Errorf("registry %q not found", registryFilter)
		}
		searchRegistries = filtered
	}

	var matches []RegistrySkillInfo
	for _, reg := range searchRegistries {
		manifest, err := rm.LoadManifest(reg.Repo)
		if err != nil {
			continue
		}
		parsed, err := ParseManifest(manifest)
		if err != nil {
			continue
		}
		for _, entry := range parsed.Entries[asset.KindSkill] {
			if entry.Name == skillName {
				matches = append(matches, RegistrySkillInfo{
					RegistryName: parsed.Name,
					RegistryRepo: reg.Repo,
					Skill:        entry,
				})
			}
		}
	}

	switch len(matches) {
	case 0:
		// List available skills to help the user
		allSkills := rm.ListSkills(searchRegistries)
		if len(allSkills) == 0 {
			return nil, fmt.Errorf("skill %q not found (no skills available in configured registries)", skillName)
		}
		var names []string
		for _, s := range allSkills {
			names = append(names, s.Skill.Name)
		}
		return nil, fmt.Errorf("skill %q not found in registries. Available: %s", skillName, strings.Join(names, ", "))
	case 1:
		return &matches[0], nil
	default:
		var registryNames []string
		for _, m := range matches {
			registryNames = append(registryNames, fmt.Sprintf("%s (%s)", m.RegistryName, m.RegistryRepo))
		}
		return nil, fmt.Errorf("skill %q found in multiple registries; use --registry to disambiguate:\n  %s",
			skillName, strings.Join(registryNames, "\n  "))
	}
}

// FindMCP searches all registries for an MCP by name.
// If registryFilter is non-empty, only that registry (matched by name or repo URL) is searched.
// Returns an error if the MCP is not found or if the name is ambiguous across registries.
func (rm *RegistryManager) FindMCP(registries []Registry, mcpName, registryFilter string) (*RegistryMCPInfo, error) {
	if mcpName == "" {
		return nil, fmt.Errorf("MCP name is required")
	}

	searchRegistries := registries
	if registryFilter != "" {
		var filtered []Registry
		for _, r := range registries {
			if r.Name == registryFilter || r.Repo == registryFilter {
				filtered = append(filtered, r)
			}
		}
		if len(filtered) == 0 {
			return nil, fmt.Errorf("registry %q not found", registryFilter)
		}
		searchRegistries = filtered
	}

	var matches []RegistryMCPInfo
	for _, reg := range searchRegistries {
		manifest, err := rm.LoadManifest(reg.Repo)
		if err != nil {
			continue
		}
		parsed, err := ParseManifest(manifest)
		if err != nil {
			continue
		}
		for _, entry := range parsed.Entries[asset.KindMCP] {
			if entry.Name == mcpName {
				matches = append(matches, RegistryMCPInfo{
					RegistryName: parsed.Name,
					RegistryRepo: reg.Repo,
					MCP:          entry,
				})
			}
		}
	}

	switch len(matches) {
	case 0:
		// List available MCPs to help the user
		allMCPs := rm.ListMCPs(searchRegistries)
		if len(allMCPs) == 0 {
			return nil, fmt.Errorf("MCP %q not found (no MCPs available in configured registries)", mcpName)
		}
		var names []string
		for _, m := range allMCPs {
			names = append(names, m.MCP.Name)
		}
		return nil, fmt.Errorf("MCP %q not found in registries. Available: %s", mcpName, strings.Join(names, ", "))
	case 1:
		return &matches[0], nil
	default:
		var registryNames []string
		for _, m := range matches {
			registryNames = append(registryNames, fmt.Sprintf("%s (%s)", m.RegistryName, m.RegistryRepo))
		}
		return nil, fmt.Errorf("MCP %q found in multiple registries; use --registry to disambiguate:\n  %s",
			mcpName, strings.Join(registryNames, "\n  "))
	}
}

// --- Registry commit resolution ---

// BuildRegistryCommitMap builds a map from canonical source strings to
// registry commit hashes. This allows update checks to skip network fetches
// for registry-pinned skills by comparing the installed commit against the
// registry's pinned commit locally.
//
// The map is built by merging two sources per registry:
//  1. Cached commits from duckrow.commits.json (hydrated unpinned skills)
//  2. Pinned commits from the manifest (explicit commit field in duckrow.json)
//
// Pinned commits take precedence over cached commits.
func BuildRegistryCommitMap(registries []Registry, rm *RegistryManager) map[string]string {
	commits := make(map[string]string)

	if len(registries) == 0 {
		return commits
	}

	for _, reg := range registries {
		regDir := filepath.Join(rm.registriesDir, RegistryDirKey(reg.Repo))

		// Layer 1: cached commits (hydrated unpinned skills).
		cached := loadCachedCommits(regDir)
		for source, commit := range cached {
			commits[source] = commit
		}

		// Layer 2: pinned commits from manifest (takes precedence).
		manifest, err := rm.LoadManifest(reg.Repo)
		if err != nil {
			continue
		}
		parsed, err := ParseManifest(manifest)
		if err != nil {
			continue
		}
		for _, entry := range parsed.Entries[asset.KindSkill] {
			if entry.Commit != "" && entry.Source != "" {
				commits[entry.Source] = entry.Commit
			}
		}
	}

	return commits
}

const cachedCommitsFile = "duckrow.commits.json"

// loadCachedCommits reads the cached commits file from a registry directory.
// Returns an empty map if the file doesn't exist or can't be parsed.
func loadCachedCommits(registryDir string) map[string]string {
	path := filepath.Join(registryDir, cachedCommitsFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return make(map[string]string)
	}

	var cached CachedCommits
	if err := json.Unmarshal(data, &cached); err != nil {
		return make(map[string]string)
	}

	if cached.Commits == nil {
		return make(map[string]string)
	}
	return cached.Commits
}

// writeCachedCommits writes resolved commits to the cache file in a registry directory.
func writeCachedCommits(registryDir string, commits map[string]string) error {
	cached := CachedCommits{
		GeneratedAt: time.Now().UTC(),
		Commits:     commits,
	}

	data, err := json.MarshalIndent(cached, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling cached commits: %w", err)
	}

	path := filepath.Join(registryDir, cachedCommitsFile)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", cachedCommitsFile, err)
	}
	return nil
}

// HydrateRegistryCommits resolves the latest commit SHA for each unpinned
// skill in the configured registries. Unpinned skills are those with a Source
// but no Commit field in the registry manifest.
//
// For each unique source repository, a shallow clone is performed and the
// latest commit for each skill's sub-path is determined via git log. Results
// are cached to duckrow.commits.json in the registry directory so that
// BuildRegistryCommitMap can include them without additional network calls.
//
// Clone errors are logged and skipped — hydration is best-effort.
// The overrides parameter maps "owner/repo" keys to clone URL overrides
// for private repositories.
func (rm *RegistryManager) HydrateRegistryCommits(registries []Registry, overrides map[string]string) {
	for _, reg := range registries {
		regDir := filepath.Join(rm.registriesDir, RegistryDirKey(reg.Repo))
		manifest, err := rm.LoadManifest(reg.Repo)
		if err != nil {
			continue
		}

		parsed, err := ParseManifest(manifest)
		if err != nil {
			continue
		}

		// Collect unpinned skills (have Source but no Commit).
		type unpinnedSkill struct {
			source  string
			subPath string
		}
		type repoRefKey struct {
			repo string
			ref  string // always "" for registry skills (they don't have a ref field)
		}

		repoGroups := make(map[repoRefKey][]unpinnedSkill)
		var repoGroupOrder []repoRefKey

		for _, entry := range parsed.Entries[asset.KindSkill] {
			if entry.Source == "" || entry.Commit != "" {
				continue // skip: no source or already pinned
			}

			rk := repoKey(entry.Source)
			sp := skillSubPath(entry.Source)
			key := repoRefKey{repo: rk}

			if _, exists := repoGroups[key]; !exists {
				repoGroupOrder = append(repoGroupOrder, key)
			}
			repoGroups[key] = append(repoGroups[key], unpinnedSkill{
				source:  entry.Source,
				subPath: sp,
			})
		}

		if len(repoGroups) == 0 {
			continue // all skills are pinned
		}

		// Resolve commits for each repo group.
		resolved := make(map[string]string)

		for _, key := range repoGroupOrder {
			skills := repoGroups[key]

			// Parse source to build clone URL.
			host, owner, repo, _, parseErr := ParseLockSource(skills[0].source)
			if parseErr != nil {
				continue
			}

			cloneURL := fmt.Sprintf("https://%s/%s/%s.git", host, owner, repo)

			// Apply clone URL override.
			repoKeyStr := strings.ToLower(owner) + "/" + strings.ToLower(repo)
			if override, ok := overrides[repoKeyStr]; ok && override != "" {
				cloneURL = override
			}

			tmpDir, cloneErr := cloneRepo(cloneURL, key.ref, false)
			if cloneErr != nil {
				continue // best-effort: skip repos that fail to clone
			}

			for _, s := range skills {
				commit, commitErr := GetSkillCommit(tmpDir, s.subPath)
				if commitErr != nil {
					continue
				}
				resolved[s.source] = commit
			}

			_ = os.RemoveAll(tmpDir)
		}

		// Write resolved commits to cache file.
		if len(resolved) > 0 {
			_ = writeCachedCommits(regDir, resolved)
		}
	}
}

// --- Internal helpers ---

// readManifest reads and parses the duckrow.json manifest from a directory.
// Supports both v1 and v2 formats transparently.
func readManifest(dir string) (*RegistryManifest, error) {
	path := filepath.Join(dir, registryManifestFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%s not found in repository", registryManifestFile)
		}
		return nil, fmt.Errorf("reading %s: %w", registryManifestFile, err)
	}

	var manifest RegistryManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", registryManifestFile, err)
	}

	// Auto-detect v1 format: has Skills/MCPs arrays but no Assets map.
	// The Warnings are populated lazily by ParseManifest.
	return &manifest, nil
}

// --- Git helpers ---

// gitClone clones a repository to the given directory.
// On failure it returns a *CloneError with classified diagnostics.
func gitClone(url, ref, destDir string, timeout time.Duration) error {
	args := []string{"clone", "--depth", "1"}
	if ref != "" {
		args = append(args, "--branch", ref)
	}
	args = append(args, url, destDir)

	cmd := exec.Command("git", args...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")

	output, err := runWithTimeout(cmd, timeout)
	if err != nil {
		return ClassifyCloneError(url, FormatCommand(url, ref), output)
	}

	return nil
}

// gitPull runs git pull in the given directory.
// On failure it returns a *CloneError with classified diagnostics.
func gitPull(dir string, timeout time.Duration) error {
	cmd := exec.Command("git", "pull", "--ff-only")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")

	output, err := runWithTimeout(cmd, timeout)
	if err != nil {
		// Determine the remote URL for error classification.
		remoteURL := gitRemoteURL(dir)
		return ClassifyCloneError(remoteURL, "git pull --ff-only", output)
	}

	return nil
}

// gitRemoteURL reads the origin remote URL from a git repository.
func gitRemoteURL(dir string) string {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
