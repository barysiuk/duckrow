# Agent Guide

DuckRow is a Go CLI tool that manages AI agent skills across multiple project folders.

## Project Structure

```
cmd/duckrow/              CLI entrypoint and integration tests
  cmd/                    Cobra command definitions (thin wrappers)
  testdata/script/        testscript integration tests (.txtar files)
  main.go                 Entrypoint
  main_test.go            TestMain + testscript runner + custom commands
internal/core/            Core library (zero UI dependencies)
  agents.json             Agent definitions (10 agents)
  types.go                Domain types
  config.go               Config management (~/.duckrow/)
  folder.go               Folder tracking
  source.go               Source URL parsing
  scanner.go              Agent detection + skill scanning
  installer.go            Skill installation
  remover.go              Skill removal
  registry.go             Private registry management
```

## Design Rules

1. **Core has zero UI dependencies** — no TUI/CLI imports in `internal/core/`
2. **Core exposes clean interfaces** — functions, structs, errors
3. **CLI commands are thin wrappers** — all logic lives in core
4. **Core is independently testable** — unit tests without CLI

## Running Tests

```bash
# All tests (unit + integration)
go test ./... -count=1

# Unit tests only (skip integration)
go test ./... -short

# Integration tests only (testscript)
go test ./cmd/duckrow/ -v -count=1 -run TestScript

# Single integration test
go test ./cmd/duckrow/ -v -count=1 -run TestScript/add_folder
```

## Integration Tests

Integration tests use [testscript](https://github.com/rogpeppe/go-internal/testscript) — `.txtar` files in `cmd/duckrow/testdata/script/`. Each file is a self-contained test scenario that runs CLI commands and verifies stdout, stderr, exit codes, and filesystem state.

Custom testscript commands available:
- `is-symlink <path>` — assert path is a symlink
- `file-contains <path> <substring>` — assert file contains text
- `dir-not-exists <path>` — assert directory does not exist
- `setup-git-repo <dir> <name> [skills...]` — create a local git repo with a duckrow.json manifest

## Key Concepts

- **Universal agents** (OpenCode, Codex, Gemini CLI, GitHub Copilot) share `.agents/skills/`
- **Non-universal agents** (Cursor, Claude Code, Goose, Windsurf, Cline, Continue) get symlinks from their own skills dir to `.agents/skills/`
- **Skills** are directories containing a `SKILL.md` file with YAML frontmatter
- **Registries** are git repos with a `duckrow.json` manifest listing available skills

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

DuckRow uses [Semantic Versioning](https://semver.org/): `vMAJOR.MINOR.PATCH`

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

