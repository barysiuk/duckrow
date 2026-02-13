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
)

const (
	registryManifestFile = "duckrow.json"
	registryCloneTimeout = 60 * time.Second
	registryPullTimeout  = 30 * time.Second
)

// RegistryManager handles registry operations: add, remove, refresh, and list skills.
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

// ListSkills returns all skills across all loaded registries.
func (rm *RegistryManager) ListSkills(registries []Registry) []RegistrySkillInfo {
	var skills []RegistrySkillInfo

	for _, reg := range registries {
		manifest, err := rm.LoadManifest(reg.Repo)
		if err != nil {
			continue
		}

		for _, skill := range manifest.Skills {
			skills = append(skills, RegistrySkillInfo{
				RegistryName: manifest.Name,
				RegistryRepo: reg.Repo,
				Skill:        skill,
			})
		}
	}

	return skills
}

// RegistrySkillInfo associates a skill entry with its registry.
type RegistrySkillInfo struct {
	RegistryName string // Display name from the manifest
	RegistryRepo string // Repo URL (unique identifier)
	Skill        SkillEntry
}

// FindSkill searches all registries for a skill by name.
// If registryFilter is non-empty, only that registry (matched by name or repo URL) is searched.
// Returns an error if the skill is not found or if the name is ambiguous across registries.
func (rm *RegistryManager) FindSkill(registries []Registry, skillName, registryFilter string) (*RegistrySkillInfo, error) {
	if skillName == "" {
		return nil, fmt.Errorf("skill name is required")
	}

	var searchRegistries []Registry
	if registryFilter != "" {
		// Filter to the specified registry (by name or repo URL)
		for _, r := range registries {
			if r.Name == registryFilter || r.Repo == registryFilter {
				searchRegistries = append(searchRegistries, r)
			}
		}
		if len(searchRegistries) == 0 {
			return nil, fmt.Errorf("registry %q not found", registryFilter)
		}
	} else {
		searchRegistries = registries
	}

	var matches []RegistrySkillInfo
	for _, reg := range searchRegistries {
		manifest, err := rm.LoadManifest(reg.Repo)
		if err != nil {
			continue
		}
		for _, skill := range manifest.Skills {
			if skill.Name == skillName {
				matches = append(matches, RegistrySkillInfo{
					RegistryName: manifest.Name,
					RegistryRepo: reg.Repo,
					Skill:        skill,
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

// readManifest reads and parses the duckrow.json manifest from a directory.
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

	return &manifest, nil
}

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
