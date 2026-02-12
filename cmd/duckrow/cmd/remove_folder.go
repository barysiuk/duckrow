package cmd

import (
	"fmt"
	"os"

	"github.com/barysiuk/duckrow/internal/core"
	"github.com/spf13/cobra"
)

var removeFolderCmd = &cobra.Command{
	Use:   "remove-folder <path>",
	Short: "Remove a folder from the tracked list",
	Long:  `Remove a project folder from DuckRow's tracked list. Does not delete any files on disk.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := newDeps()
		if err != nil {
			return err
		}

		fm := core.NewFolderManager(d.config)

		if err := fm.Remove(args[0]); err != nil {
			return err
		}

		fmt.Fprintf(os.Stdout, "Removed folder: %s\n", args[0])
		return nil
	},
}

func init() {
	rootCmd.AddCommand(removeFolderCmd)
}
