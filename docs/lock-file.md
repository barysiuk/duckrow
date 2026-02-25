# Lock File

How duckrow tracks installed skills and MCP configurations, and enables reproducible setups across a team.

## Overview

When you install a skill, duckrow records the exact git commit that was installed in a lock file called `duckrow.lock.json`. When you install an MCP server config, duckrow records the MCP name, registry source, and a hash of the config. This file lives at the project root and should be committed to version control.

The lock file enables three things:

1. **Reproducible installs** — team members cloning a repo run `duckrow sync` to get identical skills and MCP configs
2. **Update detection** — `duckrow skill outdated` shows which skills have newer commits available
3. **Controlled updates** — `duckrow skill update` moves skills forward and updates the lock file

Git commit hashes serve as the version identifier for skills. No manual version bumping is needed.

## The Lock File

`duckrow.lock.json` is created automatically on the first `duckrow skill install` or `duckrow mcp install`. It uses a unified `assets` array where each entry has a `kind` discriminator.

```json
{
  "lockVersion": 3,
  "assets": [
    {
      "kind": "skill",
      "name": "slack-digest",
      "source": "github.com/acme/skills/skills/communication/slack-digest",
      "commit": "a1b2c3d4e5f6789012345678901234567890abcd",
      "ref": "main"
    },
    {
      "kind": "skill",
      "name": "go-review",
      "source": "github.com/acme/skills/skills/engineering/go-review",
      "commit": "f6e5d4c3b2a1098765432109876543210fedcba"
    },
    {
      "kind": "mcp",
      "name": "internal-db",
      "data": {
        "registry": "my-org",
        "configHash": "sha256:a1b2c3d4...",
        "systems": ["opencode", "claude-code", "cursor"],
        "requiredEnv": ["DB_URL"]
      }
    }
  ]
}
```

### Asset fields

| Field | Description |
|-------|-------------|
| `lockVersion` | Schema version (currently `3`) |
| `assets[].kind` | Asset type: `"skill"` or `"mcp"` |
| `assets[].name` | Asset name |

### Skill-specific fields

| Field | Description |
|-------|-------------|
| `source` | Canonical source path: `host/owner/repo/path/to/skill` |
| `commit` | Full 40-character git commit SHA that was installed |
| `ref` | Branch or tag hint (optional, recorded when installing from a `/tree/<ref>/` URL) |

### MCP-specific fields

MCP entries store their metadata in a `data` map:

| Field | Description |
|-------|-------------|
| `data.registry` | Registry name the MCP was installed from |
| `data.configHash` | SHA-256 hash of the MCP config at install time |
| `data.systems` | System names whose config files were written |
| `data.requiredEnv` | Env var names required by this MCP at runtime |

Assets are sorted by kind then name in the file to keep diffs stable.

### What to Commit

```text
# Commit the lock file
git add duckrow.lock.json

# Do NOT commit installed skill files — they are reproduced by duckrow sync
echo ".agents/skills/" >> .gitignore

# Do NOT commit env var files — they contain secrets
echo ".env.duckrow" >> .gitignore
```

## Team Workflow

### Setting Up a Project

```bash
# Install skills
duckrow skill install acme/skills@slack-digest
duckrow skill install acme/skills@go-review

# Install MCP server configs
duckrow mcp install internal-db
duckrow mcp install analytics-api

# Commit the lock file
git add duckrow.lock.json
git commit -m "Add skill and MCP dependencies"
git push
```

### Cloning a Project

```bash
git clone <repo>
cd <repo>
duckrow sync
# All skills installed at the exact pinned commits
# All MCP configs written to system config files
```

### Updating Skills

```bash
# See what has updates available
duckrow skill outdated

# Update a specific skill
duckrow skill update slack-digest

# Or update everything
duckrow skill update --all

# Commit the updated lock file
git add duckrow.lock.json
git commit -m "Update slack-digest"
```

## Commands

### duckrow sync

Installs all skills and MCP configs from the lock file.

```bash
duckrow sync
duckrow sync --dir /path/to/project
duckrow sync --dry-run
duckrow sync --systems cursor,claude-code
duckrow sync --force
```

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--dir` | `-d` | string | Current directory | Target directory |
| `--dry-run` | - | bool | false | Show what would be done without making changes |
| `--systems` | - | string | - | Comma-separated system names for skill symlinks |
| `--force` | - | bool | false | Overwrite existing MCP entries in system config files |

Behavior:

- **Skills**: if a skill directory already exists, it is skipped (not reinstalled); if missing, installed at the pinned commit
- **MCPs**: if an MCP entry already exists in the system config file, it is skipped unless `--force` is used; if missing, the config is written from the current registry
- Errors are reported per item; other items continue processing

Output:

```text
Syncing from duckrow.lock.json...

Skills: 2 installed, 0 skipped, 0 errors
MCPs:   1 installed, 0 skipped, 0 errors

! The following environment variables are required:
  DB_URL  (used by internal-db)

  Add values to .env.duckrow or ~/.duckrow/.env.duckrow

Synced successfully.
```

Dry run output:

```text
install: slack-digest (commit a1b2c3d)
skip: go-review (already installed)
install: internal-db (from my-org)
```

To force reinstall of a specific skill, delete its directory and rerun `duckrow sync`.

`duckrow mcp sync` runs the MCP portion of this command independently.

### duckrow skill outdated

Shows which installed skills have a different commit available upstream.

```bash
duckrow skill outdated
duckrow skill outdated --json
duckrow skill outdated --dir /path/to/project
```

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--dir` | `-d` | string | Current directory | Target directory |
| `--json` | - | bool | false | Output as JSON for scripting |

Table output:

```text
Skill               Installed  Available     Source
slack-digest        a1b2c3d    f9e8d7c       github.com/acme/skills
go-review           f6e5d4c    (up to date)  github.com/acme/skills
custom-skill        1234567    8765432       gitlab.com/my-org/my-skills
```

The `Source` column in table output is truncated to `host/owner/repo` for readability.

JSON output includes the full canonical source path:

```json
[
  {
    "name": "slack-digest",
    "source": "github.com/acme/skills/skills/communication/slack-digest",
    "installed": "a1b2c3d4...",
    "available": "f9e8d7c6...",
    "hasUpdate": true
  },
  {
    "name": "go-review",
    "source": "github.com/acme/skills/skills/engineering/go-review",
    "installed": "f6e5d4c3...",
    "available": "f6e5d4c3...",
    "hasUpdate": false
  }
]
```

### duckrow skill update

Updates one or all skills to the available commit and writes the new commit to the lock file.

```bash
# Update a specific skill
duckrow skill update slack-digest

# Update all skills
duckrow skill update --all

# Preview without changes
duckrow skill update --all --dry-run

# Update and symlink for non-universal systems
duckrow skill update --all --systems cursor,claude-code
```

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--all` | - | bool | false | Update all skills in the lock file |
| `--dir` | `-d` | string | Current directory | Target directory |
| `--dry-run` | - | bool | false | Show what would be updated without making changes |
| `--systems` | - | string | - | Comma-separated system names to also symlink into |

Running `duckrow skill update` without a skill name or `--all` returns an error:

```text
Error: specify a skill name or use --all

Usage:
  duckrow skill update <skill-name>
  duckrow skill update --all
```

Output:

```text
Updated: slack-digest a1b2c3d -> f9e8d7c

Update: 1 updated, 1 up-to-date, 0 errors
```

Update reinstalls the skill: the existing directory and system symlinks are removed, then the skill is installed from the source at the new commit.

### How the Available Commit Is Determined

Both `outdated` and `update` use the same precedence to find the available commit for each skill:

1. **Registry commit** — if a configured registry has a commit for this skill (pinned in the manifest or resolved via hydration), that commit is used. No network fetch is needed.
2. **Lock entry ref** — if the lock entry has a `ref` (branch or tag), the latest commit on that ref is fetched from the source repository.
3. **Default branch** — otherwise, the latest commit on the repository's default branch is fetched.

Registry commits come from two sources, merged together:

- **Pinned commits** — explicit `commit` fields in the registry's `duckrow.json` manifest. These always take precedence.
- **Hydrated commits** — for skills without a `commit` field, duckrow resolves the latest commit from the source repo during registry refresh and caches the result (see [Commit Hydration](#commit-hydration) below).

### Commit Hydration

Registry manifests can list skills with or without a `commit` field. Skills with an explicit commit are **pinned** — the registry author has blessed a specific version. Skills without a commit are **unpinned** — they track whatever is latest in the source repo.

For unpinned skills, duckrow resolves the actual latest commit during registry refresh. This process is called **commit hydration**:

1. Groups unpinned skills by source repository
2. Performs a shallow clone of each unique source repo
3. Runs `git log` to determine the latest commit for each skill's sub-path
4. Caches the resolved commits to `duckrow.commits.json` in the registry's local directory

Hydration happens automatically during:

- **TUI startup** — registries are refreshed asynchronously in the background
- **TUI `[r]` refresh** — triggers a full registry refresh including hydration
- **CLI `duckrow skill outdated`** — hydrates before checking for updates
- **CLI `duckrow skill update`** — hydrates before applying updates

The cache file (`duckrow.commits.json`) is stored alongside the registry clone at `~/.duckrow/registries/<registry-key>/duckrow.commits.json`. It is not meant to be edited manually.

When building the final commit map, pinned commits from the manifest always take precedence over cached (hydrated) commits.

### Host-Agnostic Source Matching

Lock file sources may use SSH host aliases that differ from the registry's canonical host. For example, a team member might configure `github.com-work` as an SSH alias for `github.com`. This means the lock file source could be:

```
github.com-work/acme/skills/go-review
```

while the registry lists:

```
github.com/acme/skills/go-review
```

duckrow handles this by falling back to **path-based matching** when an exact source match fails. It strips the host component and compares the remaining `owner/repo/path` portion. This ensures update detection works regardless of SSH host aliases or other host variations.

## Lock File and Existing Commands

### skill install

`duckrow skill install` automatically creates or updates the lock file's assets array.

```bash
# Install and record in lock file (default)
duckrow skill install acme/skills@go-review

# Install without recording in lock file
duckrow skill install acme/skills@go-review --no-lock
```

If a skill with the same name already exists in the lock file but with a different source, a warning is printed:

```text
Warning: skill "go-review" source changed from "github.com/old-org/skills/go-review" to "github.com/new-org/skills/go-review"
```

The lock entry is replaced with the new source.

### skill uninstall

`duckrow skill uninstall` automatically removes the skill from the lock file.

```bash
# Uninstall and remove from lock file (default)
duckrow skill uninstall go-review

# Uninstall without modifying the lock file
duckrow skill uninstall go-review --no-lock
```

### skill uninstall --all

`duckrow skill uninstall --all` removes all skill entries from the lock file (it does not delete `duckrow.lock.json`, and it does not remove MCP entries):

```bash
duckrow skill uninstall --all
```

Use `--no-lock` to remove skills without touching the lock file.

### mcp install

`duckrow mcp install` adds an entry to the lock file's assets array.

```bash
# Install and record in lock file (default)
duckrow mcp install internal-db

# Install without recording in lock file
duckrow mcp install internal-db --no-lock
```

### mcp uninstall

`duckrow mcp uninstall` removes the MCP entry from the lock file.

```bash
# Uninstall and remove from lock file (default)
duckrow mcp uninstall internal-db

# Uninstall without modifying the lock file
duckrow mcp uninstall internal-db --no-lock
```

## The --no-lock Flag

The `--no-lock` flag is available on `skill install`, `skill uninstall`, `mcp install`, and `mcp uninstall`. It skips all lock file reads and writes for that command.

Use cases:

- **Ephemeral skills** — install a skill for quick testing without adding it to the project's lock file
- **Manual lock management** — when you want to control the lock file yourself

## CI/CD Integration

The lock file and `duckrow sync` are designed for CI/CD pipelines where you need skills and MCP configs installed reproducibly.

```yaml
# .github/workflows/test.yml
jobs:
  test:
    steps:
      - uses: actions/checkout@v4
      - name: Install duckrow
        run: brew install barysiuk/tap/duckrow
      - name: Install skills and MCPs
        run: duckrow sync
        env:
          DB_URL: ${{ secrets.DB_URL }}
      - name: Run agent
        run: opencode "Run the tests"
```

Skills are installed at pinned commits. MCP configs are written to system files. For stdio MCPs with required env vars, pass those vars via CI secrets — `duckrow env` resolves them from the process environment at the highest priority.

Since `sync` installs from pinned versions, builds are deterministic regardless of upstream changes.

## Source Format

The lock file uses a canonical source format: `host/owner/repo/path/to/skill`. This is normalized after installation regardless of how you specified the source on the command line.

For example, all of these inputs:

```bash
duckrow skill install acme/skills@go-review
duckrow skill install https://github.com/acme/skills.git
duckrow skill install git@github.com:acme/skills.git
```

Produce the same lock entry source:

```text
github.com/acme/skills/skills/engineering/go-review
```

The `owner/repo` shorthand assumes `github.com` as the host. For other git hosts, use the full HTTPS or SSH URL:

```bash
duckrow skill install https://gitlab.com/my-org/my-skills.git
duckrow skill install git@gitlab.com:my-org/my-skills.git
```

## Registry Commit Pinning

Registry manifests (`duckrow.json`) can include an optional `commit` field per skill:

```json
{
  "name": "my-org",
  "assets": {
    "skill": [
      {
        "name": "go-review",
        "source": "github.com/acme/skills/skills/engineering/go-review",
        "commit": "a1b2c3d4e5f6789012345678901234567890abcd"
      },
      {
        "name": "pr-guidelines",
        "source": "github.com/acme/skills/skills/engineering/pr-guidelines"
      }
    ]
  }
}
```

When a registry provides a `commit`:

- `duckrow skill outdated` compares the installed commit against the registry commit (not upstream HEAD)
- `duckrow skill update` installs the registry-pinned commit
- No network fetch is needed for that skill during `outdated` checks

This lets registry authors bless specific versions of external skills for their organization.

Skills without a `commit` field are still tracked for updates via [commit hydration](#commit-hydration) — duckrow resolves the latest commit from the source repo during registry refresh.
