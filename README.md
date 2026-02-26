<h1 align="center">🐥 duckrow</h1>

<p align="center"><i>Get your ducks in a row — manage AI agent skills across your team.</i></p>

<p align="center">
  <img src="docs/images/duckrow_tui.gif" alt="duckrow TUI" width="800" />
</p>

AI coding agents like Cursor, Claude Code, OpenCode, and GitHub Copilot use **skills** — markdown files that tell them how to behave in your project. Things like code review guidelines, test generation rules, or deployment checklists. They also use **MCP servers** — external tools that agents can call at runtime to query databases, APIs, and internal services. And they use **agents** — custom subagent personas defined in markdown files with YAML frontmatter that specialize an AI tool for a particular role.

The problem is keeping those skills, MCP configs, and agent definitions consistent. When every developer manages them by hand, they drift between repos, conventions diverge, and nobody knows what's running where.

duckrow fixes this. It gives your team a single way to distribute, install, and pin AI agent skills, MCP server configurations, and custom agents — across every project and every developer.

**How it works:**

- **Set up a private registry** — a git repo with a manifest listing your team's approved skills, MCP servers, and agents
- **Install skills by name** — `duckrow skill install code-review` pulls the right version from the registry
- **Install MCPs by name** — `duckrow mcp install internal-db` writes the MCP config into agent files automatically
- **Install agents by name** — `duckrow agent install deploy-specialist` renders the agent into each system's agents directory
- **Pin with a lock file** — every install records the exact git commit (skills, agents) or config hash (MCPs) in `duckrow.lock.json`, just like `package-lock.json` or `uv.lock`
- **Sync across the team** — teammates run `duckrow sync` and get identical skills, MCP configs, and agents, no manual setup
- **Update when ready** — `duckrow skill outdated` shows what changed, `duckrow skill update` moves forward

One binary, no dependencies. Works with any git host. Has a nice intuitive TUI.

## Quick Start

```bash
# Install
brew install barysiuk/tap/duckrow

# Bookmark a project
duckrow bookmark add ~/code/my-app

# Install a skill from a registry
duckrow skill install code-review -d ~/code/my-app

# Install a skill from a GitHub repo
duckrow skill install acme/skills -d ~/code/my-app

# Install an MCP server config
duckrow mcp install internal-db -d ~/code/my-app

# Install an agent from a registry
duckrow agent install deploy-specialist -d ~/code/my-app

# Teammates sync from the lock file (skills + MCPs + agents)
duckrow sync
```

## Interactive TUI

Run `duckrow` to launch the terminal UI. Browse installed skills, MCP configs, and agents, install from your registry, check for updates, and update skills — all without memorizing commands.

<p align="center">
  <img src="docs/images/duckrow_tui.png" alt="duckrow TUI screenshot" width="800" />
</p>

Key actions: navigate with `j`/`k`, preview a skill with `enter`, install with `i`, remove with `d`, update with `u`/`U`, refresh registries with `r`, switch folders with `b` (bookmarks), and open settings with `s`. Press `?` for the full keybinding reference.

The folder view uses tabs to switch between installed skills, MCPs, and agents. The install picker is context-aware — pressing `i` from the Skills tab shows available skills, from the MCPs tab shows available MCPs, and from the Agents tab shows available agents, each with a multi-step wizard for system selection and env var setup.

The TUI automatically detects available updates for registry-tracked skills and shows update badges inline. Registry data is refreshed asynchronously in the background — the UI is fully interactive from the first frame.

See [docs/tui.md](docs/tui.md) for the full TUI reference.

## CLI

Every action in the TUI also works as a direct command — useful for scripting, CI, or when you already know what you need.

```
$ duckrow bookmark add .
Bookmarked: /Users/me/code/my-app

$ duckrow skill install acme/skills -d .
Installed: code-review
  Path: .agents/skills/code-review
  Systems: OpenCode, Codex, Gemini CLI, GitHub Copilot

$ duckrow mcp install internal-db -d .
Installing MCP "internal-db" from registry "my-org"...

Wrote MCP config to:
  + opencode.json           (OpenCode)
  + .mcp.json               (Claude Code)
  + .cursor/mcp.json        (Cursor)

Updated duckrow.lock.json

MCP "internal-db" installed successfully.

$ duckrow agent install deploy-specialist -d .
Installed agent: deploy-specialist
  Files:
    .claude/agents/deploy-specialist.md     (Claude Code)
    .opencode/agents/deploy-specialist.md   (OpenCode)
    .github/agents/deploy-specialist.md     (GitHub Copilot)
    .gemini/agents/deploy-specialist.md     (Gemini CLI)

$ duckrow status .
Folder: /Users/me/code/my-app [tracked]
  Skills (1):
    - code-review [.agents/skills/code-review]
      Review code changes
  MCPs (1):
    - internal-db          Query the internal database  [OpenCode, Claude Code, Cursor]
  Agents (1):
    - deploy-specialist    Deployment automation agent  [Claude Code, OpenCode, GitHub Copilot, Gemini CLI]

$ duckrow skill uninstall code-review -d .
Removed: code-review
```

## Supported Systems

duckrow detects which systems you use and installs skills to the right directories automatically.

| System | Skills Directory | Type | MCP Config | Agents Directory |
|--------|-----------------|------|------------|------------------|
| OpenCode | `.agents/skills/` | Universal | `opencode.json` / `opencode.jsonc` | `.opencode/agents/` |
| Codex | `.agents/skills/` | Universal | — | — |
| Gemini CLI | `.agents/skills/` | Universal | — | `.gemini/agents/` |
| GitHub Copilot | `.agents/skills/` | Universal | `.vscode/mcp.json` | `.github/agents/` |
| Claude Code | `.claude/skills/` | Symlinked | `.mcp.json` | `.claude/agents/` |
| Cursor | `.cursor/skills/` | Symlinked | `.cursor/mcp.json` | — |
| Goose | `.goose/skills/` | Symlinked | — | — |

**Universal** systems share `.agents/skills/` — the skill is written there once.

**Symlinked** systems have their own directory. duckrow creates symlinks from their directory back to `.agents/skills/`, so each skill exists in one place but works everywhere.

Systems with an MCP Config path support `duckrow mcp install` — duckrow writes MCP server configs directly into their config files, preserving existing content and comments.

Systems with an Agents Directory support `duckrow agent install` — duckrow renders agent files directly into each system's agents directory with system-specific frontmatter overrides applied.

## Commands

For the full command reference with all flags and examples, see [docs/cli_reference.md](docs/cli_reference.md).

### Bookmarks

```
duckrow bookmark add [path]     Bookmark a folder (default: current dir)
duckrow bookmark remove <path>  Remove a bookmark
duckrow bookmark list           List all bookmarks
```

### Skills

```
duckrow skill install <source>    Install skill(s) from a source or registry
duckrow skill uninstall <name>    Remove an installed skill
duckrow skill uninstall --all     Remove all installed skills
duckrow skill list                List installed skills
duckrow skill outdated            Show skills with available updates
duckrow skill update [name]       Update skill(s) to the available commit
duckrow skill sync                Install skills from lock file
duckrow status [path]             Show skills, agents, and MCPs for a folder
duckrow sync                      Install skills, agents, and MCPs from lock file at pinned versions
```

### MCP Servers

```
duckrow mcp install <name>      Install an MCP server config from a registry
duckrow mcp uninstall <name>    Remove an installed MCP server config
duckrow mcp uninstall --all     Remove all installed MCP server configs
duckrow mcp list                List installed MCP server configs
duckrow mcp sync                Restore MCP configs from lock file
```

### Agents

```
duckrow agent install <source>    Install agent(s) from a source or registry
duckrow agent uninstall <name>    Remove an installed agent
duckrow agent uninstall --all     Remove all installed agents
duckrow agent list                List installed agents
duckrow agent sync                Install agents from lock file
```

### Registries

```
duckrow registry add <url>      Add a private skill registry
duckrow registry list           List configured registries
duckrow registry refresh [name] Refresh registry data (all if no name given)
duckrow registry remove <name>  Remove a registry
```

### Install Sources

The `skill install` and `agent install` commands accept multiple source formats:

```bash
duckrow skill install owner/repo                    # GitHub shorthand
duckrow skill install owner/repo@skill-name         # Specific skill from a repo
duckrow skill install https://github.com/owner/repo # Full URL
duckrow skill install git@host:owner/repo.git       # SSH clone URL
duckrow skill install go-review                     # Install from configured registries
```

**Flags:**

| Flag | Short | Description |
|------|-------|-------------|
| `--dir` | `-d` | Target directory (default: current directory) |
| `--registry` | `-r` | Registry to search (disambiguates duplicates) |
| `--internal` | | Include internal (hidden) skills |
| `--systems` | | Comma-separated system names for symlinks |
| `--no-lock` | | Skip writing to the lock file |

## Bookmarks

Bookmarks are project folders you want to manage with duckrow. Add any directory on your system and duckrow will keep track of which skills and systems are active there. This gives you a single view across your entire file system — no matter how many repos you work in.

```bash
duckrow bookmark add ~/code/frontend
duckrow bookmark add ~/code/backend
duckrow bookmark add ~/code/infra
```

Once bookmarked, you can check the state of every project at a glance with `duckrow status`, or switch between them in the TUI with a single keystroke. When your team approves a new skill, you can install it across multiple projects from one place instead of repeating the work in each repo.

## Private Registries

Teams can maintain a curated catalog of approved skills and MCP server configurations using a private git repository. A registry is just a git repo with a `duckrow.json` manifest at the root.

> **Full reference:** See [docs/registries.md](docs/registries.md) for the complete guide — manifest format (v1/v2), MCP configuration per system, environment variables, clone URL overrides, and more.

### Setting up a registry

Create a git repository with a `duckrow.json` file. Registries can list skills, MCP server configurations, and agents. The v2 manifest uses an `assets` map keyed by kind:

```json
{
  "name": "my-org",
  "description": "Our team's approved skills, MCPs, and agents",
  "assets": {
    "skill": [
      {
        "name": "code-review",
        "description": "Review code with our style guidelines",
        "source": "github.com/my-org/skills/code-review",
        "commit": "a1b2c3d4e5f6789012345678901234567890abcd"
      }
    ],
    "mcp": [
      {
        "name": "internal-db",
        "description": "Query the internal database",
        "command": "npx",
        "args": ["-y", "@my-org/mcp-db"],
        "env": ["DB_URL"]
      },
      {
        "name": "analytics-api",
        "description": "Access the analytics API",
        "url": "https://mcp.my-org.internal/analytics",
        "type": "http"
      }
    ],
    "agent": [
      {
        "name": "deploy-specialist",
        "description": "Deployment automation agent",
        "source": "github.com/my-org/agents/deploy-specialist",
        "commit": "b2c3d4e5f6a7890123456789012345678901bcde"
      }
    ]
  }
}
```

The v1 format with top-level `skills` and `mcps` arrays is still supported for backward compatibility.

MCP entries are either **stdio** (command-based) or **remote** (URL-based). For stdio MCPs with `env` fields, duckrow wraps the command with `duckrow env` at runtime to inject secret values from `.env.duckrow` files without committing them to the config.

### Using a registry

```bash
# Add by any git URL your machine can clone
duckrow registry add git@github.com:my-org/skill-registry.git
duckrow registry add https://github.com/my-org/skill-registry.git

# List registries and their skills
duckrow registry list --verbose

# Pull the latest changes
duckrow registry refresh

# Remove when no longer needed
duckrow registry remove my-org
```

Authentication is handled by git — if you can `git clone` the URL, duckrow can use it.

## How Skills Work

A skill is a directory containing a `SKILL.md` file with YAML frontmatter and markdown instructions for the AI agent:

```markdown
---
name: code-review
description: Review code changes
metadata:
  version: "1.0.0"
  author: my-org
---

Your skill instructions go here...
```

When you run `duckrow skill install`, duckrow:

1. Clones the source repo
2. Walks the directory tree to discover all `SKILL.md` files
3. Copies each skill to `.agents/skills/<name>/` (the canonical location)
4. Creates symlinks in each requested system's skills directory (e.g., `.cursor/skills/<name>/` -> `.agents/skills/<name>/`)
5. Records the exact git commit in `duckrow.lock.json`

This means each skill exists once on disk but is available to every system.

When you run `duckrow mcp install <name>`, duckrow:

1. Looks up the MCP entry in configured registries
2. Detects which systems in the project support MCP (or uses the `--systems` flag)
3. Writes the MCP server entry into each system's config file (e.g., `opencode.json`, `.mcp.json`, `.cursor/mcp.json`)
4. For stdio MCPs, wraps the command with `duckrow env --mcp <name>` so env var secrets are injected at runtime from `.env.duckrow`
5. Records the MCP name, registry, and config hash in `duckrow.lock.json`

Skills can also be installed directly from a configured registry by name, without knowing the source repo — see [docs/skill_install.md](docs/skill_install.md) for the full details on discovery, installation, and the registry workflow.

## How Agents Work

An agent is a single markdown file with YAML frontmatter that defines a specialized persona for an AI coding tool:

```markdown
---
description: Handles deployment automation
model: claude-sonnet-4-20250514
tools:
  - bash
  - file_editor
claude-code:
  model: claude-sonnet-4-20250514
opencode:
  model: anthropic/claude-sonnet-4-20250514
---

You are a deployment specialist. Your job is to...
```

The frontmatter uses a **passthrough approach** — duckrow does not normalize or translate field values. Tool names, model identifiers, and all other fields are passed through verbatim. Users provide **system-specific override blocks** (e.g., `claude-code:`, `opencode:`, `github-copilot:`, `gemini-cli:`) in the frontmatter. Top-level fields serve as defaults; system-specific blocks override them when rendering for that system.

When you run `duckrow agent install`, duckrow:

1. Clones the source repo
2. Discovers all markdown files with valid agent frontmatter (must have `name` and `description`)
3. For each agent-capable system (Claude Code, OpenCode, GitHub Copilot, Gemini CLI), renders a system-specific version of the agent file — merging the base frontmatter with that system's override block
4. Writes the rendered file directly into each system's agents directory (e.g., `.claude/agents/deploy-specialist.md`)
5. Records the exact git commit in `duckrow.lock.json`

Unlike skills, agents do NOT have a canonical copy on disk. Each system gets its own rendered file directly in its agents directory. This means the same source agent can produce different frontmatter for different systems while sharing the same markdown body (system prompt).

Agents can also be installed from a configured registry by name — `duckrow agent install deploy-specialist`.

## Lock File

Every `duckrow skill install` records the exact git commit in `duckrow.lock.json`. Every `duckrow mcp install` records the MCP name, registry, and a config hash. Every `duckrow agent install` records the agent name and git commit. Commit this file to version control so your team gets reproducible installs.

```bash
# Teammates clone the repo and run sync to get identical skills and MCP configs
duckrow sync

# Check which skills have newer commits available
duckrow skill outdated

# Update a specific skill (or all at once)
duckrow skill update go-review
duckrow skill update --all
```

See [docs/lock-file.md](docs/lock-file.md) for the full lock file reference.

## Configuration

duckrow stores its configuration at `~/.duckrow/config.json`:

```json
{
  "folders": [
    { "path": "/Users/me/code/my-app" },
    { "path": "/Users/me/code/other-project" }
  ],
  "registries": [
    {
      "name": "my-org",
      "repo": "git@github.com:my-org/skill-registry.git"
    }
  ]
}
```

Registry clones are cached at `~/.duckrow/registries/`.

## License

[MIT](LICENSE)
