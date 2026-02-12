package cmd

import (
	"fmt"
	"os"

	"github.com/barysiuk/duckrow/internal/core"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status [path]",
	Short: "Show skills and agents across tracked folders",
	Long: `Show installed skills, detected agents, and status for tracked folders.
If a path is given, shows status for that folder only. Otherwise shows all tracked folders.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := newDeps()
		if err != nil {
			return err
		}

		scanner := core.NewScanner(d.agents)

		if len(args) > 0 {
			// Single folder status
			return showFolderStatus(scanner, args[0])
		}

		// All tracked folders
		fm := core.NewFolderManager(d.config)
		folders, err := fm.List()
		if err != nil {
			return err
		}

		if len(folders) == 0 {
			fmt.Fprintln(os.Stdout, "No tracked folders. Use 'duckrow add' to add one.")
			return nil
		}

		for i, f := range folders {
			if i > 0 {
				fmt.Fprintln(os.Stdout)
			}
			if err := showFolderStatus(scanner, f.Path); err != nil {
				fmt.Fprintf(os.Stderr, "Error scanning %s: %v\n", f.Path, err)
			}
		}
		return nil
	},
}

func showFolderStatus(scanner *core.Scanner, path string) error {
	fmt.Fprintf(os.Stdout, "Folder: %s\n", path)

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
