package system

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/barysiuk/duckrow/internal/core/asset"
	"github.com/tailscale/hujson"
)

// BaseSystem provides default implementations for common system patterns.
// Individual systems embed this and override methods as needed.
type BaseSystem struct {
	name            string
	displayName     string
	universal       bool         // shares .agents/skills/ directly
	skillsDir       string       // project-relative skill directory
	altSkillsDirs   []string     // additional native skill directories
	globalSkillsDir string       // global skill directory (with ~ or $VAR)
	detectPaths     []string     // files/dirs to check for global installation
	configSignals   []string     // project files indicating active use
	supportedKinds  []asset.Kind // asset kinds this system supports

	// MCP config (for systems that support MCP)
	mcpConfigPath    string // project-relative MCP config file
	mcpConfigPathAlt string // alternative config path checked first
	mcpConfigKey     string // JSON key in config (e.g., "mcpServers")
	mcpConfigFormat  string // "jsonc" or "" (strict JSON)
}

func (b *BaseSystem) Name() string        { return b.name }
func (b *BaseSystem) DisplayName() string { return b.displayName }
func (b *BaseSystem) IsUniversal() bool   { return b.universal }

func (b *BaseSystem) Supports(kind asset.Kind) bool {
	for _, k := range b.supportedKinds {
		if k == kind {
			return true
		}
	}
	return false
}

func (b *BaseSystem) SupportedKinds() []asset.Kind {
	return b.supportedKinds
}

func (b *BaseSystem) IsInstalled() bool {
	for _, p := range b.detectPaths {
		if dirExists(expandPath(p)) {
			return true
		}
	}
	return false
}

func (b *BaseSystem) IsActiveInFolder(folderPath string) bool {
	for _, sig := range b.configSignals {
		p := filepath.Join(folderPath, sig)
		if pathExists(p) {
			return true
		}
	}
	// Also check if the skill directory exists (duckrow-managed presence).
	skillDir := filepath.Join(folderPath, b.skillsDir)
	if dirExists(skillDir) {
		return true
	}
	// Check alternative skill directories (e.g. .opencode/skills/).
	for _, alt := range b.altSkillsDirs {
		if dirExists(filepath.Join(folderPath, alt)) {
			return true
		}
	}
	return false
}

func (b *BaseSystem) DetectionSignals() []string {
	return b.configSignals
}

func (b *BaseSystem) AssetDir(kind asset.Kind, projectDir string) string {
	if kind == asset.KindSkill {
		return filepath.Join(projectDir, b.skillsDir)
	}
	return ""
}

// SkillsDir returns the project-relative skill directory path.
func (b *BaseSystem) SkillsDir() string { return b.skillsDir }

// AltSkillsDirs returns additional native skill directories.
func (b *BaseSystem) AltSkillsDirs() []string { return b.altSkillsDirs }

// GlobalSkillsDir returns the resolved global skill directory path.
func (b *BaseSystem) GlobalSkillsDir() string { return expandPath(b.globalSkillsDir) }

// DetectPaths returns the global detection paths (expanded).
func (b *BaseSystem) DetectPaths() []string {
	result := make([]string, len(b.detectPaths))
	for i, p := range b.detectPaths {
		result[i] = expandPath(p)
	}
	return result
}

// MCPConfigPath returns the project-relative MCP config file path.
func (b *BaseSystem) MCPConfigPath() string { return b.mcpConfigPath }

// MCPConfigPathAlt returns the alternative project-relative MCP config file path.
func (b *BaseSystem) MCPConfigPathAlt() string { return b.mcpConfigPathAlt }

// MCPConfigKey returns the JSON key used for MCP entries.
func (b *BaseSystem) MCPConfigKey() string { return b.mcpConfigKey }

// Install dispatches to kind-specific methods.
func (b *BaseSystem) Install(a asset.Asset, projectDir string, opts InstallOptions) error {
	switch a.Kind {
	case asset.KindSkill:
		return b.installSkill(a, projectDir, opts)
	case asset.KindMCP:
		return b.installMCP(a, projectDir, opts)
	default:
		return fmt.Errorf("system %s does not support asset kind %s", b.name, a.Kind)
	}
}

// Remove dispatches to kind-specific removal methods.
func (b *BaseSystem) Remove(kind asset.Kind, name string, projectDir string) error {
	switch kind {
	case asset.KindSkill:
		return b.removeSkill(name, projectDir)
	case asset.KindMCP:
		return b.removeMCP(name, projectDir)
	default:
		return fmt.Errorf("system %s does not support asset kind %s", b.name, kind)
	}
}

// Scan finds installed assets of the given kind in the project directory.
func (b *BaseSystem) Scan(kind asset.Kind, projectDir string) ([]asset.InstalledAsset, error) {
	switch kind {
	case asset.KindSkill:
		return b.scanSkills(projectDir)
	default:
		return nil, nil
	}
}

// --- Skill Installation ---

// installSkill handles the default skill installation.
// Universal systems: files already in .agents/skills/, nothing extra needed.
// Non-universal systems: create a symlink from their skillsDir to the
// canonical location, falling back to a full copy if symlink fails.
func (b *BaseSystem) installSkill(a asset.Asset, projectDir string, _ InstallOptions) error {
	if b.universal {
		// Universal systems read from .agents/skills/ directly.
		// The orchestrator handles copying to the canonical location.
		return nil
	}

	// Non-universal: create symlink from this system's skill dir to canonical.
	canonicalDir := filepath.Join(projectDir, ".agents/skills", sanitizeName(a.Name))
	agentSkillDir := filepath.Join(projectDir, b.skillsDir)

	if err := os.MkdirAll(agentSkillDir, 0o755); err != nil {
		return fmt.Errorf("creating skill dir for %s: %w", b.displayName, err)
	}

	linkPath := filepath.Join(agentSkillDir, sanitizeName(a.Name))

	// Remove existing link/dir if present.
	_ = os.RemoveAll(linkPath)

	// Create relative symlink.
	rel, err := filepath.Rel(agentSkillDir, canonicalDir)
	if err != nil {
		return fmt.Errorf("computing relative path for %s: %w", b.displayName, err)
	}

	if err := os.Symlink(rel, linkPath); err != nil {
		// Fall back to copy if symlink fails.
		if copyErr := copyDirectory(canonicalDir, linkPath); copyErr != nil {
			return fmt.Errorf("symlink and copy both failed for %s: symlink: %w, copy: %v",
				b.displayName, err, copyErr)
		}
	}

	return nil
}

// removeSkill removes a skill from this system's skill directory.
func (b *BaseSystem) removeSkill(name string, projectDir string) error {
	if b.universal {
		// Universal systems: the orchestrator removes from .agents/skills/ directly.
		return nil
	}

	linkPath := filepath.Join(projectDir, b.skillsDir, name)
	if !pathExists(linkPath) {
		return nil // nothing to remove
	}

	if err := os.RemoveAll(linkPath); err != nil {
		return fmt.Errorf("removing %s skill for %s: %w", name, b.displayName, err)
	}

	// Clean up empty skills directory, then its parent (e.g. .cursor/skills/ → .cursor/).
	skillDir := filepath.Join(projectDir, b.skillsDir)
	cleanupEmptyDir(skillDir)
	cleanupEmptyDir(filepath.Dir(skillDir))

	return nil
}

// scanSkills finds skills installed for this system.
func (b *BaseSystem) scanSkills(projectDir string) ([]asset.InstalledAsset, error) {
	dirs := append([]string{b.skillsDir}, b.altSkillsDirs...)
	seen := make(map[string]bool)
	var result []asset.InstalledAsset

	for _, relDir := range dirs {
		absDir := filepath.Join(projectDir, relDir)
		entries, err := os.ReadDir(absDir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			skillPath := filepath.Join(absDir, entry.Name())
			skillMdPath := filepath.Join(skillPath, "SKILL.md")

			handler, ok := asset.Get(asset.KindSkill)
			if !ok {
				continue
			}

			meta, err := handler.Parse(skillPath)
			if err != nil {
				continue
			}

			skillMeta, ok := meta.(asset.SkillMeta)
			if !ok {
				continue
			}

			// Use the dir name for dedup. We parse the SKILL.md for the display name.
			// Read the frontmatter to get name and description.
			fm, err := parseSkillMdForScan(skillMdPath)
			if err != nil {
				continue
			}

			if seen[fm.name] {
				continue
			}
			seen[fm.name] = true

			result = append(result, asset.InstalledAsset{
				Kind:        asset.KindSkill,
				Name:        fm.name,
				Description: fm.description,
				Author:      skillMeta.Author,
				Path:        skillPath,
				Meta:        skillMeta,
				SystemName:  b.name,
			})
		}
	}

	// Sort by name for deterministic output.
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result, nil
}

// parseSkillMdForScan is a lightweight reader that extracts name and description
// from a SKILL.md frontmatter without going through the full handler.
type skillScanInfo struct {
	name        string
	description string
}

func parseSkillMdForScan(path string) (*skillScanInfo, error) {
	// Reuse the handler's Parse, but we need name/description which are in the
	// frontmatter but not in SkillMeta. Read the YAML directly.
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	// Quick frontmatter parse.
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	content := string(data)
	if !strings.HasPrefix(strings.TrimSpace(content), "---") {
		return nil, fmt.Errorf("no frontmatter")
	}

	// Find end of frontmatter.
	idx := strings.Index(content[4:], "\n---")
	if idx < 0 {
		return nil, fmt.Errorf("no closing frontmatter")
	}
	fmContent := content[4 : 4+idx]

	// Simple line-based extraction for name and description.
	var name, desc string
	for _, line := range strings.Split(fmContent, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "name:") {
			name = strings.TrimSpace(strings.TrimPrefix(line, "name:"))
			name = strings.Trim(name, "\"'")
		}
		if strings.HasPrefix(line, "description:") {
			desc = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
			desc = strings.Trim(desc, "\"'")
		}
	}

	if name == "" {
		return nil, fmt.Errorf("no name found")
	}

	return &skillScanInfo{name: name, description: desc}, nil
}

// --- MCP Installation ---

// installMCP writes an MCP config entry into this system's config file.
// This is the default implementation; systems with unique formats override it.
func (b *BaseSystem) installMCP(a asset.Asset, projectDir string, opts InstallOptions) error {
	if b.mcpConfigPath == "" {
		return fmt.Errorf("system %s does not support MCP configuration", b.displayName)
	}

	meta, ok := a.Meta.(asset.MCPMeta)
	if !ok {
		return fmt.Errorf("expected MCPMeta, got %T", a.Meta)
	}

	configPath := b.resolveMCPConfigPath(projectDir)

	// Read existing config file content (or start with empty object).
	content, err := readConfigFile(configPath)
	if err != nil {
		return fmt.Errorf("reading config: %w", err)
	}
	if content == "" {
		content = "{}"
	}

	// Parse as JSONC/JWCC AST — preserves comments and whitespace.
	root, err := hujson.Parse([]byte(content))
	if err != nil {
		return fmt.Errorf("parsing config: %w", err)
	}

	// Check if entry already exists.
	entryPtr := "/" + jsonPointerEscape(b.mcpConfigKey) + "/" + jsonPointerEscape(a.Name)
	if root.Find(entryPtr) != nil && !opts.Force {
		return ErrAlreadyExists
	}

	// Build the MCP config value as JSON.
	mcpValueJSON := b.buildMCPConfig(meta)

	// Determine the patch operation.
	op := "add"
	if root.Find(entryPtr) != nil {
		op = "replace"
	}

	// Ensure the top-level config key object exists.
	topKeyPtr := "/" + jsonPointerEscape(b.mcpConfigKey)
	if root.Find(topKeyPtr) == nil {
		topKeyPatch := fmt.Sprintf(`[{"op":"add","path":%q,"value":{}}]`, topKeyPtr)
		if err := root.Patch([]byte(topKeyPatch)); err != nil {
			return fmt.Errorf("creating config key %q: %w", b.mcpConfigKey, err)
		}
	}

	// Apply the patch.
	patch := fmt.Sprintf(`[{"op":%q,"path":%q,"value":%s}]`, op, entryPtr, mcpValueJSON)
	if err := root.Patch([]byte(patch)); err != nil {
		return fmt.Errorf("writing MCP entry: %w", err)
	}

	// Format and finalize.
	output := b.finalizeConfig(&root)

	return writeConfigFile(configPath, string(output))
}

// removeMCP removes an MCP entry from this system's config file.
func (b *BaseSystem) removeMCP(name string, projectDir string) error {
	if b.mcpConfigPath == "" {
		return nil
	}

	configPath := b.resolveMCPConfigPath(projectDir)

	content, err := readConfigFile(configPath)
	if err != nil {
		return fmt.Errorf("reading config: %w", err)
	}
	if content == "" {
		return nil // no config file
	}

	root, err := hujson.Parse([]byte(content))
	if err != nil {
		return fmt.Errorf("parsing config: %w", err)
	}

	entryPtr := "/" + jsonPointerEscape(b.mcpConfigKey) + "/" + jsonPointerEscape(name)
	if root.Find(entryPtr) == nil {
		return nil // entry not found
	}

	patch := fmt.Sprintf(`[{"op":"remove","path":%q}]`, entryPtr)
	if err := root.Patch([]byte(patch)); err != nil {
		return fmt.Errorf("removing MCP entry: %w", err)
	}

	output := b.finalizeConfig(&root)
	return writeConfigFile(configPath, string(output))
}

// buildMCPConfig produces the default MCP JSON value for stdio MCPs.
// Systems with custom formats override Install() instead.
func (b *BaseSystem) buildMCPConfig(meta asset.MCPMeta) string {
	if meta.IsStdio() {
		// Default: { "command": "duckrow", "args": ["env", "--mcp", ...] }
		wrapperArgs := []string{"env", "--mcp", meta.Command, "--"}
		wrapperArgs = append(wrapperArgs, meta.Command)
		wrapperArgs = append(wrapperArgs, meta.Args...)

		m := map[string]interface{}{
			"command": "duckrow",
			"args":    wrapperArgs,
		}
		data, _ := json.MarshalIndent(m, "\t\t", "\t")
		return string(data)
	}

	// Remote MCP.
	mcpType := meta.Transport
	if mcpType == "" {
		mcpType = "http"
	}
	m := map[string]interface{}{
		"type": mcpType,
		"url":  meta.URL,
	}
	data, _ := json.MarshalIndent(m, "\t\t", "\t")
	return string(data)
}

// resolveMCPConfigPath resolves the full path to the MCP config file,
// checking the alternative path first.
func (b *BaseSystem) resolveMCPConfigPath(projectDir string) string {
	if b.mcpConfigPathAlt != "" {
		altPath := filepath.Join(projectDir, b.mcpConfigPathAlt)
		if _, err := os.Stat(altPath); err == nil {
			return altPath
		}
	}
	return filepath.Join(projectDir, b.mcpConfigPath)
}

// ResolveMCPConfigPathRel returns the project-relative config path,
// checking for the alternative path on disk first.
func (b *BaseSystem) ResolveMCPConfigPathRel(projectDir string) string {
	if b.mcpConfigPath == "" {
		return ""
	}
	if b.mcpConfigPathAlt != "" {
		altPath := filepath.Join(projectDir, b.mcpConfigPathAlt)
		if _, err := os.Stat(altPath); err == nil {
			return b.mcpConfigPathAlt
		}
	}
	return b.mcpConfigPath
}

// finalizeConfig formats the JSONC AST and produces final output bytes.
func (b *BaseSystem) finalizeConfig(root *hujson.Value) []byte {
	root.Format()
	removeTrailingCommas(root)

	if b.mcpConfigFormat != "jsonc" {
		root.Standardize()
	}

	return root.Pack()
}

// --- Shared Helpers ---

// expandPath expands ~ to home directory and $VAR / $XDG_CONFIG to env values.
func expandPath(p string) string {
	// Handle $XDG_CONFIG
	if strings.Contains(p, "$XDG_CONFIG") {
		xdgConfig := os.Getenv("XDG_CONFIG_HOME")
		if xdgConfig == "" {
			home, _ := os.UserHomeDir()
			xdgConfig = filepath.Join(home, ".config")
		}
		p = strings.ReplaceAll(p, "$XDG_CONFIG", xdgConfig)
	}

	// Handle other env vars.
	if strings.Contains(p, "$") {
		p = os.Expand(p, func(key string) string {
			if key == "XDG_CONFIG" {
				return ""
			}
			return os.Getenv(key)
		})
	}

	// Handle ~
	if strings.HasPrefix(p, "~/") {
		home, _ := os.UserHomeDir()
		p = filepath.Join(home, p[2:])
	} else if p == "~" {
		home, _ := os.UserHomeDir()
		p = home
	}

	return p
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func pathExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func sanitizeName(name string) string {
	name = strings.ToLower(name)
	// Replace non-alphanumeric chars (except hyphen) with hyphen.
	var result []byte
	for i := 0; i < len(name); i++ {
		c := name[i]
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' {
			result = append(result, c)
		} else {
			result = append(result, '-')
		}
	}
	name = strings.Trim(string(result), "-.")
	if len(name) > 255 {
		name = name[:255]
	}
	if name == "" {
		name = "unnamed-skill"
	}
	return name
}

func cleanupEmptyDir(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	if len(entries) == 0 {
		_ = os.Remove(dir)
	}
}

// readConfigFile reads a config file. Returns empty string if not found.
func readConfigFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

// writeConfigFile writes content atomically, creating parent directories.
func writeConfigFile(path string, content string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("renaming temp file: %w", err)
	}
	return nil
}

// jsonPointerEscape escapes a string for use as a JSON Pointer token (RFC 6901).
func jsonPointerEscape(s string) string {
	result := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '~':
			result = append(result, '~', '0')
		case '/':
			result = append(result, '~', '1')
		default:
			result = append(result, s[i])
		}
	}
	return string(result)
}

// removeTrailingCommas walks the JSONC AST and removes trailing commas.
func removeTrailingCommas(v *hujson.Value) {
	switch vv := v.Value.(type) {
	case *hujson.Object:
		for i := range vv.Members {
			removeTrailingCommas(&vv.Members[i].Name)
			removeTrailingCommas(&vv.Members[i].Value)
		}
		if len(vv.Members) > 0 {
			vv.Members[len(vv.Members)-1].Value.AfterExtra = nil
		}
	case *hujson.Array:
		for i := range vv.Elements {
			removeTrailingCommas(&vv.Elements[i])
		}
		if len(vv.Elements) > 0 {
			vv.Elements[len(vv.Elements)-1].AfterExtra = nil
		}
	}
}

// copyDirectory copies the contents of src to dst, excluding certain files.
func copyDirectory(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		baseName := filepath.Base(path)
		// Skip excluded files.
		switch baseName {
		case "README.md", "metadata.json", ".git":
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		// Skip files/dirs starting with _.
		if strings.HasPrefix(baseName, "_") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		dstPath := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(dstPath, 0o755)
		}

		return copyFile(path, dstPath)
	})
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, info.Mode())
}

// parseJSONC parses a JSONC string into a hujson.Value.
func parseJSONC(content string) (*hujson.Value, error) {
	root, err := hujson.Parse([]byte(content))
	if err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	return &root, nil
}

// patchAndWrite is a shared helper for systems that override MCP installation.
// It ensures the top-level key exists, applies an add/replace patch, and writes.
func (b *BaseSystem) patchAndWrite(root *hujson.Value, entryPtr, valueJSON, configPath string) error {
	op := "add"
	if root.Find(entryPtr) != nil {
		op = "replace"
	}

	// Ensure the top-level config key object exists.
	topKeyPtr := "/" + jsonPointerEscape(b.mcpConfigKey)
	if root.Find(topKeyPtr) == nil {
		topKeyPatch := fmt.Sprintf(`[{"op":"add","path":%q,"value":{}}]`, topKeyPtr)
		if err := root.Patch([]byte(topKeyPatch)); err != nil {
			return fmt.Errorf("creating config key %q: %w", b.mcpConfigKey, err)
		}
	}

	patch := fmt.Sprintf(`[{"op":%q,"path":%q,"value":%s}]`, op, entryPtr, valueJSON)
	if err := root.Patch([]byte(patch)); err != nil {
		return fmt.Errorf("writing MCP entry: %w", err)
	}

	output := b.finalizeConfig(root)
	return writeConfigFile(configPath, string(output))
}
