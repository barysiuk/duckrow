package core

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
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
// It reads SKILL.md files from the canonical .agents/skills/ directory
// and checks which agents have each skill via their skill directories.
func (s *Scanner) ScanFolder(folderPath string) ([]InstalledSkill, error) {
	canonicalDir := filepath.Join(folderPath, canonicalSkillsDir)

	entries, err := os.ReadDir(canonicalDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No skills installed
		}
		return nil, fmt.Errorf("reading skills directory: %w", err)
	}

	var skills []InstalledSkill
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillPath := filepath.Join(canonicalDir, entry.Name())
		skillMdPath := filepath.Join(skillPath, skillFileName)

		metadata, err := ParseSkillMd(skillMdPath)
		if err != nil {
			continue // Skip entries without valid SKILL.md
		}

		agents := s.detectAgentsForSkill(folderPath, entry.Name())

		skills = append(skills, InstalledSkill{
			Name:        metadata.Name,
			Description: metadata.Description,
			Version:     metadata.Metadata.Version,
			Author:      metadata.Metadata.Author,
			Path:        skillPath,
			Agents:      agents,
		})
	}

	return skills, nil
}

// DetectAgents returns the names of agents detected in a folder
// (i.e., agents whose skill directories exist in the folder).
func (s *Scanner) DetectAgents(folderPath string) []string {
	var detected []string
	seen := make(map[string]bool)

	for _, agent := range s.agents {
		skillDir := filepath.Join(folderPath, agent.SkillsDir)
		if dirExists(skillDir) && !seen[agent.DisplayName] {
			detected = append(detected, agent.DisplayName)
			seen[agent.DisplayName] = true
		}
	}
	return detected
}

// detectAgentsForSkill checks which agents have a specific skill installed
// by looking for the skill directory or symlink in each agent's skill directory.
func (s *Scanner) detectAgentsForSkill(folderPath, skillDirName string) []string {
	var agents []string
	seen := make(map[string]bool)

	for _, agent := range s.agents {
		agentSkillPath := filepath.Join(folderPath, agent.SkillsDir, skillDirName)
		if pathExists(agentSkillPath) && !seen[agent.DisplayName] {
			agents = append(agents, agent.DisplayName)
			seen[agent.DisplayName] = true
		}
	}
	return agents
}

// ParseSkillMd reads and parses the YAML frontmatter from a SKILL.md file.
func ParseSkillMd(path string) (*SkillMetadata, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	defer f.Close()

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
