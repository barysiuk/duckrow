package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/barysiuk/duckrow/internal/core"
)

// appView represents the active screen.
type appView int

const (
	viewFolder        appView = iota // Main folder view (default)
	viewFolderPicker                 // Folder picker overlay
	viewInstallPicker                // Install skill picker overlay
	viewSettings                     // Settings overlay
	viewSkillPreview                 // SKILL.md preview overlay
)

// App is the root Bubbletea model for DuckRow.
type App struct {
	// Core dependencies.
	config   *core.ConfigManager
	agents   []core.AgentDef
	scanner  *core.Scanner
	folders  *core.FolderManager
	registry *core.RegistryManager

	// View state.
	activeView appView
	width      int
	height     int
	ready      bool

	// Active folder context.
	cwd          string // Directory where duckrow was launched
	activeFolder string // Currently viewed folder path
	isTracked    bool   // Whether activeFolder is in the tracked list

	// Sub-models.
	folder   folderModel
	picker   pickerModel
	install  installModel
	settings settingsModel

	// Skill preview.
	previewViewport viewport.Model
	previewTitle    string

	// Help bar.
	help help.Model

	// Shared data.
	cfg            *core.Config
	folderStatus   []core.FolderStatus
	registrySkills []core.RegistrySkillInfo

	// Active folder's computed data.
	activeFolderStatus *core.FolderStatus

	// Error / status display.
	err        error
	statusText string
}

// NewApp creates a new App model with the given core dependencies.
func NewApp(config *core.ConfigManager, agents []core.AgentDef) App {
	scanner := core.NewScanner(agents)
	foldersManager := core.NewFolderManager(config)
	registryMgr := core.NewRegistryManager(config.RegistriesDir())

	cwd, _ := os.Getwd()

	h := help.New()
	h.ShortSeparator = "  |  "

	return App{
		config:       config,
		agents:       agents,
		scanner:      scanner,
		folders:      foldersManager,
		registry:     registryMgr,
		cwd:          cwd,
		activeFolder: cwd,
		folder:       newFolderModel(),
		picker:       newPickerModel(),
		install:      newInstallModel(),
		settings:     newSettingsModel(),
		help:         h,
	}
}

// --- Messages ---

type loadedDataMsg struct {
	cfg            *core.Config
	folderStatus   []core.FolderStatus
	registrySkills []core.RegistrySkillInfo
	err            error
}

type errMsg struct {
	err error
}

type statusMsg struct {
	text string
}

type installDoneMsg struct {
	skillName string
	folder    string
	err       error
}

// openPreviewMsg is sent by the folder model to open the SKILL.md preview.
type openPreviewMsg struct {
	title   string
	content string
}

// --- Init / Update / View ---

func (a App) Init() tea.Cmd {
	return a.loadDataCmd
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.ready = true
		a.help.Width = msg.Width
		a.propagateSize()
		return a, nil

	case loadedDataMsg:
		if msg.err != nil {
			a.err = msg.err
			return a, nil
		}
		a.cfg = msg.cfg
		a.folderStatus = msg.folderStatus
		a.registrySkills = msg.registrySkills
		a.err = nil
		a.refreshActiveFolder()
		a.pushDataToSubModels()
		// Re-propagate sizes — isTracked may have changed, affecting height budgets.
		if a.ready {
			a.propagateSize()
		}
		return a, nil

	case installDoneMsg:
		a.install.setInstalling(false)
		if msg.err != nil {
			a.statusText = fmt.Sprintf("Error: %v", msg.err)
		} else {
			a.statusText = fmt.Sprintf("Installed %s", msg.skillName)
			a.activeView = viewFolder
		}
		return a, a.loadDataCmd

	case openPreviewMsg:
		a.activeView = viewSkillPreview
		a.previewTitle = msg.title
		w, h := a.innerContentSize()
		// -2 for preview's own header + footer lines.
		vp := viewport.New(w, max(0, h-2))
		vp.SetContent(msg.content)
		a.previewViewport = vp
		return a, nil

	case statusMsg:
		a.statusText = msg.text
		return a, nil

	case errMsg:
		a.err = msg.err
		return a, nil

	case tea.KeyMsg:
		// Clear status on any keypress.
		a.statusText = ""

		// Handle skill preview keys separately — viewport needs arrow/pgup/pgdn.
		if a.activeView == viewSkillPreview {
			if key.Matches(msg, keys.Back) || key.Matches(msg, keys.Quit) {
				a.activeView = viewFolder
				return a, nil
			}
			var cmd tea.Cmd
			a.previewViewport, cmd = a.previewViewport.Update(msg)
			return a, cmd
		}

		// Global quit (unless input is focused).
		if key.Matches(msg, keys.Quit) {
			if a.activeView == viewSettings && a.settings.inputFocused() {
				break
			}
			if a.activeView == viewInstallPicker && a.install.isInstalling() {
				break // Don't quit during install
			}
			// Don't quit while filtering in any list view.
			if a.isListFiltering() {
				break
			}
			return a, tea.Quit
		}

		// Global back: return to folder view from overlays.
		if key.Matches(msg, keys.Back) {
			if a.activeView == viewSettings && a.settings.inputFocused() {
				break // Let settings handle esc for input
			}
			// Don't intercept esc while filtering — let the list handle it.
			if a.isListFiltering() {
				break
			}
			if a.activeView != viewFolder {
				a.activeView = viewFolder
				return a, nil
			}
		}

		// View-switching keys (only from folder view, and not while filtering).
		if a.activeView == viewFolder && !a.folder.list.SettingFilter() {
			switch {
			case key.Matches(msg, keys.ChangeFolder):
				a.activeView = viewFolderPicker
				a.picker = a.picker.activate(a.activeFolder, a.folderStatus)
				return a, nil
			case key.Matches(msg, keys.Install):
				if len(a.registrySkills) > 0 {
					a.activeView = viewInstallPicker
					a.install = a.install.activate(a.activeFolder, a.registrySkills, a.activeFolderStatus)
				}
				return a, nil
			case key.Matches(msg, keys.Settings):
				a.activeView = viewSettings
				return a, nil
			case key.Matches(msg, keys.AddFolder):
				if !a.isTracked {
					return a, a.addActiveFolder()
				}
				return a, nil
			}
		}
	}

	// Delegate to active sub-model.
	var cmd tea.Cmd
	switch a.activeView {
	case viewFolder:
		a.folder, cmd = a.folder.update(msg, &a)
	case viewFolderPicker:
		a.picker, cmd = a.picker.update(msg, &a)
	case viewInstallPicker:
		a.install, cmd = a.install.update(msg, &a)
	case viewSettings:
		a.settings, cmd = a.settings.update(msg, &a)
	}

	return a, cmd
}

func (a App) View() string {
	if !a.ready {
		return "Loading..."
	}

	// Layout: fixed header + optional status + flex content box + fixed footer.
	// Header and footer always render. Content box gets whatever remains.
	//
	// Frame sizes are read from contentStyle via GetVerticalFrameSize() etc.
	// so the layout adapts automatically if contentStyle changes.

	// 1. Render fixed chrome (header, status banners, help bar).
	header := a.renderHeader()
	helpBar := a.renderHelpBar()

	var statusBanner string
	if a.err != nil {
		statusBanner = errorStyle.Render(fmt.Sprintf(" Error: %v ", a.err))
	}
	if a.statusText != "" {
		if strings.HasPrefix(a.statusText, "Error:") {
			statusBanner = errorStyle.Render(" " + a.statusText + " ")
		} else {
			statusBanner = installedStyle.Render(" " + a.statusText + " ")
		}
	}

	// 2. Measure fixed chrome height.
	//    JoinVertical adds \n between each block. We always have at least
	//    3 blocks (header, styled, helpBar) → 2 separators. A status banner
	//    adds 1 more block → 1 more separator.
	separators := 2 // between header/styled and styled/helpBar
	chromeH := lipgloss.Height(header)
	if statusBanner != "" {
		chromeH += lipgloss.Height(statusBanner)
		separators++
	}
	chromeH += lipgloss.Height(helpBar)
	chromeH += separators // \n separators added by JoinVertical

	// 3. Compute content box dimensions from contentStyle's own frame sizes.
	//    frameV/H = border + padding combined.
	//    borderV/H = just the border (Width/Height include padding, exclude border).
	frameV := contentStyle.GetVerticalFrameSize()
	frameH := contentStyle.GetHorizontalFrameSize()
	borderV := contentStyle.GetVerticalBorderSize()
	borderH := contentStyle.GetHorizontalBorderSize()

	// Width/Height for contentStyle include padding but exclude border.
	innerW := max(0, a.width-borderH)
	innerH := max(0, a.height-chromeH-borderV)

	// Text area inside the box (after border + padding).
	textW := max(0, a.width-frameH)
	textH := max(0, a.height-chromeH-frameV)

	// 4. Render active view content.
	content := ""
	switch a.activeView {
	case viewFolder:
		content = a.folder.view()
	case viewFolderPicker:
		content = a.picker.view()
	case viewInstallPicker:
		content = a.install.view()
	case viewSettings:
		content = a.settings.view()
	case viewSkillPreview:
		content = a.renderPreview()
	}

	// Clamp content to the text area so it can't inflate the box.
	// clampWidth prevents wrapping; clampHeight prevents overflow.
	content = clampWidth(content, textW)
	content = clampHeight(content, textH)

	styled := contentStyle.
		Width(innerW).
		Height(innerH).
		Render(content)

	// 5. Assemble with lipgloss.JoinVertical for clean stacking.
	parts := []string{header}
	if statusBanner != "" {
		parts = append(parts, statusBanner)
	}
	parts = append(parts, styled, helpBar)

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (a App) renderHeader() string {
	logo := logoStyle.Render(" DuckRow ")
	path := headerPathStyle.Render(shortenPath(a.activeFolder))

	var hints string
	switch a.activeView {
	case viewFolder:
		hints = headerHintStyle.Render("[c] change  [s] settings")
	case viewFolderPicker:
		hints = headerHintStyle.Render("Select Folder")
	case viewInstallPicker:
		hints = headerHintStyle.Render("Install Skill")
	case viewSettings:
		hints = headerHintStyle.Render("Settings")
	case viewSkillPreview:
		hints = headerHintStyle.Render(a.previewTitle)
	}

	// Right-align hints.
	left := lipgloss.JoinHorizontal(lipgloss.Top, logo, " ", path)
	gap := a.width - lipgloss.Width(left) - lipgloss.Width(hints) - 1
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + hints
}

func (a App) renderHelpBar() string {
	var km help.KeyMap

	switch a.activeView {
	case viewFolder:
		km = folderHelpKeyMap{isTracked: a.isTracked}
	case viewFolderPicker:
		km = pickerHelpKeyMap{}
	case viewInstallPicker:
		km = installHelpKeyMap{}
	case viewSettings:
		km = settingsHelpKeyMap{}
	case viewSkillPreview:
		km = previewHelpKeyMap{}
	}

	return helpStyle.Render(a.help.View(km))
}

func (a App) renderPreview() string {
	w, _ := a.innerContentSize()
	title := viewportTitleStyle.Render(" " + a.previewTitle + " ")
	line := strings.Repeat("─", max(0, w-lipgloss.Width(title)))
	header := lipgloss.JoinHorizontal(lipgloss.Center, title, mutedStyle.Render(line))

	pct := fmt.Sprintf(" %3.0f%% ", a.previewViewport.ScrollPercent()*100)
	footer := mutedStyle.Render(pct)

	return header + "\n" + a.previewViewport.View() + "\n" + footer
}

// isListFiltering returns true if any list sub-model is currently in filter mode.
func (a App) isListFiltering() bool {
	switch a.activeView {
	case viewFolder:
		return a.folder.list.SettingFilter()
	case viewFolderPicker:
		return a.picker.list.SettingFilter()
	case viewInstallPicker:
		return a.install.list.SettingFilter()
	}
	return false
}

// --- Data management ---

func (a App) loadDataCmd() tea.Msg {
	cfg, err := a.config.Load()
	if err != nil {
		return loadedDataMsg{err: err}
	}

	var statuses []core.FolderStatus
	for _, folder := range cfg.Folders {
		skills, scanErr := a.scanner.ScanFolder(folder.Path)
		agents := a.scanner.DetectAgents(folder.Path)
		statuses = append(statuses, core.FolderStatus{
			Folder: folder,
			Skills: skills,
			Agents: agents,
			Error:  scanErr,
		})
	}

	regSkills := a.registry.ListSkills(cfg.Registries)

	return loadedDataMsg{
		cfg:            cfg,
		folderStatus:   statuses,
		registrySkills: regSkills,
	}
}

func (a *App) refreshActiveFolder() {
	a.isTracked = false
	a.activeFolderStatus = nil

	for i := range a.folderStatus {
		if a.folderStatus[i].Folder.Path == a.activeFolder {
			a.isTracked = true
			a.activeFolderStatus = &a.folderStatus[i]
			return
		}
	}

	// Active folder not tracked -- scan it anyway for display.
	skills, scanErr := a.scanner.ScanFolder(a.activeFolder)
	agents := a.scanner.DetectAgents(a.activeFolder)
	status := &core.FolderStatus{
		Folder: core.TrackedFolder{Path: a.activeFolder},
		Skills: skills,
		Agents: agents,
		Error:  scanErr,
	}
	a.activeFolderStatus = status
}

func (a *App) pushDataToSubModels() {
	a.folder = a.folder.setData(a.activeFolderStatus, a.isTracked, a.registrySkills)
	a.settings = a.settings.setData(a.cfg)
}

func (a *App) propagateSize() {
	w, h := a.innerContentSize()
	// innerContentSize returns the text content area (after border + padding).
	// Sub-models render into this space.
	a.folder = a.folder.setSize(w, h)
	a.picker = a.picker.setSize(w, h)
	a.install = a.install.setSize(w, h)
	a.settings = a.settings.setSize(w, h)

	// Update preview viewport if active.
	if a.activeView == viewSkillPreview {
		a.previewViewport.Width = w
		a.previewViewport.Height = max(0, h-2) // header + footer lines within preview
	}
}

// innerContentSize computes the text content area available to sub-models.
// This is the space inside contentStyle after border AND padding are removed.
// Frame sizes are read from contentStyle itself via GetVerticalFrameSize() etc.
// so this adapts automatically if contentStyle changes.
func (a App) innerContentSize() (width, height int) {
	// Measure actual rendered chrome heights.
	header := a.renderHeader()
	helpBar := a.renderHelpBar()

	// JoinVertical adds \n between blocks. Always 3 blocks minimum
	// (header, styled, helpBar) → 2 separators. A status banner (err or
	// statusText, never both displayed) adds 1 more block → 1 more separator.
	separators := 2
	chromeH := lipgloss.Height(header)
	hasStatus := a.err != nil || a.statusText != ""
	if hasStatus {
		chromeH++ // single status banner line
		separators++
	}
	chromeH += lipgloss.Height(helpBar)
	chromeH += separators

	// Frame = border + padding. Subtract the full frame to get the text area.
	frameV := contentStyle.GetVerticalFrameSize()
	frameH := contentStyle.GetHorizontalFrameSize()

	width = max(0, a.width-frameH)
	height = max(0, a.height-chromeH-frameV)

	return width, height
}

func (a *App) setActiveFolder(path string) {
	a.activeFolder = path
	a.refreshActiveFolder()
	a.pushDataToSubModels()
}

func (a *App) addActiveFolder() tea.Cmd {
	path := a.activeFolder
	return func() tea.Msg {
		fm := a.folders
		if err := fm.Add(path); err != nil {
			return errMsg{err: err}
		}
		return a.loadDataCmd()
	}
}

func (a *App) reloadConfig() tea.Cmd {
	return a.loadDataCmd
}

// clampHeight truncates content to at most maxLines lines.
// This is a safety net — if a sub-model renders more lines than its
// allocated height, we truncate rather than pushing the header off-screen.
func clampHeight(content string, maxLines int) string {
	if maxLines <= 0 {
		return ""
	}
	lines := strings.Split(content, "\n")
	if len(lines) <= maxLines {
		return content
	}
	return strings.Join(lines[:maxLines], "\n")
}

// clampWidth truncates each line to at most maxWidth visible characters
// (ANSI-escape aware). This prevents lipgloss from wrapping long lines
// inside a Width()-constrained box, which would inflate its rendered height
// and push the bottom border off-screen.
func clampWidth(content string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if lipgloss.Width(line) > maxWidth {
			lines[i] = ansi.Truncate(line, maxWidth, "")
		}
	}
	return strings.Join(lines, "\n")
}
