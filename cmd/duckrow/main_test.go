package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/barysiuk/duckrow/cmd/duckrow/cmd"
	"github.com/rogpeppe/go-internal/testscript"
)

func TestMain(m *testing.M) {
	testscript.Main(m, map[string]func(){
		"duckrow": func() {
			if err := cmd.Execute(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	})
}

func TestScript(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir:                 filepath.Join("testdata", "script"),
		RequireExplicitExec: true,
		Setup: func(e *testscript.Env) error {
			// Set HOME to WORK so ~/.duckrow/ is created inside the temp dir
			e.Vars = append(e.Vars, "HOME="+e.WorkDir)
			return nil
		},
		Cmds: map[string]func(ts *testscript.TestScript, neg bool, args []string){
			// is-symlink asserts that a path is (or is not) a symlink.
			// Usage: [!] is-symlink <path>
			"is-symlink": cmdIsSymlink,

			// file-contains asserts that a file contains (or doesn't contain) a substring.
			// Usage: [!] file-contains <path> <substring>
			"file-contains": cmdFileContains,

			// setup-git-repo creates a local git repo with a duckrow.json manifest.
			// Usage: setup-git-repo <dir> <registry-name> [skill-name...]
			// Creates a git repo at <dir> with a manifest containing the given skills.
			"setup-git-repo": cmdSetupGitRepo,

			// dir-not-exists asserts that a directory does not exist.
			// Usage: [!] dir-not-exists <path>
			"dir-not-exists": cmdDirNotExists,

			// setup-config-override writes a config.json with a clone URL override.
			// Usage: setup-config-override <repo-key> <clone-url>
			// Creates ~/.duckrow/config.json with the override mapping.
			// <repo-key> is "owner/repo" and <clone-url> is the target URL.
			"setup-config-override": cmdSetupConfigOverride,

			// setup-registry-config adds a clone URL override to the existing config
			// (preserving any registries from prior 'duckrow registry add' calls).
			// Usage: setup-registry-config <override-key> <override-url>
			"setup-registry-config": cmdSetupRegistryConfig,

			// setup-mcp-registry creates a git repo with a duckrow.json containing MCP entries.
			// Usage: setup-mcp-registry <dir> <registry-name> <mcp-spec> [mcp-spec...]
			// Stdio:  "name:command" or "name:command:ENV1,ENV2"
			// Remote: "name:remote:url" or "name:remote:url:type"
			"setup-mcp-registry": cmdSetupMCPRegistry,

			// write-env-file writes key=value pairs to a .env.duckrow file.
			// Usage: write-env-file <dir> <key>=<value> [key=value...]
			"write-env-file": cmdWriteEnvFile,
		},
	})
}

// cmdIsSymlink checks if a path is a symlink.
func cmdIsSymlink(ts *testscript.TestScript, neg bool, args []string) {
	if len(args) != 1 {
		ts.Fatalf("usage: is-symlink <path>")
	}
	path := ts.MkAbs(args[0])
	fi, err := os.Lstat(path)
	isSymlink := err == nil && fi.Mode()&os.ModeSymlink != 0

	if neg {
		if isSymlink {
			ts.Fatalf("%s is a symlink (expected not to be)", args[0])
		}
	} else {
		if !isSymlink {
			if err != nil {
				ts.Fatalf("%s: %v", args[0], err)
			}
			ts.Fatalf("%s is not a symlink (mode: %s)", args[0], fi.Mode())
		}
	}
}

// cmdFileContains checks if a file contains a substring.
func cmdFileContains(ts *testscript.TestScript, neg bool, args []string) {
	if len(args) < 2 {
		ts.Fatalf("usage: file-contains <path> <substring>")
	}
	path := ts.MkAbs(args[0])
	substr := args[1]

	data, err := os.ReadFile(path)
	if err != nil {
		ts.Fatalf("reading %s: %v", args[0], err)
	}

	contains := containsString(string(data), substr)
	if neg {
		if contains {
			ts.Fatalf("file %s contains %q (expected not to)", args[0], substr)
		}
	} else {
		if !contains {
			ts.Fatalf("file %s does not contain %q\nContent:\n%s", args[0], substr, string(data))
		}
	}
}

// cmdDirNotExists checks that a directory does not exist.
func cmdDirNotExists(ts *testscript.TestScript, neg bool, args []string) {
	if len(args) != 1 {
		ts.Fatalf("usage: dir-not-exists <path>")
	}
	path := ts.MkAbs(args[0])
	_, err := os.Stat(path)
	doesNotExist := os.IsNotExist(err)

	if neg {
		// ! dir-not-exists == dir exists
		if doesNotExist {
			ts.Fatalf("%s does not exist (expected it to exist)", args[0])
		}
	} else {
		if !doesNotExist {
			ts.Fatalf("%s exists (expected it not to)", args[0])
		}
	}
}

// cmdSetupGitRepo creates a local git repo with a duckrow.json manifest.
func cmdSetupGitRepo(ts *testscript.TestScript, neg bool, args []string) {
	if neg {
		ts.Fatalf("setup-git-repo does not support negation")
	}
	if len(args) < 2 {
		ts.Fatalf("usage: setup-git-repo <dir> <registry-name> [skill-name...]")
	}

	dir := ts.MkAbs(args[0])
	registryName := args[1]
	skillNames := args[2:]

	if err := os.MkdirAll(dir, 0o755); err != nil {
		ts.Fatalf("creating dir: %v", err)
	}

	// Build manifest
	type skillEntry struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Source      string `json:"source"`
		Version     string `json:"version,omitempty"`
	}
	type manifest struct {
		Name        string       `json:"name"`
		Description string       `json:"description,omitempty"`
		Skills      []skillEntry `json:"skills"`
	}

	m := manifest{
		Name:        registryName,
		Description: registryName + " skills",
	}
	for _, name := range skillNames {
		m.Skills = append(m.Skills, skillEntry{
			Name:        name,
			Description: "Skill " + name,
			Source:      registryName + "/" + name,
			Version:     "1.0.0",
		})
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		ts.Fatalf("marshaling manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "duckrow.json"), data, 0o644); err != nil {
		ts.Fatalf("writing manifest: %v", err)
	}

	// Initialize git repo
	gitEnv := append(os.Environ(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)

	runGit := func(gitArgs ...string) {
		c := exec.Command("git", gitArgs...)
		c.Dir = dir
		c.Env = gitEnv
		out, err := c.CombinedOutput()
		if err != nil {
			ts.Fatalf("git %v: %v\n%s", gitArgs, err, out)
		}
	}

	runGit("init")
	runGit("checkout", "-b", "main")
	runGit("add", ".")
	runGit("commit", "-m", "initial")
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// cmdSetupConfigOverride creates a config.json with a clone URL override.
func cmdSetupConfigOverride(ts *testscript.TestScript, neg bool, args []string) {
	if neg {
		ts.Fatalf("setup-config-override does not support negation")
	}
	if len(args) != 2 {
		ts.Fatalf("usage: setup-config-override <repo-key> <clone-url>")
	}

	repoKey := args[0]
	cloneURL := ts.MkAbs(args[1]) // Resolve relative paths to absolute

	// Config lives at $HOME/.duckrow/config.json (HOME is set to WORK in setup)
	home := ts.Getenv("HOME")
	configDir := filepath.Join(home, ".duckrow")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		ts.Fatalf("creating config dir: %v", err)
	}

	type settings struct {
		AutoAddCurrentDir bool              `json:"autoAddCurrentDir"`
		CloneURLOverrides map[string]string `json:"cloneURLOverrides,omitempty"`
	}
	type config struct {
		Folders    []interface{} `json:"folders"`
		Registries []interface{} `json:"registries"`
		Settings   settings      `json:"settings"`
	}

	cfg := config{
		Folders:    []interface{}{},
		Registries: []interface{}{},
		Settings: settings{
			AutoAddCurrentDir: true,
			CloneURLOverrides: map[string]string{
				repoKey: cloneURL,
			},
		},
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		ts.Fatalf("marshaling config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.json"), data, 0o644); err != nil {
		ts.Fatalf("writing config: %v", err)
	}
}

// cmdSetupRegistryConfig adds a clone URL override to the existing config,
// preserving any registries that were previously added via `duckrow registry add`.
// Usage: setup-registry-config <override-key> <override-url>
func cmdSetupRegistryConfig(ts *testscript.TestScript, neg bool, args []string) {
	if neg {
		ts.Fatalf("setup-registry-config does not support negation")
	}
	if len(args) != 2 {
		ts.Fatalf("usage: setup-registry-config <override-key> <override-url>")
	}

	overrideKey := args[0]
	overrideURL := ts.MkAbs(args[1])

	home := ts.Getenv("HOME")
	configDir := filepath.Join(home, ".duckrow")
	configPath := filepath.Join(configDir, "config.json")

	// Read existing config (must exist from prior registry add)
	data, err := os.ReadFile(configPath)
	if err != nil {
		ts.Fatalf("reading existing config: %v (run 'duckrow registry add' first)", err)
	}

	var cfg map[string]interface{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		ts.Fatalf("parsing config: %v", err)
	}

	// Get or create settings
	settings, ok := cfg["settings"].(map[string]interface{})
	if !ok {
		settings = map[string]interface{}{}
	}

	// Get or create overrides map
	overrides, ok := settings["cloneURLOverrides"].(map[string]interface{})
	if !ok {
		overrides = map[string]interface{}{}
	}
	overrides[overrideKey] = overrideURL
	settings["cloneURLOverrides"] = overrides
	cfg["settings"] = settings

	data, err = json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		ts.Fatalf("marshaling config: %v", err)
	}
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		ts.Fatalf("writing config: %v", err)
	}
}

// cmdSetupMCPRegistry creates a local git repo with a duckrow.json manifest containing MCP entries.
// Usage: setup-mcp-registry <dir> <registry-name> <mcp-spec> [mcp-spec...]
//
// Stdio MCP spec:  "name:command" or "name:command:ENV1,ENV2"
// Remote MCP spec: "name:remote:url" or "name:remote:url:type" (type defaults to "http")
func cmdSetupMCPRegistry(ts *testscript.TestScript, neg bool, args []string) {
	if neg {
		ts.Fatalf("setup-mcp-registry does not support negation")
	}
	if len(args) < 3 {
		ts.Fatalf("usage: setup-mcp-registry <dir> <registry-name> <mcp-spec> [mcp-spec...]")
	}

	dir := ts.MkAbs(args[0])
	registryName := args[1]
	mcpSpecs := args[2:]

	if err := os.MkdirAll(dir, 0o755); err != nil {
		ts.Fatalf("creating dir: %v", err)
	}

	// Build manifest with MCPs
	type mcpEntry struct {
		Name        string   `json:"name"`
		Description string   `json:"description,omitempty"`
		Command     string   `json:"command,omitempty"`
		Args        []string `json:"args,omitempty"`
		Env         []string `json:"env,omitempty"`
		URL         string   `json:"url,omitempty"`
		Type        string   `json:"type,omitempty"`
	}
	type manifest struct {
		Name        string     `json:"name"`
		Description string     `json:"description,omitempty"`
		Skills      []struct{} `json:"skills"`
		MCPs        []mcpEntry `json:"mcps"`
	}

	m := manifest{
		Name:        registryName,
		Description: registryName + " registry",
		Skills:      []struct{}{},
	}

	for _, spec := range mcpSpecs {
		parts := splitN(spec, ":", 3)
		if len(parts) < 2 {
			ts.Fatalf("invalid mcp-spec %q: expected name:command[:ENV1,ENV2] or name:remote:url[:type]", spec)
		}
		name := parts[0]

		if parts[1] == "remote" {
			// Remote MCP: name:remote:url[:type]
			// The URL may contain colons (e.g., https://...), so we split
			// the rest into URL and optional transport by looking for a
			// known transport suffix after the last colon.
			if len(parts) < 3 {
				ts.Fatalf("invalid remote mcp-spec %q: expected name:remote:url[:type]", spec)
			}
			rest := parts[2]
			transport := "http"
			url := rest
			// Check if the rest ends with :type where type is a known transport
			if lastColon := lastIndexOf(rest, ":"); lastColon >= 0 {
				suffix := rest[lastColon+1:]
				if suffix == "http" || suffix == "sse" || suffix == "streamable-http" {
					transport = suffix
					url = rest[:lastColon]
				}
			}
			m.MCPs = append(m.MCPs, mcpEntry{
				Name:        name,
				Description: name + " remote MCP server",
				URL:         url,
				Type:        transport,
			})
		} else {
			// Stdio MCP: name:command[:ENV1,ENV2]
			command := parts[1]
			entry := mcpEntry{
				Name:        name,
				Description: name + " MCP server",
				Command:     command,
			}

			// Parse env vars if present
			if len(parts) >= 3 && parts[2] != "" {
				envVars := splitN(parts[2], ",", -1)
				entry.Env = envVars
			}

			m.MCPs = append(m.MCPs, entry)
		}
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		ts.Fatalf("marshaling manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "duckrow.json"), data, 0o644); err != nil {
		ts.Fatalf("writing manifest: %v", err)
	}

	// Initialize git repo
	gitEnv := append(os.Environ(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)

	runGit := func(gitArgs ...string) {
		c := exec.Command("git", gitArgs...)
		c.Dir = dir
		c.Env = gitEnv
		out, err := c.CombinedOutput()
		if err != nil {
			ts.Fatalf("git %v: %v\n%s", gitArgs, err, out)
		}
	}

	runGit("init")
	runGit("checkout", "-b", "main")
	runGit("add", ".")
	runGit("commit", "-m", "initial")
}

// cmdWriteEnvFile writes key=value pairs to a .env.duckrow file.
// Usage: write-env-file <dir> <key>=<value> [key=value...]
func cmdWriteEnvFile(ts *testscript.TestScript, neg bool, args []string) {
	if neg {
		ts.Fatalf("write-env-file does not support negation")
	}
	if len(args) < 2 {
		ts.Fatalf("usage: write-env-file <dir> <key>=<value> [key=value...]")
	}

	dir := ts.MkAbs(args[0])
	pairs := args[1:]

	envPath := filepath.Join(dir, ".env.duckrow")

	var content string
	for _, pair := range pairs {
		content += pair + "\n"
	}

	if err := os.WriteFile(envPath, []byte(content), 0o644); err != nil {
		ts.Fatalf("writing env file: %v", err)
	}
}

// splitN is a helper that splits a string by sep into at most n parts.
// If n < 0, returns all parts.
func splitN(s, sep string, n int) []string {
	if n < 0 {
		// Split all
		var parts []string
		for {
			idx := indexOf(s, sep)
			if idx < 0 {
				parts = append(parts, s)
				break
			}
			parts = append(parts, s[:idx])
			s = s[idx+len(sep):]
		}
		return parts
	}

	var parts []string
	for i := 0; i < n-1; i++ {
		idx := indexOf(s, sep)
		if idx < 0 {
			break
		}
		parts = append(parts, s[:idx])
		s = s[idx+len(sep):]
	}
	parts = append(parts, s)
	return parts
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func lastIndexOf(s, substr string) int {
	for i := len(s) - len(substr); i >= 0; i-- {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
