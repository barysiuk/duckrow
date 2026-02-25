# Registries

How to create and use private registries to distribute skills and MCP server configurations across your team.

## Overview

A registry is a git repository containing a `duckrow.json` manifest at the root. The manifest catalogs available skills and MCP server configurations. Teams use registries to maintain a curated set of approved assets that any developer can install by name.

```bash
# Add a registry
duckrow registry add git@github.com:my-org/skill-registry.git

# Install a skill by name
duckrow skill install code-review

# Install an MCP by name
duckrow mcp install internal-db
```

Registries are cloned locally to `~/.duckrow/registries/` and refreshed on demand. Authentication is handled by git — if you can `git clone` the URL, duckrow can use it.

## Creating a Registry

### 1. Create a git repository

Any git repository works — GitHub, GitLab, Bitbucket, self-hosted, etc. The repository just needs a `duckrow.json` file at its root.

### 2. Write the manifest

The manifest lists the assets your team can install. It supports two asset kinds: **skills** and **MCP servers**.

```json
{
  "version": 2,
  "name": "my-org",
  "description": "Our team's approved skills and MCPs",
  "assets": {
    "skill": [
      {
        "name": "code-review",
        "description": "Review code with our style guidelines",
        "source": "github.com/my-org/skills/skills/code-review"
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
      }
    ]
  }
}
```

### 3. Push and share

```bash
git add duckrow.json
git commit -m "Add skill registry manifest"
git push
```

Share the clone URL with your team. Each developer adds the registry once and can install any listed asset by name.

## Manifest Format

### Top-level fields

| Field | Required | Description |
|-------|----------|-------------|
| `version` | No | Manifest version. Use `2` for the current format. Omitting defaults to v1. |
| `name` | Yes | Display name for the registry (used in CLI output and TUI) |
| `description` | No | Human-readable description |
| `assets` | Yes (v2) | Map of asset arrays, keyed by kind (`"skill"`, `"mcp"`) |

### Legacy v1 format

The v1 format uses top-level `skills` and `mcps` arrays instead of the `assets` map. It is still supported for backward compatibility:

```json
{
  "name": "my-org",
  "skills": [ ... ],
  "mcps": [ ... ]
}
```

When `version` is omitted or set to `1`, duckrow treats the manifest as v1. The v2 format with `assets` is recommended for new registries.

## Adding Skills to a Registry

Skills in a registry point to a source repository where the actual `SKILL.md` files live. The registry manifest doesn't contain the skill content — it tells duckrow where to find it.

### Skill entry fields

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Skill name (must match the `name` field in `SKILL.md`) |
| `description` | No | Human-readable description (shown in TUI and `registry list --verbose`) |
| `source` | Yes | Canonical source path in `host/owner/repo/path/to/skill` format |
| `commit` | No | Pin to a specific git commit SHA. Omit to track the latest. |

### Source format

The `source` field uses a canonical format: `host/owner/repo/path/to/skill`. This tells duckrow which repository to clone and where within it the skill lives.

```json
{
  "name": "go-review",
  "description": "Reviews Go code for best practices",
  "source": "github.com/acme/skills/skills/engineering/go-review"
}
```

This means: clone `github.com/acme/skills`, then look for a skill named `go-review` under `skills/engineering/go-review/`.

### Pinning vs tracking latest

**Pinned skills** have an explicit `commit` field. When a developer runs `duckrow skill install go-review`, duckrow clones the source repo at that exact commit. Updates are controlled by the registry author — change the `commit` field and push.

```json
{
  "name": "go-review",
  "source": "github.com/acme/skills/skills/engineering/go-review",
  "commit": "a1b2c3d4e5f6789012345678901234567890abcd"
}
```

**Unpinned skills** omit the `commit` field. duckrow resolves the latest commit automatically through a process called [commit hydration](#commit-hydration). This is useful when you want developers to always get the latest version.

```json
{
  "name": "go-review",
  "source": "github.com/acme/skills/skills/engineering/go-review"
}
```

### Example: multi-skill registry

A registry can list skills from multiple source repositories:

```json
{
  "version": 2,
  "name": "acme-engineering",
  "description": "ACME engineering skills and tools",
  "assets": {
    "skill": [
      {
        "name": "go-review",
        "description": "Go code review guidelines",
        "source": "github.com/acme/go-skills/skills/go-review",
        "commit": "a1b2c3d4e5f6789012345678901234567890abcd"
      },
      {
        "name": "python-lint",
        "description": "Python linting rules",
        "source": "github.com/acme/python-skills/skills/python-lint"
      },
      {
        "name": "deploy-checklist",
        "description": "Pre-deployment verification steps",
        "source": "github.com/acme/ops-skills/skills/deploy-checklist",
        "commit": "f6e5d4c3b2a1098765432109876543210fedcba"
      }
    ]
  }
}
```

### Installing a skill from a registry

```bash
# Install by name — duckrow looks it up in configured registries
duckrow skill install go-review

# If the same name exists in multiple registries, disambiguate
duckrow skill install go-review --registry acme-engineering

# Install into a specific directory
duckrow skill install go-review -d ~/code/my-project
```

When a skill name is used (not a URL or GitHub shorthand), duckrow:

1. Searches all configured registries for a skill with that name
2. If found in exactly one registry, reads the `source` field
3. Clones the source repo (at the pinned commit if set, or latest otherwise)
4. Installs the skill to `.agents/skills/<name>/`
5. Records the commit in `duckrow.lock.json`

If the skill is found in multiple registries, duckrow returns an error asking you to use `--registry` to disambiguate.

## Adding MCP Servers to a Registry

MCP (Model Context Protocol) servers are external tools that AI agents can call at runtime. Unlike skills (which are files copied to disk), MCP entries are **config-only** — duckrow writes them directly into system config files like `opencode.json`, `.mcp.json`, and `.cursor/mcp.json`.

There are two types of MCP servers: **stdio** (command-based) and **remote** (URL-based).

### Stdio MCP servers

Stdio MCPs run as local processes. The agent launches the command and communicates with it over stdin/stdout.

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | MCP server name (used as the config key) |
| `description` | No | Human-readable description |
| `command` | Yes | The executable to run (e.g., `npx`, `uvx`, `node`) |
| `args` | No | Array of command-line arguments |
| `env` | No | Map of environment variables required at runtime |

```json
{
  "name": "internal-db",
  "description": "Query the internal database",
  "command": "npx",
  "args": ["-y", "@my-org/mcp-db"],
  "env": {
    "DB_URL": ""
  }
}
```

#### Environment variables

The `env` field declares which environment variables the MCP server needs at runtime. The values in the manifest are ignored — only the key names matter. duckrow uses them to:

1. Prompt for values during TUI install (if not already set)
2. Record the required vars in the lock file
3. Inject values at runtime via the `duckrow env` wrapper

When a stdio MCP is installed, duckrow wraps the command so that `duckrow env` runs first and injects the required environment variables:

```
# What gets written to the config file:
duckrow env --mcp internal-db -- npx -y @my-org/mcp-db

# At runtime, duckrow env:
# 1. Reads required env vars from duckrow.lock.json
# 2. Resolves values from (in priority order):
#    - Process environment
#    - Project .env.duckrow
#    - Global ~/.duckrow/.env.duckrow
# 3. Execs the real command with those variables set
```

This means secrets never appear in committed config files. Developers store them in `.env.duckrow` (gitignored) and the wrapper injects them at runtime.

#### Setting env var values

After installing an MCP that requires environment variables, add the values to one of two locations:

```bash
# Project-level (gitignored, only this repo)
echo "DB_URL=postgres://localhost/mydb" >> .env.duckrow

# Global (all projects using this MCP)
echo "DB_URL=postgres://localhost/mydb" >> ~/.duckrow/.env.duckrow
```

The TUI install wizard prompts for each missing value and offers a choice between project and global storage.

The `.env.duckrow` file supports `KEY=VALUE`, quoted values (`KEY="VALUE"` or `KEY='VALUE'`), comments (`# ...`), and the `export` prefix.

### Remote MCP servers

Remote MCPs connect to a URL endpoint. No local process is launched — the agent communicates with the server over HTTP.

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | MCP server name (used as the config key) |
| `description` | No | Human-readable description |
| `url` | Yes | The endpoint URL |
| `type` | Yes | Transport type: `"http"`, `"sse"`, or `"streamable-http"` |

```json
{
  "name": "analytics-api",
  "description": "Access the analytics API",
  "url": "https://mcp.my-org.internal/analytics",
  "type": "http"
}
```

Remote MCPs don't support `env` — authentication is assumed to be handled by the endpoint itself (e.g., network-level auth, API gateway).

### Validation rules

Each MCP entry must have exactly one of `command` or `url`. duckrow emits warnings during manifest parsing for:

- Missing `name` field
- Missing both `command` and `url`
- Having both `command` and `url`
- Remote MCPs missing `type`

### Where MCP configs are written

When you run `duckrow mcp install`, duckrow writes the MCP entry into the config files of detected MCP-capable systems:

| System | Config File | Config Key |
|--------|-------------|------------|
| OpenCode | `opencode.json` / `opencode.jsonc` | `mcp` |
| GitHub Copilot | `.vscode/mcp.json` | `servers` |
| Cursor | `.cursor/mcp.json` | `mcpServers` |
| Claude Code | `.mcp.json` | `mcpServers` |

Each system has its own JSON structure. duckrow handles the format differences — you only need to define the MCP once in the registry.

### Example: mixed MCP registry

```json
{
  "version": 2,
  "name": "acme-tools",
  "description": "ACME internal tools and APIs",
  "assets": {
    "mcp": [
      {
        "name": "internal-db",
        "description": "Query the internal PostgreSQL database",
        "command": "npx",
        "args": ["-y", "@acme/mcp-postgres"],
        "env": {
          "DATABASE_URL": ""
        }
      },
      {
        "name": "jira-search",
        "description": "Search JIRA issues and create tickets",
        "command": "uvx",
        "args": ["mcp-jira"],
        "env": {
          "JIRA_URL": "",
          "JIRA_TOKEN": ""
        }
      },
      {
        "name": "internal-docs",
        "description": "Search internal documentation",
        "url": "https://mcp.acme.internal/docs",
        "type": "http"
      }
    ]
  }
}
```

### Installing an MCP from a registry

```bash
# Install for all detected MCP-capable systems
duckrow mcp install internal-db

# Install from a specific registry
duckrow mcp install internal-db --registry acme-tools

# Install for specific systems only
duckrow mcp install internal-db --systems opencode,cursor

# Overwrite an existing entry
duckrow mcp install internal-db --force
```

## Combining Skills and MCPs

A single registry can contain both skills and MCPs. This is the recommended approach — one registry per team or organization.

```json
{
  "version": 2,
  "name": "acme",
  "description": "ACME team skills and tools",
  "assets": {
    "skill": [
      {
        "name": "code-review",
        "description": "Code review guidelines",
        "source": "github.com/acme/skills/skills/code-review",
        "commit": "a1b2c3d4e5f6789012345678901234567890abcd"
      },
      {
        "name": "test-generation",
        "description": "Generate unit tests for Go packages",
        "source": "github.com/acme/skills/skills/test-generation"
      }
    ],
    "mcp": [
      {
        "name": "internal-db",
        "description": "Query the internal database",
        "command": "npx",
        "args": ["-y", "@acme/mcp-db"],
        "env": {
          "DB_URL": ""
        }
      },
      {
        "name": "deploy-api",
        "description": "Trigger deployments via API",
        "url": "https://mcp.acme.internal/deploy",
        "type": "http"
      }
    ]
  }
}
```

Install everything a new team member needs:

```bash
# Add the registry (one-time setup)
duckrow registry add git@github.com:acme/skill-registry.git

# Install skills
duckrow skill install code-review
duckrow skill install test-generation

# Install MCPs
duckrow mcp install internal-db
duckrow mcp install deploy-api

# Commit the lock file so teammates can sync
git add duckrow.lock.json
git commit -m "Add team skills and MCP dependencies"
```

## Managing Registries

### Adding a registry

```bash
# HTTPS
duckrow registry add https://github.com/acme/skill-registry.git

# SSH
duckrow registry add git@github.com:acme/skill-registry.git
```

duckrow clones the repository to `~/.duckrow/registries/` and parses the manifest. The registry name comes from the `name` field in `duckrow.json`.

### Listing registries

```bash
# Names and asset counts
duckrow registry list

# Include full asset details
duckrow registry list --verbose
```

### Refreshing

```bash
# Refresh all registries (git pull + commit hydration)
duckrow registry refresh

# Refresh a specific registry
duckrow registry refresh acme
```

Refreshing pulls the latest changes from the remote and runs commit hydration for unpinned skills.

### Removing a registry

```bash
duckrow registry remove acme
```

This removes the registry from the config and deletes the local clone. Installed assets are not affected.

### Multiple registries

You can configure multiple registries. When installing by name, duckrow searches all registries. If a name exists in more than one registry, you must use `--registry` to disambiguate:

```bash
# Error: found "code-review" in registries: acme, other-org
duckrow skill install code-review

# Specify which registry
duckrow skill install code-review --registry acme
```

## Commit Hydration

When a registry lists skills without a `commit` field (unpinned), duckrow needs to determine what the latest commit is. This process is called **commit hydration**.

### How it works

1. Groups unpinned skills by source repository
2. Performs a shallow clone of each unique source repo
3. Runs `git log` to determine the latest commit for each skill's sub-path
4. Caches the resolved commits to `duckrow.commits.json` alongside the registry clone

### When it runs

- **TUI startup** — registries are refreshed asynchronously in the background
- **TUI `[r]` refresh** — triggers a full registry refresh including hydration
- **`duckrow skill outdated`** — hydrates before checking for updates
- **`duckrow skill update`** — hydrates before applying updates
- **`duckrow registry refresh`** — hydrates as part of the refresh

### Pinned vs hydrated precedence

When building the commit map for update detection, pinned commits from the manifest always take precedence over cached (hydrated) commits. This means a registry author can override the latest by explicitly pinning a commit.

## Clone URL Overrides

If a skill source uses HTTPS but your team requires SSH authentication, you can configure clone URL overrides in `~/.duckrow/config.json`:

```json
{
  "settings": {
    "cloneURLOverrides": {
      "acme/private-skills": "git@github.com:acme/private-skills.git"
    }
  }
}
```

The key is `owner/repo` (lowercase). When duckrow resolves a source matching that key, it uses the override URL for cloning instead of constructing one from the source path.

## TUI Registry Workflows

The TUI provides visual workflows for registry management.

### Installing from a registry

1. Press `i` from the folder view to open the install picker
2. The picker shows all available assets from configured registries (filtered by the active tab — Skills or MCP Servers)
3. Select an asset and press `enter`
4. **For skills**: a system selection step appears if non-universal systems are detected
5. **For MCPs**: a multi-step wizard handles system selection, env var preview, and value entry
6. The asset is installed and the lock file is updated

### Managing registries

Press `s` to open the Settings view, which shows all configured registries. From there:

- Press `enter` to add a new registry (opens a URL entry wizard)
- Press `d` to remove a selected registry
- Press `r` to refresh a selected registry

### Background refresh

On TUI startup, registries are refreshed asynchronously. A spinner appears in the status bar during this process. The TUI remains fully interactive — you don't need to wait for the refresh to complete before browsing or installing.

When the refresh finishes, update indicators appear on skills that have newer commits available.
