package asset

import (
	"testing"
)

func TestHandlerRegistry(t *testing.T) {
	// Skill and MCP handlers should be auto-registered via init().
	skillH, ok := Get(KindSkill)
	if !ok {
		t.Fatal("skill handler not registered")
	}
	if skillH.Kind() != KindSkill {
		t.Errorf("skill handler Kind() = %q", skillH.Kind())
	}

	mcpH, ok := Get(KindMCP)
	if !ok {
		t.Fatal("MCP handler not registered")
	}
	if mcpH.Kind() != KindMCP {
		t.Errorf("MCP handler Kind() = %q", mcpH.Kind())
	}
}

func TestAll(t *testing.T) {
	all := All()
	if len(all) < 2 {
		t.Errorf("expected at least 2 handlers, got %d", len(all))
	}
}

func TestKinds(t *testing.T) {
	kinds := Kinds()
	if len(kinds) < 2 {
		t.Fatalf("expected at least 2 kinds, got %d", len(kinds))
	}
	// Should be deterministic: skill first, then mcp.
	if kinds[0] != KindSkill {
		t.Errorf("kinds[0] = %q, want %q", kinds[0], KindSkill)
	}
	if kinds[1] != KindMCP {
		t.Errorf("kinds[1] = %q, want %q", kinds[1], KindMCP)
	}
}

func TestGetUnknown(t *testing.T) {
	_, ok := Get(Kind("nonexistent"))
	if ok {
		t.Error("expected Get for unknown kind to return false")
	}
}

func TestHashBytes(t *testing.T) {
	h := hashBytes([]byte("hello"))
	if h == "" {
		t.Error("hashBytes returned empty string")
	}
	if len(h) != 7+64 { // "sha256:" + 64 hex chars
		t.Errorf("hashBytes returned %d chars, want %d", len(h), 7+64)
	}

	// Same input should produce same hash.
	h2 := hashBytes([]byte("hello"))
	if h != h2 {
		t.Error("hashBytes not deterministic")
	}

	// Different input should produce different hash.
	h3 := hashBytes([]byte("world"))
	if h == h3 {
		t.Error("hashBytes returned same hash for different input")
	}
}
