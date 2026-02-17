package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/barysiuk/duckrow/internal/core"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status [path]",
	Short: "Show installed skills for the current folder",
	Long: `Show installed skills and tracking status for a folder.
If a path is given, shows status for that folder. Otherwise shows status for the current directory.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := newDeps()
		if err != nil {
			return err
		}

		scanner := core.NewScanner(d.agents)
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

		// Show folder status with tracking indicator
		if err := showFolderStatus(scanner, absPath, tracked); err != nil {
			return err
		}

		// If not tracked, show a hint
		if !tracked {
			fmt.Fprintln(os.Stdout)
			fmt.Fprintln(os.Stdout, "This folder is not tracked by DuckRow.")
			if len(args) > 0 {
				fmt.Fprintf(os.Stdout, "To add it, run: duckrow add %s\n", args[0])
			} else {
				fmt.Fprintln(os.Stdout, "To add it, run: duckrow add .")
			}
		}

		return nil
	},
}

func showFolderStatus(scanner *core.Scanner, path string, tracked bool) error {
	trackLabel := "[not tracked]"
	if tracked {
		trackLabel = "[tracked]"
	}
	fmt.Fprintf(os.Stdout, "Folder: %s %s\n", path, trackLabel)

	skills, err := scanner.ScanFolder(path)
	if err != nil {
		return fmt.Errorf("scanning folder: %w", err)
	}

	if len(skills) == 0 {
		fmt.Fprintln(os.Stdout, "  Skills: none installed")
		return nil
	}

	fmt.Fprintf(os.Stdout, "  Skills (%d):\n", len(skills))
	for _, s := range skills {
		// Show relative path from the folder root
		relPath := skillRelPath(path, s.Path)
		fmt.Fprintf(os.Stdout, "    - %s [%s]\n", s.Name, relPath)
		if s.Description != "" {
			fmt.Fprintf(os.Stdout, "      %s\n", s.Description)
		}
	}
	return nil
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
