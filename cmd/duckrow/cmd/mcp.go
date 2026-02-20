package cmd

import (
	"fmt"
	"os"
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
				fmt.Fprintf(os.Stdout, "  + %-24s (%s)\n", ar.Agent.MCPConfigPath, ar.Agent.DisplayName)
				installedAgentNames = append(installedAgentNames, ar.Agent.Name)
			case "skipped":
				fmt.Fprintf(os.Stdout, "  ! %-24s %q %s\n", ar.Agent.MCPConfigPath, mcpName, ar.Message)
				// Still count as targeted for lock file.
				installedAgentNames = append(installedAgentNames, ar.Agent.Name)
			case "error":
				fmt.Fprintf(os.Stderr, "  x %-24s error: %s\n", ar.Agent.MCPConfigPath, ar.Message)
			}
		}

		// Update lock file.
		if !noLock {
			requiredEnv := core.ExtractRequiredEnv(mcpInfo.MCP.Env)
			lockEntry := core.LockedMCP{
				Name:        mcpName,
				Registry:    mcpInfo.RegistryName,
				ConfigHash:  core.ComputeConfigHash(mcpInfo.MCP),
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
				fmt.Fprintf(os.Stdout, "  - %-24s (%s)\n", ar.Agent.MCPConfigPath, ar.Agent.DisplayName)
			case "skipped":
				fmt.Fprintf(os.Stdout, "  . %-24s %s\n", ar.Agent.MCPConfigPath, ar.Message)
			case "error":
				fmt.Fprintf(os.Stderr, "  x %-24s error: %s\n", ar.Agent.MCPConfigPath, ar.Message)
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

	// Wire up subcommands.
	mcpCmd.AddCommand(mcpInstallCmd)
	mcpCmd.AddCommand(mcpUninstallCmd)
	rootCmd.AddCommand(mcpCmd)
}
