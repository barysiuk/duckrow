package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/barysiuk/duckrow/internal/core"
	"github.com/barysiuk/duckrow/internal/core/asset"
	"github.com/barysiuk/duckrow/internal/core/system"
)

// installFilter determines which asset kinds are shown in the install picker.
type installFilter asset.Kind

// agentCheckbox represents one agent in the selection list.
type agentCheckbox struct {
	system  system.System
	checked bool
}

// assetInstalledMsg is sent when an asset install completes.
type assetInstalledMsg struct {
	kind   asset.Kind
	name   string
	folder string
	err    error
}

// assetRemovedMsg is sent when an asset removal completes.
type assetRemovedMsg struct {
	kind asset.Kind
	name string
	err  error
}

// openAssetWizardMsg is emitted by installModel when an asset is selected.
// It carries all the data needed to start the asset install wizard.
type openAssetWizardMsg struct {
	asset        core.RegistryAssetInfo
	allSystems   []system.System
	activeFolder string
}

// envVarStatus tracks the resolution status of a single env var.
type envVarStatus struct {
	name   string
	isSet  bool
	source string // "project" or "global" if set
}

// envSaveDoneMsg is sent after saving an env var value.
type envSaveDoneMsg struct {
	err error
}

// isSensitiveVarName returns true if the var name suggests a sensitive value
// that should be masked during input.
func isSensitiveVarName(name string) bool {
	upper := strings.ToUpper(name)
	return strings.Contains(upper, "TOKEN") ||
		strings.Contains(upper, "KEY") ||
		strings.Contains(upper, "SECRET") ||
		strings.Contains(upper, "PASSWORD")
}

// installModel is the install picker that shows registry assets not yet
// installed in the active folder. When the user selects an item, it emits
// openAssetWizardMsg; the actual install flow is handled by asset_wizard.go.
type installModel struct {
	width  int
	height int

	// Bubbles list for available assets.
	list list.Model

	// Filter: asset kind shown in the list.
	filter installFilter

	// Data (set on activate).
	activeFolder  string
	available     []core.RegistryAssetInfo // Filtered: only NOT installed assets
	allSystems    []system.System          // All system definitions
	allRegAssets  []core.RegistryAssetInfo // All registry assets (for filtering)
	installedMCPs []assetItem              // Currently installed MCPs (for filtering)
}

func newInstallModel() installModel {
	l := list.New(nil, registryAssetDelegate{}, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)
	l.DisableQuitKeybindings()
	l.SetShowPagination(false)

	return installModel{
		list: l,
	}
}

func (m installModel) setSize(width, height int) installModel {
	m.width = width
	m.height = height
	m.list.SetSize(width, max(1, height))
	return m
}

// activate is called when the install picker opens. It filters registry items
// to show only those NOT already installed in the active folder, scoped to
// the given filter (asset kind).
func (m installModel) activate(filter installFilter, activeFolder string, regAssets []core.RegistryAssetInfo, folderStatus *core.FolderStatus, systems []system.System) installModel {
	m.activeFolder = activeFolder
	m.allSystems = systems
	m.filter = filter

	// Build set of installed names for the filter kind.
	installed := make(map[string]bool)
	switch asset.Kind(filter) {
	case asset.KindMCP:
		for _, mcp := range m.installedMCPs {
			if mcp.locked != nil {
				installed[mcp.locked.Name] = true
			}
		}
	default:
		if folderStatus != nil {
			for _, a := range folderStatus.Assets[asset.Kind(filter)] {
				installed[a.Name] = true
			}
		}
	}

	// Filter to available (not installed) assets of the selected kind.
	m.available = nil
	for _, info := range regAssets {
		if info.Kind != asset.Kind(filter) {
			continue
		}
		if !installed[info.Entry.Name] {
			m.available = append(m.available, info)
		}
	}

	// Build list items scoped to the active filter.
	items := registryAssetsToItems(m.available)
	m.list.SetItems(items)
	m.list.ResetFilter()

	// Select first selectable item (skip separator).
	if len(items) > 0 {
		if _, ok := items[0].(registrySeparatorItem); ok && len(items) > 1 {
			m.list.Select(1)
		} else {
			m.list.Select(0)
		}
	}

	return m
}

// setMCPData sets the MCP data needed for filtering in the install picker.
// Called from app.go before activate.
func (m installModel) setMCPData(regAssets []core.RegistryAssetInfo, installedMCPs []assetItem) installModel {
	m.allRegAssets = regAssets
	m.installedMCPs = installedMCPs
	m.list.ResetFilter()
	return m
}

func (m installModel) update(msg tea.Msg) (installModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Don't intercept keys while filtering.
		if m.list.SettingFilter() {
			break
		}

		switch {
		case key.Matches(msg, keys.Enter):
			return m.handleItemSelected()
		}
	}

	// Forward to list for navigation + filtering.
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)

	// Skip separator items.
	m.skipSeparators()

	return m, cmd
}

// handleItemSelected emits the appropriate wizard message for the selected item.
func (m installModel) handleItemSelected() (installModel, tea.Cmd) {
	item := m.list.SelectedItem()
	if item == nil {
		return m, nil
	}

	switch it := item.(type) {
	case registryAssetItem:
		return m, func() tea.Msg {
			return openAssetWizardMsg{
				asset:        it.info,
				allSystems:   m.allSystems,
				activeFolder: m.activeFolder,
			}
		}
	}

	return m, nil
}

// skipSeparators moves the cursor off separator items.
func (m *installModel) skipSeparators() {
	items := m.list.Items()
	idx := m.list.Index()
	if idx >= 0 && idx < len(items) {
		if _, ok := items[idx].(registrySeparatorItem); ok {
			if idx+1 < len(items) {
				m.list.Select(idx + 1)
			} else if idx-1 >= 0 {
				m.list.Select(idx - 1)
			}
		}
	}
}

func (m installModel) view() string {
	if len(m.available) == 0 {
		handler, _ := asset.Get(asset.Kind(m.filter))
		label := "assets"
		if handler != nil {
			label = strings.ToLower(handler.DisplayName()) + "s"
		}
		return mutedStyle.Render("  All registry " + label + " are already installed.")
	}

	// Size list to fill available space.
	m.list.SetSize(m.width, max(1, m.height))

	return m.list.View()
}

// (buildRegistryAssets removed — the unified core.RegistryAssetInfo is used directly)
