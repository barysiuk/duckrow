package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/barysiuk/duckrow/internal/core"
)

// settingsSection defines navigable sections in settings.
type settingsSection int

const (
	settingsRegistries settingsSection = iota
	settingsAddRegistry
	settingsFolders
)

// settingsModel is the settings/configuration screen.
type settingsModel struct {
	width  int
	height int

	// Navigation.
	section settingsSection
	cursor  int // Cursor within the current section.

	// Input mode for adding registry.
	inputMode    bool
	inputSection settingsSection // Which section triggered the input.
	textInput    textinput.Model

	// Data.
	cfg *core.Config
}

func newSettingsModel() settingsModel {
	ti := textinput.New()
	ti.Placeholder = "Enter URL..."
	ti.CharLimit = 256

	return settingsModel{
		textInput: ti,
	}
}

func (m settingsModel) setSize(width, height int) settingsModel {
	m.width = width
	m.height = height
	return m
}

func (m settingsModel) setData(cfg *core.Config) settingsModel {
	m.cfg = cfg
	return m
}

func (m settingsModel) inputFocused() bool {
	return m.inputMode
}

func (m settingsModel) update(msg tea.Msg, app *App) (settingsModel, tea.Cmd) {
	if m.cfg == nil {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Input mode handling.
		if m.inputMode {
			switch {
			case key.Matches(msg, keys.Back):
				m.inputMode = false
				m.textInput.Blur()
				m.textInput.SetValue("")
				return m, nil
			case key.Matches(msg, keys.Enter):
				value := m.textInput.Value()
				m.inputMode = false
				m.textInput.Blur()
				m.textInput.SetValue("")
				if value != "" {
					return m, m.handleInputSubmit(value, app)
				}
				return m, nil
			default:
				var cmd tea.Cmd
				m.textInput, cmd = m.textInput.Update(msg)
				return m, cmd
			}
		}

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
			return m, m.handleDelete(app)

		case key.Matches(msg, keys.Refresh):
			if m.section == settingsRegistries && len(m.cfg.Registries) > 0 {
				return m, m.refreshSelectedRegistry(app)
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
	case settingsFolders:
		if m.cursor > 0 {
			m.cursor--
		} else {
			m.section = settingsAddRegistry
			m.cursor = 0
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
		if len(m.cfg.Folders) > 0 {
			m.section = settingsFolders
			m.cursor = 0
		}
	case settingsFolders:
		if m.cursor < len(m.cfg.Folders)-1 {
			m.cursor++
		}
	}
	return m
}

func (m settingsModel) handleEnter(app *App) (settingsModel, tea.Cmd) {
	switch m.section {
	case settingsAddRegistry:
		m.inputMode = true
		m.inputSection = settingsAddRegistry
		m.textInput.Placeholder = "Git repository URL..."
		m.textInput.Focus()
		return m, m.textInput.Cursor.BlinkCmd()
	}
	return m, nil
}

func (m settingsModel) handleInputSubmit(value string, app *App) tea.Cmd {
	switch m.inputSection {
	case settingsAddRegistry:
		return func() tea.Msg {
			regMgr := core.NewRegistryManager(app.config.RegistriesDir())
			manifest, err := regMgr.Add(value)
			if err != nil {
				return errMsg{err: fmt.Errorf("adding registry: %w", err)}
			}

			// Save registry to config.
			cfg, err := app.config.Load()
			if err != nil {
				return errMsg{err: err}
			}
			cfg.Registries = append(cfg.Registries, core.Registry{
				Name: manifest.Name,
				Repo: value,
			})
			if err := app.config.Save(cfg); err != nil {
				return errMsg{err: err}
			}
			return app.reloadConfig()()
		}
	}
	return nil
}

func (m settingsModel) handleDelete(app *App) tea.Cmd {
	switch m.section {
	case settingsRegistries:
		if m.cursor < len(m.cfg.Registries) {
			reg := m.cfg.Registries[m.cursor]
			return func() tea.Msg {
				regMgr := core.NewRegistryManager(app.config.RegistriesDir())
				_ = regMgr.Remove(reg.Name)

				cfg, err := app.config.Load()
				if err != nil {
					return errMsg{err: err}
				}
				newRegs := make([]core.Registry, 0, len(cfg.Registries))
				for _, r := range cfg.Registries {
					if r.Name != reg.Name {
						newRegs = append(newRegs, r)
					}
				}
				cfg.Registries = newRegs
				if err := app.config.Save(cfg); err != nil {
					return errMsg{err: err}
				}
				return app.reloadConfig()()
			}
		}

	case settingsFolders:
		if m.cursor < len(m.cfg.Folders) {
			path := m.cfg.Folders[m.cursor].Path
			return func() tea.Msg {
				if err := app.folders.Remove(path); err != nil {
					return errMsg{err: err}
				}
				return app.reloadConfig()()
			}
		}
	}
	return nil
}

func (m settingsModel) refreshSelectedRegistry(app *App) tea.Cmd {
	if m.cursor >= len(m.cfg.Registries) {
		return nil
	}
	name := m.cfg.Registries[m.cursor].Name
	return func() tea.Msg {
		regMgr := core.NewRegistryManager(app.config.RegistriesDir())
		_, err := regMgr.Refresh(name)
		if err != nil {
			return errMsg{err: fmt.Errorf("refreshing %s: %w", name, err)}
		}
		return app.reloadConfig()()
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
		isSelected := !m.inputMode && m.section == settingsRegistries && i == m.cursor
		b.WriteString(m.renderRegistryRow(reg, isSelected))
	}

	// Add Registry action (or inline input).
	b.WriteString("\n")
	if m.inputMode && m.inputSection == settingsAddRegistry {
		b.WriteString("  " + m.textInput.View())
	} else {
		isAddReg := !m.inputMode && m.section == settingsAddRegistry
		if isAddReg {
			b.WriteString(selectedItemStyle.Render("  + Add Registry"))
		} else {
			b.WriteString(mutedStyle.Render("  + Add Registry"))
		}
	}
	b.WriteString("\n\n")

	// Folders section.
	b.WriteString(renderSectionHeader("FOLDERS", m.width))
	b.WriteString("\n")

	if len(m.cfg.Folders) == 0 {
		b.WriteString(mutedStyle.Render("    No folders tracked"))
		b.WriteString("\n")
	}

	for i, folder := range m.cfg.Folders {
		isSelected := !m.inputMode && m.section == settingsFolders && i == m.cursor
		indicator := "    "
		if isSelected {
			indicator = "  > "
		}
		path := shortenPath(folder.Path)
		if isSelected {
			b.WriteString(indicator + selectedItemStyle.Render(path))
		} else {
			b.WriteString(indicator + normalItemStyle.Render(path))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("    To add a directory, launch ") + normalItemStyle.Render("duckrow") + mutedStyle.Render(" in that folder and press [a]"))
	b.WriteString("\n")

	return b.String()
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
