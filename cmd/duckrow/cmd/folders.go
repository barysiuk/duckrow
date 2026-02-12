package cmd

import (
	"fmt"
	"os"

	"github.com/barysiuk/duckrow/internal/core"
	"github.com/spf13/cobra"
)

var foldersCmd = &cobra.Command{
	Use:   "folders",
	Short: "List all tracked folders",
	Long:  `List all project folders currently tracked by DuckRow.`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := newDeps()
		if err != nil {
			return err
		}

		fm := core.NewFolderManager(d.config)
		folders, err := fm.List()
		if err != nil {
			return err
		}

		if len(folders) == 0 {
			fmt.Fprintln(os.Stdout, "No tracked folders. Use 'duckrow add' to add one.")
			return nil
		}

		fmt.Fprintf(os.Stdout, "Tracked folders (%d):\n", len(folders))
		for _, f := range folders {
			fmt.Fprintf(os.Stdout, "  %s\n", f.Path)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(foldersCmd)
}
