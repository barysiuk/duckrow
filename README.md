# DuckRow

*"Get your ducks in a row" — manage AI agent skills across all your projects.*

Heavily inspired by [Vercel's Skill CLI](https://github.com/vercel-labs/skills), DuckRow goes further — it manages skills across multiple projects at once and supports private registries for distributing skills within your organization securely.

AI coding agents like Cursor, Claude Code, OpenCode, and others use **skills** — markdown files that shape how they behave in your project. Think code review guidelines, test generation rules, or deployment checklists.

The problem: skills get scattered across projects, every agent has its own directory convention, and there's no way to see what's installed where. If you work across multiple repos, you're on your own.

DuckRow fixes this. It tracks your project folders, installs and removes skills with automatic agent detection, manages private skill registries for your team, and gives you a single command to see everything. One binary, no dependencies.

## Quick Start

### Install

```bash
# Homebrew
brew install barysiuk/tap/duckrow

# Go
go install github.com/barysiuk/duckrow/cmd/duckrow@latest

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

DuckRow detects which agents you use and installs skills to the right directories automatically.

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
| Continue | `.continue/skills/` | Symlinked |

**Universal** agents share `.agents/skills/` — the skill is written there once.

**Symlinked** agents have their own directory. DuckRow creates symlinks from their directory back to `.agents/skills/`, so each skill exists in one place but works everywhere.

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

Authentication is handled by git — if you can `git clone` the URL, DuckRow can use it.

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

When you run `duckrow install`, DuckRow:

1. Copies the skill to `.agents/skills/<name>/` (the canonical location)
2. Detects which agents are present on your system
3. Creates symlinks in each detected agent's skills directory (e.g., `.cursor/skills/<name>/` -> `.agents/skills/<name>/`)

This means each skill exists once on disk but is available to every agent.

## Configuration

DuckRow stores its configuration at `~/.duckrow/config.json`:

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
