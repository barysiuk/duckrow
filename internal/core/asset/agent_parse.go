package asset

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// ParseAgentFile reads a Markdown file with YAML frontmatter and returns
// the parsed AgentData (frontmatter map + body string).
func ParseAgentFile(path string) (*AgentData, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	return ParseAgentContent(raw, path)
}

// ParseAgentContent parses agent content from raw bytes. The source parameter
// is used only for error messages.
func ParseAgentContent(raw []byte, source string) (*AgentData, error) {
	content := string(raw)

	// Must start with frontmatter delimiter.
	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "---") {
		return nil, fmt.Errorf("no frontmatter in %s", source)
	}

	// Find the frontmatter boundaries.
	// First "---" is at the start; find the closing "---".
	start := strings.Index(content, "---")
	rest := content[start+3:]

	// Skip the newline after opening ---
	if len(rest) > 0 && rest[0] == '\n' {
		rest = rest[1:]
	} else if len(rest) > 1 && rest[0] == '\r' && rest[1] == '\n' {
		rest = rest[2:]
	}

	end := strings.Index(rest, "\n---")
	if end < 0 {
		return nil, fmt.Errorf("no closing frontmatter delimiter in %s", source)
	}

	fmContent := rest[:end]
	body := rest[end+4:] // skip "\n---"

	// Strip leading newline from body.
	if len(body) > 0 && body[0] == '\n' {
		body = body[1:]
	} else if len(body) > 1 && body[0] == '\r' && body[1] == '\n' {
		body = body[2:]
	}

	// Parse YAML frontmatter into a generic map.
	var fm map[string]any
	if err := yaml.Unmarshal([]byte(fmContent), &fm); err != nil {
		return nil, fmt.Errorf("parsing frontmatter in %s: %w", source, err)
	}

	if fm == nil {
		fm = make(map[string]any)
	}

	return &AgentData{
		Frontmatter: fm,
		Body:        body,
	}, nil
}
