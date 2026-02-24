# Terminal UI

duckrow includes an interactive terminal UI built with [Bubble Tea](https://github.com/charmbracelet/bubbletea). Launch it by running `duckrow` without any subcommands.

<p align="center">
  <img src="images/duckrow_tui.png" alt="duckrow TUI screenshot" width="800" />
</p>

## Views

The TUI has several views you navigate between:

| View | Purpose | Enter via |
|------|---------|-----------|
| **Folder** | Main view — shows installed skills and MCPs for the active folder | Default on launch |
| **Bookmarks** | Switch between bookmarked folders | `b` from folder view |
| **Install** | Browse and install registry skills and MCPs | `i` from folder view |
| **Settings** | Manage registries | `s` from folder view |
| **Preview** | Read a skill's SKILL.md content | `enter` on a skill |

## Keybindings

### Folder View (Main)

The folder view has two sections: **Skills** (top) and **MCPs** (bottom). Use `j`/`k` to navigate within a section; at the boundary between sections the cursor crosses over automatically.

| Key | Action | Notes |
|-----|--------|-------|
| `j` / `k` | Move up/down | Arrow keys also work; crosses between Skills and MCPs sections |
| `enter` | Preview skill | Opens SKILL.md in a scrollable view (Skills section only) |
| `/` | Filter skills | Type to search, `esc` to clear (Skills section only) |
| `d` | Remove item | Removes selected skill or MCP; confirmation prompt before removal |
| `u` | Update skill | Only shown when the selected skill has an update (Skills section only) |
| `U` | Update all | Only shown when any skill has an update |
| `r` | Refresh | Refreshes registries and reloads data |
| `i` | Install | Opens install picker (requires configured registries) |
| `c` | Change folder | Opens bookmarks view |
| `s` | Settings | Opens registry management |
| `a` | Add folder | Bookmark the active folder (shown when folder is not bookmarked) |
| `q` | Quit | `ctrl+c` also works |
| `?` | Help | Toggle keybinding reference |

### Bookmarks

| Key | Action |
|-----|--------|
| `j` / `k` | Move up/down |
| `enter` | Select folder |
| `/` | Filter folders |
| `a` | Bookmark folder |
| `d` | Remove bookmark |
| `esc` | Back to folder view |

### Install Picker

| Key | Action |
|-----|--------|
| `j` / `k` | Move up/down |
| `enter` | Install selected skill or MCP |
| `/` | Filter skills and MCPs |
| `esc` | Back to folder view |

**Skill install flow:** after selecting a skill, an agent selection screen appears if non-universal agents are detected. Use `space`/`x` to toggle agents, `a` to select all/none, and `enter` to confirm.

**MCP install flow:** selecting an MCP opens a multi-step workflow:

1. **Agent selection** — choose which MCP-capable agents to configure (OpenCode, Claude Code, Cursor, GitHub Copilot). Detected agents are pre-selected; toggle with `space`/`x`.
2. **Preview** — shows the MCP details and the status of any required environment variables (already set, missing, etc.)
3. **Env var entry** — if required env vars are missing, you are prompted to enter each value one at a time. After entering a value, choose whether to save it to the **project** `.env.duckrow` or to the **global** `~/.duckrow/.env.duckrow`.
4. **Install** — duckrow writes the MCP config into each agent's config file and updates the lock file.

### Settings

| Key | Action |
|-----|--------|
| `j` / `k` | Move up/down |
| `enter` | Add a new registry |
| `d` | Remove selected registry |
| `r` | Refresh selected registry |
| `esc` | Back to folder view |
| `q` | Quit |

### Skill Preview

| Key | Action |
|-----|--------|
| `j` / `k` | Scroll up/down |
| `esc` | Back to folder view |

## Update Detection

The TUI detects available updates for installed skills by comparing the commit in your lock file (`duckrow.lock.json`) against the commit in your configured registries.

### What gets checked

Only **registry-tracked skills** are checked for updates. Skills installed from ad-hoc sources (direct URLs, GitHub shorthand) without a matching registry entry will not show update badges.

### How it works

1. On startup, duckrow loads the registry commit map from cached data (instant, no network)
2. In parallel, an async registry refresh runs in the background — pulling latest registry data and [hydrating unpinned commits](lock-file.md#commit-hydration)
3. A spinning indicator shows "refreshing" in the header while this runs
4. When the refresh completes, the skill list updates automatically with any new update badges

The TUI remains fully interactive during the background refresh.

### Visual indicators

When updates are available:

- The section header shows the count: `SKILLS (3 installed, 2 updates available)`
- Each skill with an update shows an `(update available)` badge
- The footer shows the total update count with an `[u] Update` hint
- The `u` and `U` keybindings appear in the help bar

### Updating skills

**Single skill** — select the skill with an update and press `u`. A confirmation dialog shows the old and new commit hashes (e.g., `Update go-review? (a1b2c3d -> f9e8d7c)`). Confirm to proceed.

**All skills** — press `U` to update all skills with available updates at once. A confirmation dialog shows the total count. Updates are applied sequentially; if one fails, the rest continue. A summary toast shows the result (e.g., `Updated 3 skills` or `Updated 2 skills, 1 errors`).

Updates preserve existing agent symlinks — no agent selection is needed during updates.

### Refreshing

Press `r` in the folder view to manually trigger a registry refresh. This:

1. Pulls latest changes for all configured registries (`git pull`)
2. Hydrates unpinned skill commits (shallow clone + `git log`)
3. Rebuilds the commit map
4. Reloads folder data

The refresh runs asynchronously with a spinner indicator. You can continue browsing while it runs.

## Toast Notifications

The TUI uses toast notifications for feedback:

- **Success** (green) — skill installed, updated, or removed; MCP installed or removed
- **Warning** (amber) — partial success (e.g., bulk update with some errors)
- **Error** (red) — operation failed

Toasts dismiss automatically after a short delay.

## MCP Management

The folder view shows installed MCPs below the skills list in a separate **MCPS** section. Each row shows the MCP name, its description (if available from the registry), and the agents it is configured for.

### Installing MCPs

Press `i` to open the install picker. MCPs from configured registries that are not yet installed appear alongside available skills. Select an MCP and follow the multi-step install workflow (see [Install Picker](#install-picker) above).

### Removing MCPs

Navigate to the MCPs section with `j` (past the last skill), select the MCP, and press `d`. A confirmation prompt shows before removal. duckrow removes the MCP entry from all agent config files that contain it and updates the lock file.

### Env Var Entry Flow

When installing an MCP that requires environment variables (e.g., API keys, database URLs), the TUI prompts you to enter values for any that are not already set:

1. The preview screen lists each required env var and its current status (set or missing)
2. For each missing var, an input field appears with the var name as the prompt
3. After entering a value, choose the save location:
   - **Project** — saved to `.env.duckrow` in the project root (gitignored automatically)
   - **Global** — saved to `~/.duckrow/.env.duckrow` (applies to all projects)
4. The TUI proceeds to install once all vars are handled

If you skip entering a value, installation proceeds with a warning. You can add the value to `.env.duckrow` manually at any time before running the MCP.
