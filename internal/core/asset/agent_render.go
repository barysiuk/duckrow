package asset

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// SystemOverrideKeys are the recognized system-specific override block keys.
var SystemOverrideKeys = []string{
	"claude-code",
	"opencode",
	"github-copilot",
	"gemini-cli",
}

// RenderForSystem applies the merge algorithm to produce system-specific
// agent file content (YAML frontmatter + Markdown body).
//
// Algorithm:
//  1. Start with a shallow copy of all top-level fields.
//  2. Remove ALL system override blocks from the merged result.
//  3. Apply the target system's override block (if any) — overrides replace.
//  4. Remove "name" field (derived from filename) — except for Gemini CLI.
//  5. Render merged frontmatter + original body.
func RenderForSystem(data *AgentData, systemKey string) ([]byte, error) {
	if data == nil {
		return nil, fmt.Errorf("agent data is nil")
	}

	// 1. Shallow copy all top-level fields.
	merged := make(map[string]any, len(data.Frontmatter))
	for k, v := range data.Frontmatter {
		merged[k] = v
	}

	// 2. Extract the target system's override block before stripping.
	var override map[string]any
	if raw, ok := merged[systemKey]; ok {
		if m, ok2 := raw.(map[string]any); ok2 {
			override = m
		}
	}

	// 3. Remove ALL system override blocks.
	for _, key := range SystemOverrideKeys {
		delete(merged, key)
	}

	// 4. Apply this system's override block.
	for k, v := range override {
		merged[k] = v
	}

	// 5. Remove "name" field (derived from filename) — except for Gemini CLI.
	if systemKey != "gemini-cli" {
		delete(merged, "name")
	}

	// Render YAML frontmatter with consistent field ordering.
	yamlBytes, err := marshalOrderedYAML(merged)
	if err != nil {
		return nil, fmt.Errorf("marshaling frontmatter: %w", err)
	}

	// Build the final output: ---\n<yaml>\n---\n\n<body>
	var buf bytes.Buffer
	buf.WriteString("---\n")
	buf.Write(yamlBytes)
	buf.WriteString("---\n")
	if data.Body != "" {
		buf.WriteString("\n")
		buf.WriteString(data.Body)
		// Ensure trailing newline.
		if !strings.HasSuffix(data.Body, "\n") {
			buf.WriteString("\n")
		}
	}

	return buf.Bytes(), nil
}

// marshalOrderedYAML serializes a map to YAML with a defined field order:
// 1. name (if present, for Gemini CLI)
// 2. description
// 3. model
// 4. tools
// 5. all other fields in alphabetical order
func marshalOrderedYAML(m map[string]any) ([]byte, error) {
	if len(m) == 0 {
		return nil, nil
	}

	// Define priority fields in order.
	priority := []string{"name", "description", "model", "tools"}

	// Collect remaining keys.
	prioritySet := make(map[string]bool)
	for _, k := range priority {
		prioritySet[k] = true
	}

	var rest []string
	for k := range m {
		if !prioritySet[k] {
			rest = append(rest, k)
		}
	}
	sort.Strings(rest)

	// Build ordered key list: priority keys (if present) then rest.
	var ordered []string
	for _, k := range priority {
		if _, ok := m[k]; ok {
			ordered = append(ordered, k)
		}
	}
	ordered = append(ordered, rest...)

	// Build a yaml.Node to control key order.
	doc := &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!!map",
	}

	for _, key := range ordered {
		keyNode := &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!str",
			Value: key,
		}

		valNode, err := encodeValue(m[key])
		if err != nil {
			return nil, fmt.Errorf("encoding field %q: %w", key, err)
		}

		doc.Content = append(doc.Content, keyNode, valNode)
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// encodeValue converts a Go value to a yaml.Node for ordered output.
func encodeValue(v any) (*yaml.Node, error) {
	// Use the standard encoder via marshal/unmarshal roundtrip to get a node.
	data, err := yaml.Marshal(v)
	if err != nil {
		return nil, err
	}

	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return nil, err
	}

	// Unmarshal wraps in a document node; return the actual content node.
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		return node.Content[0], nil
	}
	return &node, nil
}
