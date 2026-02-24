package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/barysiuk/duckrow/internal/core"
)

// settingsSection defines navigable sections in settings.
type settingsSection int

const (
	settingsRegistries settingsSection = iota
	settingsAddRegistry
)

// openRegistryWizardMsg is sent when the user selects "+ Add Registry".
type openRegistryWizardMsg struct{}

// settingsModel is the settings/configuration screen.
type settingsModel struct {
	width  int
	height int

	// Navigation.
	section settingsSection
	cursor  int // Cursor within the current section.

	// Data.
	cfg     *core.Config
	version string // App version (e.g. "0.3.0", "dev").
}

func newSettingsModel() settingsModel {
	return settingsModel{}
}

func (m settingsModel) setSize(width, height int) settingsModel {
	m.width = width
	m.height = height
	return m
}

func (m settingsModel) setData(cfg *core.Config, version string) settingsModel {
	m.cfg = cfg
	m.version = version
	return m
}

func (m settingsModel) update(msg tea.Msg, app *App) (settingsModel, tea.Cmd) {
	if m.cfg == nil {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Up):
			m = m.moveCursorUp()
			return m, nil

		case key.Matches(msg, keys.Down):
			m = m.moveCursorDown()
			return m, nil

		case key.Matches(msg, keys.Enter):
			return m.handleEnter(app)

		case key.Matches(msg, keys.Delete):
			return m.handleDelete(app)

		case key.Matches(msg, keys.Refresh):
			if m.section == settingsRegistries && len(m.cfg.Registries) > 0 {
				refreshCmd := m.refreshSelectedRegistry(app)
				var taskCmd tea.Cmd
				app.statusBar, taskCmd = app.statusBar.update(taskStartedMsg{})
				return m, tea.Batch(refreshCmd, taskCmd)
			}
			return m, nil
		}
	}

	return m, nil
}

func (m settingsModel) moveCursorUp() settingsModel {
	switch m.section {
	case settingsRegistries:
		if m.cursor > 0 {
			m.cursor--
		}
	case settingsAddRegistry:
		if len(m.cfg.Registries) > 0 {
			m.section = settingsRegistries
			m.cursor = len(m.cfg.Registries) - 1
		}
	}
	return m
}

func (m settingsModel) moveCursorDown() settingsModel {
	switch m.section {
	case settingsRegistries:
		if m.cursor < len(m.cfg.Registries)-1 {
			m.cursor++
		} else {
			m.section = settingsAddRegistry
			m.cursor = 0
		}
	case settingsAddRegistry:
		// No more sections below.
	}
	return m
}

func (m settingsModel) handleEnter(app *App) (settingsModel, tea.Cmd) {
	switch m.section {
	case settingsAddRegistry:
		// Open the registry wizard overlay.
		return m, func() tea.Msg { return openRegistryWizardMsg{} }
	}
	return m, nil
}

func (m settingsModel) handleDelete(app *App) (settingsModel, tea.Cmd) {
	switch m.section {
	case settingsRegistries:
		if m.cursor < len(m.cfg.Registries) {
			reg := m.cfg.Registries[m.cursor]
			deleteCmd := func() tea.Msg {
				regMgr := core.NewRegistryManager(app.config.RegistriesDir())
				_ = regMgr.Remove(reg.Repo)

				cfg, err := app.config.Load()
				if err != nil {
					return errMsg{err: err}
				}
				newRegs := make([]core.Registry, 0, len(cfg.Registries))
				for _, r := range cfg.Registries {
					if r.Repo != reg.Repo {
						newRegs = append(newRegs, r)
					}
				}
				cfg.Registries = newRegs
				if err := app.config.Save(cfg); err != nil {
					return errMsg{err: err}
				}
				return app.reloadConfig()()
			}
			app.confirm = app.confirm.show(
				fmt.Sprintf("Remove registry %s?", reg.Name),
				deleteCmd,
			)
			return m, nil
		}
	}
	return m, nil
}

func (m settingsModel) refreshSelectedRegistry(app *App) tea.Cmd {
	if m.cursor >= len(m.cfg.Registries) {
		return nil
	}
	reg := m.cfg.Registries[m.cursor]
	return func() tea.Msg {
		regMgr := core.NewRegistryManager(app.config.RegistriesDir())
		manifest, err := regMgr.Refresh(reg.Repo)
		if err != nil {
			// Use registryAddDoneMsg so app.go can detect clone errors
			// from gitPull and show the clone error overlay.
			return registryAddDoneMsg{url: reg.Repo, err: fmt.Errorf("refreshing %s: %w", reg.Name, err)}
		}
		return registryAddDoneMsg{url: reg.Repo, name: reg.Name, warnings: manifest.Warnings}
	}
}

func (m settingsModel) view() string {
	if m.cfg == nil {
		return mutedStyle.Render("  Loading settings...")
	}

	var b strings.Builder

	// Registries section.
	b.WriteString(renderSectionHeader("REGISTRIES", m.width))
	b.WriteString("\n")

	if len(m.cfg.Registries) == 0 {
		b.WriteString(mutedStyle.Render("    No registries configured"))
		b.WriteString("\n")
	}

	for i, reg := range m.cfg.Registries {
		isSelected := m.section == settingsRegistries && i == m.cursor
		b.WriteString(m.renderRegistryRow(reg, isSelected))
	}

	// Add Registry action.
	b.WriteString("\n")
	isAddReg := m.section == settingsAddRegistry
	if isAddReg {
		b.WriteString(selectedItemStyle.Render("  + Add Registry"))
	} else {
		b.WriteString(mutedStyle.Render("  + Add Registry"))
	}
	b.WriteString("\n")

	// Footer: version + learn more link, pinned to the bottom.
	content := b.String()
	footer := m.renderFooter()
	footerLines := strings.Count(footer, "\n") + 1
	contentLines := strings.Count(content, "\n")
	gap := m.height - contentLines - footerLines
	if gap > 0 {
		content += strings.Repeat("\n", gap)
	}
	content += footer

	return content
}

func (m settingsModel) renderRegistryRow(reg core.Registry, selected bool) string {
	indicator := "    "
	if selected {
		indicator = "  > "
	}

	var b strings.Builder
	if selected {
		b.WriteString(indicator + selectedItemStyle.Render(reg.Name))
	} else {
		b.WriteString(indicator + normalItemStyle.Render(reg.Name))
	}
	b.WriteString("  " + mutedStyle.Render(reg.Repo))
	b.WriteString("\n")

	return b.String()
}

func (m settingsModel) renderFooter() string {
	ver := m.version
	if ver == "" {
		ver = "dev"
	}
	versionLine := mutedStyle.Render("  duckrow ver: " + ver)
	learnMore := mutedStyle.Render("  Learn more: ") + mutedStyle.Render("https://github.com/barysiuk/duckrow")
	return versionLine + "\n" + learnMore
}
