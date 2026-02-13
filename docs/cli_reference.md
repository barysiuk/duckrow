# CLI Command Reference

Complete reference for all duckrow CLI commands.

## Root

```bash
duckrow          # Launch interactive TUI
```

Running without arguments or subcommands opens the terminal UI.

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

Install skills from a git repository, local path, or configured registry.

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

# Install from a local directory
duckrow install ./my-local-skills

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
| `source` | No* | - | Source to install from (repo, URL, or local path) |

*Either `source` or `--skill` (without source) is required.

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--dir` | `-d` | string | Current directory | Target project directory |
| `--skill` | `-s` | string | - | Install only a specific skill by name |
| `--registry` | `-r` | string | - | Registry to search (only with `--skill`, no source) |
| `--internal` | - | bool | false | Include internal skills |
| `--agents` | - | string | - | Comma-separated agent names for symlinks |

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
  uninstall <skill-name>             Remove an installed skill
    --dir, -d <path>                   Target directory
  uninstall-all                      Remove all installed skills
    --dir, -d <path>                   Target directory
  registry                           Manage skill registries
    add <repo-url>                     Add a registry
    list                               List registries
      --verbose, -v                      Show skill details
    refresh [name-or-repo]             Refresh registry data
    remove <name-or-repo>              Remove a registry
```
