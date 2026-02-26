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

## Bookmarks

### bookmark add

Bookmark a project folder.

```bash
# Add current directory
duckrow bookmark add

# Add a specific folder
duckrow bookmark add /path/to/project
```

| Argument | Required | Default | Description |
|----------|----------|---------|-------------|
| `path` | No | Current directory | Folder path to bookmark |

### bookmark list

List all bookmarked folders.

```bash
duckrow bookmark list
```

No arguments or flags.

### bookmark remove

Remove a folder from the bookmarks. Does not delete any files on disk.

```bash
duckrow bookmark remove /path/to/project
```

| Argument | Required | Description |
|----------|----------|-------------|
| `path` | Yes | Path of the folder to remove |

## Status

### status

Show installed skills, agents, MCP configurations, and bookmark status for a folder.

```bash
# Current directory
duckrow status

# Specific folder
duckrow status /path/to/project
```

| Argument | Required | Default | Description |
|----------|----------|---------|-------------|
| `path` | No | Current directory | Folder to inspect |

## Skill Management

Skills are managed through the `duckrow skill` subcommand group.

### skill install

Install skills from a git repository or configured registry.

```bash
# Install a skill from a configured registry (by name)
duckrow skill install go-review

# Install all skills from a GitHub repo
duckrow skill install acme/skills

# Install a specific skill using @ syntax
duckrow skill install acme/skills@go-review

# Install from a full URL
duckrow skill install https://github.com/acme/skills.git

# Install from an SSH clone URL
duckrow skill install git@github.com:acme/skills.git

# Install into a specific project directory
duckrow skill install acme/skills --dir /path/to/project

# Install including internal (hidden) skills
duckrow skill install acme/skills --internal

# Install and create symlinks for non-universal systems
duckrow skill install acme/skills --systems cursor,claude-code

# Disambiguate when the same skill name exists in multiple registries
duckrow skill install go-review --registry my-org
```

| Argument | Required | Description |
|----------|----------|-------------|
| `source-or-name` | Yes | Source to install from (repo shorthand, URL, SSH, or registry skill name) |

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--dir` | `-d` | string | Current directory | Target project directory |
| `--registry` | `-r` | string | - | Registry to search (disambiguates duplicates) |
| `--internal` | - | bool | false | Include internal skills |
| `--systems` | - | string | - | Comma-separated system names for symlinks |
| `--no-lock` | - | bool | false | Skip writing to lock file |
| `--force` | - | bool | false | Overwrite existing |

### skill uninstall

Remove an installed skill. Deletes the canonical copy and all system symlinks.

```bash
# Remove a specific skill from current directory
duckrow skill uninstall go-review

# Remove from a specific directory
duckrow skill uninstall go-review --dir /path/to/project

# Remove all installed skills
duckrow skill uninstall --all
```

| Argument | Required | Description |
|----------|----------|-------------|
| `name` | No* | Name of the skill to remove |

*Either `name` or `--all` is required.

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--dir` | `-d` | string | Current directory | Target directory |
| `--all` | - | bool | false | Remove all installed skills |
| `--no-lock` | - | bool | false | Skip writing to lock file |

### skill list

List installed skills in a directory.

```bash
# List skills in current directory
duckrow skill list

# List in a specific directory
duckrow skill list --dir /path/to/project

# Output as JSON
duckrow skill list --json
```

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--dir` | `-d` | string | Current directory | Target directory |
| `--json` | - | bool | false | Output as JSON |

### skill outdated

Show which installed skills have newer commits available. Before checking, this command refreshes the commit cache for unpinned registry skills (see [commit hydration](lock-file.md#commit-hydration)).

```bash
# Check current directory
duckrow skill outdated

# Check a specific directory
duckrow skill outdated --dir /path/to/project

# Output as JSON for scripting
duckrow skill outdated --json
```

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--dir` | `-d` | string | Current directory | Target directory |
| `--json` | - | bool | false | Output as JSON for scripting |

### skill update

Update one or all skills to the available commit and update the lock file. Before checking for updates, this command refreshes the commit cache for unpinned registry skills (see [commit hydration](lock-file.md#commit-hydration)).

```bash
# Update a specific skill
duckrow skill update go-review

# Update all skills
duckrow skill update --all

# Preview what would be updated
duckrow skill update --all --dry-run

# Update with system symlinks
duckrow skill update go-review --systems cursor
```

Running `duckrow skill update` without arguments or `--all` returns an error with a usage hint.

| Argument | Required | Default | Description |
|----------|----------|---------|-------------|
| `name` | No* | - | Name of the skill to update |

*Either `name` or `--all` is required.

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--dir` | `-d` | string | Current directory | Target directory |
| `--all` | - | bool | false | Update all skills in the lock file |
| `--dry-run` | - | bool | false | Show what would be updated without making changes |
| `--systems` | - | string | - | Comma-separated system names for symlinks |

### skill sync

Install skills from the lock file at their pinned versions.

```bash
# Sync skills in current directory
duckrow skill sync

# Sync into a specific directory
duckrow skill sync --dir /path/to/project

# Preview what would be installed
duckrow skill sync --dry-run

# Also create symlinks for non-universal systems
duckrow skill sync --systems cursor,claude-code
```

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--dir` | `-d` | string | Current directory | Target directory |
| `--dry-run` | - | bool | false | Show what would be done without making changes |
| `--force` | - | bool | false | Overwrite existing |
| `--systems` | - | string | - | Comma-separated system names for skill symlinks |

## MCP Server Management

MCP (Model Context Protocol) servers are external tools that AI agents can call at runtime — for querying databases, APIs, internal services, and more. duckrow installs MCP server configurations into system config files directly from your registry.

### mcp install

Install an MCP server configuration from a configured registry. Writes the config into system-specific config files for detected systems. For stdio MCPs, the command is wrapped with `duckrow env` to inject environment variable secrets at runtime.

```bash
# Install for all MCP-capable systems detected in the project
duckrow mcp install internal-db

# Install from a specific registry
duckrow mcp install internal-db --registry my-org

# Install for specific systems only
duckrow mcp install internal-db --systems cursor,claude-code

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
| `--systems` | - | string | - | Comma-separated system names to target |
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

Remove an installed MCP server configuration from system config files. Reads the lock file to determine which systems contain the entry.

```bash
# Remove a specific MCP from current directory
duckrow mcp uninstall internal-db

# Remove from a specific directory
duckrow mcp uninstall internal-db --dir /path/to/project

# Remove all installed MCPs
duckrow mcp uninstall --all

# Remove without touching the lock file
duckrow mcp uninstall internal-db --no-lock
```

| Argument | Required | Description |
|----------|----------|-------------|
| `name` | No* | MCP server name to remove |

*Either `name` or `--all` is required.

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--dir` | `-d` | string | Current directory | Target directory |
| `--all` | - | bool | false | Remove all installed MCPs |
| `--no-lock` | - | bool | false | Skip writing to lock file |

### mcp list

List installed MCP server configurations.

```bash
# List MCPs in current directory
duckrow mcp list

# List in a specific directory
duckrow mcp list --dir /path/to/project

# Output as JSON
duckrow mcp list --json
```

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--dir` | `-d` | string | Current directory | Target directory |
| `--json` | - | bool | false | Output as JSON |

### mcp sync

Restore MCP server configurations from `duckrow.lock.json`. For each MCP entry in the lock file, looks up the current config in the registry and writes it to system config files. Existing entries are skipped unless `--force` is used.

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
| `--force` | - | bool | false | Overwrite existing MCP entries in system config files |
| `--systems` | - | string | - | Comma-separated system names to target |

## Top-Level Sync

### sync

Install all skills, agents, and MCP configs declared in `duckrow.lock.json` at their pinned versions. Skills whose directories already exist are skipped. Agent files that already exist are skipped unless `--force` is used. MCP entries that already exist in system config files are skipped unless `--force` is used.

This command runs `duckrow skill sync`, `duckrow agent sync`, and `duckrow mcp sync` in a single pass.

```bash
# Sync everything in current directory
duckrow sync

# Sync into a specific directory
duckrow sync --dir /path/to/project

# Preview what would be installed
duckrow sync --dry-run

# Also create symlinks for non-universal systems (skills only)
duckrow sync --systems cursor,claude-code

# Overwrite existing MCP entries in system config files
duckrow sync --force
```

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--dir` | `-d` | string | Current directory | Target directory |
| `--dry-run` | - | bool | false | Show what would be done without making changes |
| `--systems` | - | string | - | Comma-separated system names for skill symlinks |
| `--force` | - | bool | false | Overwrite existing MCP entries in system config files |

To force reinstall of a specific skill, delete its directory and rerun `duckrow sync`.

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

## Environment Variables

### env

Internal runtime helper that injects environment variables into MCP server processes. This command is written into system config files by `duckrow mcp install` and is **not intended to be invoked directly** by users.

When an MCP server is launched by a system, duckrow intercepts the process, reads the required env vars for that MCP from `duckrow.lock.json`, resolves their values from available sources, and execs the real MCP command with those variables set.

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
  bookmark                           Manage bookmarks
    add [path]                         Bookmark a folder
    list                               List all bookmarks
    remove <path>                      Remove a bookmark
  status [path]                      Show installed skills, agents, and MCPs for a folder
  sync                               Install skills, agents, and MCPs from lock file
    --dir, -d <path>                   Target directory
    --dry-run                          Preview without changes
    --force                            Overwrite existing MCP entries
    --systems <names>                  System names for skill symlinks
  skill                              Manage skills
    install <source-or-name>           Install skill(s)
      --dir, -d <path>                   Target directory
      --registry, -r <name>              Registry filter
      --internal                         Include internal skills
      --systems <names>                  System names for symlinks
      --no-lock                          Skip writing to lock file
      --force                            Overwrite existing
    uninstall [name]                   Remove an installed skill
      --dir, -d <path>                   Target directory
      --all                              Remove all skills
      --no-lock                          Skip writing to lock file
    list                               List installed skills
      --dir, -d <path>                   Target directory
      --json                             Output as JSON
    sync                               Install skills from lock file
      --dir, -d <path>                   Target directory
      --dry-run                          Preview without changes
      --force                            Overwrite existing
      --systems <names>                  System names for symlinks
    outdated                           Show skills with available updates
      --dir, -d <path>                   Target directory
      --json                             Output as JSON
    update [name]                      Update skill(s) to available commit
      --dir, -d <path>                   Target directory
      --all                              Update all skills
      --dry-run                          Preview without changes
      --systems <names>                  System names for symlinks
  mcp                                Manage MCP server configurations
    install <name>                     Install an MCP config from a registry
      --dir, -d <path>                   Target directory
      --registry, -r <name>              Registry filter
      --systems <names>                  System names to target
      --no-lock                          Skip writing to lock file
      --force                            Overwrite existing entry
    uninstall [name]                   Remove an installed MCP config
      --dir, -d <path>                   Target directory
      --all                              Remove all MCPs
      --no-lock                          Skip writing to lock file
    list                               List installed MCP configs
      --dir, -d <path>                   Target directory
      --json                             Output as JSON
    sync                               Restore MCP configs from lock file
      --dir, -d <path>                   Target directory
      --dry-run                          Preview without changes
      --force                            Overwrite existing entries
      --systems <names>                  System names to target
  agent                              Manage agents
    install <source-or-name>           Install agent(s)
      --dir, -d <path>                   Target directory
      --registry, -r <name>              Registry filter
      --systems <names>                  System names to target
      --no-lock                          Skip writing to lock file
      --force                            Overwrite existing
    uninstall [name]                   Remove an installed agent
      --dir, -d <path>                   Target directory
      --all                              Remove all agents
      --no-lock                          Skip writing to lock file
    list                               List installed agents
      --dir, -d <path>                   Target directory
      --json                             Output as JSON
    sync                               Install agents from lock file
      --dir, -d <path>                   Target directory
      --dry-run                          Preview without changes
      --force                            Overwrite existing
      --systems <names>                  System names to target
  env --mcp <name> -- <cmd> [args]   Runtime env injector (internal use)
  registry                           Manage skill registries
    add <repo-url>                     Add a registry
    list                               List registries
      --verbose, -v                      Show skill, MCP, and agent details
    refresh [name-or-repo]             Refresh registry data
    remove <name-or-repo>              Remove a registry
```
