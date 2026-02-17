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

Show installed skills and detected agents for a folder.

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

Install all skills declared in `duckrow.lock.json` at their pinned commits. Skills whose directories already exist are skipped.

```bash
# Sync skills in current directory
duckrow sync

# Sync into a specific directory
duckrow sync --dir /path/to/project

# Preview what would be installed
duckrow sync --dry-run

# Also create symlinks for non-universal agents
duckrow sync --agents cursor,claude-code
```

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--dir` | `-d` | string | Current directory | Target directory |
| `--dry-run` | - | bool | false | Show what would be done without making changes |
| `--agents` | - | string | - | Comma-separated agent names for symlinks |

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
| `--verbose` | `-v` | bool | false | Show skills in each registry |

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

## Command Tree

```
duckrow                              Launch interactive TUI
  version                            Print version information
  add [path]                         Add a folder to the tracked list
  folders                            List all tracked folders
  remove-folder <path>               Remove a folder from the tracked list
  status [path]                      Show installed skills for a folder
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
  sync                               Install skills from lock file
    --dir, -d <path>                   Target directory
    --dry-run                          Preview without changes
    --agents <names>                   Agent names for symlinks
  outdated                           Show skills with available updates
    --dir, -d <path>                   Target directory
    --json                             Output as JSON
  update [skill-name]                Update skill(s) to available commit
    --dir, -d <path>                   Target directory
    --all                              Update all skills
    --dry-run                          Preview without changes
    --agents <names>                   Agent names for symlinks
  registry                           Manage skill registries
    add <repo-url>                     Add a registry
    list                               List registries
      --verbose, -v                      Show skill details
    refresh [name-or-repo]             Refresh registry data
    remove <name-or-repo>              Remove a registry
```
