package cmd

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/barysiuk/duckrow/internal/tui"
)

// Version info set via ldflags at build time.
var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "duckrow",
	Short: "Get your ducks in a row - manage AI agent skills across projects",
	Long: `DuckRow manages AI agent skills and MCP configurations across multiple project folders.

Bookmark folders, install and remove skills and MCPs, manage private registries,
and see what's installed everywhere - all from a single tool.

Run without arguments to launch the interactive TUI.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := newDeps()
		if err != nil {
			return err
		}

		app := tui.NewApp(d.config, Version)
		p := tea.NewProgram(app, tea.WithAltScreen())
		_, err = p.Run()
		return err
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("duckrow %s (commit: %s, built: %s)\n", Version, Commit, Date)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	registerAssetCommands()
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
