package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/barysiuk/duckrow/internal/core"
	"github.com/spf13/cobra"
)

var bookmarkCmd = &cobra.Command{
	Use:   "bookmark",
	Short: "Manage bookmarks",
	Long:  `Add, list, and remove project folder bookmarks.`,
}

// ---------------------------------------------------------------------------
// bookmark add
// ---------------------------------------------------------------------------

var bookmarkAddCmd = &cobra.Command{
	Use:   "add [path]",
	Short: "Bookmark a folder",
	Long:  `Add a project folder to DuckRow's bookmarks. Defaults to the current directory if no path is given.`,
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

		fmt.Fprintf(os.Stdout, "Bookmarked: %s\n", displayPath)
		return nil
	},
}

// ---------------------------------------------------------------------------
// bookmark list
// ---------------------------------------------------------------------------

var bookmarkListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all bookmarks",
	Long:  `List all project folders currently bookmarked by DuckRow.`,
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
			fmt.Fprintln(os.Stdout, "No bookmarks. Use 'duckrow bookmark add' to add one.")
			return nil
		}

		fmt.Fprintf(os.Stdout, "Bookmarks (%d):\n", len(folders))
		for _, f := range folders {
			fmt.Fprintf(os.Stdout, "  %s\n", f.Path)
		}
		return nil
	},
}

// ---------------------------------------------------------------------------
// bookmark remove
// ---------------------------------------------------------------------------

var bookmarkRemoveCmd = &cobra.Command{
	Use:   "remove <path>",
	Short: "Remove a bookmark",
	Long:  `Remove a project folder from DuckRow's bookmarks. Does not delete any files on disk.`,
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

		fmt.Fprintf(os.Stdout, "Removed bookmark: %s\n", args[0])
		return nil
	},
}

func init() {
	bookmarkCmd.AddCommand(bookmarkAddCmd)
	bookmarkCmd.AddCommand(bookmarkListCmd)
	bookmarkCmd.AddCommand(bookmarkRemoveCmd)
	rootCmd.AddCommand(bookmarkCmd)
}
