package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// parseEnvFile tests
// ---------------------------------------------------------------------------

func TestParseEnvFile_BasicKeyValue(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env.duckrow")
	content := "DATABASE_URL=postgres://localhost/mydb\nAPI_KEY=abc123\n"
	if err := os.WriteFile(envPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	env := parseEnvFile(envPath)

	if env["DATABASE_URL"] != "postgres://localhost/mydb" {
		t.Errorf("DATABASE_URL = %q, want \"postgres://localhost/mydb\"", env["DATABASE_URL"])
	}
	if env["API_KEY"] != "abc123" {
		t.Errorf("API_KEY = %q, want \"abc123\"", env["API_KEY"])
	}
}

func TestParseEnvFile_QuotedValues(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env.duckrow")
	content := `DOUBLE="hello world"
SINGLE='hello world'
UNQUOTED=hello world
`
	if err := os.WriteFile(envPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	env := parseEnvFile(envPath)

	if env["DOUBLE"] != "hello world" {
		t.Errorf("DOUBLE = %q, want \"hello world\"", env["DOUBLE"])
	}
	if env["SINGLE"] != "hello world" {
		t.Errorf("SINGLE = %q, want \"hello world\"", env["SINGLE"])
	}
	if env["UNQUOTED"] != "hello world" {
		t.Errorf("UNQUOTED = %q, want \"hello world\"", env["UNQUOTED"])
	}
}

func TestParseEnvFile_CommentsAndBlanks(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env.duckrow")
	content := `# This is a comment
KEY1=val1

# Another comment
KEY2=val2
`
	if err := os.WriteFile(envPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	env := parseEnvFile(envPath)

	if len(env) != 2 {
		t.Fatalf("len(env) = %d, want 2", len(env))
	}
	if env["KEY1"] != "val1" {
		t.Errorf("KEY1 = %q, want \"val1\"", env["KEY1"])
	}
	if env["KEY2"] != "val2" {
		t.Errorf("KEY2 = %q, want \"val2\"", env["KEY2"])
	}
}

func TestParseEnvFile_ExportPrefix(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env.duckrow")
	content := "export MY_VAR=myvalue\n"
	if err := os.WriteFile(envPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	env := parseEnvFile(envPath)

	if env["MY_VAR"] != "myvalue" {
		t.Errorf("MY_VAR = %q, want \"myvalue\"", env["MY_VAR"])
	}
}

func TestParseEnvFile_NotExists(t *testing.T) {
	env := parseEnvFile("/nonexistent/path/.env.duckrow")
	if env != nil {
		t.Errorf("expected nil, got %v", env)
	}
}

func TestParseEnvFile_NoEqualsSign(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env.duckrow")
	content := "INVALID_LINE_NO_EQUALS\nVALID=yes\n"
	if err := os.WriteFile(envPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	env := parseEnvFile(envPath)

	if len(env) != 1 {
		t.Fatalf("len(env) = %d, want 1", len(env))
	}
	if env["VALID"] != "yes" {
		t.Errorf("VALID = %q, want \"yes\"", env["VALID"])
	}
}

// ---------------------------------------------------------------------------
// EnvResolver tests
// ---------------------------------------------------------------------------

func TestEnvResolver_ProcessEnvHighestPriority(t *testing.T) {
	projectDir := t.TempDir()
	globalDir := t.TempDir()

	// Set value in all three sources.
	t.Setenv("TEST_VAR", "from-process")

	projectEnvPath := filepath.Join(projectDir, ".env.duckrow")
	if err := os.WriteFile(projectEnvPath, []byte("TEST_VAR=from-project\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	globalEnvPath := filepath.Join(globalDir, ".env.duckrow")
	if err := os.WriteFile(globalEnvPath, []byte("TEST_VAR=from-global\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	resolver := NewEnvResolver(projectDir, globalDir)
	resolved, missing := resolver.ResolveEnv([]string{"TEST_VAR"})

	if len(missing) != 0 {
		t.Errorf("missing = %v, want empty", missing)
	}
	if resolved["TEST_VAR"] != "from-process" {
		t.Errorf("TEST_VAR = %q, want \"from-process\"", resolved["TEST_VAR"])
	}
}

func TestEnvResolver_ProjectOverridesGlobal(t *testing.T) {
	projectDir := t.TempDir()
	globalDir := t.TempDir()

	// Only project and global (no process env).
	projectEnvPath := filepath.Join(projectDir, ".env.duckrow")
	if err := os.WriteFile(projectEnvPath, []byte("TEST_VAR=from-project\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	globalEnvPath := filepath.Join(globalDir, ".env.duckrow")
	if err := os.WriteFile(globalEnvPath, []byte("TEST_VAR=from-global\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	resolver := NewEnvResolver(projectDir, globalDir)
	resolved, missing := resolver.ResolveEnv([]string{"TEST_VAR"})

	if len(missing) != 0 {
		t.Errorf("missing = %v, want empty", missing)
	}
	if resolved["TEST_VAR"] != "from-project" {
		t.Errorf("TEST_VAR = %q, want \"from-project\"", resolved["TEST_VAR"])
	}
}

func TestEnvResolver_FallsBackToGlobal(t *testing.T) {
	projectDir := t.TempDir()
	globalDir := t.TempDir()

	// Only global.
	globalEnvPath := filepath.Join(globalDir, ".env.duckrow")
	if err := os.WriteFile(globalEnvPath, []byte("TEST_VAR=from-global\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	resolver := NewEnvResolver(projectDir, globalDir)
	resolved, missing := resolver.ResolveEnv([]string{"TEST_VAR"})

	if len(missing) != 0 {
		t.Errorf("missing = %v, want empty", missing)
	}
	if resolved["TEST_VAR"] != "from-global" {
		t.Errorf("TEST_VAR = %q, want \"from-global\"", resolved["TEST_VAR"])
	}
}

func TestEnvResolver_ReportsMissing(t *testing.T) {
	projectDir := t.TempDir()
	globalDir := t.TempDir()

	resolver := NewEnvResolver(projectDir, globalDir)
	resolved, missing := resolver.ResolveEnv([]string{"MISSING_VAR"})

	if len(resolved) != 0 {
		t.Errorf("resolved = %v, want empty", resolved)
	}
	if len(missing) != 1 || missing[0] != "MISSING_VAR" {
		t.Errorf("missing = %v, want [\"MISSING_VAR\"]", missing)
	}
}

func TestEnvResolver_MultipleVars(t *testing.T) {
	projectDir := t.TempDir()
	globalDir := t.TempDir()

	projectEnvPath := filepath.Join(projectDir, ".env.duckrow")
	if err := os.WriteFile(projectEnvPath, []byte("VAR_A=a\nVAR_B=b\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	resolver := NewEnvResolver(projectDir, globalDir)
	resolved, missing := resolver.ResolveEnv([]string{"VAR_A", "VAR_B", "VAR_C"})

	if resolved["VAR_A"] != "a" {
		t.Errorf("VAR_A = %q, want \"a\"", resolved["VAR_A"])
	}
	if resolved["VAR_B"] != "b" {
		t.Errorf("VAR_B = %q, want \"b\"", resolved["VAR_B"])
	}
	if len(missing) != 1 || missing[0] != "VAR_C" {
		t.Errorf("missing = %v, want [\"VAR_C\"]", missing)
	}
}

func TestEnvResolver_EmptyRequiredVars(t *testing.T) {
	resolver := NewEnvResolver(t.TempDir(), t.TempDir())
	resolved, missing := resolver.ResolveEnv(nil)

	if resolved != nil {
		t.Errorf("resolved = %v, want nil", resolved)
	}
	if missing != nil {
		t.Errorf("missing = %v, want nil", missing)
	}
}

// ---------------------------------------------------------------------------
// EnsureGitignore tests
// ---------------------------------------------------------------------------

func TestEnsureGitignore_CreatesNew(t *testing.T) {
	dir := t.TempDir()

	if err := EnsureGitignore(dir); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(data), ".env.duckrow") {
		t.Error(".gitignore does not contain .env.duckrow")
	}
}

func TestEnsureGitignore_AppendsToExisting(t *testing.T) {
	dir := t.TempDir()

	// Create existing .gitignore.
	gitignorePath := filepath.Join(dir, ".gitignore")
	if err := os.WriteFile(gitignorePath, []byte("node_modules/\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := EnsureGitignore(dir); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)
	if !strings.Contains(content, "node_modules/") {
		t.Error("existing content was lost")
	}
	if !strings.Contains(content, ".env.duckrow") {
		t.Error(".env.duckrow was not appended")
	}
}

func TestEnsureGitignore_SkipsIfAlreadyPresent(t *testing.T) {
	dir := t.TempDir()

	gitignorePath := filepath.Join(dir, ".gitignore")
	if err := os.WriteFile(gitignorePath, []byte(".env.duckrow\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := EnsureGitignore(dir); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatal(err)
	}

	// Count occurrences — should be exactly 1.
	count := strings.Count(string(data), ".env.duckrow")
	if count != 1 {
		t.Errorf(".env.duckrow appears %d times, want 1", count)
	}
}
