package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/barysiuk/duckrow/internal/core"
	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add [path]",
	Short: "Add a folder to the tracked list",
	Long:  `Add a project folder to DuckRow's tracked list. Defaults to the current directory if no path is given.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := newDeps()
		if err != nil {
			return err
		}

		fm := core.NewFolderManager(d.config)

		path := ""
		if len(args) > 0 {
			path = args[0]
		}

		if err := fm.Add(path); err != nil {
			return err
		}

		// Resolve the path for display (same logic as FolderManager)
		displayPath := path
		if displayPath == "" {
			displayPath, _ = os.Getwd()
		} else {
			displayPath, _ = filepath.Abs(displayPath)
		}

		fmt.Fprintf(os.Stdout, "Added folder: %s\n", displayPath)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(addCmd)
}
