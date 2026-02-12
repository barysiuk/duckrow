package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
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
	Long: `DuckRow manages AI agent skills across multiple project folders.

Track folders, install and remove skills, manage private registries,
and see what's installed everywhere - all from a single tool.`,
	SilenceUsage:  true,
	SilenceErrors: true,
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
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
