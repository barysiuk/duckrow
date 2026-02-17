# Skill Installation

How duckrow discovers, installs, and manages skills.

## What Is a Skill

A skill is a directory containing a `SKILL.md` file. The file has YAML frontmatter followed by markdown content that provides instructions to an AI agent.

### SKILL.md Format

```yaml
---
name: go-review                # REQUIRED
description: Reviews Go code   # Optional
license: MIT                   # Optional
metadata:                      # Optional block
  author: acme                 # Optional
  version: 1.0.0               # Optional
  internal: true               # Optional, defaults to false
  argument-hint: "file path"   # Optional
---

# Go Review

(Markdown body -- the actual instructions for the AI agent)
```

Only `name` is mandatory. Skills without a valid `name` field are silently skipped during discovery.

## Source Formats

duckrow accepts several source formats for `duckrow install`:

| Format | Example | What Happens |
|--------|---------|-------------|
| GitHub shorthand | `acme/skills` | Clones `https://github.com/acme/skills.git` |
| Skill filter syntax | `acme/skills@go-review` | Clones repo, installs only `go-review` |
| Subpath syntax | `acme/skills/path/to/skill` | Clones repo, searches only within subpath |
| HTTPS URL | `https://github.com/acme/skills.git` | Clones from full URL |
| SSH URL | `git@github.com:acme/skills.git` | Clones via SSH |
| Registry skill | `--skill go-review` (no source) | Looks up skill in configured registries |

GitHub and GitLab hosts are auto-detected from URLs. Other hosts fall back to generic git.

### HTTPS URL with Branch and Subpath

GitHub `/tree/` URLs are parsed to extract branch and subpath:

```
https://github.com/acme/skills/tree/develop/backend/go-review
```

This sets `Ref=develop` and `SubPath=backend/go-review`, cloning the `develop` branch and narrowing the search to that subdirectory.

## How Skill Discovery Works

After cloning the source repository, duckrow walks the directory tree recursively looking for `SKILL.md` files.

### Directory Traversal Rules

The walker scans every directory with these exceptions:

**Skipped directories:**
- Hidden directories (starting with `.`) except `.agents`
- `node_modules`
- `vendor`
- `__pycache__`

**For each `SKILL.md` found:**
1. Parses the YAML frontmatter
2. Validates that `name` is non-empty (skips if missing)
3. If `metadata.internal: true` and `--internal` was not passed, skips the skill
4. Adds to the discovered skills list
5. Deduplicates by directory path

### Source Repo Structure

Skills can live at any depth in the source repo. duckrow finds them all.

**Single-skill repo:**
```
my-repo/
  SKILL.md
  helpers.go
```

**Multi-skill repo:**
```
my-repo/
  go-review/
    SKILL.md
    examples/
  python-lint/
    SKILL.md
    rules.yaml
```

**Deeply nested:**
```
my-repo/
  skills/
    backend/
      go-review/
        SKILL.md
    frontend/
      react-patterns/
        SKILL.md
```

## The Install Process

### Step 1: Obtain Source

| Source type | Action |
|-------------|--------|
| Git repo (GitHub/GitLab/SSH/HTTPS) | Shallow clone (`--depth 1`) to temp directory |
| Registry (`--skill` without source) | Looks up `Source` field in registry manifest, then clones that repo |
| Lock file commit | Uses `git init` + `git fetch --depth 1 origin <commit>` to clone at exact commit |

Git clones use `GIT_TERMINAL_PROMPT=0` to prevent interactive auth prompts and have a 60-second timeout.

### Step 2: Apply Skill Filter

If `--skill <name>` or `@skill-name` syntax was used, only skills matching that exact name (case-sensitive) are kept. If no match is found, an error lists all available skill names.

### Step 3: Copy to Canonical Location

Each discovered skill is copied to:

```
<project>/.agents/skills/<sanitized-name>/
```

The name is sanitized: lowercased, non-alphanumeric characters replaced with `-`, trimmed, capped at 255 characters.

**Files excluded from copy:**
- `README.md`
- `metadata.json`
- `.git`
- Any file or directory starting with `_`

Install is always a full overwrite -- the target directory is deleted and recreated.

### Step 4: Create Agent Symlinks

For non-universal agents specified via `--agents`, relative symlinks are created from the agent's skill directory back to the canonical location:

```
.cursor/skills/go-review -> ../../.agents/skills/go-review
```

If symlink creation fails (e.g., on Windows), falls back to a full directory copy.

## Agents

duckrow knows about 9 AI coding agents, split into two categories.

### Universal Agents

These read directly from `.agents/skills/` -- they always receive installed skills with no extra setup:

| Agent | Skills Directory | Alt Directories |
|-------|-----------------|-----------------|
| OpenCode | `.agents/skills` | `.opencode/skills` |
| Codex | `.agents/skills` | - |
| Gemini CLI | `.agents/skills` | - |
| GitHub Copilot | `.agents/skills` | `.github/skills` |

### Non-Universal Agents

These have their own skill directories. They only receive skills when explicitly requested via `--agents`:

| Agent | Skills Directory |
|-------|-----------------|
| Claude Code | `.claude/skills` |
| Cursor | `.cursor/skills` |
| Goose | `.goose/skills` |
| Windsurf | `.windsurf/skills` |
| Cline | `.cline/skills` |

When `--agents cursor,claude-code` is passed, duckrow:
1. Copies files to `.agents/skills/<skill>/` (canonical)
2. Creates symlinks from `.cursor/skills/<skill>` and `.claude/skills/<skill>` to the canonical location

### Resulting Directory Structure

After `duckrow install acme/skills --agents cursor,claude-code`:

```
project/
  .agents/
    skills/
      go-review/              # Real files (canonical copy)
        SKILL.md
        ...
      python-lint/            # Real files
        SKILL.md
        ...
  .cursor/
    skills/
      go-review -> ../../.agents/skills/go-review
      python-lint -> ../../.agents/skills/python-lint
  .claude/
    skills/
      go-review -> ../../.agents/skills/go-review
      python-lint -> ../../.agents/skills/python-lint
```

## Internal Skills

Skills can declare `metadata.internal: true` in their SKILL.md frontmatter. These are hidden by default -- `duckrow install` and `duckrow status` will not show them.

To include internal skills, pass `--internal`:

```bash
duckrow install acme/skills --internal
```

When installing from a registry (via `--skill` without a source), internal skills are automatically included.

Use case: organization-private registries with sensitive or specialized instructions that should not be surfaced to general users browsing a repo.

## Installing from Registries

Registries are git repos containing a `duckrow.json` manifest that catalogs available skills and their source locations.

### Registry Manifest Format

```json
{
  "name": "my-org",
  "description": "Our team skills",
  "skills": [
    {
      "name": "go-review",
      "description": "Reviews Go code for best practices",
      "source": "github.com/acme/go-skills/skills/go-review",
      "commit": "a1b2c3d4e5f6789012345678901234567890abcd"
    }
  ]
}
```

The `source` field should use canonical format (`host/owner/repo/path`). GitHub shorthand and URLs are also accepted but canonical format is preferred for multi-host support and lock file matching. The `commit` field is optional — when present, it pins the skill to that exact git commit.

### Workflow

```bash
# 1. Add a registry
duckrow registry add https://github.com/acme/skill-registry.git

# 2. See what's available
duckrow registry list --verbose

# 3. Install by skill name -- no need to know the repo
duckrow install --skill go-review

# 4. If the same skill name exists in multiple registries, disambiguate
duckrow install --skill go-review --registry my-org
```

When `--skill` is used without a source argument:
1. duckrow searches all configured registries for the skill name
2. If found in exactly one registry, it reads the `Source` field
3. Parses the source and clones/installs normally
4. If found in multiple registries, it errors and asks you to use `--registry`

## Clone URL Overrides

If a source repo requires SSH auth but the manifest uses HTTPS, you can configure a clone URL override in `~/.duckrow/config.json`:

```json
{
  "settings": {
    "cloneURLOverrides": {
      "acme/private-skills": "git@github.com:acme/private-skills.git"
    }
  }
}
```

The key is `owner/repo` (lowercase). When duckrow resolves a source matching that key, it uses the override URL instead.

## Uninstalling

```bash
# Remove a specific skill (canonical copy + all agent symlinks)
duckrow uninstall go-review

# Remove all skills
duckrow uninstall-all
```

Both commands accept `--dir` to target a specific directory and `--no-lock` to skip updating the lock file.

## Lock File

Every install records the exact git commit in `duckrow.lock.json`. This enables reproducible installs via `duckrow sync`, update detection via `duckrow outdated`, and controlled updates via `duckrow update`.

See [lock-file.md](lock-file.md) for the full reference.
