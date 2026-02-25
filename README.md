<h1 align="center">🐥 duckrow</h1>

<p align="center"><i>Get your ducks in a row — manage AI agent skills across your team.</i></p>

<p align="center">
  <img src="docs/images/duckrow_tui.gif" alt="duckrow TUI" width="800" />
</p>

AI coding agents like Cursor, Claude Code, OpenCode, and GitHub Copilot use **skills** — markdown files that tell them how to behave in your project. Things like code review guidelines, test generation rules, or deployment checklists. They also use **MCP servers** — external tools that agents can call at runtime to query databases, APIs, and internal services.

The problem is keeping those skills and MCP configs consistent. When every developer manages them by hand, they drift between repos, conventions diverge, and nobody knows what's running where.

duckrow fixes this. It gives your team a single way to distribute, install, and pin AI agent skills and MCP server configurations — across every project and every developer.

**How it works:**

- **Set up a private registry** — a git repo with a manifest listing your team's approved skills and MCP servers
- **Install skills by name** — `duckrow skill install code-review` pulls the right version from the registry
- **Install MCPs by name** — `duckrow mcp install internal-db` writes the MCP config into agent files automatically
- **Pin with a lock file** — every install records the exact git commit (skills) or config hash (MCPs) in `duckrow.lock.json`, just like `package-lock.json` or `uv.lock`
- **Sync across the team** — teammates run `duckrow sync` and get identical skills and MCP configs, no manual setup
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

# Teammates sync from the lock file (skills + MCPs)
duckrow sync
```

## Interactive TUI

Run `duckrow` to launch the terminal UI. Browse installed skills and MCP configs, install from your registry, check for updates, and update skills — all without memorizing commands.

<p align="center">
  <img src="docs/images/duckrow_tui.png" alt="duckrow TUI screenshot" width="800" />
</p>

Key actions: navigate with `j`/`k`, preview a skill with `enter`, install with `i`, remove with `d`, update with `u`/`U`, refresh registries with `r`, switch folders with `b` (bookmarks), and open settings with `s`. Press `?` for the full keybinding reference.

The folder view uses tabs to switch between installed skills and MCPs. The install picker is context-aware — pressing `i` from the Skills tab shows available skills, and from the MCPs tab shows available MCPs, each with a multi-step wizard for system selection and env var setup.

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

$ duckrow status .
Folder: /Users/me/code/my-app [tracked]
  Skills (1):
    - code-review [.agents/skills/code-review]
      Review code changes
  MCPs (1):
    - internal-db          Query the internal database  [OpenCode, Claude Code, Cursor]

$ duckrow skill uninstall code-review -d .
Removed: code-review
```

## Supported Systems

duckrow detects which systems you use and installs skills to the right directories automatically.

| System | Skills Directory | Type | MCP Config |
|--------|-----------------|------|------------|
| OpenCode | `.agents/skills/` | Universal | `opencode.json` / `opencode.jsonc` |
| Codex | `.agents/skills/` | Universal | — |
| Gemini CLI | `.agents/skills/` | Universal | — |
| GitHub Copilot | `.agents/skills/` | Universal | `.vscode/mcp.json` |
| Claude Code | `.claude/skills/` | Symlinked | `.mcp.json` |
| Cursor | `.cursor/skills/` | Symlinked | `.cursor/mcp.json` |
| Goose | `.goose/skills/` | Symlinked | — |

**Universal** systems share `.agents/skills/` — the skill is written there once.

**Symlinked** systems have their own directory. duckrow creates symlinks from their directory back to `.agents/skills/`, so each skill exists in one place but works everywhere.

Systems with an MCP Config path support `duckrow mcp install` — duckrow writes MCP server configs directly into their config files, preserving existing content and comments.

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
duckrow status [path]             Show skills and MCPs for a folder
duckrow sync                      Install skills and MCPs from lock file at pinned versions
```

### MCP Servers

```
duckrow mcp install <name>      Install an MCP server config from a registry
duckrow mcp uninstall <name>    Remove an installed MCP server config
duckrow mcp uninstall --all     Remove all installed MCP server configs
duckrow mcp list                List installed MCP server configs
duckrow mcp sync                Restore MCP configs from lock file
```

### Registries

```
duckrow registry add <url>      Add a private skill registry
duckrow registry list           List configured registries
duckrow registry refresh [name] Refresh registry data (all if no name given)
duckrow registry remove <name>  Remove a registry
```

### Install Sources

The `skill install` command accepts multiple source formats:

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

Create a git repository with a `duckrow.json` file. Registries can list both skills and MCP server configurations. The v2 manifest uses an `assets` map keyed by kind:

```json
{
  "name": "my-org",
  "description": "Our team's approved skills and MCPs",
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
        "env": {
          "DB_URL": ""
        }
      },
      {
        "name": "analytics-api",
        "description": "Access the analytics API",
        "url": "https://mcp.my-org.internal/analytics",
        "type": "http"
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

## Lock File

Every `duckrow skill install` records the exact git commit in `duckrow.lock.json`. Every `duckrow mcp install` records the MCP name, registry, and a config hash. Commit this file to version control so your team gets reproducible installs.

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
