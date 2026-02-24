# CLI Command Reference

Complete reference for all duckrow CLI commands.

## Root

```bash
duckrow          # Launch interactive TUI
```

Running without arguments or subcommands opens the terminal UI. See [docs/tui.md](tui.md) for the full TUI reference including keybindings and workflows.

## Version

```bash
duckrow version
```

Prints version, commit hash, and build date.

## Folder Management

### add

Add a project folder to the tracked list.

```bash
# Add current directory
duckrow add

# Add a specific folder
duckrow add /path/to/project
```

| Argument | Required | Default | Description |
|----------|----------|---------|-------------|
| `path` | No | Current directory | Folder path to track |

### folders

List all tracked folders.

```bash
duckrow folders
```

No arguments or flags.

### remove-folder

Remove a folder from the tracked list. Does not delete any files on disk.

```bash
duckrow remove-folder /path/to/project
```

| Argument | Required | Description |
|----------|----------|-------------|
| `path` | Yes | Path of the folder to un-track |

## Skill Status

### status

Show installed skills, MCP configurations, and tracking status for a folder.

```bash
# Current directory
duckrow status

# Specific folder
duckrow status /path/to/project
```

| Argument | Required | Default | Description |
|----------|----------|---------|-------------|
| `path` | No | Current directory | Folder to inspect |

## Skill Installation

### install

Install skills from a git repository or configured registry.

```bash
# Install all skills from a GitHub repo
duckrow install acme/skills

# Install a specific skill using @ syntax
duckrow install acme/skills@go-review

# Install a specific skill using --skill flag
duckrow install acme/skills --skill go-review

# Install from a full URL
duckrow install https://github.com/acme/skills.git

# Install from an SSH clone URL
duckrow install git@github.com:acme/skills.git

# Install into a specific project directory
duckrow install acme/skills --dir /path/to/project

# Install including internal (hidden) skills
duckrow install acme/skills --internal

# Install and create symlinks for non-universal agents
duckrow install acme/skills --agents cursor,claude-code

# Install a skill from a configured registry (no source needed)
duckrow install --skill go-review

# Disambiguate when the same skill name exists in multiple registries
duckrow install --skill go-review --registry my-org
```

| Argument | Required | Default | Description |
|----------|----------|---------|-------------|
| `source` | No* | - | Source to install from (repo shorthand, URL, or SSH) |

*Either `source` or `--skill` (without source) is required.

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--dir` | `-d` | string | Current directory | Target project directory |
| `--skill` | `-s` | string | - | Install only a specific skill by name |
| `--registry` | `-r` | string | - | Registry to search (only with `--skill`, no source) |
| `--internal` | - | bool | false | Include internal skills |
| `--agents` | - | string | - | Comma-separated agent names for symlinks |
| `--no-lock` | - | bool | false | Skip writing to lock file |

### uninstall

Remove an installed skill. Deletes the canonical copy and all agent symlinks.

```bash
# Remove from current directory
duckrow uninstall go-review

# Remove from a specific directory
duckrow uninstall go-review --dir /path/to/project
```

| Argument | Required | Description |
|----------|----------|-------------|
| `skill-name` | Yes | Name of the skill to remove |

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--dir` | `-d` | string | Current directory | Target directory |
| `--no-lock` | - | bool | false | Skip writing to lock file |

### uninstall-all

Remove all installed skills from a directory.

```bash
# Remove all from current directory
duckrow uninstall-all

# Remove all from a specific directory
duckrow uninstall-all --dir /path/to/project
```

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--dir` | `-d` | string | Current directory | Target directory |
| `--no-lock` | - | bool | false | Skip writing to lock file |

## Lock File Commands

### sync

Install all skills and MCP configs declared in `duckrow.lock.json` at their pinned versions. Skills whose directories already exist are skipped. MCP entries that already exist in agent config files are skipped unless `--force` is used.

This command is equivalent to running `duckrow mcp sync` plus the skill sync logic in a single pass.

```bash
# Sync everything in current directory
duckrow sync

# Sync into a specific directory
duckrow sync --dir /path/to/project

# Preview what would be installed
duckrow sync --dry-run

# Also create symlinks for non-universal agents (skills only)
duckrow sync --agents cursor,claude-code

# Overwrite existing MCP entries in agent config files
duckrow sync --force
```

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--dir` | `-d` | string | Current directory | Target directory |
| `--dry-run` | - | bool | false | Show what would be done without making changes |
| `--agents` | - | string | - | Comma-separated agent names for skill symlinks |
| `--force` | - | bool | false | Overwrite existing MCP entries in agent config files |

To force reinstall of a specific skill, delete its directory and rerun `duckrow sync`.

### outdated

Show which installed skills have newer commits available. Before checking, this command refreshes the commit cache for unpinned registry skills (see [commit hydration](lock-file.md#commit-hydration)).

```bash
# Check current directory
duckrow outdated

# Check a specific directory
duckrow outdated --dir /path/to/project

# Output as JSON for scripting
duckrow outdated --json
```

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--dir` | `-d` | string | Current directory | Target directory |
| `--json` | - | bool | false | Output as JSON for scripting |

### update

Update one or all skills to the available commit and update the lock file. Before checking for updates, this command refreshes the commit cache for unpinned registry skills (see [commit hydration](lock-file.md#commit-hydration)).

```bash
# Update a specific skill
duckrow update go-review

# Update all skills
duckrow update --all

# Preview what would be updated
duckrow update --all --dry-run

# Update with agent symlinks
duckrow update go-review --agents cursor
```

Running `duckrow update` without arguments or `--all` returns an error with a usage hint.

| Argument | Required | Default | Description |
|----------|----------|---------|-------------|
| `skill-name` | No* | - | Name of the skill to update |

*Either `skill-name` or `--all` is required.

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--dir` | `-d` | string | Current directory | Target directory |
| `--all` | - | bool | false | Update all skills in the lock file |
| `--dry-run` | - | bool | false | Show what would be updated without making changes |
| `--agents` | - | string | - | Comma-separated agent names for symlinks |

## Registry Management

### registry add

Add a private skill registry by cloning its git repository.

```bash
duckrow registry add https://github.com/acme/skill-registry.git
duckrow registry add git@github.com:acme/skill-registry.git
```

| Argument | Required | Description |
|----------|----------|-------------|
| `repo-url` | Yes | Git repository URL for the registry |

The repository must contain a `duckrow.json` manifest at its root.

### registry list

List all configured registries.

```bash
# Names and skill counts
duckrow registry list

# Include skill details
duckrow registry list --verbose
```

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--verbose` | `-v` | bool | false | Show skills and MCPs in each registry |

### registry refresh

Pull latest changes for registries.

```bash
# Refresh all registries
duckrow registry refresh

# Refresh a specific registry (by name or repo URL)
duckrow registry refresh my-org
duckrow registry refresh https://github.com/acme/skill-registry.git
```

| Argument | Required | Default | Description |
|----------|----------|---------|-------------|
| `name-or-repo` | No | All registries | Registry name or repo URL |

### registry remove

Remove a registry from config and delete its local clone.

```bash
duckrow registry remove my-org
duckrow registry remove https://github.com/acme/skill-registry.git
```

| Argument | Required | Description |
|----------|----------|-------------|
| `name-or-repo` | Yes | Registry name or repo URL |

## MCP Server Management

MCP (Model Context Protocol) servers are external tools that AI agents can call at runtime — for querying databases, APIs, internal services, and more. duckrow installs MCP server configurations into agent config files directly from your registry.

### mcp install

Install an MCP server configuration from a configured registry. Writes the config into agent-specific config files for detected agents. For stdio MCPs, the command is wrapped with `duckrow env` to inject environment variable secrets at runtime.

```bash
# Install for all MCP-capable agents detected in the project
duckrow mcp install internal-db

# Install from a specific registry
duckrow mcp install internal-db --registry my-org

# Install for specific agents only
duckrow mcp install internal-db --agents cursor,claude-code

# Install into a specific directory
duckrow mcp install internal-db --dir /path/to/project

# Overwrite an existing entry with the same name
duckrow mcp install internal-db --force
```

| Argument | Required | Description |
|----------|----------|-------------|
| `name` | Yes | MCP server name as listed in the registry |

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--dir` | `-d` | string | Current directory | Target project directory |
| `--registry` | `-r` | string | - | Registry to search (disambiguates duplicates) |
| `--agents` | - | string | - | Comma-separated agent names to target |
| `--no-lock` | - | bool | false | Skip writing to lock file |
| `--force` | - | bool | false | Overwrite existing MCP entry with the same name |

Output example:

```
Installing MCP "internal-db" from registry "my-org"...

Wrote MCP config to:
  + opencode.json           (OpenCode)
  + .mcp.json               (Claude Code)
  + .cursor/mcp.json        (Cursor)

Updated duckrow.lock.json

! The following environment variables are required:
  DB_URL  (used by internal-db)

  Add values to .env.duckrow or ~/.duckrow/.env.duckrow

MCP "internal-db" installed successfully.
```

### mcp uninstall

Remove an installed MCP server configuration from agent config files. Reads the lock file to determine which agents contain the entry.

```bash
# Remove from current directory
duckrow mcp uninstall internal-db

# Remove from a specific directory
duckrow mcp uninstall internal-db --dir /path/to/project

# Remove without touching the lock file
duckrow mcp uninstall internal-db --no-lock
```

| Argument | Required | Description |
|----------|----------|-------------|
| `name` | Yes | MCP server name to remove |

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--dir` | `-d` | string | Current directory | Target directory |
| `--no-lock` | - | bool | false | Skip writing to lock file |

### mcp sync

Restore MCP server configurations from `duckrow.lock.json`. For each MCP entry in the lock file, looks up the current config in the registry and writes it to agent config files. Existing entries are skipped unless `--force` is used.

```bash
# Sync MCP configs in current directory
duckrow mcp sync

# Sync into a specific directory
duckrow mcp sync --dir /path/to/project

# Preview what would be installed
duckrow mcp sync --dry-run

# Overwrite existing MCP entries
duckrow mcp sync --force
```

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--dir` | `-d` | string | Current directory | Target directory |
| `--dry-run` | - | bool | false | Show what would be done without making changes |
| `--force` | - | bool | false | Overwrite existing MCP entries in agent config files |
| `--agents` | - | string | - | Comma-separated agent names to target |

## Environment Variables

### env

Internal runtime helper that injects environment variables into MCP server processes. This command is written into agent config files by `duckrow mcp install` and is **not intended to be invoked directly** by users.

When an MCP server is launched by an agent, duckrow intercepts the process, reads the required env vars for that MCP from `duckrow.lock.json`, resolves their values from available sources, and execs the real MCP command with those variables set.

**Resolution precedence (highest to lowest):**

1. Process environment (`export VAR=value`)
2. Project `.env.duckrow` (in the project root)
3. Global `~/.duckrow/.env.duckrow`

**Storing env var values:**

```bash
# Project-level (only applies to this repo, gitignored)
echo "DB_URL=postgres://localhost/mydb" >> .env.duckrow

# Global (applies to all projects using this MCP)
echo "DB_URL=postgres://localhost/mydb" >> ~/.duckrow/.env.duckrow
```

The project `.env.duckrow` is automatically added to `.gitignore` by the TUI during MCP install (when you choose project-level storage). Never commit secret values.

## Command Tree

```
duckrow                              Launch interactive TUI
  version                            Print version information
  add [path]                         Add a folder to the tracked list
  folders                            List all tracked folders
  remove-folder <path>               Remove a folder from the tracked list
  status [path]                      Show installed skills and MCPs for a folder
  install [source]                   Install skill(s)
    --dir, -d <path>                   Target directory
    --skill, -s <name>                 Specific skill name
    --registry, -r <name>              Registry filter (with --skill only)
    --internal                         Include internal skills
    --agents <names>                   Agent names for symlinks
    --no-lock                          Skip writing to lock file
  uninstall <skill-name>             Remove an installed skill
    --dir, -d <path>                   Target directory
    --no-lock                          Skip writing to lock file
  uninstall-all                      Remove all installed skills
    --dir, -d <path>                   Target directory
    --no-lock                          Skip writing to lock file
  sync                               Install skills and MCPs from lock file
    --dir, -d <path>                   Target directory
    --dry-run                          Preview without changes
    --force                            Overwrite existing MCP entries
    --agents <names>                   Agent names for skill symlinks
  outdated                           Show skills with available updates
    --dir, -d <path>                   Target directory
    --json                             Output as JSON
  update [skill-name]                Update skill(s) to available commit
    --dir, -d <path>                   Target directory
    --all                              Update all skills
    --dry-run                          Preview without changes
    --agents <names>                   Agent names for symlinks
  mcp                                Manage MCP server configurations
    install <name>                     Install an MCP config from a registry
      --dir, -d <path>                   Target directory
      --registry, -r <name>              Registry filter
      --agents <names>                   Agent names to target
      --no-lock                          Skip writing to lock file
      --force                            Overwrite existing entry
    uninstall <name>                   Remove an installed MCP config
      --dir, -d <path>                   Target directory
      --no-lock                          Skip writing to lock file
    sync                               Restore MCP configs from lock file
      --dir, -d <path>                   Target directory
      --dry-run                          Preview without changes
      --force                            Overwrite existing entries
      --agents <names>                   Agent names to target
  env --mcp <name> -- <cmd> [args]   Runtime env injector (internal use)
  registry                           Manage skill registries
    add <repo-url>                     Add a registry
    list                               List registries
      --verbose, -v                      Show skill and MCP details
    refresh [name-or-repo]             Refresh registry data
    remove <name-or-repo>              Remove a registry
```
