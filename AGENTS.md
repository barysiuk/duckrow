# Agent Guide

duckrow is a Go CLI tool that manages AI agent skills across multiple project folders.

## Project Structure

```
cmd/duckrow/              CLI entrypoint and integration tests
  cmd/                    Cobra command definitions (thin wrappers)
  testdata/script/        testscript integration tests (.txtar files)
  main.go                 Entrypoint
  main_test.go            TestMain + testscript runner + custom commands
internal/core/            Core library (zero UI dependencies)
  asset/                  Asset handler interfaces and implementations (skill, MCP, agent)
  system/                 System interfaces and implementations (7 systems)
  auth.go                 Clone error classification, SSH/HTTPS hints
  compat.go               Legacy type adapters for backward compatibility
  config.go               Config management (~/.duckrow/)
  env.go                  Environment variable resolution for MCP servers
  folder.go               Folder tracking
  helpers.go              Shared utility functions
  lockfile.go             Lock file v3 (unified assets array)
  orchestrator.go         Coordination layer for install/remove/scan
  registry.go             Private registry management (v1/v2 manifests)
  source.go               Source URL parsing
  types.go                Domain types
internal/tui/             Interactive terminal UI (Bubble Tea)
  app.go                  Main TUI model, view routing, data loading
  folder.go               Folder view — skill list, preview, removal
  install.go              Install view — skill/MCP picking list
  asset_wizard.go         Asset install wizard (system selection, env vars)
  wizard.go               Shared wizard component
  registry_wizard.go      Registry add wizard
  bookmarks.go            Bookmarks view — folder switching
  sidebar.go              Sidebar panel — folder info, systems
  tabs.go                 Tab bar component
  statusbar.go            Status bar (transient messages, spinner)
  settings.go             Settings view — registry management
  confirm.go              Confirmation dialog
  clone_error.go          Clone error handling with retry flow
  items.go                List item delegates for bubbles components
  keys.go                 Keybinding definitions
  theme.go                Shared lipgloss styles and colors
```

## Design Rules

1. **Core has zero UI dependencies** — no TUI/CLI imports in `internal/core/`
2. **Core exposes clean interfaces** — `asset.Handler` and `system.System` interfaces
3. **TUI consumes core** — `internal/tui/` builds the interactive UI on top of core
4. **CLI commands are thin wrappers** — subcommands in `cmd/` delegate to core; the root command launches the TUI
5. **Core is independently testable** — unit tests without CLI or TUI
6. **Pluggable architecture** — new asset kinds (skill, MCP, agent, future: rule) and systems are added by implementing interfaces, not by modifying switch blocks

## Running Tests

```bash
# All tests (unit + integration)
go test ./... -count=1

# Unit tests only (skip integration)
go test ./... -short

# Integration tests only (testscript)
go test ./cmd/duckrow/ -v -count=1 -run TestScript

# Single integration test
go test ./cmd/duckrow/ -v -count=1 -run TestScript/bookmark_add
```

## Integration Tests

Integration tests use [testscript](https://github.com/rogpeppe/go-internal/testscript) — `.txtar` files in `cmd/duckrow/testdata/script/`. Each file is a self-contained test scenario that runs CLI commands and verifies stdout, stderr, exit codes, and filesystem state.

Custom testscript commands available:
- `is-symlink <path>` — assert path is a symlink
- `file-contains <path> <substring>` — assert file contains text
- `dir-not-exists <path>` — assert directory does not exist
- `setup-git-repo <dir> <name> [skills...]` — create a local git repo with a duckrow.json manifest
- `setup-config-override <repo-key> <clone-url>` — create a config with a clone URL override mapping
- `setup-registry-config <override-key> <override-url>` — add a clone URL override to the existing config (preserving registries)
- `setup-mcp-registry <dir> <registry-name> <mcp-spec...>` — create a git repo with a duckrow.json containing MCP entries
- `setup-agent-repo <dir> <agent-name:description...>` — create a local git repo containing agent .md files
- `setup-agent-registry <dir> <registry-name> <agent-name:description:source...>` — create a git repo with a duckrow.json manifest listing agent entries and .md files
- `write-env-file <dir> <key=value...>` — write key=value pairs to a .env.duckrow file

## Key Concepts

- **Universal systems** (OpenCode, Codex, Gemini CLI, GitHub Copilot) share `.agents/skills/`
- **Non-universal systems** (Cursor, Claude Code, Goose) get symlinks from their own skills dir to `.agents/skills/`
- **Skills** are directories containing a `SKILL.md` file with YAML frontmatter
- **MCP servers** are config entries written into system-specific config files
- **Registries** are git repos with a `duckrow.json` manifest listing available skills, MCPs, and agents
- **Asset handlers** (`asset.Handler`) define how each kind is discovered, installed, and removed
- **Systems** (`system.System`) define where assets are stored and how configs are written

## Branch Naming

Use prefixed branch names:

```
feat/short-description     New features
fix/short-description      Bug fixes
refactor/short-description Code restructuring
docs/short-description     Documentation only
test/short-description     Adding or updating tests
ci/short-description       CI/CD changes
chore/short-description    Maintenance, dependencies
```

Examples: `feat/registry-search`, `fix/symlink-cleanup`, `refactor/installer-options`

## Commit Messages

Follow [Conventional Commits](https://www.conventionalcommits.org/). The format:

```
<type>(<scope>): <short summary>

<body — explain WHY, not what>

<footer — breaking changes, issue refs>
```

**Type** must be one of: `feat`, `fix`, `refactor`, `test`, `docs`, `ci`, `chore`

**Scope** is optional but encouraged: `core`, `cli`, `registry`, `installer`, `scanner`, `ci`

**Summary** line: imperative mood, lowercase, no period, max 72 chars.

**Body**: Explain the motivation — what was wrong, why this approach. Wrap at 80 chars.

Good:
```
feat(registry): add refresh command for all registries

Previously users had to refresh registries one at a time. Running
`duckrow registry refresh` without a name argument now refreshes
all configured registries in sequence.
```

```
fix(installer): skip internal skills unless flag is set

Internal skills (metadata.internal: true) were being installed by
default, which exposed hidden skills to agents that shouldn't see
them. Now requires --internal flag.

Closes #42
```

Bad:
```
updated stuff
fixed bug
WIP
```

## Before Committing

**MANDATORY**: Always run the full test suite before committing and pushing. Never commit code that hasn't been verified locally.

```bash
# Must pass — this is what CI runs
go test ./... -count=1

# Also check formatting and lint
gofmt -l .
go vet ./...
golangci-lint run ./...
```

If any of these fail, fix the issues before committing. Do not push broken code and rely on CI to catch it.

### Pre-commit Hook

A pre-commit hook is provided in `.githooks/`. To enable it:

```bash
git config core.hooksPath .githooks
```

The hook runs formatting checks, `go vet`, `golangci-lint`, and short tests before each commit. If any check fails, the commit is blocked.

If you add a new CLI command or change behavior, add or update the corresponding `.txtar` integration test in `cmd/duckrow/testdata/script/`.

## Versioning and Releases

duckrow uses [Semantic Versioning](https://semver.org/): `vMAJOR.MINOR.PATCH`

- **PATCH** (`v0.1.0` -> `v0.1.1`): Bug fixes, no behavior changes
- **MINOR** (`v0.1.1` -> `v0.2.0`): New features, backward compatible
- **MAJOR** (`v0.2.0` -> `v1.0.0`): Breaking changes

The version is injected at build time via git tags and ldflags — there is no hardcoded version file. The source of truth is the git tag.

### Release Process

1. Ensure all changes are merged to `main` and CI is green
2. Decide the next version based on the changes since the last tag
3. Tag and push:

```bash
git tag v0.1.0
git push origin v0.1.0
```

4. The `release.yaml` workflow runs automatically:
   - Runs tests as a sanity check
   - Builds binaries for linux/darwin/windows (amd64 + arm64)
   - Creates a GitHub Release with changelog
   - Publishes the Homebrew formula to `barysiuk/homebrew-tap`
