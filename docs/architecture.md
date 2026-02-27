# Architecture

How duckrow is structured internally, how the pluggable architecture works, and how to extend it.

## Core Abstractions

duckrow is built around two core abstractions:

- **Asset** -- a system-agnostic unit that duckrow manages. Today this means skills (markdown-based instructions), MCP server configurations, and agents (custom subagent personas). The architecture supports future kinds like rules, hooks, or routines without structural changes.
- **System** -- an AI coding tool that consumes assets. Each system is a self-contained unit that knows its own paths, config formats, and detection logic. Systems include OpenCode, Cursor, Claude Code, GitHub Copilot, Codex, Gemini CLI, and Goose.

A third component, the **Orchestrator**, coordinates these two during lifecycle operations (install, remove, scan, sync). It is both kind-agnostic and system-agnostic -- it talks to assets and systems exclusively through their interfaces.

```
                    +--------------+
                    | Orchestrator |
                    +------+-------+
                           |
              +------------+------------+
              |                         |
      +-------v-------+        +-------v-------+
      | Asset Handlers |        |    Systems    |
      |  (per kind)    |        | (per AI tool) |
      +----------------+        +---------------+
      | SkillHandler   |        | OpenCode      |
      | MCPHandler     |        | Cursor        |
      | AgentHandler   |        | Claude Code   |
      | (future kinds) |        | Copilot       |
      +----------------+        | Codex         |
                                | Gemini CLI    |
                                | Goose         |
                                | (future tools)|
                                +---------------+
```

Both assets and systems use the same registration pattern: each implementation lives in its own file and registers itself via Go's `init()` function. The rest of the codebase discovers them dynamically at runtime -- there is no central list to maintain.

## Project Structure

```
cmd/duckrow/
  main.go                 Entrypoint
  cmd/                    Cobra command definitions (thin wrappers)
  testdata/script/        Integration tests (.txtar files)

internal/core/
  asset/                  Asset abstraction (Handler interface + built-in kinds)
  system/                 System abstraction (System interface + built-in systems)
  orchestrator.go         Coordinates assets and systems
  lockfile.go             Lock file management (v3 format)
  registry.go             Registry manifest parsing and lookup
  compat.go               Legacy type adapters for backward compatibility
  types.go                Shared domain types

internal/tui/             Interactive terminal UI (Bubble Tea)
```

Three rules govern the package boundaries:

1. **`asset/` and `system/` have zero dependencies on each other.** Assets don't know about systems. Systems import `asset` only for the `Kind` type and `Asset` struct.
2. **`internal/core/` has zero UI dependencies.** No TUI or CLI imports.
3. **The orchestrator lives in `internal/core/`** so it can import both `asset/` and `system/` without circular dependencies.

## Assets

An asset kind is defined by implementing the `Handler` interface in `internal/core/asset/`. A handler knows how to discover, parse, validate, and serialize assets of its kind. It does NOT know anything about systems.

The two key responsibilities of a handler:

- **Discovery and validation** -- given a cloned git repository, find all assets of this kind and validate them. For skills, this means walking the filesystem for `SKILL.md` files and parsing their YAML frontmatter. For MCPs, discovery returns nothing (MCPs are config-only, defined in registries).
- **Serialization** -- convert between registry manifest entries, lock file entries, and in-memory representations. Each kind has its own manifest format and lock data shape.

### Built-in Kinds

**Skills** are file-based assets. A skill is a directory containing a `SKILL.md` file with YAML frontmatter (author, version, description). Skills are cloned from git repositories, stored in `.agents/skills/`, and discovered by walking the filesystem.

**MCP Servers** are config-only assets. An MCP is defined in a registry manifest with a command, args, and environment variables. MCPs are not stored on disk as files -- they are written into system-specific JSON config files (e.g., `.cursor/mcp.json`, `opencode.json`).

**Agents** are file-based assets with per-system rendering. An agent is a single markdown file with YAML frontmatter defining a specialized persona. Unlike skills, agents do NOT have a canonical copy on disk -- each agent-capable system (Claude Code, OpenCode, GitHub Copilot, Gemini CLI) gets its own rendered file in its agents directory (e.g., `.claude/agents/`, `.opencode/agents/`). The rendering process merges base frontmatter with system-specific override blocks, passing all field values through verbatim.

These differences -- file-based, config-only, and rendered-per-system -- are handled entirely within the handler implementations. The orchestrator and TUI don't need to care.

### Registration

Each handler registers itself at import time via `init()`. The global registry provides lookup functions like `asset.Get(kind)`, `asset.All()`, and `asset.Kinds()`. The `Kinds()` function returns kinds in a stable order (skill, MCP, agent, then any future kinds), which the TUI uses for tab ordering.

## Systems

A system is defined by implementing the `System` interface in `internal/core/system/`. Each system is a self-contained unit that knows:

- **Identity** -- machine name (`"cursor"`) and display name (`"Cursor"`)
- **Detection** -- how to tell if the tool is globally installed and if it has config artifacts in a project folder
- **Asset support** -- which asset kinds it handles
- **Installation** -- how to accept an asset (copy files, create symlinks, write JSON config)
- **Classification** -- whether it is "universal" (shares `.agents/skills/`) or has its own directory

### BaseSystem

Most systems embed `BaseSystem`, which provides sensible defaults for all interface methods. The defaults handle:

- **Skill installation** -- universal systems do nothing (the orchestrator handles the canonical copy in `.agents/skills/`). Non-universal systems create a relative symlink from their own skills directory to the canonical location.
- **Agent installation** -- renders the agent markdown file with system-specific frontmatter overrides applied, then writes it directly into the system's agents directory (e.g., `.claude/agents/`, `.opencode/agents/`).
- **MCP installation** -- reads or creates a JSON/JSONC config file, patches in the MCP server entry under the system's config key using JSON Pointer operations.
- **Detection** -- checks `configSignals` (project-level files like `opencode.json`, `.cursor/`) and `detectPaths` (global install locations like `~/.cursor/`).

Systems that need custom behavior override specific methods. For example, OpenCode and GitHub Copilot override `Install()` because their MCP config format differs from the standard `{ "command": "...", "args": [...] }` shape. They handle MCP installation themselves and delegate skill installation back to `BaseSystem`.

### Built-in Systems

| System | Universal | Skills Dir | MCP Support | Agents Dir | Custom Install |
|--------|-----------|-----------|-------------|------------|----------------|
| OpenCode | yes | `.agents/skills` | yes | `.opencode/agents` | yes |
| GitHub Copilot | yes | `.agents/skills` | yes | `.github/agents` | yes |
| Codex | yes | `.agents/skills` | no | — | no |
| Gemini CLI | yes | `.agents/skills` | no | `.gemini/agents` | no |
| Cursor | no | `.cursor/skills` | yes | — | no |
| Claude Code | no | `.claude/skills` | yes | `.claude/agents` | no |
| Goose | no | `.goose/skills` | no | — | no |

### Universal vs. Non-Universal

Universal systems share the `.agents/skills/` directory directly. When a skill is installed, it is copied once to `.agents/skills/<name>/`, and all four universal systems (OpenCode, Copilot, Codex, Gemini CLI) see it immediately.

Non-universal systems have their own skills directory (e.g., `.cursor/skills/`). The orchestrator creates symlinks from the system's directory to the canonical `.agents/skills/` copy, so the skill content is never duplicated.

### Detection

Two levels of detection serve different purposes:

- **`ActiveInFolder(path)`** checks only unique config signals -- files like `opencode.json`, directories like `.cursor/`, files like `CLAUDE.md`. This is used by the sidebar to show which systems are actually configured in a project.
- **`DetectInFolder(path)`** checks config signals plus global installation (`~/.cursor/` exists, etc.). This is used by the install wizard, where you want to show all systems you *could* install assets to.

This distinction exists because universal systems all share `.agents/skills/`. Without separate detection logic, any folder with that directory would incorrectly show all four universal systems in the sidebar.

## Orchestrator

The orchestrator (`internal/core/orchestrator.go`) is the coordination layer. It connects asset handlers with systems during lifecycle operations:

- **Install** -- clones the source repo, asks the handler to discover and validate assets, copies them to the canonical location, then calls `Install()` on each target system.
- **Remove** -- calls `Remove()` on each relevant system, then cleans up the canonical copy.
- **Scan** -- iterates all systems and all kinds, collects installed assets, deduplicates by name.
- **Sync** -- reads the lock file, reinstalls each asset at its pinned commit.

The orchestrator never checks what kind an asset is or which system it's talking to. It dispatches everything through the interfaces. This is what makes the architecture pluggable -- the orchestrator doesn't need to change when a new kind or system is added.

## Data Formats

### Lock File

`duckrow.lock.json` uses a unified v3 format with a single `assets` array. Each entry has a `kind` discriminator, and kind-specific fields go in the `data` map:

```json
{
  "lockVersion": 3,
  "assets": [
    {
      "kind": "skill",
      "name": "go-review",
      "source": "github.com/acme/skills/skills/engineering/go-review",
      "commit": "a1b2c3d4...",
      "ref": "main"
    },
    {
      "kind": "mcp",
      "name": "internal-db",
      "data": {
        "registry": "my-org",
        "configHash": "sha256:abc123...",
        "systems": ["cursor", "claude-code"],
        "requiredEnv": ["DB_HOST", "DB_PASSWORD"]
      }
    },
    {
      "kind": "agent",
      "name": "deploy-specialist",
      "source": "github.com/acme/agents/deploy-specialist",
      "commit": "b2c3d4e5..."
    }
  ]
}
```

The lock file reader transparently handles v1 and v2 formats (separate `skills`/`mcps` arrays) and migrates them to v3 in memory.

### Registry Manifests

Registry manifests (`duckrow.json`) use a v2 format with an `assets` map keyed by kind. Parsing delegates to each handler's `ParseManifestEntries()` method, so adding a new kind automatically enables it in registries. v1 manifests (separate `skills`/`mcps` arrays) are supported transparently.

## CLI and TUI

### CLI

CLI commands are generated dynamically. The `registerAssetCommands()` function iterates `asset.Kinds()` and creates a full set of subcommands (`install`, `uninstall`, `list`, `sync`) for each kind. The `outdated` and `update` subcommands are generated for source-based kinds (skills and agents) but not for MCPs. The `--systems` flag lets users target specific systems.

### TUI

The TUI discovers everything at runtime:

- **Folder view** creates one tab per registered kind, labeled with `handler.DisplayName() + "s"` (e.g., "Skills", "MCP Servers", "Agents").
- **Install picker** groups registry assets by kind using the same dynamic labels.
- **Install wizard** is unified -- one wizard handles all kinds, with kind-specific steps (MCP preview, skill agent selection) dispatched internally.
- **Sidebar** shows detected systems via `system.ActiveInFolder()`, using each system's `DisplayName()`.
- **Messages** are unified -- `assetInstalledMsg` and `assetRemovedMsg` carry the asset kind, so the TUI handles all kinds generically.

---

## Extending duckrow

### Adding a New Asset Kind

To add a new asset kind (e.g., rules), create one file: `internal/core/asset/rule.go`. Implement the `Handler` interface -- define the kind constant, metadata struct, and the six interface methods (discovery, parsing, validation, manifest parsing, lock data). Call `Register()` in `init()`.

Then update the systems that should support the new kind by adding it to their `supportedKinds` list. If a system needs custom install behavior, override `Install()`.

Everything else is automatic:

- The CLI generates `duckrow rule install/uninstall/list/sync`
- The TUI adds a "Rules" tab, includes rules in the install picker, and the wizard handles them
- The lock file stores entries with `"kind": "rule"`
- Registry manifests parse entries from `"assets": { "rule": [...] }`
- `duckrow sync` includes rules alongside skills, MCPs, and agents

No changes needed to the orchestrator, CLI commands, TUI views, lock file format, or registry parsing.

### Adding a New System

To add a new AI coding tool (e.g., Windsurf), create one file: `internal/core/system/windsurf.go`. Embed `BaseSystem`, configure the fields (name, skills directory, config signals, detection paths, supported kinds, MCP config details), and call `Register()` in `init()`.

If the system uses a non-standard MCP config format, override `Install()` and handle MCP installation directly, delegating other kinds back to `BaseSystem`. See `opencode.go` and `github_copilot.go` for examples.

Everything else is automatic:

- Detection functions include the new system when its config signals are present
- Skills are symlinked, MCPs are written to the config file
- The `--systems` flag accepts the new system's name
- The TUI shows the system in the sidebar and install wizard
- Scanning discovers installed assets in the system's directory

No changes needed to the CLI, TUI, orchestrator, or other system files.
