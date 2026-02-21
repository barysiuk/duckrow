package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/barysiuk/duckrow/internal/core"
	"github.com/spf13/cobra"
)

var registryCmd = &cobra.Command{
	Use:   "registry",
	Short: "Manage registries",
	Long:  `Add, list, refresh, and remove private registries.`,
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

		// Check if registry with same repo already exists in config
		for _, r := range cfg.Registries {
			if r.Repo == args[0] {
				fmt.Fprintf(os.Stdout, "Updated registry: %s (%s)\n", manifest.Name, registrySummary(manifest))
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

		fmt.Fprintf(os.Stdout, "Added registry: %s (%s)\n", manifest.Name, registrySummary(manifest))
		if manifest.Description != "" {
			fmt.Fprintf(os.Stdout, "  %s\n", manifest.Description)
		}
		printManifestWarnings(manifest)
		return nil
	},
}

var registryListCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured registries",
	Long:  `List all configured registries and their available skills and MCPs.`,
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
			manifest, err := rm.LoadManifest(reg.Repo)
			if err != nil {
				fmt.Fprintf(os.Stdout, "  %s  %s  (error: %v)\n", reg.Name, reg.Repo, err)
				continue
			}

			// Build summary counts.
			parts := []string{}
			if len(manifest.Skills) > 0 {
				parts = append(parts, fmt.Sprintf("%d skills", len(manifest.Skills)))
			}
			if len(manifest.MCPs) > 0 {
				parts = append(parts, fmt.Sprintf("%d MCPs", len(manifest.MCPs)))
			}
			summary := "empty"
			if len(parts) > 0 {
				summary = strings.Join(parts, ", ")
			}

			fmt.Fprintf(os.Stdout, "  %s  %s  (%s)\n", reg.Name, reg.Repo, summary)

			if verbose {
				if len(manifest.Skills) > 0 {
					fmt.Fprintln(os.Stdout, "    Skills:")
					for _, s := range manifest.Skills {
						fmt.Fprintf(os.Stdout, "      - %s: %s\n", s.Name, s.Description)
					}
				}
				if len(manifest.MCPs) > 0 {
					fmt.Fprintln(os.Stdout, "    MCPs:")
					for _, m := range manifest.MCPs {
						fmt.Fprintf(os.Stdout, "      - %s: %s\n", m.Name, m.Description)
					}
				}
			}
		}
		return nil
	},
}

var registryRefreshCmd = &cobra.Command{
	Use:   "refresh [name-or-repo]",
	Short: "Refresh registry data",
	Long:  `Pull latest changes for a registry. If no argument given, refreshes all registries. Accepts a registry name or repo URL.`,
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
			// Find the registry by name or repo URL
			reg, err := findRegistry(cfg.Registries, args[0])
			if err != nil {
				return err
			}
			manifest, err := rm.Refresh(reg.Repo)
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stdout, "Refreshed: %s (%s)\n", manifest.Name, registrySummary(manifest))
			printManifestWarnings(manifest)
			return nil
		}

		// Refresh all
		if len(cfg.Registries) == 0 {
			fmt.Fprintln(os.Stdout, "No registries configured.")
			return nil
		}

		for _, reg := range cfg.Registries {
			manifest, err := rm.Refresh(reg.Repo)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error refreshing %s: %v\n", reg.Name, err)
				continue
			}
			fmt.Fprintf(os.Stdout, "Refreshed: %s (%s)\n", manifest.Name, registrySummary(manifest))
			printManifestWarnings(manifest)
		}
		return nil
	},
}

var registryRemoveCmd = &cobra.Command{
	Use:   "remove <name-or-repo>",
	Short: "Remove a registry",
	Long:  `Remove a registry from the config and delete its local clone. Accepts a registry name or repo URL.`,
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

		// Find the registry by name or repo URL
		reg, err := findRegistry(cfg.Registries, args[0])
		if err != nil {
			return err
		}

		// Remove from config (match by repo URL for precision)
		registries := make([]core.Registry, 0, len(cfg.Registries))
		for _, r := range cfg.Registries {
			if r.Repo == reg.Repo {
				continue
			}
			registries = append(registries, r)
		}

		cfg.Registries = registries
		if err := d.config.Save(cfg); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		// Remove local clone
		rm := core.NewRegistryManager(d.config.RegistriesDir())
		if err := rm.Remove(reg.Repo); err != nil {
			// Not fatal — config is already updated
			fmt.Fprintf(os.Stderr, "Warning: could not remove local clone: %v\n", err)
		}

		fmt.Fprintf(os.Stdout, "Removed registry: %s\n", reg.Name)
		return nil
	},
}

// findRegistry resolves a registry argument (name or repo URL) to a single Registry.
// If the argument matches a repo URL exactly, that registry is returned.
// If it matches a name and only one registry has that name, it is returned.
// If multiple registries share the name, an error lists the repo URLs.
func findRegistry(registries []core.Registry, arg string) (*core.Registry, error) {
	// Try exact repo match first
	for i := range registries {
		if registries[i].Repo == arg {
			return &registries[i], nil
		}
	}

	// Try name match
	var matches []core.Registry
	for _, r := range registries {
		if r.Name == arg {
			matches = append(matches, r)
		}
	}

	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("registry %q not found", arg)
	case 1:
		return &matches[0], nil
	default:
		var repos []string
		for _, m := range matches {
			repos = append(repos, m.Repo)
		}
		return nil, fmt.Errorf("multiple registries named %q; specify the repo URL instead:\n  %s",
			arg, strings.Join(repos, "\n  "))
	}
}

// printManifestWarnings prints any validation warnings from a registry manifest to stderr.
func printManifestWarnings(manifest *core.RegistryManifest) {
	for _, w := range manifest.Warnings {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", w)
	}
}

// registrySummary returns a human-readable summary of a registry manifest's contents.
// e.g. "3 skills, 2 MCPs" or "3 skills" or "2 MCPs" or "empty".
func registrySummary(manifest *core.RegistryManifest) string {
	var parts []string
	if len(manifest.Skills) > 0 {
		parts = append(parts, fmt.Sprintf("%d skills", len(manifest.Skills)))
	}
	if len(manifest.MCPs) > 0 {
		parts = append(parts, fmt.Sprintf("%d MCPs", len(manifest.MCPs)))
	}
	if len(parts) == 0 {
		return "empty"
	}
	return strings.Join(parts, ", ")
}

func init() {
	registryListCmd.Flags().BoolP("verbose", "v", false, "Show skills and MCPs in each registry")
	registryCmd.AddCommand(registryAddCmd)
	registryCmd.AddCommand(registryListCmd)
	registryCmd.AddCommand(registryRefreshCmd)
	registryCmd.AddCommand(registryRemoveCmd)
	rootCmd.AddCommand(registryCmd)
}
