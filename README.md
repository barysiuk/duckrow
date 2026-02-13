# duckrow

*"Get your ducks in a row" — manage AI agent skills across your team.*

<p align="center">
  <img src="docs/images/duckrow_tui.gif" alt="duckrow TUI" width="800" />
</p>

duckrow helps engineering teams curate, distribute, and manage approved AI agent skills across every project and every developer. Set up a private registry once, and your entire team gets the same vetted skills — code review guidelines, test generation rules, deployment checklists — installed consistently wherever they work.

AI coding agents like Cursor, Claude Code, OpenCode, and others use **skills** — markdown files that shape how they behave in a project. Without coordination, skills drift between repos, conventions diverge, and there's no visibility into what's running where.

duckrow solves this at the team level. Maintain a private skill registry in a git repo your organization controls, then let developers browse, install, and update from that catalog. The interactive TUI gives everyone a single view of installed skills, available registry skills, and which agents are active — right in the terminal. One binary, no dependencies.

## Quick Start

### Install

```bash
# Homebrew
brew install barysiuk/tap/duckrow

# Or grab a binary from GitHub Releases
# https://github.com/barysiuk/duckrow/releases
```

### First steps

```bash
# Start tracking a project
duckrow add ~/code/my-app

# Install a skill from GitHub
duckrow install owner/repo -d ~/code/my-app

# See what's installed across all your projects
duckrow status
```

## Demo

### TUI

Run `duckrow` in any project folder to launch the interactive terminal UI. Browse installed skills, install from your team's registry, remove what you don't need — all without memorizing commands.

```bash
duckrow
```

<p align="center">
  <img src="docs/images/duckrow_tui.png" alt="duckrow TUI screenshot" width="800" />
</p>

### CLI

Every action available in the TUI also works as a direct command — useful for scripting, CI, or when you already know what you need.

```
$ duckrow add .
Added folder: /Users/me/code/my-app

$ duckrow install vercel/agents -d . --skill code-review
Installed: code-review
  Path: .agents/skills/code-review
  Agents: OpenCode, Codex, Gemini CLI, GitHub Copilot

$ duckrow status .
Folder: /Users/me/code/my-app
  Agents: OpenCode, Codex, Gemini CLI, GitHub Copilot
  Skills (1):
    - code-review v1.0.0 [OpenCode, Codex, Gemini CLI, GitHub Copilot]
      Review code changes

$ duckrow install ./my-local-skills -d .
Installed: test-gen
  Path: .agents/skills/test-gen
  Agents: OpenCode, Codex, Gemini CLI, GitHub Copilot

$ duckrow status
Folder: /Users/me/code/my-app
  Agents: OpenCode, Codex, Gemini CLI, GitHub Copilot
  Skills (2):
    - code-review v1.0.0 [OpenCode, Codex, Gemini CLI, GitHub Copilot]
      Review code changes
    - test-gen v1.0.0 [OpenCode, Codex, Gemini CLI, GitHub Copilot]
      Generate test cases

$ duckrow uninstall code-review -d .
Removed: code-review
```

## Supported Agents

duckrow detects which agents you use and installs skills to the right directories automatically.

| Agent | Skills Directory | Type |
|-------|-----------------|------|
| OpenCode | `.agents/skills/` | Universal |
| Codex | `.agents/skills/` | Universal |
| Gemini CLI | `.agents/skills/` | Universal |
| GitHub Copilot | `.agents/skills/` | Universal |
| Claude Code | `.claude/skills/` | Symlinked |
| Cursor | `.cursor/skills/` | Symlinked |
| Goose | `.goose/skills/` | Symlinked |
| Windsurf | `.windsurf/skills/` | Symlinked |
| Cline | `.cline/skills/` | Symlinked |

**Universal** agents share `.agents/skills/` — the skill is written there once.

**Symlinked** agents have their own directory. duckrow creates symlinks from their directory back to `.agents/skills/`, so each skill exists in one place but works everywhere.

## Commands

### Folder Management

```
duckrow add [path]              Add a folder to the tracked list (default: current dir)
duckrow remove-folder <path>    Remove a folder from the tracked list
duckrow folders                 List all tracked folders
```

### Skills

```
duckrow install <source>        Install skill(s) from a source
duckrow uninstall <skill-name>  Remove an installed skill
duckrow uninstall-all           Remove all installed skills
duckrow status [path]           Show skills and agents for tracked folders
```

### Registries

```
duckrow registry add <url>      Add a private skill registry
duckrow registry list           List configured registries
duckrow registry refresh [name] Refresh registry data (all if no name given)
duckrow registry remove <name>  Remove a registry
```

### Install Sources

The `install` command accepts multiple source formats:

```bash
duckrow install owner/repo                    # GitHub shorthand
duckrow install owner/repo@skill-name         # Specific skill from a repo
duckrow install ./local/path                  # Local directory
duckrow install https://github.com/owner/repo # Full URL
duckrow install git@host:owner/repo.git       # SSH clone URL
```

**Flags:**

| Flag | Short | Description |
|------|-------|-------------|
| `--dir` | `-d` | Target directory (default: current directory) |
| `--skill` | `-s` | Install only a specific skill by name |
| `--internal` | | Include internal (hidden) skills |

## Folders

Folders are bookmarks for your projects. Add any directory on your system and duckrow will track which skills and agents are active there. This gives you a single view across your entire file system — no matter how many repos you work in.

```bash
duckrow add ~/code/frontend
duckrow add ~/code/backend
duckrow add ~/code/infra
```

Once tracked, you can check the state of every project at a glance with `duckrow status`, or switch between them in the TUI with a single keystroke. When your team approves a new skill, you can install it across multiple projects from one place instead of repeating the work in each repo.

## Private Registries

Teams can maintain a curated catalog of approved skills using a private git repository. A registry is just a git repo with a `duckrow.json` manifest at the root.

### Setting up a registry

Create a git repository with a `duckrow.json` file:

```json
{
  "name": "my-org",
  "description": "Our team's approved skills",
  "skills": [
    {
      "name": "code-review",
      "description": "Review code with our style guidelines",
      "source": "github.com/my-org/skills/code-review",
      "version": "1.2.0"
    },
    {
      "name": "pr-guidelines",
      "description": "PR description and review standards",
      "source": "github.com/my-org/engineering-skills",
      "version": "2.0.0"
    }
  ]
}
```

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

A skill is a directory containing a `SKILL.md` file (and optionally other markdown files). The `SKILL.md` has YAML frontmatter with metadata:

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

When you run `duckrow install`, duckrow:

1. Copies the skill to `.agents/skills/<name>/` (the canonical location)
2. Detects which agents are present on your system
3. Creates symlinks in each detected agent's skills directory (e.g., `.cursor/skills/<name>/` -> `.agents/skills/<name>/`)

This means each skill exists once on disk but is available to every agent.

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
