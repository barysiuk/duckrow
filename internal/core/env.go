package core

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	envFileName = ".env.duckrow"
)

// EnvResolver resolves environment variable values for MCP servers.
// It follows the precedence: process env > project .env.duckrow > global .env.duckrow.
type EnvResolver struct {
	projectDir string
	globalDir  string // ~/.duckrow/
}

// NewEnvResolver creates an EnvResolver for the given project directory.
// globalDir defaults to ~/.duckrow/ if empty.
func NewEnvResolver(projectDir, globalDir string) *EnvResolver {
	if globalDir == "" {
		home, _ := os.UserHomeDir()
		globalDir = filepath.Join(home, configDirName)
	}
	return &EnvResolver{
		projectDir: projectDir,
		globalDir:  globalDir,
	}
}

// ResolveEnv resolves the values for the given required env var names.
// Returns a map of var name -> value for vars that were found.
// Vars not found in any source are omitted from the returned map.
//
// Precedence (highest to lowest):
//  1. Process environment (os.LookupEnv)
//  2. Project .env.duckrow (in projectDir)
//  3. Global ~/.duckrow/.env.duckrow
func (r *EnvResolver) ResolveEnv(requiredVars []string) (map[string]string, []string) {
	if len(requiredVars) == 0 {
		return nil, nil
	}

	// Load env files.
	globalEnv := parseEnvFile(filepath.Join(r.globalDir, envFileName))
	projectEnv := parseEnvFile(filepath.Join(r.projectDir, envFileName))

	resolved := make(map[string]string, len(requiredVars))
	var missing []string

	for _, name := range requiredVars {
		// 1. Process environment (highest priority).
		if val, ok := os.LookupEnv(name); ok {
			resolved[name] = val
			continue
		}

		// 2. Project .env.duckrow.
		if val, ok := projectEnv[name]; ok {
			resolved[name] = val
			continue
		}

		// 3. Global .env.duckrow.
		if val, ok := globalEnv[name]; ok {
			resolved[name] = val
			continue
		}

		missing = append(missing, name)
	}

	return resolved, missing
}

// parseEnvFile parses a .env file and returns key-value pairs.
// Returns an empty map if the file does not exist or cannot be read.
// Supports:
//   - KEY=VALUE
//   - KEY="VALUE" (strips outer double quotes)
//   - KEY='VALUE' (strips outer single quotes)
//   - Lines starting with # are comments
//   - Empty lines are skipped
//   - export KEY=VALUE (strips optional export prefix)
func parseEnvFile(path string) map[string]string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()

	env := make(map[string]string)
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments.
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Strip optional "export " prefix.
		line = strings.TrimPrefix(line, "export ")

		// Split on first '='.
		idx := strings.IndexByte(line, '=')
		if idx < 0 {
			continue
		}

		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])

		// Strip surrounding quotes.
		if len(val) >= 2 {
			if (val[0] == '"' && val[len(val)-1] == '"') ||
				(val[0] == '\'' && val[len(val)-1] == '\'') {
				val = val[1 : len(val)-1]
			}
		}

		if key != "" {
			env[key] = val
		}
	}

	return env
}

// EnvSource indicates where an env var value was resolved from.
type EnvSource string

const (
	EnvSourceProcess EnvSource = "process"
	EnvSourceProject EnvSource = "project"
	EnvSourceGlobal  EnvSource = "global"
)

// ResolvedEnvVar holds a resolved env var value and its source.
type ResolvedEnvVar struct {
	Name   string
	Value  string
	Source EnvSource
}

// ResolveEnvWithSource resolves the values for the given required env var names,
// returning both the value and where it was found. Vars not found in any source
// are included with an empty Source.
//
// Precedence (highest to lowest):
//  1. Process environment (os.LookupEnv)
//  2. Project .env.duckrow (in projectDir)
//  3. Global ~/.duckrow/.env.duckrow
func (r *EnvResolver) ResolveEnvWithSource(requiredVars []string) []ResolvedEnvVar {
	if len(requiredVars) == 0 {
		return nil
	}

	// Load env files.
	globalEnv := parseEnvFile(filepath.Join(r.globalDir, envFileName))
	projectEnv := parseEnvFile(filepath.Join(r.projectDir, envFileName))

	results := make([]ResolvedEnvVar, len(requiredVars))
	for i, name := range requiredVars {
		results[i] = ResolvedEnvVar{Name: name}

		// 1. Process environment (highest priority).
		if val, ok := os.LookupEnv(name); ok {
			results[i].Value = val
			results[i].Source = EnvSourceProcess
			continue
		}

		// 2. Project .env.duckrow.
		if val, ok := projectEnv[name]; ok {
			results[i].Value = val
			results[i].Source = EnvSourceProject
			continue
		}

		// 3. Global .env.duckrow.
		if val, ok := globalEnv[name]; ok {
			results[i].Value = val
			results[i].Source = EnvSourceGlobal
			continue
		}
	}

	return results
}

// WriteEnvVar writes or updates a single env var in a .env.duckrow file.
// If the file doesn't exist, it creates it. If the var already exists, its value
// is updated in place.
func WriteEnvVar(dir, name, value string) error {
	path := filepath.Join(dir, envFileName)

	// Ensure parent directory exists.
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}

	// Read existing content.
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading %s: %w", path, err)
	}

	// Quote the value if it contains spaces, # or newlines.
	quotedValue := value
	if strings.ContainsAny(value, " #\t\n\"") {
		quotedValue = `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
	}

	newLine := name + "=" + quotedValue

	if len(data) == 0 {
		// New file — just write the single entry.
		return os.WriteFile(path, []byte(newLine+"\n"), 0o600)
	}

	// Existing file — try to update the var in place.
	lines := strings.Split(string(data), "\n")
	found := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		trimmed = strings.TrimPrefix(trimmed, "export ")
		if idx := strings.IndexByte(trimmed, '='); idx >= 0 {
			k := strings.TrimSpace(trimmed[:idx])
			if k == name {
				lines[i] = newLine
				found = true
				break
			}
		}
	}

	if !found {
		// Append at end, ensuring a newline before it.
		content := string(data)
		if !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		content += newLine + "\n"
		return os.WriteFile(path, []byte(content), 0o600)
	}

	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o600)
}

// EnsureGitignore adds .env.duckrow to the project's .gitignore if not already present.
// Creates the .gitignore file if it does not exist.
func EnsureGitignore(projectDir string) error {
	gitignorePath := filepath.Join(projectDir, ".gitignore")

	// Check if .gitignore exists and already contains .env.duckrow.
	data, err := os.ReadFile(gitignorePath)
	if err == nil {
		// File exists — check if it already contains the entry.
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) == envFileName {
				return nil // Already present.
			}
		}
		// Append to existing file.
		content := string(data)
		if !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		content += envFileName + "\n"
		return os.WriteFile(gitignorePath, []byte(content), 0o644)
	}

	if !os.IsNotExist(err) {
		return fmt.Errorf("reading .gitignore: %w", err)
	}

	// Create new .gitignore with just the env file entry.
	return os.WriteFile(gitignorePath, []byte(envFileName+"\n"), 0o644)
}
