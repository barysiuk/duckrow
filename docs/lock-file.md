# Lock File

How duckrow tracks installed skills and enables reproducible setups across a team.

## Overview

When you install a skill, duckrow records the exact git commit that was installed in a lock file called `duckrow.lock.json`. This file lives at the project root and should be committed to version control.

The lock file enables three things:

1. **Reproducible installs** — team members cloning a repo run `duckrow sync` to get identical skills
2. **Update detection** — `duckrow outdated` shows which skills have newer commits available
3. **Controlled updates** — `duckrow update` moves skills forward and updates the lock file

Git commit hashes serve as the version identifier. No manual version bumping is needed.

## The Lock File

`duckrow.lock.json` is created automatically on the first `duckrow install`. It tracks every installed skill with its source and pinned commit.

```json
{
  "lockVersion": 1,
  "skills": [
    {
      "name": "slack-digest",
      "source": "github.com/acme/skills/skills/communication/slack-digest",
      "commit": "a1b2c3d4e5f6789012345678901234567890abcd",
      "ref": "main"
    },
    {
      "name": "go-review",
      "source": "github.com/acme/skills/skills/engineering/go-review",
      "commit": "f6e5d4c3b2a1098765432109876543210fedcba"
    }
  ]
}
```

| Field | Description |
|-------|-------------|
| `lockVersion` | Schema version (currently `1`) |
| `skills[].name` | Skill name (directory name under `.agents/skills/`) |
| `skills[].source` | Canonical source path: `host/owner/repo/path/to/skill` |
| `skills[].commit` | Full 40-character git commit SHA that was installed |
| `skills[].ref` | Branch or tag hint (optional, recorded when installing from a `/tree/<ref>/` URL) |

Skills are sorted by name in the file to keep diffs stable.

### What to Commit

```text
# Commit the lock file
git add duckrow.lock.json

# Do NOT commit installed skill files — they are reproduced by duckrow sync
echo ".agents/skills/" >> .gitignore
```

## Team Workflow

### Setting Up a Project

```bash
# Install skills
duckrow install acme/skills@slack-digest
duckrow install acme/skills@go-review

# Commit the lock file
git add duckrow.lock.json
git commit -m "Add skill dependencies"
git push
```

### Cloning a Project

```bash
git clone <repo>
cd <repo>
duckrow sync
# All skills installed at the exact pinned commits
```

### Updating Skills

```bash
# See what has updates available
duckrow outdated

# Update a specific skill
duckrow update slack-digest

# Or update everything
duckrow update --all

# Commit the updated lock file
git add duckrow.lock.json
git commit -m "Update slack-digest"
```

## Commands

### duckrow sync

Installs all skills from the lock file at their pinned commits.

```bash
duckrow sync
duckrow sync --dir /path/to/project
duckrow sync --dry-run
duckrow sync --agents cursor,claude-code
```

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--dir` | `-d` | string | Current directory | Target directory |
| `--dry-run` | - | bool | false | Show what would be done without making changes |
| `--agents` | - | string | - | Comma-separated agent names to also symlink into |

Behavior:

- If a skill directory already exists, it is skipped (not reinstalled)
- If a skill directory is missing, it is installed at the pinned commit
- Errors are reported per skill; other skills continue processing
- Skills are always installed to `.agents/skills/` (universal agents). To also create symlinks for non-universal agents, pass `--agents`

To force a reinstall of a specific skill, delete its directory and rerun `duckrow sync`:

```bash
rm -rf .agents/skills/slack-digest
duckrow sync
```

Output:

```text
Installed: slack-digest
Installed: go-review

Synced: 2 installed, 0 skipped, 0 errors
```

Dry run output:

```text
install: slack-digest (commit a1b2c3d)
skip: go-review (already installed)
```

### duckrow outdated

Shows which installed skills have a different commit available upstream.

```bash
duckrow outdated
duckrow outdated --json
duckrow outdated --dir /path/to/project
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

### duckrow update

Updates one or all skills to the available commit and writes the new commit to the lock file.

```bash
# Update a specific skill
duckrow update slack-digest

# Update all skills
duckrow update --all

# Preview without changes
duckrow update --all --dry-run

# Update and symlink for non-universal agents
duckrow update --all --agents cursor,claude-code
```

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--all` | - | bool | false | Update all skills in the lock file |
| `--dir` | `-d` | string | Current directory | Target directory |
| `--dry-run` | - | bool | false | Show what would be updated without making changes |
| `--agents` | - | string | - | Comma-separated agent names to also symlink into |

Running `duckrow update` without a skill name or `--all` returns an error:

```text
Error: specify a skill name or use --all

Usage:
  duckrow update <skill-name>
  duckrow update --all
```

Output:

```text
Updated: slack-digest a1b2c3d -> f9e8d7c

Update: 1 updated, 1 up-to-date, 0 errors
```

Update reinstalls the skill: the existing directory and agent symlinks are removed, then the skill is installed from the source at the new commit.

### How the Available Commit Is Determined

Both `outdated` and `update` use the same precedence to find the available commit for each skill:

1. **Registry commit** — if a configured registry has a `commit` field for this skill, that commit is used. This lets registry authors pin specific blessed versions.
2. **Lock entry ref** — if the lock entry has a `ref` (branch or tag), the latest commit on that ref is fetched from the source repository.
3. **Default branch** — otherwise, the latest commit on the repository's default branch is fetched.

## Lock File and Existing Commands

### install

`duckrow install` automatically creates or updates the lock file.

```bash
# Install and record in lock file (default)
duckrow install acme/skills@go-review

# Install without recording in lock file
duckrow install acme/skills@go-review --no-lock
```

If a skill with the same name already exists in the lock file but with a different source, a warning is printed:

```text
Warning: skill "go-review" source changed from "github.com/old-org/skills/go-review" to "github.com/new-org/skills/go-review"
```

The lock entry is replaced with the new source.

### uninstall

`duckrow uninstall` automatically removes the skill from the lock file.

```bash
# Uninstall and remove from lock file (default)
duckrow uninstall go-review

# Uninstall without modifying the lock file
duckrow uninstall go-review --no-lock
```

### uninstall-all

`duckrow uninstall-all` writes an empty lock file (it does not delete `duckrow.lock.json`):

```bash
duckrow uninstall-all
```

Resulting lock file:

```json
{
  "lockVersion": 1,
  "skills": []
}
```

Use `--no-lock` to remove skills without touching the lock file.

## The --no-lock Flag

The `--no-lock` flag is available on `install`, `uninstall`, and `uninstall-all`. It skips all lock file reads and writes for that command.

Use cases:

- **Ephemeral skills** — install a skill for quick testing without adding it to the project's lock file
- **Manual lock management** — when you want to control the lock file yourself

## CI/CD Integration

The lock file and `duckrow sync` are designed for CI/CD pipelines where you need skills installed reproducibly.

```yaml
# .github/workflows/test.yml
jobs:
  test:
    steps:
      - uses: actions/checkout@v4
      - name: Install duckrow
        run: brew install barysiuk/tap/duckrow
      - name: Install skills
        run: duckrow sync
      - name: Run agent
        run: opencode "Run the tests"
```

Since `sync` installs from pinned commits, builds are deterministic regardless of upstream changes.

## Source Format

The lock file uses a canonical source format: `host/owner/repo/path/to/skill`. This is normalized after installation regardless of how you specified the source on the command line.

For example, all of these inputs:

```bash
duckrow install acme/skills@go-review
duckrow install https://github.com/acme/skills.git --skill go-review
duckrow install git@github.com:acme/skills.git --skill go-review
```

Produce the same lock entry source:

```text
github.com/acme/skills/skills/engineering/go-review
```

The `owner/repo` shorthand assumes `github.com` as the host. For other git hosts, use the full HTTPS or SSH URL:

```bash
duckrow install https://gitlab.com/my-org/my-skills.git
duckrow install git@gitlab.com:my-org/my-skills.git
```

## Registry Commit Pinning

Registry manifests (`duckrow.json`) can include an optional `commit` field per skill:

```json
{
  "name": "my-org",
  "skills": [
    {
      "name": "go-review",
      "source": "github.com/acme/skills/skills/engineering/go-review",
      "commit": "a1b2c3d4e5f6789012345678901234567890abcd"
    }
  ]
}
```

When a registry provides a `commit`:

- `duckrow outdated` compares the installed commit against the registry commit (not upstream HEAD)
- `duckrow update` installs the registry-pinned commit
- No network fetch is needed for that skill during `outdated` checks

This lets registry authors bless specific versions of external skills for their organization.
