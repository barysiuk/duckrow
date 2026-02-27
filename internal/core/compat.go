// compat.go provides backward-compatible types and functions so that CLI and
// TUI consumer code compiles without modification during the Phase 2 migration.
//
// Every symbol here wraps the new sub-package APIs (asset/ and system/).
// This file will be removed in Phase 3/4 when consumers are migrated directly.
package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/barysiuk/duckrow/internal/core/asset"
	"github.com/barysiuk/duckrow/internal/core/system"
)

// ---------------------------------------------------------------------------
// AgentDef — backward-compatible struct wrapping system.System
// ---------------------------------------------------------------------------

// AgentDef is a backward-compatible view of a system.System for consumers
// that access struct fields directly (e.g., .Name, .DisplayName, .Universal).
type AgentDef struct {
	Name             string
	DisplayName      string
	SkillsDir        string
	AltSkillsDirs    []string
	GlobalSkillsDir  string
	DetectPaths      []string
	Universal        bool
	MCPConfigPath    string
	MCPConfigPathAlt string
	MCPConfigKey     string
	MCPConfigFormat  string

	// sys holds the underlying system for delegation.
	sys system.System
}

// System returns the underlying system.System.
func (a AgentDef) System() system.System { return a.sys }

// agentDefFromSystem builds an AgentDef from a system.System.
func agentDefFromSystem(s system.System) AgentDef {
	def := AgentDef{
		Name:        s.Name(),
		DisplayName: s.DisplayName(),
		Universal:   s.IsUniversal(),
		sys:         s,
	}

	// Extract BaseSystem fields via the typed accessor methods.
	type baseAccessor interface {
		SkillsDir() string
		AltSkillsDirs() []string
		GlobalSkillsDir() string
		DetectPaths() []string
		MCPConfigPath() string
		MCPConfigPathAlt() string
		MCPConfigKey() string
	}
	if ba, ok := s.(baseAccessor); ok {
		def.SkillsDir = ba.SkillsDir()
		def.AltSkillsDirs = ba.AltSkillsDirs()
		def.GlobalSkillsDir = ba.GlobalSkillsDir()
		def.DetectPaths = ba.DetectPaths()
		def.MCPConfigPath = ba.MCPConfigPath()
		def.MCPConfigPathAlt = ba.MCPConfigPathAlt()
		def.MCPConfigKey = ba.MCPConfigKey()
	}

	return def
}

// agentDefsFromSystems converts a slice of systems to AgentDefs.
func agentDefsFromSystems(systems []system.System) []AgentDef {
	result := make([]AgentDef, len(systems))
	for i, s := range systems {
		result[i] = agentDefFromSystem(s)
	}
	return result
}

// systemsFromAgentDefs extracts the underlying systems from AgentDefs.
func systemsFromAgentDefs(defs []AgentDef) []system.System {
	result := make([]system.System, len(defs))
	for i, d := range defs {
		result[i] = d.sys
	}
	return result
}

// LoadAgents returns all registered systems as AgentDefs.
func LoadAgents() ([]AgentDef, error) {
	return agentDefsFromSystems(system.All()), nil
}

// DetectAgents returns agents whose global detect paths exist on disk.
func DetectAgents(agents []AgentDef) []AgentDef {
	var result []AgentDef
	for _, a := range agents {
		for _, p := range a.DetectPaths {
			if _, err := os.Stat(p); err == nil {
				result = append(result, a)
				break
			}
		}
	}
	return result
}

// DetectAgentsInFolder returns agents active in the given folder.
// Only agents from the provided list are considered. An agent is considered
// active if its skills directory exists in the folder or if it is globally
// installed (any detect path exists on disk).
func DetectAgentsInFolder(agents []AgentDef, dir string) []AgentDef {
	var result []AgentDef
	for _, a := range agents {
		if a.sys != nil && a.sys.IsActiveInFolder(dir) {
			result = append(result, a)
			continue
		}
		// Fallback for AgentDefs without a backing system:
		// check if the agent's project-local skills dir exists.
		if a.sys == nil && a.SkillsDir != "" {
			if _, err := os.Stat(filepath.Join(dir, a.SkillsDir)); err == nil {
				result = append(result, a)
				continue
			}
		}
		// Fallback: check detect paths for global installation.
		for _, p := range a.DetectPaths {
			if _, err := os.Stat(p); err == nil {
				result = append(result, a)
				break
			}
		}
	}
	return result
}

// DetectActiveAgents returns the names of agents active in the given folder.
func DetectActiveAgents(agents []AgentDef, dir string) []string {
	detected := system.DetectInFolder(dir)
	return system.Names(detected)
}

// GetUniversalAgents filters to universal agents.
func GetUniversalAgents(agents []AgentDef) []AgentDef {
	var result []AgentDef
	for _, a := range agents {
		if a.Universal {
			result = append(result, a)
		}
	}
	return result
}

// GetNonUniversalAgents filters to non-universal agents.
func GetNonUniversalAgents(agents []AgentDef) []AgentDef {
	var result []AgentDef
	for _, a := range agents {
		if !a.Universal {
			result = append(result, a)
		}
	}
	return result
}

// GetMCPCapableAgents filters to agents that support MCP.
func GetMCPCapableAgents(agents []AgentDef) []AgentDef {
	var result []AgentDef
	for _, a := range agents {
		if a.MCPConfigPath != "" {
			result = append(result, a)
		}
	}
	return result
}

// ResolveAgentsByNames resolves agent names to AgentDefs.
func ResolveAgentsByNames(agents []AgentDef, names []string) ([]AgentDef, error) {
	byName := make(map[string]AgentDef, len(agents))
	for _, a := range agents {
		byName[a.Name] = a
	}

	result := make([]AgentDef, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if a, ok := byName[name]; ok {
			result = append(result, a)
		} else {
			var valid []string
			for _, a := range agents {
				valid = append(valid, a.Name)
			}
			return nil, fmt.Errorf("unknown agent %q; available: %s",
				name, strings.Join(valid, ", "))
		}
	}
	return result, nil
}

// ResolveMCPConfigPath returns the absolute MCP config path for an agent,
// checking the alternative path on disk first.
func ResolveMCPConfigPath(agent AgentDef, projectDir string) string {
	rel := ResolveMCPConfigPathRel(agent, projectDir)
	if rel == "" {
		return ""
	}
	return filepath.Join(projectDir, rel)
}

// ResolveMCPConfigPathRel returns the project-relative MCP config path for
// an agent, checking the alternative path on disk first.
func ResolveMCPConfigPathRel(agent AgentDef, projectDir string) string {
	type configPathResolver interface {
		ResolveMCPConfigPathRel(projectDir string) string
	}
	if r, ok := agent.sys.(configPathResolver); ok {
		return r.ResolveMCPConfigPathRel(projectDir)
	}
	// Fallback for AgentDefs constructed without a backing system.
	if agent.MCPConfigPath == "" {
		return ""
	}
	if agent.MCPConfigPathAlt != "" {
		altPath := filepath.Join(projectDir, agent.MCPConfigPathAlt)
		if _, err := os.Stat(altPath); err == nil {
			return agent.MCPConfigPathAlt
		}
	}
	return agent.MCPConfigPath
}

// ---------------------------------------------------------------------------
// InstalledSkill — backward-compatible wrapper for display in the TUI
// ---------------------------------------------------------------------------

// InstalledSkill wraps asset.InstalledAsset for consumers that expect a struct
// with fields: Name, Description, Author, Path, Agents.
type InstalledSkill struct {
	Name        string
	Description string
	Author      string
	Path        string
	Agents      []string
}

// InstalledSkillFromAsset converts an asset.InstalledAsset to InstalledSkill.
func InstalledSkillFromAsset(a asset.InstalledAsset) InstalledSkill {
	return InstalledSkill{
		Name:        a.Name,
		Description: a.Description,
		Author:      a.Author,
		Path:        a.Path,
	}
}

// InstalledSkillsFromAssets converts a slice of InstalledAssets to InstalledSkills.
func InstalledSkillsFromAssets(assets []asset.InstalledAsset) []InstalledSkill {
	result := make([]InstalledSkill, len(assets))
	for i, a := range assets {
		result[i] = InstalledSkillFromAsset(a)
	}
	return result
}

// ---------------------------------------------------------------------------
// LockedSkill / LockedMCP — backward-compatible lock file entry types
// ---------------------------------------------------------------------------

// LockedSkill represents a skill entry in the lock file.
type LockedSkill struct {
	Name   string `json:"name"`
	Source string `json:"source"`
	Commit string `json:"commit"`
	Ref    string `json:"ref,omitempty"`
}

// LockedMCP represents an MCP entry in the lock file.
type LockedMCP struct {
	Name        string   `json:"name"`
	Registry    string   `json:"registry"`
	ConfigHash  string   `json:"configHash"`
	RequiredEnv []string `json:"requiredEnv,omitempty"`
}

// LockFile compat helpers — provide .Skills and .MCPs accessors.

// LockFileForSkills creates a LockFile with both the Skills compat field
// and the canonical Assets field populated from the given locked skills.
func LockFileForSkills(lockVersion int, skills []LockedSkill) *LockFile {
	lf := &LockFile{
		LockVersion: lockVersion,
		Skills:      skills,
	}
	for _, s := range skills {
		lf.Assets = append(lf.Assets, asset.LockedAsset{
			Kind:   asset.KindSkill,
			Name:   s.Name,
			Source: s.Source,
			Commit: s.Commit,
			Ref:    s.Ref,
		})
	}
	return lf
}

// LockedSkills returns all skill entries from the lock file.
func (lf *LockFile) LockedSkills() []LockedSkill {
	if lf == nil {
		return nil
	}
	var result []LockedSkill
	for _, a := range lf.Assets {
		if a.Kind == asset.KindSkill {
			result = append(result, LockedSkill{
				Name:   a.Name,
				Source: a.Source,
				Commit: a.Commit,
				Ref:    a.Ref,
			})
		}
	}
	return result
}

// LockedMCPs returns all MCP entries from the lock file.
func (lf *LockFile) LockedMCPs() []LockedMCP {
	if lf == nil {
		return nil
	}
	var result []LockedMCP
	for _, a := range lf.Assets {
		if a.Kind == asset.KindMCP {
			m := LockedMCP{Name: a.Name}
			if v, ok := a.Data["registry"].(string); ok {
				m.Registry = v
			}
			if v, ok := a.Data["configHash"].(string); ok {
				m.ConfigHash = v
			}
			if envs, ok := a.Data["requiredEnv"].([]interface{}); ok {
				for _, ev := range envs {
					if s, ok := ev.(string); ok {
						m.RequiredEnv = append(m.RequiredEnv, s)
					}
				}
			}
			if envs, ok := a.Data["requiredEnv"].([]string); ok {
				m.RequiredEnv = envs
			}
			result = append(result, m)
		}
	}
	return result
}

// AddOrUpdateLockEntry upserts a locked skill entry.
func AddOrUpdateLockEntry(dir string, entry LockedSkill) error {
	return AddOrUpdateAsset(dir, asset.LockedAsset{
		Kind:   asset.KindSkill,
		Name:   entry.Name,
		Source: entry.Source,
		Commit: entry.Commit,
		Ref:    entry.Ref,
	})
}

// RemoveLockEntry removes a locked skill entry.
func RemoveLockEntry(dir string, name string) error {
	return RemoveAssetEntry(dir, asset.KindSkill, name)
}

// AddOrUpdateMCPLockEntry upserts a locked MCP entry.
func AddOrUpdateMCPLockEntry(dir string, entry LockedMCP) error {
	data := map[string]any{
		"registry":   entry.Registry,
		"configHash": entry.ConfigHash,
	}
	if len(entry.RequiredEnv) > 0 {
		data["requiredEnv"] = entry.RequiredEnv
	}
	return AddOrUpdateAsset(dir, asset.LockedAsset{
		Kind: asset.KindMCP,
		Name: entry.Name,
		Data: data,
	})
}

// RemoveMCPLockEntry removes a locked MCP entry.
func RemoveMCPLockEntry(dir string, name string) error {
	return RemoveAssetEntry(dir, asset.KindMCP, name)
}

// ---------------------------------------------------------------------------
// MCPEntry — backward-compatible flat struct for MCP config data
// ---------------------------------------------------------------------------

// MCPEntry is a flat representation of an MCP configuration, combining
// asset.RegistryEntry identity fields with asset.MCPMeta config fields.
type MCPEntry struct {
	Name        string
	Description string
	Command     string
	Args        []string
	Env         map[string]string
	URL         string
	Type        string // transport type: "http", "sse", "streamable-http"
}

// MCPEntryFromRegistryEntry builds an MCPEntry from an asset.RegistryEntry.
func MCPEntryFromRegistryEntry(e asset.RegistryEntry) MCPEntry {
	entry := MCPEntry{
		Name:        e.Name,
		Description: e.Description,
	}
	if meta, ok := e.Meta.(asset.MCPMeta); ok {
		entry.Command = meta.Command
		entry.Args = meta.Args
		entry.Env = meta.Env
		entry.URL = meta.URL
		entry.Type = meta.Transport
	}
	return entry
}

// ToMCPMeta converts an MCPEntry to asset.MCPMeta for the new APIs.
func (e MCPEntry) ToMCPMeta() asset.MCPMeta {
	return asset.MCPMeta{
		Command:   e.Command,
		Args:      e.Args,
		Env:       e.Env,
		URL:       e.URL,
		Transport: e.Type,
	}
}

// ---------------------------------------------------------------------------
// Installer — backward-compatible wrapper around Orchestrator
// ---------------------------------------------------------------------------

// InstallOptions configures a skill installation.
type InstallOptions struct {
	TargetDir       string
	SkillFilter     string
	IncludeInternal bool
	IsInternal      bool
	TargetAgents    []AgentDef
	Commit          string
}

// InstalledSkillResult represents a single installed skill from an install operation.
type InstalledSkillResult struct {
	Name   string
	Path   string
	Source string
	Commit string
	Ref    string
	Agents []string
}

// InstallResult is the result of an install operation.
type InstallResult struct {
	InstalledSkills []InstalledSkillResult
}

// Installer wraps the Orchestrator for backward-compatible skill installation.
type Installer struct {
	agents []AgentDef
	orch   *Orchestrator
}

// NewInstaller creates an Installer for backward compatibility.
func NewInstaller(agents []AgentDef) *Installer {
	return &Installer{
		agents: agents,
		orch:   NewOrchestrator(),
	}
}

// InstallFromSource installs skills from a parsed source.
func (inst *Installer) InstallFromSource(source *ParsedSource, opts InstallOptions) (*InstallResult, error) {
	var targetSystems []system.System
	if len(opts.TargetAgents) > 0 {
		targetSystems = systemsFromAgentDefs(opts.TargetAgents)
	}

	results, err := inst.orch.InstallFromSource(source, asset.KindSkill, OrchestratorInstallOptions{
		TargetDir:       opts.TargetDir,
		TargetSystems:   targetSystems,
		IncludeInternal: opts.IncludeInternal,
		NameFilter:      opts.SkillFilter,
		Commit:          opts.Commit,
	})
	if err != nil {
		return nil, err
	}

	ir := &InstallResult{}
	for _, r := range results {
		// Build canonical source string.
		src := NormalizeSource(source.Host, source.Owner, source.Repo, "")
		if r.Asset.PreparedPath != "" {
			// Use the asset's source if available.
			src = r.Asset.Source
		}
		if src == "" {
			src = NormalizeSource(source.Host, source.Owner, source.Repo, "")
		}

		ir.InstalledSkills = append(ir.InstalledSkills, InstalledSkillResult{
			Name:   r.Asset.Name,
			Path:   r.Asset.PreparedPath,
			Source: src,
			Commit: r.Commit,
			Ref:    r.Ref,
			Agents: r.Systems,
		})
	}
	return ir, nil
}

// ---------------------------------------------------------------------------
// Remover — backward-compatible wrapper around Orchestrator
// ---------------------------------------------------------------------------

// RemoveOptions configures a skill removal.
type RemoveOptions struct {
	TargetDir string
}

// RemoveResult is the result of a single skill removal.
type RemoveResult struct {
	Name            string
	RemovedSymlinks []string
}

// Remover wraps the Orchestrator for backward-compatible skill removal.
type Remover struct {
	agents []AgentDef
	orch   *Orchestrator
}

// NewRemover creates a Remover for backward compatibility.
func NewRemover(agents []AgentDef) *Remover {
	return &Remover{
		agents: agents,
		orch:   NewOrchestrator(),
	}
}

// Remove removes a single skill by name.
func (rem *Remover) Remove(name string, opts RemoveOptions) (*RemoveResult, error) {
	systems := systemsFromAgentDefs(rem.agents)
	err := rem.orch.RemoveAsset(asset.KindSkill, name, opts.TargetDir, systems)
	if err != nil {
		return nil, err
	}
	return &RemoveResult{Name: name}, nil
}

// RemoveAll removes all installed skills.
func (rem *Remover) RemoveAll(opts RemoveOptions) ([]RemoveResult, error) {
	// Scan to find installed skills.
	allInstalled, err := rem.orch.ScanFolder(opts.TargetDir)
	if err != nil {
		return nil, fmt.Errorf("scanning folder: %w", err)
	}

	skills := allInstalled[asset.KindSkill]
	if len(skills) == 0 {
		return nil, nil
	}

	systems := systemsFromAgentDefs(rem.agents)
	var results []RemoveResult
	for _, s := range skills {
		err := rem.orch.RemoveAsset(asset.KindSkill, s.Name, opts.TargetDir, systems)
		if err != nil {
			return nil, fmt.Errorf("removing %q: %w", s.Name, err)
		}
		results = append(results, RemoveResult{Name: s.Name})
	}
	return results, nil
}

// ---------------------------------------------------------------------------
// Scanner — backward-compatible wrapper around Orchestrator
// ---------------------------------------------------------------------------

// Scanner wraps the Orchestrator for backward-compatible skill scanning.
type Scanner struct {
	agents []AgentDef
	orch   *Orchestrator
}

// NewScanner creates a Scanner for backward compatibility.
func NewScanner(agents []AgentDef) *Scanner {
	return &Scanner{
		agents: agents,
		orch:   NewOrchestrator(),
	}
}

// ScanFolder returns all installed skills in a folder.
func (sc *Scanner) ScanFolder(dir string) ([]InstalledSkill, error) {
	allInstalled, err := sc.orch.ScanFolder(dir)
	if err != nil {
		return nil, err
	}
	return InstalledSkillsFromAssets(allInstalled[asset.KindSkill]), nil
}

// DetectAgents returns the display names of agents active in the given folder.
func (sc *Scanner) DetectAgents(dir string) []string {
	return system.Names(system.DetectInFolder(dir))
}

// ---------------------------------------------------------------------------
// MCP install/uninstall — backward-compatible wrappers
// ---------------------------------------------------------------------------

// MCPInstallOptions configures an MCP installation.
type MCPInstallOptions struct {
	ProjectDir   string
	TargetAgents []AgentDef
	Force        bool
}

// MCPAgentResult is the outcome for a single agent during MCP install/uninstall.
type MCPAgentResult struct {
	Agent      AgentDef
	ConfigPath string
	Action     string // "wrote", "skipped", "removed", "error"
	Message    string
}

// MCPInstallResult is the result of an MCP install operation.
type MCPInstallResult struct {
	AgentResults []MCPAgentResult
}

// InstallMCPConfig writes an MCP config entry into agent config files.
func InstallMCPConfig(mcp MCPEntry, opts MCPInstallOptions) (*MCPInstallResult, error) {
	orch := NewOrchestrator()
	a := asset.Asset{
		Kind:        asset.KindMCP,
		Name:        mcp.Name,
		Description: mcp.Description,
		Meta:        mcp.ToMCPMeta(),
	}

	result := &MCPInstallResult{}
	for _, agent := range opts.TargetAgents {
		sys := agent.sys
		if sys == nil {
			continue
		}
		if !sys.Supports(asset.KindMCP) {
			continue
		}

		configPath := ResolveMCPConfigPathRel(agent, opts.ProjectDir)

		err := sys.Install(a, opts.ProjectDir, system.InstallOptions{
			Force: opts.Force,
		})
		if err != nil {
			if errors.Is(err, system.ErrAlreadyExists) {
				result.AgentResults = append(result.AgentResults, MCPAgentResult{
					Agent:      agent,
					ConfigPath: configPath,
					Action:     "skipped",
					Message:    "already exists",
				})
				continue
			}
			result.AgentResults = append(result.AgentResults, MCPAgentResult{
				Agent:      agent,
				ConfigPath: configPath,
				Action:     "error",
				Message:    err.Error(),
			})
			continue
		}

		result.AgentResults = append(result.AgentResults, MCPAgentResult{
			Agent:      agent,
			ConfigPath: configPath,
			Action:     "wrote",
		})
	}

	_ = orch // Orchestrator available for future use.
	return result, nil
}

// MCPUninstallOptions configures an MCP uninstallation.
type MCPUninstallOptions struct {
	ProjectDir string
}

// MCPUninstallResult is the result of an MCP uninstall operation.
type MCPUninstallResult struct {
	AgentResults []MCPAgentResult
}

// UninstallMCPConfig removes an MCP config entry from agent config files.
func UninstallMCPConfig(name string, agents []AgentDef, opts MCPUninstallOptions) (*MCPUninstallResult, error) {
	result := &MCPUninstallResult{}
	for _, agent := range agents {
		sys := agent.sys
		if sys == nil {
			continue
		}
		if !sys.Supports(asset.KindMCP) {
			continue
		}

		configPath := ResolveMCPConfigPathRel(agent, opts.ProjectDir)

		err := sys.Remove(asset.KindMCP, name, opts.ProjectDir)
		if err != nil {
			result.AgentResults = append(result.AgentResults, MCPAgentResult{
				Agent:      agent,
				ConfigPath: configPath,
				Action:     "error",
				Message:    err.Error(),
			})
			continue
		}

		result.AgentResults = append(result.AgentResults, MCPAgentResult{
			Agent:      agent,
			ConfigPath: configPath,
			Action:     "removed",
		})
	}
	return result, nil
}
