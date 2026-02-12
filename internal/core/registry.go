package core

import (
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

// Add clones a registry repo and returns the parsed manifest.
// The registry name is derived from the manifest's "name" field.
func (rm *RegistryManager) Add(repoURL string) (*RegistryManifest, error) {
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

	// Move to permanent location
	destDir := filepath.Join(rm.registriesDir, manifest.Name)
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

// Remove deletes a registry clone from disk.
func (rm *RegistryManager) Remove(name string) error {
	if name == "" {
		return fmt.Errorf("registry name is required")
	}

	dir := filepath.Join(rm.registriesDir, name)
	if !dirExists(dir) {
		return fmt.Errorf("registry %q not found", name)
	}

	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("removing registry %q: %w", name, err)
	}

	return nil
}

// Refresh runs git pull on a registry clone to update it.
func (rm *RegistryManager) Refresh(name string) (*RegistryManifest, error) {
	if name == "" {
		return nil, fmt.Errorf("registry name is required")
	}

	dir := filepath.Join(rm.registriesDir, name)
	if !dirExists(dir) {
		return nil, fmt.Errorf("registry %q not found", name)
	}

	if err := gitPull(dir, registryPullTimeout); err != nil {
		return nil, fmt.Errorf("refreshing registry %q: %w", name, err)
	}

	manifest, err := readManifest(dir)
	if err != nil {
		return nil, fmt.Errorf("reading manifest after refresh: %w", err)
	}

	return manifest, nil
}

// RefreshAll refreshes all registered registries.
func (rm *RegistryManager) RefreshAll(registries []Registry) (map[string]*RegistryManifest, error) {
	results := make(map[string]*RegistryManifest)

	for _, reg := range registries {
		manifest, err := rm.Refresh(reg.Name)
		if err != nil {
			// Continue with other registries but record the error
			continue
		}
		results[reg.Name] = manifest
	}

	return results, nil
}

// LoadManifest reads and parses the manifest for a named registry.
func (rm *RegistryManager) LoadManifest(name string) (*RegistryManifest, error) {
	dir := filepath.Join(rm.registriesDir, name)
	if !dirExists(dir) {
		return nil, fmt.Errorf("registry %q not found", name)
	}

	return readManifest(dir)
}

// LoadAllManifests loads manifests for all given registries.
// Registries that fail to load are silently skipped.
func (rm *RegistryManager) LoadAllManifests(registries []Registry) map[string]*RegistryManifest {
	results := make(map[string]*RegistryManifest)

	for _, reg := range registries {
		manifest, err := rm.LoadManifest(reg.Name)
		if err != nil {
			continue
		}
		results[reg.Name] = manifest
	}

	return results
}

// ListSkills returns all skills across all loaded registries.
func (rm *RegistryManager) ListSkills(registries []Registry) []RegistrySkillInfo {
	var skills []RegistrySkillInfo

	for _, reg := range registries {
		manifest, err := rm.LoadManifest(reg.Name)
		if err != nil {
			continue
		}

		for _, skill := range manifest.Skills {
			skills = append(skills, RegistrySkillInfo{
				RegistryName: manifest.Name,
				Skill:        skill,
			})
		}
	}

	return skills
}

// RegistrySkillInfo associates a skill entry with its registry name.
type RegistrySkillInfo struct {
	RegistryName string
	Skill        SkillEntry
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
		return fmt.Errorf("git clone failed: %w\nOutput: %s", err, strings.TrimSpace(output))
	}

	return nil
}

// gitPull runs git pull in the given directory.
func gitPull(dir string, timeout time.Duration) error {
	cmd := exec.Command("git", "pull", "--ff-only")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")

	output, err := runWithTimeout(cmd, timeout)
	if err != nil {
		return fmt.Errorf("git pull failed: %w\nOutput: %s", err, strings.TrimSpace(output))
	}

	return nil
}
