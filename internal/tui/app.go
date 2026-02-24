package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
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
	viewCloneError                   // Clone error overlay
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
	folder     folderModel
	picker     pickerModel
	install    installModel
	settings   settingsModel
	cloneError cloneErrorModel

	// View the user was on before clone error overlay opened (for going back).
	previousView appView

	// Skill preview.
	previewViewport viewport.Model
	previewTitle    string
	previewLoading  bool
	previewSpinner  spinner.Model

	// Cached glamour renderer (lazy-initialized on first preview).
	glamourRenderer *glamour.TermRenderer

	// Help bar.
	help help.Model

	// Shared data.
	cfg            *core.Config
	folderStatus   []core.FolderStatus
	registrySkills []core.RegistrySkillInfo
	registryMCPs   []core.RegistryMCPInfo

	// Active folder's computed data.
	activeFolderStatus *core.FolderStatus
	activeFolderMCPs   []mcpItem // Installed MCPs for the active folder

	// Registry commit map: source -> commit (built from registry manifests).
	registryCommits map[string]string

	// Update info for the active folder's skills: skill name -> update info.
	updateInfo map[string]core.UpdateInfo

	// Status bar (replaces toast + refresh spinner).
	statusBar statusBarModel

	// Confirmation dialog (replaces help bar when active).
	confirm confirmModel
}

// NewApp creates a new App model with the given core dependencies.
func NewApp(config *core.ConfigManager, agents []core.AgentDef) App {
	scanner := core.NewScanner(agents)
	foldersManager := core.NewFolderManager(config)
	registryMgr := core.NewRegistryManager(config.RegistriesDir())

	cwd, _ := os.Getwd()

	h := help.New()
	h.ShortSeparator = "  |  "

	s := spinner.New(
		spinner.WithSpinner(spinner.Dot),
		spinner.WithStyle(spinnerStyle),
	)

	return App{
		config:         config,
		agents:         agents,
		scanner:        scanner,
		folders:        foldersManager,
		registry:       registryMgr,
		cwd:            cwd,
		activeFolder:   cwd,
		folder:         newFolderModel(),
		picker:         newPickerModel(),
		install:        newInstallModel(),
		settings:       newSettingsModel(),
		cloneError:     newCloneErrorModel(),
		help:           h,
		previewSpinner: s,
		statusBar:      newStatusBarModel(),
		confirm:        newConfirmModel(),
	}
}

// --- Messages ---

type loadedDataMsg struct {
	cfg             *core.Config
	folderStatus    []core.FolderStatus
	registrySkills  []core.RegistrySkillInfo
	registryMCPs    []core.RegistryMCPInfo
	registryCommits map[string]string // source -> commit from registries
	err             error
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

type updateDoneMsg struct {
	skillName string
	err       error
}

type bulkUpdateDoneMsg struct {
	updated int
	errors  int
}

// registryRefreshDoneMsg is sent when the async registry refresh completes.
type registryRefreshDoneMsg struct {
	registryCommits map[string]string // source -> latest commit
	registrySkills  []core.RegistrySkillInfo
	registryMCPs    []core.RegistryMCPInfo
}

// startRegistryRefreshMsg triggers the async registry refresh and shows the spinner.
type startRegistryRefreshMsg struct{}

// openPreviewMsg is sent by the folder model to open the SKILL.md preview.
type openPreviewMsg struct {
	title   string
	content string
}

// previewRenderedMsg is sent when background glamour rendering completes.
type previewRenderedMsg struct {
	content  string
	renderer *glamour.TermRenderer
}

// --- Init / Update / View ---

func (a App) Init() tea.Cmd {
	return tea.Batch(a.loadDataCmd, a.startRegistryRefreshCmd)
}

// startRegistryRefreshCmd sets the refreshing flag and kicks off the async refresh.
// This is a two-step pattern: first send a message to update UI state, then run the async work.
func (a App) startRegistryRefreshCmd() tea.Msg {
	return startRegistryRefreshMsg{}
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
			var cmd tea.Cmd
			a.statusBar, cmd = a.statusBar.showMsg(fmt.Sprintf("Error: %v", msg.err), statusError)
			return a, cmd
		}
		a.cfg = msg.cfg
		a.folderStatus = msg.folderStatus
		a.registrySkills = msg.registrySkills
		a.registryMCPs = msg.registryMCPs
		a.registryCommits = msg.registryCommits
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
			// Check if this is a clone error — if so, show the clone error overlay.
			if ce, ok := core.IsCloneError(msg.err); ok {
				a.previousView = a.activeView
				a.activeView = viewCloneError
				a.cloneError = a.cloneError.activateForInstall(
					ce,
					a.install.selectedSkillInfo(),
					a.install.activeFolder,
					a.install.selectedTargetAgents(),
				)
				return a, nil
			}
			var cmd tea.Cmd
			a.statusBar, cmd = a.statusBar.showMsg(fmt.Sprintf("Error: %v", msg.err), statusError)
			return a, tea.Batch(cmd, a.loadDataCmd)
		}
		var cmd tea.Cmd
		a.statusBar, cmd = a.statusBar.showMsg(fmt.Sprintf("Installed %s", msg.skillName), statusSuccess)
		a.activeView = viewFolder
		return a, tea.Batch(cmd, a.loadDataCmd)

	case mcpInstallDoneMsg:
		a.install.setInstalling(false)
		if msg.err != nil {
			var cmd tea.Cmd
			a.statusBar, cmd = a.statusBar.showMsg(fmt.Sprintf("Error: %v", msg.err), statusError)
			return a, tea.Batch(cmd, a.loadDataCmd)
		}
		var cmd tea.Cmd
		a.statusBar, cmd = a.statusBar.showMsg(fmt.Sprintf("Installed MCP %s", msg.mcpName), statusSuccess)
		a.activeView = viewFolder
		return a, tea.Batch(cmd, a.loadDataCmd)

	case updateDoneMsg:
		if msg.err != nil {
			var cmd tea.Cmd
			a.statusBar, cmd = a.statusBar.showMsg(fmt.Sprintf("Error updating %s: %v", msg.skillName, msg.err), statusError)
			return a, tea.Batch(cmd, a.loadDataCmd)
		}
		var cmd tea.Cmd
		a.statusBar, cmd = a.statusBar.showMsg(fmt.Sprintf("Updated %s", msg.skillName), statusSuccess)
		return a, tea.Batch(cmd, a.loadDataCmd)

	case bulkUpdateDoneMsg:
		var cmd tea.Cmd
		if msg.errors > 0 {
			a.statusBar, cmd = a.statusBar.showMsg(
				fmt.Sprintf("Updated %d skills, %d errors", msg.updated, msg.errors), statusWarning)
		} else {
			a.statusBar, cmd = a.statusBar.showMsg(
				fmt.Sprintf("Updated %d skills", msg.updated), statusSuccess)
		}
		return a, tea.Batch(cmd, a.loadDataCmd)

	case startRegistryRefreshMsg:
		var cmd tea.Cmd
		a.statusBar, cmd = a.statusBar.update(taskStartedMsg{})
		return a, tea.Batch(cmd, a.refreshRegistriesCmd)

	case registryRefreshDoneMsg:
		var cmd tea.Cmd
		a.statusBar, cmd = a.statusBar.update(taskDoneMsg{})
		a.registryCommits = msg.registryCommits
		a.registrySkills = msg.registrySkills
		a.registryMCPs = msg.registryMCPs
		a.refreshActiveFolder()
		a.pushDataToSubModels()
		return a, cmd

	case registryAddDoneMsg:
		// Close the task counter that was started in settings (add/refresh).
		var taskCmd tea.Cmd
		a.statusBar, taskCmd = a.statusBar.update(taskDoneMsg{})

		if msg.err != nil {
			// Check if this is a clone error.
			if ce, ok := core.IsCloneError(msg.err); ok {
				a.previousView = a.activeView
				a.activeView = viewCloneError
				a.cloneError = a.cloneError.activateForRegistryAdd(ce, msg.url)
				return a, taskCmd
			}
			var cmd tea.Cmd
			a.statusBar, cmd = a.statusBar.showMsg(fmt.Sprintf("Error: %v", msg.err), statusError)
			return a, tea.Batch(taskCmd, cmd, a.loadDataCmd)
		}
		var cmd tea.Cmd
		if len(msg.warnings) > 0 {
			a.statusBar, cmd = a.statusBar.showMsg(
				fmt.Sprintf("Registry %s: %d warning(s)", msg.name, len(msg.warnings)),
				statusWarning)
		} else {
			a.statusBar, cmd = a.statusBar.showMsg(fmt.Sprintf("Added registry %s", msg.name), statusSuccess)
		}
		// Reload data and trigger async registry refresh (hydration + commit map rebuild).
		return a, tea.Batch(taskCmd, cmd, a.loadDataCmd, a.startRegistryRefreshCmd)

	case cloneRetryResultMsg:
		// Result from a retry initiated from the clone error overlay.
		if msg.cloneErr != nil {
			// Clone failed again — update the overlay with the new error.
			a.cloneError = a.cloneError.handleRetryResult(msg)
			return a, nil
		}
		if msg.postCloneErr != nil {
			// Clone succeeded but post-clone step failed — keep overlay visible.
			a.cloneError = a.cloneError.handleRetryResult(msg)
			return a, nil
		}
		// Full success — dismiss the overlay and reload data.
		a.cloneError = a.cloneError.handleRetryResult(msg)
		var successMsg string
		switch msg.origin {
		case retryOriginInstall:
			successMsg = fmt.Sprintf("Installed %s", msg.skillName)
			a.activeView = viewFolder
		case retryOriginRegistryAdd:
			successMsg = fmt.Sprintf("Added registry %s", msg.registryName)
			a.activeView = a.previousView
		}
		var cmd tea.Cmd
		a.statusBar, cmd = a.statusBar.showMsg(successMsg, statusSuccess)
		return a, tea.Batch(cmd, a.loadDataCmd)

	case openPreviewMsg:
		a.activeView = viewSkillPreview
		a.previewTitle = msg.title
		a.previewLoading = true
		w, h := a.innerContentSize()
		// -4 for preview's own header, separator, footer, and separator lines.
		vp := viewport.New(w, max(0, h-4))
		a.previewViewport = vp

		// Render markdown in background to avoid blocking the UI.
		rawContent := msg.content
		cachedRenderer := a.glamourRenderer
		renderCmd := func() tea.Msg {
			r := cachedRenderer
			if r == nil {
				var err error
				r, err = glamour.NewTermRenderer(
					glamour.WithAutoStyle(),
					glamour.WithWordWrap(w),
				)
				if err != nil {
					return previewRenderedMsg{content: rawContent}
				}
			}
			rendered, err := r.Render(rawContent)
			if err != nil {
				rendered = rawContent
			}
			return previewRenderedMsg{
				content:  strings.TrimRight(rendered, "\n"),
				renderer: r,
			}
		}
		return a, tea.Batch(a.previewSpinner.Tick, renderCmd)

	case previewRenderedMsg:
		a.previewLoading = false
		a.previewViewport.SetContent(msg.content)
		// Cache the renderer for future previews.
		if msg.renderer != nil {
			a.glamourRenderer = msg.renderer
		}
		return a, nil

	case spinner.TickMsg:
		// Route spinner ticks to all active consumers.
		// Multiple spinners can be active simultaneously (e.g. status bar + preview),
		// so we collect commands from each and batch them.
		var cmds []tea.Cmd
		if a.statusBar.tasksRunning() {
			var cmd tea.Cmd
			a.statusBar, cmd = a.statusBar.update(msg)
			cmds = append(cmds, cmd)
		}
		if a.previewLoading {
			var cmd tea.Cmd
			a.previewSpinner, cmd = a.previewSpinner.Update(msg)
			cmds = append(cmds, cmd)
		}
		if a.activeView == viewCloneError && a.cloneError.isRetrying() {
			var cmd tea.Cmd
			a.cloneError, cmd = a.cloneError.update(msg, &a)
			cmds = append(cmds, cmd)
		}
		if len(cmds) > 0 {
			return a, tea.Batch(cmds...)
		}
		// Fall through to delegate section for install spinner, etc.

	case statusDismissMsg:
		var cmd tea.Cmd
		a.statusBar, cmd = a.statusBar.update(msg)
		return a, cmd

	case taskStartedMsg:
		var cmd tea.Cmd
		a.statusBar, cmd = a.statusBar.update(msg)
		return a, cmd

	case taskDoneMsg:
		var cmd tea.Cmd
		a.statusBar, cmd = a.statusBar.update(msg)
		return a, cmd

	case statusMsg:
		var cmd tea.Cmd
		a.statusBar, cmd = a.statusBar.showMsg(msg.text, statusSuccess)
		return a, cmd

	case errMsg:
		var cmd tea.Cmd
		a.statusBar, cmd = a.statusBar.showMsg(fmt.Sprintf("Error: %v", msg.err), statusError)
		return a, cmd

	case confirmResultMsg:
		// Confirmation result — currently a no-op at the app level.
		// Individual callers react via the onConfirm command they provided.
		return a, nil

	case tea.KeyMsg:
		// Confirmation dialog intercepts all keys when active.
		if a.confirm.active {
			var cmd tea.Cmd
			var consumed bool
			a.confirm, cmd, consumed = a.confirm.update(msg)
			if consumed {
				return a, cmd
			}
		}

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

		// Handle clone error keys — the overlay manages its own input.
		if a.activeView == viewCloneError {
			// While retrying, ignore all key input.
			if a.cloneError.isRetrying() {
				return a, nil
			}
			// Editing mode: don't intercept esc/q globally.
			if a.cloneError.editing {
				break
			}
			if key.Matches(msg, keys.Back) || key.Matches(msg, keys.Quit) {
				a.activeView = a.previousView
				return a, nil
			}
		}

		// Global quit (unless input is focused).
		if key.Matches(msg, keys.Quit) {
			if a.activeView == viewSettings && a.settings.inputFocused() {
				break
			}
			if a.activeView == viewInstallPicker && a.install.isInstalling() {
				break // Don't quit during install
			}
			if a.activeView == viewCloneError {
				break // Handled above
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
			if a.activeView == viewCloneError {
				break // Handled above
			}
			// Don't intercept esc while filtering — let the list handle it.
			if a.isListFiltering() {
				break
			}
			// Don't intercept esc during agent selection or MCP phases — let install model handle it.
			if a.activeView == viewInstallPicker && (a.install.isSelectingAgents() || a.install.isMCPPhase()) {
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
			case key.Matches(msg, keys.Bookmarks):
				a.activeView = viewFolderPicker
				a.picker = a.picker.activate(a.activeFolder, a.folderStatus)
				return a, nil
			case key.Matches(msg, keys.Install):
				if len(a.registrySkills) > 0 || len(a.registryMCPs) > 0 {
					a.activeView = viewInstallPicker
					a.install = a.install.setMCPData(a.registryMCPs, a.activeFolderMCPs)
					a.install = a.install.activate(a.activeFolder, a.registrySkills, a.activeFolderStatus, a.agents)
				}
				return a, nil
			case key.Matches(msg, keys.Settings):
				a.activeView = viewSettings
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
	case viewCloneError:
		a.cloneError, cmd = a.cloneError.update(msg, &a)
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

	// 1. Render fixed chrome (header, status bar).
	header := a.renderHeader()
	helpBar := a.statusBar.view(a.renderHelpBar())

	// 2. Measure fixed chrome height.
	//    JoinVertical adds \n between each block. We always have
	//    3 blocks (header, styled, helpBar) → 2 separators.
	separators := 2 // between header/styled and styled/helpBar
	chromeH := lipgloss.Height(header)
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
	case viewCloneError:
		content = a.cloneError.view()
	}

	// If a confirmation dialog is active, overlay it on the content area.
	if a.confirm.active {
		content = a.confirm.view()
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
	parts := []string{header, styled, helpBar}

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (a App) renderHeader() string {
	logo := logoStyle.Render("🐤duckrow")
	path := headerPathStyle.Render(shortenPath(a.activeFolder))

	var hints string
	switch a.activeView {
	case viewFolder:
		hints = headerHintStyle.Render("[b] bookmarks  [s] settings")
	case viewFolderPicker:
		hints = headerHintStyle.Render("Select Folder")
	case viewInstallPicker:
		if a.install.isSelectingAgents() {
			hints = headerHintStyle.Render("Select Agents")
		} else if a.install.phase == installPhaseEnvEntry {
			hints = headerHintStyle.Render("Configure Env Vars")
		} else if a.install.isMCPPhase() {
			hints = headerHintStyle.Render("Install MCP")
		} else {
			hints = headerHintStyle.Render("Install")
		}
	case viewSettings:
		hints = headerHintStyle.Render("Settings")
	case viewSkillPreview:
		hints = headerHintStyle.Render(a.previewTitle)
	case viewCloneError:
		if a.cloneError.isRetrying() {
			hints = headerHintStyle.Render("Cloning...")
		} else if a.cloneError.postCloneErr != nil {
			hints = headerHintStyle.Render("Clone Result")
		} else {
			hints = headerHintStyle.Render("Clone Error")
		}
	}

	// Indent 1 char to align with content box's left border.
	indent := " "
	left := lipgloss.JoinHorizontal(lipgloss.Top, indent, logo, " ", path)
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
		km = folderHelpKeyMap{updatesAvailable: len(a.updateInfo) > 0}
	case viewFolderPicker:
		km = pickerHelpKeyMap{}
	case viewInstallPicker:
		if a.install.isSelectingAgents() {
			km = agentSelectHelpKeyMap{}
		} else if a.install.phase == installPhaseMCPPreview {
			km = mcpPreviewHelpKeyMap{hasEnvVars: len(a.install.mcpEnvStatus) > 0}
		} else if a.install.phase == installPhaseEnvEntry {
			km = envEntryHelpKeyMap{}
		} else {
			km = installHelpKeyMap{}
		}
	case viewSettings:
		km = settingsHelpKeyMap{}
	case viewSkillPreview:
		km = previewHelpKeyMap{}
	case viewCloneError:
		km = cloneErrorHelpKeyMap{editing: a.cloneError.editing, retrying: a.cloneError.isRetrying()}
	}

	// Indent 1 char to align with content box's left border.
	return " " + helpStyle.Render(a.help.View(km))
}

func (a App) renderPreview() string {
	w, _ := a.innerContentSize()
	title := viewportTitleStyle.Render(" " + a.previewTitle + " ")
	line := strings.Repeat("─", max(0, w-lipgloss.Width(title)))
	header := lipgloss.JoinHorizontal(lipgloss.Center, title, mutedStyle.Render(line))

	if a.previewLoading {
		loading := a.previewSpinner.View() + " Rendering preview..."
		return header + "\n\n" + loading
	}

	pct := fmt.Sprintf(" %3.0f%% ", a.previewViewport.ScrollPercent()*100)
	footer := previewPctStyle.Render(pct)

	return header + "\n\n" + a.previewViewport.View() + "\n\n" + footer
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
	regMCPs := a.registry.ListMCPs(cfg.Registries)

	// Build registry commit map for update detection.
	registryCommits := core.BuildRegistryCommitMap(cfg.Registries, a.registry)

	return loadedDataMsg{
		cfg:             cfg,
		folderStatus:    statuses,
		registrySkills:  regSkills,
		registryMCPs:    regMCPs,
		registryCommits: registryCommits,
	}
}

func (a *App) refreshActiveFolder() {
	a.isTracked = false
	a.activeFolderStatus = nil
	a.updateInfo = nil
	a.activeFolderMCPs = nil

	for i := range a.folderStatus {
		if a.folderStatus[i].Folder.Path == a.activeFolder {
			a.isTracked = true
			a.activeFolderStatus = &a.folderStatus[i]
			break
		}
	}

	if a.activeFolderStatus == nil {
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

	// Load MCPs from lock file for the active folder.
	lf, lfErr := core.ReadLockFile(a.activeFolder)
	if lfErr == nil && lf != nil {
		// Build description lookup from registry MCPs.
		mcpDescriptions := make(map[string]string)
		for _, rm := range a.registryMCPs {
			mcpDescriptions[rm.MCP.Name] = rm.MCP.Description
		}

		a.activeFolderMCPs = make([]mcpItem, len(lf.MCPs))
		for i, locked := range lf.MCPs {
			a.activeFolderMCPs[i] = mcpItem{
				locked: locked,
				desc:   mcpDescriptions[locked.Name],
			}
		}
	}

	// Compute update info by comparing lock file commits against registry commits.
	if len(a.registryCommits) > 0 {
		if lfErr == nil && lf != nil {
			pathIndex := core.BuildPathIndex(a.registryCommits)
			a.updateInfo = make(map[string]core.UpdateInfo)
			for _, skill := range lf.Skills {
				if regCommit := core.LookupRegistryCommit(skill.Source, a.registryCommits, pathIndex); regCommit != "" {
					if skill.Commit != regCommit {
						a.updateInfo[skill.Name] = core.UpdateInfo{
							Name:            skill.Name,
							Source:          skill.Source,
							InstalledCommit: skill.Commit,
							AvailableCommit: regCommit,
							HasUpdate:       true,
						}
					}
				}
			}
		}
	}
}

func (a *App) pushDataToSubModels() {
	a.folder = a.folder.setData(a.activeFolderStatus, a.isTracked, a.registrySkills, a.registryMCPs, a.updateInfo, a.activeFolderMCPs)
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
	a.cloneError = a.cloneError.setSize(w, h)
	a.confirm = a.confirm.setSize(w, h)
	a.statusBar.width = a.width

	// Update preview viewport if active.
	if a.activeView == viewSkillPreview {
		a.previewViewport.Width = w
		a.previewViewport.Height = max(0, h-4) // header + separator + footer + separator
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

	// JoinVertical adds \n between blocks. Always 3 blocks
	// (header, styled, helpBar) → 2 separators.
	// The status bar replaces the help bar in-place, so no extra block is needed.
	separators := 2
	chromeH := lipgloss.Height(header)
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

func (a *App) reloadConfig() tea.Cmd {
	return a.loadDataCmd
}

// refreshRegistriesCmd refreshes all registries (network call), hydrates
// unpinned skill commits, and returns the updated commit map plus refreshed
// skill and MCP lists from the updated manifests.
// This runs asynchronously — the TUI remains responsive while it executes.
func (a App) refreshRegistriesCmd() tea.Msg {
	cfg, err := a.config.Load()
	if err != nil {
		return registryRefreshDoneMsg{}
	}

	if len(cfg.Registries) > 0 {
		// Refresh registries (git pull).
		// Errors are intentionally ignored — stale data is acceptable.
		_, _ = a.registry.RefreshAll(cfg.Registries)

		// Hydrate unpinned skills: resolve latest commits via shallow clone.
		// Best-effort — clone errors are silently skipped.
		a.registry.HydrateRegistryCommits(cfg.Registries, cfg.Settings.CloneURLOverrides)
	}

	registryCommits := core.BuildRegistryCommitMap(cfg.Registries, a.registry)

	// Re-list skills and MCPs from the refreshed manifests so the TUI
	// picks up any new entries that were added since the initial load.
	regSkills := a.registry.ListSkills(cfg.Registries)
	regMCPs := a.registry.ListMCPs(cfg.Registries)

	return registryRefreshDoneMsg{
		registryCommits: registryCommits,
		registrySkills:  regSkills,
		registryMCPs:    regMCPs,
	}
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
