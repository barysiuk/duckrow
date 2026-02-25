package asset

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMCPHandler_Kind(t *testing.T) {
	h := &MCPHandler{}
	if h.Kind() != KindMCP {
		t.Errorf("Kind() = %q, want %q", h.Kind(), KindMCP)
	}
	if h.DisplayName() != "MCP Server" {
		t.Errorf("DisplayName() = %q, want %q", h.DisplayName(), "MCP Server")
	}
}

func TestMCPMeta_AssetKind(t *testing.T) {
	m := MCPMeta{Command: "test"}
	if m.AssetKind() != KindMCP {
		t.Errorf("AssetKind() = %q, want %q", m.AssetKind(), KindMCP)
	}
}

func TestMCPMeta_IsStdio(t *testing.T) {
	stdio := MCPMeta{Command: "npx"}
	if !stdio.IsStdio() {
		t.Error("expected IsStdio() = true")
	}
	if stdio.IsRemote() {
		t.Error("expected IsRemote() = false for stdio")
	}

	remote := MCPMeta{URL: "https://example.com", Transport: "http"}
	if remote.IsStdio() {
		t.Error("expected IsStdio() = false for remote")
	}
	if !remote.IsRemote() {
		t.Error("expected IsRemote() = true")
	}
}

func TestMCPHandler_Discover(t *testing.T) {
	h := &MCPHandler{}
	assets, err := h.Discover("/tmp", DiscoverOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if assets != nil {
		t.Errorf("expected nil, got %v", assets)
	}
}

func TestMCPHandler_Parse(t *testing.T) {
	h := &MCPHandler{}
	_, err := h.Parse("/tmp")
	if err == nil {
		t.Error("expected error for MCP Parse()")
	}
}

func TestMCPHandler_Validate(t *testing.T) {
	h := &MCPHandler{}

	// Valid stdio.
	err := h.Validate(Asset{
		Name: "db-server",
		Meta: MCPMeta{Command: "npx", Args: []string{"-y", "@internal/db"}},
	})
	if err != nil {
		t.Errorf("unexpected error for valid stdio: %v", err)
	}

	// Valid remote.
	err = h.Validate(Asset{
		Name: "remote-mcp",
		Meta: MCPMeta{URL: "https://example.com", Transport: "http"},
	})
	if err != nil {
		t.Errorf("unexpected error for valid remote: %v", err)
	}

	// Missing name.
	err = h.Validate(Asset{
		Name: "",
		Meta: MCPMeta{Command: "npx"},
	})
	if err == nil {
		t.Error("expected error for missing name")
	}

	// Neither command nor url.
	err = h.Validate(Asset{
		Name: "empty",
		Meta: MCPMeta{},
	})
	if err == nil {
		t.Error("expected error for empty MCP")
	}

	// Both command and url.
	err = h.Validate(Asset{
		Name: "both",
		Meta: MCPMeta{Command: "npx", URL: "https://example.com"},
	})
	if err == nil {
		t.Error("expected error for both command and url")
	}

	// URL without transport.
	err = h.Validate(Asset{
		Name: "no-transport",
		Meta: MCPMeta{URL: "https://example.com"},
	})
	if err == nil {
		t.Error("expected error for URL without transport")
	}

	// Wrong meta type.
	err = h.Validate(Asset{
		Name: "wrong-meta",
		Meta: SkillMeta{},
	})
	if err == nil {
		t.Error("expected error for wrong meta type")
	}
}

func TestMCPHandler_ParseManifestEntries(t *testing.T) {
	h := &MCPHandler{}

	raw := json.RawMessage(`[
		{
			"name": "db-server",
			"description": "Database access",
			"command": "npx",
			"args": ["-y", "@internal/db"],
			"env": {"DB_URL": "$DB_URL"}
		},
		{
			"name": "remote-api",
			"description": "Remote API",
			"url": "https://api.example.com",
			"type": "http"
		}
	]`)

	entries, err := h.ParseManifestEntries(raw)
	if err != nil {
		t.Fatalf("ParseManifestEntries() error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	// Stdio entry.
	if entries[0].Name != "db-server" {
		t.Errorf("entries[0].Name = %q", entries[0].Name)
	}
	meta0, ok := entries[0].Meta.(MCPMeta)
	if !ok {
		t.Fatalf("entries[0].Meta is %T, want MCPMeta", entries[0].Meta)
	}
	if meta0.Command != "npx" {
		t.Errorf("Command = %q", meta0.Command)
	}
	if len(meta0.Args) != 2 {
		t.Errorf("Args len = %d, want 2", len(meta0.Args))
	}
	if meta0.Env["DB_URL"] != "$DB_URL" {
		t.Errorf("Env[DB_URL] = %q", meta0.Env["DB_URL"])
	}

	// Remote entry.
	meta1, ok := entries[1].Meta.(MCPMeta)
	if !ok {
		t.Fatalf("entries[1].Meta is %T, want MCPMeta", entries[1].Meta)
	}
	if meta1.URL != "https://api.example.com" {
		t.Errorf("URL = %q", meta1.URL)
	}
	if meta1.Transport != "http" {
		t.Errorf("Transport = %q", meta1.Transport)
	}
}

func TestMCPHandler_LockData(t *testing.T) {
	h := &MCPHandler{}

	a := Asset{
		Kind: KindMCP,
		Name: "db-server",
		Meta: MCPMeta{
			Command: "npx",
			Args:    []string{"-y", "@internal/db"},
			Env:     map[string]string{"DB_URL": "$DB_URL"},
		},
	}
	info := InstallInfo{
		Registry:    "my-org",
		SystemNames: []string{"cursor", "opencode"},
	}

	locked := h.LockData(a, info)
	if locked.Kind != KindMCP {
		t.Errorf("Kind = %q", locked.Kind)
	}
	if locked.Name != "db-server" {
		t.Errorf("Name = %q", locked.Name)
	}
	if locked.Data["registry"] != "my-org" {
		t.Errorf("registry = %v", locked.Data["registry"])
	}

	// configHash should be a sha256 string.
	hash, ok := locked.Data["configHash"].(string)
	if !ok {
		t.Fatalf("configHash is %T", locked.Data["configHash"])
	}
	if !strings.HasPrefix(hash, "sha256:") {
		t.Errorf("configHash = %q, want sha256: prefix", hash)
	}

	systems, ok := locked.Data["systems"].([]string)
	if !ok {
		t.Fatalf("systems is %T", locked.Data["systems"])
	}
	if len(systems) != 2 {
		t.Errorf("systems len = %d, want 2", len(systems))
	}

	env, ok := locked.Data["requiredEnv"].([]string)
	if !ok {
		t.Fatalf("requiredEnv is %T", locked.Data["requiredEnv"])
	}
	if len(env) != 1 {
		t.Errorf("requiredEnv len = %d, want 1", len(env))
	}
}
