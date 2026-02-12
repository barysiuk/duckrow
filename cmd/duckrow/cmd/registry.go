package cmd

import (
	"fmt"
	"os"

	"github.com/barysiuk/duckrow/internal/core"
	"github.com/spf13/cobra"
)

var registryCmd = &cobra.Command{
	Use:   "registry",
	Short: "Manage skill registries",
	Long:  `Add, list, refresh, and remove private skill registries.`,
}

var registryAddCmd = &cobra.Command{
	Use:   "add <repo-url>",
	Short: "Add a skill registry",
	Long: `Add a private skill registry by cloning its git repository.
The repository must contain a duckrow.json manifest at its root.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := newDeps()
		if err != nil {
			return err
		}

		cfg, err := d.config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		rm := core.NewRegistryManager(d.config.RegistriesDir())
		manifest, err := rm.Add(args[0])
		if err != nil {
			return err
		}

		// Check if registry already exists in config
		for _, r := range cfg.Registries {
			if r.Name == manifest.Name {
				fmt.Fprintf(os.Stdout, "Updated registry: %s (%d skills)\n", manifest.Name, len(manifest.Skills))
				return nil
			}
		}

		// Add to config
		cfg.Registries = append(cfg.Registries, core.Registry{
			Name: manifest.Name,
			Repo: args[0],
		})

		if err := d.config.Save(cfg); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		fmt.Fprintf(os.Stdout, "Added registry: %s (%d skills)\n", manifest.Name, len(manifest.Skills))
		if manifest.Description != "" {
			fmt.Fprintf(os.Stdout, "  %s\n", manifest.Description)
		}
		return nil
	},
}

var registryListCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured registries",
	Long:  `List all configured skill registries and their available skills.`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := newDeps()
		if err != nil {
			return err
		}

		cfg, err := d.config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		if len(cfg.Registries) == 0 {
			fmt.Fprintln(os.Stdout, "No registries configured. Use 'duckrow registry add <url>' to add one.")
			return nil
		}

		rm := core.NewRegistryManager(d.config.RegistriesDir())
		verbose, _ := cmd.Flags().GetBool("verbose")

		fmt.Fprintf(os.Stdout, "Registries (%d):\n", len(cfg.Registries))
		for _, reg := range cfg.Registries {
			manifest, err := rm.LoadManifest(reg.Name)
			if err != nil {
				fmt.Fprintf(os.Stdout, "  %s  %s  (error: %v)\n", reg.Name, reg.Repo, err)
				continue
			}

			fmt.Fprintf(os.Stdout, "  %s  %s  (%d skills)\n", reg.Name, reg.Repo, len(manifest.Skills))

			if verbose {
				for _, s := range manifest.Skills {
					version := ""
					if s.Version != "" {
						version = " v" + s.Version
					}
					fmt.Fprintf(os.Stdout, "    - %s%s: %s\n", s.Name, version, s.Description)
				}
			}
		}
		return nil
	},
}

var registryRefreshCmd = &cobra.Command{
	Use:   "refresh [name]",
	Short: "Refresh registry data",
	Long:  `Pull latest changes for a registry. If no name given, refreshes all registries.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := newDeps()
		if err != nil {
			return err
		}

		cfg, err := d.config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		rm := core.NewRegistryManager(d.config.RegistriesDir())

		if len(args) > 0 {
			// Refresh specific registry
			manifest, err := rm.Refresh(args[0])
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stdout, "Refreshed: %s (%d skills)\n", manifest.Name, len(manifest.Skills))
			return nil
		}

		// Refresh all
		if len(cfg.Registries) == 0 {
			fmt.Fprintln(os.Stdout, "No registries configured.")
			return nil
		}

		for _, reg := range cfg.Registries {
			manifest, err := rm.Refresh(reg.Name)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error refreshing %s: %v\n", reg.Name, err)
				continue
			}
			fmt.Fprintf(os.Stdout, "Refreshed: %s (%d skills)\n", manifest.Name, len(manifest.Skills))
		}
		return nil
	},
}

var registryRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a registry",
	Long:  `Remove a registry from the config and delete its local clone.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := newDeps()
		if err != nil {
			return err
		}

		cfg, err := d.config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		// Remove from config
		found := false
		registries := make([]core.Registry, 0, len(cfg.Registries))
		for _, r := range cfg.Registries {
			if r.Name == args[0] {
				found = true
				continue
			}
			registries = append(registries, r)
		}

		if !found {
			return fmt.Errorf("registry %q not found in config", args[0])
		}

		cfg.Registries = registries
		if err := d.config.Save(cfg); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		// Remove local clone
		rm := core.NewRegistryManager(d.config.RegistriesDir())
		if err := rm.Remove(args[0]); err != nil {
			// Not fatal — config is already updated
			fmt.Fprintf(os.Stderr, "Warning: could not remove local clone: %v\n", err)
		}

		fmt.Fprintf(os.Stdout, "Removed registry: %s\n", args[0])
		return nil
	},
}

func init() {
	registryListCmd.Flags().BoolP("verbose", "v", false, "Show skills in each registry")
	registryCmd.AddCommand(registryAddCmd)
	registryCmd.AddCommand(registryListCmd)
	registryCmd.AddCommand(registryRefreshCmd)
	registryCmd.AddCommand(registryRemoveCmd)
	rootCmd.AddCommand(registryCmd)
}
