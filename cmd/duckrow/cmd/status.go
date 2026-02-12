package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/barysiuk/duckrow/internal/core"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status [path]",
	Short: "Show skills and agents for the current folder",
	Long: `Show installed skills, detected agents, and tracking status for a folder.
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

	agents := scanner.DetectAgents(path)
	if len(agents) > 0 {
		fmt.Fprintf(os.Stdout, "  Agents: %s\n", joinStrings(agents))
	} else {
		fmt.Fprintln(os.Stdout, "  Agents: none detected")
	}

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
		version := ""
		if s.Version != "" {
			version = " v" + s.Version
		}
		agentInfo := ""
		if len(s.Agents) > 0 {
			agentInfo = fmt.Sprintf(" [%s]", joinStrings(s.Agents))
		}
		fmt.Fprintf(os.Stdout, "    - %s%s%s\n", s.Name, version, agentInfo)
		if s.Description != "" {
			fmt.Fprintf(os.Stdout, "      %s\n", s.Description)
		}
	}
	return nil
}

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

func init() {
	rootCmd.AddCommand(statusCmd)
}
