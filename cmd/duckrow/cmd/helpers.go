package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/barysiuk/duckrow/internal/core/system"
	"github.com/spf13/cobra"
)

// joinStrings concatenates string slices with ", " separator.
func joinStrings(ss []string) string {
	if len(ss) == 0 {
		return ""
	}
	result := ss[0]
	for _, s := range ss[1:] {
		result += ", " + s
	}
	return result
}

// truncateSource returns the host/owner/repo portion of a canonical source.
func truncateSource(source string) string {
	parts := strings.Split(source, "/")
	if len(parts) > 3 {
		return parts[0] + "/" + parts[1] + "/" + parts[2]
	}
	return source
}

// resolveTargetDir resolves the --dir flag or falls back to cwd.
func resolveTargetDir(cmd *cobra.Command) (string, error) {
	dir, _ := cmd.Flags().GetString("dir")
	if dir != "" {
		return dir, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting current directory: %w", err)
	}
	return cwd, nil
}

// resolveTargetSystems parses the --systems flag into []system.System.
// Returns nil (meaning "use defaults") if the flag is empty.
// Also checks the hidden --agents alias for backward compatibility.
func resolveTargetSystems(cmd *cobra.Command) ([]system.System, error) {
	flag, _ := cmd.Flags().GetString("systems")
	if flag == "" {
		// Check hidden --agents alias.
		flag, _ = cmd.Flags().GetString("agents")
	}
	if flag == "" {
		return nil, nil
	}

	names := strings.Split(flag, ",")
	for i := range names {
		names[i] = strings.TrimSpace(names[i])
	}
	return system.ByNames(names)
}

// addSystemsFlag adds both --systems and the hidden --agents alias to a command.
func addSystemsFlag(cmd *cobra.Command) {
	cmd.Flags().String("systems", "", "Comma-separated system names (e.g. cursor,claude-code)")
	cmd.Flags().String("agents", "", "Alias for --systems (deprecated)")
	_ = cmd.Flags().MarkHidden("agents")
}
