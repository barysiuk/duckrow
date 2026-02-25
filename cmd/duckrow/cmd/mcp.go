package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/barysiuk/duckrow/internal/core"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Manage MCP server configurations",
	Long:  `Install, uninstall, and manage MCP server configurations for AI agents.`,
}

// ---------------------------------------------------------------------------
// mcp install
// ---------------------------------------------------------------------------

var mcpInstallCmd = &cobra.Command{
	Use:   "install <name>",
	Short: "Install an MCP server configuration from a registry",
	Long: `Install an MCP server configuration from a configured registry.

The name argument is looked up in configured registries. The MCP config is
written into agent-specific config files for detected agents.

For stdio MCPs, the command is wrapped with duckrow env to inject environment
variables from .env.duckrow at runtime.

Examples:
  duckrow mcp install internal-db
  duckrow mcp install internal-db --registry my-org
  duckrow mcp install internal-db --agents cursor,claude-code`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := newDeps()
		if err != nil {
			return err
		}

		registryFilter, _ := cmd.Flags().GetString("registry")
		noLock, _ := cmd.Flags().GetBool("no-lock")
		force, _ := cmd.Flags().GetBool("force")
		agentsFlag, _ := cmd.Flags().GetString("agents")

		cfg, err := d.config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		mcpName := args[0]

		// Look up MCP in registries.
		rm := core.NewRegistryManager(d.config.RegistriesDir())
		mcpInfo, findErr := rm.FindMCP(cfg.Registries, mcpName, registryFilter)
		if findErr != nil {
			return findErr
		}

		targetDir, _ := cmd.Flags().GetString("dir")
		if targetDir == "" {
			targetDir, err = os.Getwd()
			if err != nil {
				return fmt.Errorf("getting current directory: %w", err)
			}
		}

		// Resolve target agents.
		var targetAgents []core.AgentDef
		if agentsFlag != "" {
			names := strings.Split(agentsFlag, ",")
			for i := range names {
				names[i] = strings.TrimSpace(names[i])
			}
			resolved, resolveErr := core.ResolveAgentsByNames(d.agents, names)
			if resolveErr != nil {
				return resolveErr
			}
			// Filter to MCP-capable only.
			for _, a := range resolved {
				if a.MCPConfigPath != "" {
					targetAgents = append(targetAgents, a)
				}
			}
			if len(targetAgents) == 0 {
				return fmt.Errorf("none of the specified agents support MCP configurations")
			}
		} else {
			// Default: all MCP-capable agents detected in the folder.
			detected := core.DetectAgentsInFolder(d.agents, targetDir)
			targetAgents = core.GetMCPCapableAgents(detected)
			if len(targetAgents) == 0 {
				// Fall back to all MCP-capable agents.
				targetAgents = core.GetMCPCapableAgents(d.agents)
			}
		}

		fmt.Fprintf(os.Stdout, "Installing MCP %q from registry %q...\n\n", mcpName, mcpInfo.RegistryName)

		// Install MCP config into agent files.
		result, err := core.InstallMCPConfig(mcpInfo.MCP, core.MCPInstallOptions{
			ProjectDir:   targetDir,
			TargetAgents: targetAgents,
			Force:        force,
		})
		if err != nil {
			return err
		}

		// Report results.
		fmt.Fprintln(os.Stdout, "Wrote MCP config to:")
		var installedAgentNames []string
		for _, ar := range result.AgentResults {
			switch ar.Action {
			case "wrote":
				fmt.Fprintf(os.Stdout, "  + %-24s (%s)\n", ar.ConfigPath, ar.Agent.DisplayName)
				installedAgentNames = append(installedAgentNames, ar.Agent.Name)
			case "skipped":
				fmt.Fprintf(os.Stdout, "  ! %-24s %q %s\n", ar.ConfigPath, mcpName, ar.Message)
				// Still count as targeted for lock file.
				installedAgentNames = append(installedAgentNames, ar.Agent.Name)
			case "error":
				fmt.Fprintf(os.Stderr, "  x %-24s error: %s\n", ar.ConfigPath, ar.Message)
			}
		}

		// Update lock file.
		if !noLock {
			requiredEnv := core.ExtractRequiredEnv(mcpInfo.MCP.Env)
			lockEntry := core.LockedMCP{
				Name:        mcpName,
				Registry:    mcpInfo.RegistryName,
				ConfigHash:  core.ComputeConfigHash(mcpInfo.MCP.ToMCPMeta()),
				Agents:      installedAgentNames,
				RequiredEnv: requiredEnv,
			}
			if lockErr := core.AddOrUpdateMCPLockEntry(targetDir, lockEntry); lockErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to update lock file: %v\n", lockErr)
			} else {
				fmt.Fprintln(os.Stdout, "\nUpdated duckrow.lock.json")
			}

			// Show required env var warning.
			if len(requiredEnv) > 0 {
				fmt.Fprintln(os.Stdout, "\n! The following environment variables are required:")
				for _, v := range requiredEnv {
					fmt.Fprintf(os.Stdout, "  %s  (used by %s)\n", v, mcpName)
				}
				fmt.Fprintln(os.Stdout, "\n  Add values to .env.duckrow or ~/.duckrow/.env.duckrow")
			}
		}

		fmt.Fprintf(os.Stdout, "\nMCP %q installed successfully.\n", mcpName)
		return nil
	},
}

// ---------------------------------------------------------------------------
// mcp uninstall
// ---------------------------------------------------------------------------

var mcpUninstallCmd = &cobra.Command{
	Use:   "uninstall <name>",
	Short: "Remove an installed MCP server configuration",
	Long: `Remove an MCP server configuration from agent config files.

Reads the lock file to determine which agent configs contain the entry,
removes the entry from each, and updates the lock file.

Example:
  duckrow mcp uninstall internal-db`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := newDeps()
		if err != nil {
			return err
		}

		noLock, _ := cmd.Flags().GetBool("no-lock")
		mcpName := args[0]

		targetDir, _ := cmd.Flags().GetString("dir")
		if targetDir == "" {
			targetDir, err = os.Getwd()
			if err != nil {
				return fmt.Errorf("getting current directory: %w", err)
			}
		}

		// Read lock file to find which agents have this MCP.
		lf, err := core.ReadLockFile(targetDir)
		if err != nil {
			return fmt.Errorf("reading lock file: %w", err)
		}
		if lf == nil {
			return fmt.Errorf("no duckrow.lock.json found in %s", targetDir)
		}

		var lockedMCP *core.LockedMCP
		for i := range lf.MCPs {
			if lf.MCPs[i].Name == mcpName {
				lockedMCP = &lf.MCPs[i]
				break
			}
		}
		if lockedMCP == nil {
			return fmt.Errorf("MCP %q not found in lock file", mcpName)
		}

		// Resolve agents from lock entry.
		targetAgents, err := core.ResolveAgentsByNames(d.agents, lockedMCP.Agents)
		if err != nil {
			// Some agents may have been removed; filter to what we know.
			targetAgents = core.GetMCPCapableAgents(d.agents)
		}

		fmt.Fprintf(os.Stdout, "Removing MCP %q...\n\n", mcpName)

		// Remove MCP from agent configs.
		result, err := core.UninstallMCPConfig(mcpName, targetAgents, core.MCPUninstallOptions{
			ProjectDir: targetDir,
		})
		if err != nil {
			return err
		}

		// Report results.
		fmt.Fprintln(os.Stdout, "Removed from:")
		for _, ar := range result.AgentResults {
			switch ar.Action {
			case "removed":
				fmt.Fprintf(os.Stdout, "  - %-24s (%s)\n", ar.ConfigPath, ar.Agent.DisplayName)
			case "skipped":
				fmt.Fprintf(os.Stdout, "  . %-24s %s\n", ar.ConfigPath, ar.Message)
			case "error":
				fmt.Fprintf(os.Stderr, "  x %-24s error: %s\n", ar.ConfigPath, ar.Message)
			}
		}

		// Update lock file.
		if !noLock {
			if lockErr := core.RemoveMCPLockEntry(targetDir, mcpName); lockErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to update lock file: %v\n", lockErr)
			} else {
				fmt.Fprintln(os.Stdout, "\nUpdated duckrow.lock.json")
			}
		}

		fmt.Fprintf(os.Stdout, "\nMCP %q removed.\n", mcpName)
		return nil
	},
}

// ---------------------------------------------------------------------------
// mcp sync
// ---------------------------------------------------------------------------

var mcpSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Restore MCP configs from lock file",
	Long: `Restore MCP server configurations from duckrow.lock.json.

For each MCP entry in the lock file, looks up the current config in the
registry and writes it to agent config files. Existing entries are skipped
unless --force is used.

This command enforces the lock file and does not fetch upstream updates.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		result, err := runMCPSync(cmd)
		if err != nil {
			return err
		}

		fmt.Fprintf(os.Stdout, "\nMCPs: %d installed, %d skipped, %d errors\n",
			result.installed, result.skipped, result.errors)

		// Show required env vars summary.
		printRequiredEnvSummary(result.requiredEnv)

		if result.errors > 0 {
			return fmt.Errorf("%d MCP(s) failed to sync", result.errors)
		}
		return nil
	},
}

// mcpSyncResult holds the summary of an MCP sync operation.
type mcpSyncResult struct {
	installed   int
	skipped     int
	errors      int
	requiredEnv map[string][]string // envVar -> []mcpName
}

// runMCPSync contains the shared MCP sync logic used by both
// `duckrow mcp sync` and the root-level `duckrow sync`.
func runMCPSync(cmd *cobra.Command) (*mcpSyncResult, error) {
	d, err := newDeps()
	if err != nil {
		return nil, err
	}

	targetDir, _ := cmd.Flags().GetString("dir")
	if targetDir == "" {
		targetDir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("getting current directory: %w", err)
		}
	}

	dryRun, _ := cmd.Flags().GetBool("dry-run")
	force, _ := cmd.Flags().GetBool("force")
	agentsFlag, _ := cmd.Flags().GetString("agents")

	lf, err := core.ReadLockFile(targetDir)
	if err != nil {
		return nil, fmt.Errorf("reading lock file: %w", err)
	}
	if lf == nil {
		return nil, fmt.Errorf("no duckrow.lock.json found in %s", targetDir)
	}

	result := &mcpSyncResult{
		requiredEnv: make(map[string][]string),
	}

	if len(lf.MCPs) == 0 {
		return result, nil
	}

	cfg, err := d.config.Load()
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	rm := core.NewRegistryManager(d.config.RegistriesDir())

	// Resolve target agents.
	var targetAgents []core.AgentDef
	if agentsFlag != "" {
		names := strings.Split(agentsFlag, ",")
		for i := range names {
			names[i] = strings.TrimSpace(names[i])
		}
		resolved, resolveErr := core.ResolveAgentsByNames(d.agents, names)
		if resolveErr != nil {
			return nil, resolveErr
		}
		for _, a := range resolved {
			if a.MCPConfigPath != "" {
				targetAgents = append(targetAgents, a)
			}
		}
	}

	for _, lockedMCP := range lf.MCPs {
		// Look up the MCP config from the registry.
		mcpInfo, findErr := rm.FindMCP(cfg.Registries, lockedMCP.Name, "")
		if findErr != nil {
			fmt.Fprintf(os.Stderr, "! MCP %q: registry %q not configured\n", lockedMCP.Name, lockedMCP.Registry)
			fmt.Fprintf(os.Stderr, "  Run: duckrow registry add <url>\n")
			result.errors++
			continue
		}

		if dryRun {
			fmt.Fprintf(os.Stdout, "install: %s (from %s)\n", lockedMCP.Name, mcpInfo.RegistryName)
			result.installed++
			// Collect required env vars for summary.
			for _, v := range lockedMCP.RequiredEnv {
				result.requiredEnv[v] = append(result.requiredEnv[v], lockedMCP.Name)
			}
			continue
		}

		// Determine agents for this MCP.
		agents := targetAgents
		if len(agents) == 0 {
			// Use the agents from the lock entry, resolved to agent definitions.
			resolved, resolveErr := core.ResolveAgentsByNames(d.agents, lockedMCP.Agents)
			if resolveErr != nil {
				// Fall back to all MCP-capable agents.
				resolved = core.GetMCPCapableAgents(d.agents)
			}
			agents = resolved
		}

		// Install MCP config.
		installResult, installErr := core.InstallMCPConfig(mcpInfo.MCP, core.MCPInstallOptions{
			ProjectDir:   targetDir,
			TargetAgents: agents,
			Force:        force,
		})
		if installErr != nil {
			fmt.Fprintf(os.Stderr, "Error: %s: %v\n", lockedMCP.Name, installErr)
			result.errors++
			continue
		}

		// Count actions.
		wrote := false
		for _, ar := range installResult.AgentResults {
			if ar.Action == "wrote" {
				wrote = true
			}
		}
		if wrote {
			fmt.Fprintf(os.Stdout, "Installed: %s\n", lockedMCP.Name)
			result.installed++
		} else {
			result.skipped++
		}

		// Collect required env vars for summary.
		for _, v := range lockedMCP.RequiredEnv {
			result.requiredEnv[v] = append(result.requiredEnv[v], lockedMCP.Name)
		}
	}

	return result, nil
}

// printRequiredEnvSummary prints a warning about required environment variables
// collected during MCP sync. envMap maps env var name -> []MCP names that use it.
func printRequiredEnvSummary(envMap map[string][]string) {
	if len(envMap) == 0 {
		return
	}

	// Collect and sort env var names for deterministic output.
	var vars []string
	for v := range envMap {
		vars = append(vars, v)
	}
	sort.Strings(vars)

	fmt.Fprintln(os.Stdout, "\n! The following environment variables are required:")
	for _, v := range vars {
		mcps := envMap[v]
		fmt.Fprintf(os.Stdout, "  %s  (used by %s)\n", v, strings.Join(mcps, ", "))
	}
	fmt.Fprintln(os.Stdout, "\n  Add values to .env.duckrow or ~/.duckrow/.env.duckrow")
}

func init() {
	// mcp install flags
	mcpInstallCmd.Flags().StringP("dir", "d", "", "Target directory (default: current directory)")
	mcpInstallCmd.Flags().StringP("registry", "r", "", "Registry to search (disambiguates duplicates)")
	mcpInstallCmd.Flags().String("agents", "", "Comma-separated agent names to target (e.g. cursor,claude-code)")
	mcpInstallCmd.Flags().Bool("no-lock", false, "Install without updating the lock file")
	mcpInstallCmd.Flags().Bool("force", false, "Overwrite existing MCP entry with the same name")

	// mcp uninstall flags
	mcpUninstallCmd.Flags().StringP("dir", "d", "", "Target directory (default: current directory)")
	mcpUninstallCmd.Flags().Bool("no-lock", false, "Remove without updating the lock file")

	// mcp sync flags
	mcpSyncCmd.Flags().StringP("dir", "d", "", "Target directory (default: current directory)")
	mcpSyncCmd.Flags().Bool("dry-run", false, "Show what would be done without making changes")
	mcpSyncCmd.Flags().Bool("force", false, "Overwrite existing MCP entries in agent config files")
	mcpSyncCmd.Flags().String("agents", "", "Comma-separated agent names to target (e.g. cursor,claude-code)")

	// Wire up subcommands.
	mcpCmd.AddCommand(mcpInstallCmd)
	mcpCmd.AddCommand(mcpUninstallCmd)
	mcpCmd.AddCommand(mcpSyncCmd)
	rootCmd.AddCommand(mcpCmd)
}
