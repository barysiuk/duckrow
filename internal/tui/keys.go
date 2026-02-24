package tui

import "github.com/charmbracelet/bubbles/key"

// keyMap defines the keybindings for the TUI.
type keyMap struct {
	Quit      key.Binding
	Help      key.Binding
	Up        key.Binding
	Down      key.Binding
	Enter     key.Binding
	Back      key.Binding
	Bookmarks key.Binding
	Install   key.Binding
	Settings  key.Binding
	Bookmark  key.Binding
	Delete    key.Binding
	Refresh   key.Binding
	Filter    key.Binding
	Edit      key.Binding
	Retry     key.Binding
	Toggle    key.Binding
	ToggleAll key.Binding
	Update    key.Binding
	UpdateAll key.Binding
	Configure key.Binding
	Tab       key.Binding
}

var keys = keyMap{
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("k/up", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("j/down", "down"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select"),
	),
	Back: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "back"),
	),
	Bookmarks: key.NewBinding(
		key.WithKeys("b"),
		key.WithHelp("b", "bookmarks"),
	),
	Install: key.NewBinding(
		key.WithKeys("i"),
		key.WithHelp("i", "install"),
	),
	Settings: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "settings"),
	),
	Bookmark: key.NewBinding(
		key.WithKeys("b"),
		key.WithHelp("b", "bookmark"),
	),
	Delete: key.NewBinding(
		key.WithKeys("d", "delete"),
		key.WithHelp("d", "remove"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "refresh"),
	),
	Filter: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "filter"),
	),
	Edit: key.NewBinding(
		key.WithKeys("e"),
		key.WithHelp("e", "edit URL"),
	),
	Retry: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "retry"),
	),
	Toggle: key.NewBinding(
		key.WithKeys(" ", "x"),
		key.WithHelp("space/x", "toggle"),
	),
	ToggleAll: key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "all/none"),
	),
	Update: key.NewBinding(
		key.WithKeys("u"),
		key.WithHelp("u", "update"),
	),
	UpdateAll: key.NewBinding(
		key.WithKeys("U"),
		key.WithHelp("U", "update all"),
	),
	Configure: key.NewBinding(
		key.WithKeys("c"),
		key.WithHelp("c", "configure env vars"),
	),
	Tab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "switch save location"),
	),
}

// ---------------------------------------------------------------------------
// Per-view help keymaps for the help.Model component.
// Each implements help.KeyMap (ShortHelp + FullHelp).
// ---------------------------------------------------------------------------

// folderHelpKeyMap is shown in the folder view.
type folderHelpKeyMap struct {
	updatesAvailable bool
}

func (k folderHelpKeyMap) ShortHelp() []key.Binding {
	bindings := []key.Binding{
		keys.Up, keys.Down, keys.Enter,
		keys.Filter,
	}
	if k.updatesAvailable {
		bindings = append(bindings, keys.Update, keys.UpdateAll)
	}
	bindings = append(bindings,
		keys.Delete, keys.Refresh,
		keys.Install, keys.Bookmarks, keys.Settings, keys.Quit,
	)
	return bindings
}

func (k folderHelpKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{k.ShortHelp()}
}

// pickerHelpKeyMap is shown in the folder picker.
type pickerHelpKeyMap struct{}

func (k pickerHelpKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		keys.Up, keys.Down, keys.Enter, keys.Filter,
		keys.Bookmark, keys.Delete, keys.Back,
	}
}

func (k pickerHelpKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{k.ShortHelp()}
}

// installHelpKeyMap is shown in the install picker.
type installHelpKeyMap struct{}

func (k installHelpKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		keys.Up, keys.Down, keys.Enter, keys.Filter, keys.Back,
	}
}

func (k installHelpKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{k.ShortHelp()}
}

// agentSelectHelpKeyMap is shown in the agent selection phase.
type agentSelectHelpKeyMap struct{}

func (k agentSelectHelpKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		keys.Up, keys.Down, keys.Toggle, keys.ToggleAll,
		keys.Enter, keys.Back,
	}
}

func (k agentSelectHelpKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{k.ShortHelp()}
}

// mcpPreviewHelpKeyMap is shown in the MCP preview/confirmation phase.
type mcpPreviewHelpKeyMap struct {
	hasEnvVars bool
}

func (k mcpPreviewHelpKeyMap) ShortHelp() []key.Binding {
	bindings := []key.Binding{keys.Enter}
	if k.hasEnvVars {
		bindings = append(bindings, keys.Configure)
	}
	bindings = append(bindings, keys.Back)
	return bindings
}

func (k mcpPreviewHelpKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{k.ShortHelp()}
}

// envEntryHelpKeyMap is shown during the env var entry flow.
type envEntryHelpKeyMap struct{}

func (k envEntryHelpKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		keys.Enter, keys.Tab, keys.Back,
	}
}

func (k envEntryHelpKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{k.ShortHelp()}
}

// settingsHelpKeyMap is shown in the settings view.
type settingsHelpKeyMap struct{}

func (k settingsHelpKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		keys.Up, keys.Down, keys.Enter,
		keys.Delete, keys.Refresh, keys.Back, keys.Quit,
	}
}

func (k settingsHelpKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{k.ShortHelp()}
}

// previewHelpKeyMap is shown in the SKILL.md preview.
type previewHelpKeyMap struct{}

func (k previewHelpKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		keys.Up, keys.Down, keys.Back,
	}
}

func (k previewHelpKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{k.ShortHelp()}
}

// cloneErrorHelpKeyMap is shown in the clone error overlay.
type cloneErrorHelpKeyMap struct {
	editing  bool
	retrying bool
}

func (k cloneErrorHelpKeyMap) ShortHelp() []key.Binding {
	if k.retrying {
		return []key.Binding{} // No keys during retry.
	}
	if k.editing {
		return []key.Binding{
			keys.Enter, keys.Back,
		}
	}
	return []key.Binding{
		keys.Edit, keys.Retry, keys.Back,
	}
}

func (k cloneErrorHelpKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{k.ShortHelp()}
}
