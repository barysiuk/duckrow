package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/barysiuk/duckrow/internal/core"
	"github.com/barysiuk/duckrow/internal/core/asset"
	"github.com/barysiuk/duckrow/internal/core/system"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status [path]",
	Short: "Show installed skills and MCPs for the current folder",
	Long: `Show installed skills, MCP configurations, and tracking status for a folder.
If a path is given, shows status for that folder. Otherwise shows status for the current directory.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := newDeps()
		if err != nil {
			return err
		}

		fm := core.NewFolderManager(d.config)

		// Determine target path: explicit argument or current directory
		var targetPath string
		if len(args) > 0 {
			targetPath = args[0]
		} else {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getting current directory: %w", err)
			}
			targetPath = cwd
		}

		// Resolve to absolute path for display and tracking check
		absPath, err := filepath.Abs(targetPath)
		if err != nil {
			return fmt.Errorf("resolving path: %w", err)
		}

		// Check tracking state
		tracked, _ := fm.IsTracked(absPath)

		// Build MCP description lookup from registries (best-effort).
		mcpDescriptions := buildMCPDescriptionMap(d)

		// Show folder status with tracking indicator
		if err := showFolderStatus(absPath, tracked, mcpDescriptions); err != nil {
			return err
		}

		// If not tracked, show a hint
		if !tracked {
			fmt.Fprintln(os.Stdout)
			fmt.Fprintln(os.Stdout, "This folder is not tracked by DuckRow.")
			if len(args) > 0 {
				fmt.Fprintf(os.Stdout, "To add it, run: duckrow bookmark add %s\n", args[0])
			} else {
				fmt.Fprintln(os.Stdout, "To add it, run: duckrow bookmark add .")
			}
		}

		return nil
	},
}

func showFolderStatus(path string, tracked bool, mcpDescriptions map[string]string) error {
	trackLabel := "[not tracked]"
	if tracked {
		trackLabel = "[tracked]"
	}
	fmt.Fprintf(os.Stdout, "Folder: %s %s\n", path, trackLabel)

	orch := core.NewOrchestrator()
	allInstalled, err := orch.ScanFolder(path)
	if err != nil {
		return fmt.Errorf("scanning folder: %w", err)
	}

	skills := allInstalled[asset.KindSkill]

	if len(skills) == 0 {
		fmt.Fprintln(os.Stdout, "  Skills: none installed")
	} else {
		fmt.Fprintf(os.Stdout, "  Skills (%d):\n", len(skills))
		for _, s := range skills {
			// Show relative path from the folder root
			relPath := skillRelPath(path, s.Path)
			fmt.Fprintf(os.Stdout, "    - %s [%s]\n", s.Name, relPath)
			if s.Description != "" {
				fmt.Fprintf(os.Stdout, "      %s\n", s.Description)
			}
		}
	}

	// Show MCPs from the lock file (MCPs are config-only, not on disk).
	lf, _ := core.ReadLockFile(path)
	if lf != nil && len(lf.MCPs) > 0 {
		fmt.Fprintf(os.Stdout, "  MCPs (%d):\n", len(lf.MCPs))
		for _, m := range lf.MCPs {
			displayNames := systemDisplayNames(m.Agents)
			desc := mcpDescriptions[m.Name]
			if desc != "" {
				fmt.Fprintf(os.Stdout, "    - %-18s %s  [%s]\n", m.Name, desc, displayNames)
			} else {
				fmt.Fprintf(os.Stdout, "    - %-18s [%s]\n", m.Name, displayNames)
			}
		}
	}

	return nil
}

// systemDisplayNames converts system names to display names for status output.
func systemDisplayNames(sysNames []string) string {
	var names []string
	for _, name := range sysNames {
		if sys, ok := system.ByName(name); ok {
			names = append(names, sys.DisplayName())
		} else {
			names = append(names, name)
		}
	}
	return strings.Join(names, ", ")
}

// buildMCPDescriptionMap loads MCP descriptions from configured registries (best-effort).
// Returns a map of MCP name -> description.
func buildMCPDescriptionMap(d *deps) map[string]string {
	descriptions := make(map[string]string)

	cfg, err := d.config.Load()
	if err != nil || len(cfg.Registries) == 0 {
		return descriptions
	}

	rm := core.NewRegistryManager(d.config.RegistriesDir())
	allMCPs := rm.ListMCPs(cfg.Registries)
	for _, m := range allMCPs {
		if m.MCP.Description != "" {
			descriptions[m.MCP.Name] = m.MCP.Description
		}
	}
	return descriptions
}

// skillRelPath returns the skill path relative to the folder root,
// using forward slashes for consistent display.
// Falls back to the absolute path if the relative conversion fails.
func skillRelPath(folderPath, skillPath string) string {
	rel, err := filepath.Rel(folderPath, skillPath)
	if err != nil {
		return skillPath
	}
	return strings.ReplaceAll(rel, string(filepath.Separator), "/")
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
