package core

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	skillFileName      = "SKILL.md"
	canonicalSkillsDir = ".agents/skills"
)

// Scanner scans folders for installed skills.
type Scanner struct {
	agents []AgentDef
}

// NewScanner creates a Scanner with the given agent definitions.
func NewScanner(agents []AgentDef) *Scanner {
	return &Scanner{agents: agents}
}

// ScanFolder scans a project folder for installed skills.
// It reads SKILL.md files from all agent skill directories (both primary skillsDir
// and altSkillsDirs) and deduplicates by skill name, preferring the canonical
// .agents/skills/ path when a skill appears in multiple locations.
func (s *Scanner) ScanFolder(folderPath string) ([]InstalledSkill, error) {
	// Collect all unique skill directory paths from agent definitions
	skillsDirs := s.allSkillsDirs()

	// Track discovered skills by name for deduplication.
	// Prefer canonical path when a skill exists in multiple directories.
	type candidate struct {
		skill       InstalledSkill
		isCanonical bool
	}
	found := make(map[string]candidate)

	for _, relDir := range skillsDirs {
		absDir := filepath.Join(folderPath, relDir)
		isCanonical := relDir == canonicalSkillsDir

		entries, err := os.ReadDir(absDir)
		if err != nil {
			continue // Directory doesn't exist or can't be read
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			skillPath := filepath.Join(absDir, entry.Name())
			skillMdPath := filepath.Join(skillPath, skillFileName)

			metadata, err := ParseSkillMd(skillMdPath)
			if err != nil {
				continue // Skip entries without valid SKILL.md
			}

			existing, exists := found[metadata.Name]
			if exists && existing.isCanonical {
				continue // Canonical path wins, skip this duplicate
			}

			agents := s.detectAgentsForSkill(folderPath, entry.Name())

			found[metadata.Name] = candidate{
				skill: InstalledSkill{
					Name:        metadata.Name,
					Description: metadata.Description,
					Version:     metadata.Metadata.Version,
					Author:      metadata.Metadata.Author,
					Path:        skillPath,
					Agents:      agents,
				},
				isCanonical: isCanonical,
			}
		}
	}

	if len(found) == 0 {
		return nil, nil
	}

	// Collect results in a stable order (sorted by name)
	names := make([]string, 0, len(found))
	for name := range found {
		names = append(names, name)
	}
	sort.Strings(names)

	skills := make([]InstalledSkill, 0, len(found))
	for _, name := range names {
		skills = append(skills, found[name].skill)
	}

	return skills, nil
}

// DetectAgents returns the names of agents detected in a folder
// (i.e., agents whose skill directories exist in the folder).
// Checks both primary skillsDir and altSkillsDirs for each agent.
func (s *Scanner) DetectAgents(folderPath string) []string {
	var detected []string
	seen := make(map[string]bool)

	for _, agent := range s.agents {
		if seen[agent.DisplayName] {
			continue
		}

		dirs := append([]string{agent.SkillsDir}, agent.AltSkillsDirs...)
		for _, dir := range dirs {
			skillDir := filepath.Join(folderPath, dir)
			if dirExists(skillDir) {
				detected = append(detected, agent.DisplayName)
				seen[agent.DisplayName] = true
				break
			}
		}
	}
	return detected
}

// detectAgentsForSkill checks which agents have a specific skill installed
// by looking for the skill directory or symlink in each agent's skill directories
// (both primary skillsDir and altSkillsDirs).
func (s *Scanner) detectAgentsForSkill(folderPath, skillDirName string) []string {
	var agents []string
	seen := make(map[string]bool)

	for _, agent := range s.agents {
		if seen[agent.DisplayName] {
			continue
		}

		dirs := append([]string{agent.SkillsDir}, agent.AltSkillsDirs...)
		for _, dir := range dirs {
			agentSkillPath := filepath.Join(folderPath, dir, skillDirName)
			if pathExists(agentSkillPath) {
				agents = append(agents, agent.DisplayName)
				seen[agent.DisplayName] = true
				break
			}
		}
	}
	return agents
}

// allSkillsDirs returns all unique skill directory paths from all agent definitions
// (both primary skillsDir and altSkillsDirs), with the canonical path first.
func (s *Scanner) allSkillsDirs() []string {
	seen := make(map[string]bool)
	// Start with canonical dir to ensure it's checked first
	dirs := []string{canonicalSkillsDir}
	seen[canonicalSkillsDir] = true

	for _, agent := range s.agents {
		for _, dir := range append([]string{agent.SkillsDir}, agent.AltSkillsDirs...) {
			if !seen[dir] {
				dirs = append(dirs, dir)
				seen[dir] = true
			}
		}
	}
	return dirs
}

// ParseSkillMd reads and parses the YAML frontmatter from a SKILL.md file.
func ParseSkillMd(path string) (*SkillMetadata, error) {
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

	var metadata SkillMetadata
	if err := yaml.Unmarshal([]byte(frontmatter.String()), &metadata); err != nil {
		return nil, fmt.Errorf("parsing frontmatter in %s: %w", path, err)
	}

	if metadata.Name == "" {
		return nil, fmt.Errorf("SKILL.md missing name field: %s", path)
	}

	return &metadata, nil
}

// DiscoverSkills finds all SKILL.md files in a directory tree.
// Used during installation to discover skills in a cloned repository.
func DiscoverSkills(basePath string, subPath string, includeInternal bool) ([]DiscoveredSkill, error) {
	searchPath := basePath
	if subPath != "" {
		searchPath = filepath.Join(basePath, subPath)
	}

	var skills []DiscoveredSkill

	// First check if searchPath itself contains a SKILL.md
	skillMdPath := filepath.Join(searchPath, skillFileName)
	if fileExists(skillMdPath) {
		metadata, err := ParseSkillMd(skillMdPath)
		if err == nil && (includeInternal || !metadata.Metadata.Internal) {
			skills = append(skills, DiscoveredSkill{
				Metadata: *metadata,
				Path:     searchPath,
			})
			return skills, nil
		}
	}

	// Scan subdirectories for SKILL.md files
	dirs := []string{searchPath}

	// Also check common skill subdirectories
	commonSubdirs := []string{"skills", ".agents/skills"}
	for _, sub := range commonSubdirs {
		candidate := filepath.Join(searchPath, sub)
		if dirExists(candidate) {
			dirs = append(dirs, candidate)
		}
	}

	seen := make(map[string]bool)
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			skillDir := filepath.Join(dir, entry.Name())
			mdPath := filepath.Join(skillDir, skillFileName)
			if !fileExists(mdPath) {
				continue
			}
			if seen[skillDir] {
				continue
			}
			seen[skillDir] = true

			metadata, err := ParseSkillMd(mdPath)
			if err != nil {
				continue
			}
			if !includeInternal && metadata.Metadata.Internal {
				continue
			}
			skills = append(skills, DiscoveredSkill{
				Metadata: *metadata,
				Path:     skillDir,
			})
		}
	}

	return skills, nil
}

// DiscoveredSkill represents a skill found during repository scanning.
type DiscoveredSkill struct {
	Metadata SkillMetadata
	Path     string // Directory containing the SKILL.md
}

func pathExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
