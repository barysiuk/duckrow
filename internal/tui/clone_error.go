package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/barysiuk/duckrow/internal/core"
)

// cloneRetryOrigin identifies what initiated the clone that failed,
// so we know how to retry and what to do on success.
type cloneRetryOrigin int

const (
	retryOriginInstall     cloneRetryOrigin = iota // Skill install from registry
	retryOriginRegistryAdd                         // Adding a new registry
)

// cloneErrorModel is the overlay shown when a git clone fails.
// It displays the command, error, hints, and lets the user edit the URL and retry.
//
// States:
//   - error view: shows clone error details, hints, edit/retry/esc keys
//   - editing: text input for URL editing
//   - retrying: spinner while retry is in progress
//   - post-clone error: clone succeeded but a later step failed (e.g. "no skills found")
type cloneErrorModel struct {
	width  int
	height int

	// The structured clone error from the initial (or last) clone failure.
	cloneErr *core.CloneError

	// Editing state.
	editing   bool
	textInput textinput.Model

	// Retry state.
	retrying bool
	retryURL string // The URL being retried (for display).
	spinner  spinner.Model

	// Post-clone error: clone succeeded but something after it failed.
	// When set, the overlay shows clone success + this error instead of dismissing.
	postCloneErr error

	// Retry context — what to do when the user retries.
	origin cloneRetryOrigin

	// For retryOriginInstall: the skill info and target folder.
	installSkill        core.RegistrySkillInfo
	installFolder       string
	installTargetAgents []core.AgentDef

	// For retryOriginRegistryAdd: the original registry URL.
	registryURL string

	// Scroll offset for the error view when it's tall.
	scrollOffset int
}

func newCloneErrorModel() cloneErrorModel {
	ti := textinput.New()
	ti.Placeholder = "Enter clone URL..."
	ti.CharLimit = 512

	s := spinner.New(
		spinner.WithSpinner(spinner.Dot),
		spinner.WithStyle(spinnerStyle),
	)

	return cloneErrorModel{
		textInput: ti,
		spinner:   s,
	}
}

func (m cloneErrorModel) setSize(width, height int) cloneErrorModel {
	m.width = width
	m.height = height
	return m
}

// activateForInstall sets up the clone error overlay for a failed skill install.
func (m cloneErrorModel) activateForInstall(ce *core.CloneError, skill core.RegistrySkillInfo, folder string, targetAgents []core.AgentDef) cloneErrorModel {
	m.cloneErr = ce
	m.origin = retryOriginInstall
	m.installSkill = skill
	m.installFolder = folder
	m.installTargetAgents = targetAgents
	m.registryURL = ""
	m.editing = false
	m.retrying = false
	m.retryURL = ""
	m.postCloneErr = nil
	m.scrollOffset = 0
	m.textInput.SetValue(ce.URL)
	return m
}

// activateForRegistryAdd sets up the clone error overlay for a failed registry add.
func (m cloneErrorModel) activateForRegistryAdd(ce *core.CloneError, url string) cloneErrorModel {
	m.cloneErr = ce
	m.origin = retryOriginRegistryAdd
	m.installSkill = core.RegistrySkillInfo{}
	m.installFolder = ""
	m.registryURL = url
	m.editing = false
	m.retrying = false
	m.retryURL = ""
	m.postCloneErr = nil
	m.scrollOffset = 0
	m.textInput.SetValue(ce.URL)
	return m
}

func (m cloneErrorModel) isRetrying() bool {
	return m.retrying
}

func (m cloneErrorModel) update(msg tea.Msg, app *App) (cloneErrorModel, tea.Cmd) {
	// Handle spinner ticks while retrying.
	if m.retrying {
		if tickMsg, ok := msg.(spinner.TickMsg); ok {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(tickMsg)
			return m, cmd
		}
		// Ignore all other messages (including keys) while retrying.
		return m, nil
	}

	if m.cloneErr == nil {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.editing {
			switch {
			case key.Matches(msg, keys.Back):
				// Cancel editing, return to error view.
				m.editing = false
				m.textInput.Blur()
				return m, nil
			case key.Matches(msg, keys.Enter):
				// Submit edited URL, trigger retry.
				newURL := strings.TrimSpace(m.textInput.Value())
				m.editing = false
				m.textInput.Blur()
				if newURL != "" {
					return m.startRetry(app, newURL)
				}
				return m, nil
			default:
				var cmd tea.Cmd
				m.textInput, cmd = m.textInput.Update(msg)
				return m, cmd
			}
		}

		// Non-editing mode.
		switch {
		case key.Matches(msg, keys.Edit):
			m.editing = true
			m.postCloneErr = nil // Clear post-clone error when editing.
			m.textInput.Focus()
			return m, m.textInput.Cursor.BlinkCmd()
		case key.Matches(msg, keys.Retry):
			// Retry with the current URL (may have been edited previously).
			url := m.cloneErr.URL
			if v := strings.TrimSpace(m.textInput.Value()); v != "" {
				url = v
			}
			return m.startRetry(app, url)
		case key.Matches(msg, keys.Back):
			// Cancel — return to previous view.
			return m, nil // App handles this via global esc
		case key.Matches(msg, keys.Down):
			m.scrollOffset++
			return m, nil
		case key.Matches(msg, keys.Up):
			if m.scrollOffset > 0 {
				m.scrollOffset--
			}
			return m, nil
		}
	}

	return m, nil
}

// startRetry transitions to the retrying state and launches the retry command.
func (m cloneErrorModel) startRetry(app *App, url string) (cloneErrorModel, tea.Cmd) {
	m.retrying = true
	m.retryURL = url
	m.postCloneErr = nil
	m.scrollOffset = 0

	retryCmd := m.buildRetryCmd(app, url)
	return m, tea.Batch(m.spinner.Tick, retryCmd)
}

// handleRetryResult processes the result of a retry attempt.
// Returns the updated model and whether the retry fully succeeded (caller should dismiss).
func (m cloneErrorModel) handleRetryResult(result cloneRetryResultMsg) cloneErrorModel {
	m.retrying = false

	if result.cloneErr != nil {
		// Clone failed again — update the error display.
		m.cloneErr = result.cloneErr
		m.textInput.SetValue(result.cloneErr.URL)
		m.postCloneErr = nil
		m.scrollOffset = 0
		return m
	}

	if result.postCloneErr != nil {
		// Clone succeeded but a post-clone step failed.
		// Keep the overlay visible showing the post-clone error.
		m.postCloneErr = result.postCloneErr
		m.scrollOffset = 0
		return m
	}

	// Full success — caller will dismiss the overlay.
	return m
}

func (m cloneErrorModel) buildRetryCmd(app *App, url string) tea.Cmd {
	switch m.origin {
	case retryOriginInstall:
		return m.retryInstallCmd(app, url)
	case retryOriginRegistryAdd:
		return m.retryRegistryAddCmd(app, url)
	}
	return nil
}

func (m cloneErrorModel) retryInstallCmd(app *App, url string) tea.Cmd {
	skill := m.installSkill
	folder := m.installFolder
	targetAgents := m.installTargetAgents

	return func() tea.Msg {
		// Parse the (possibly edited) URL to get a valid source.
		source, err := core.ParseSource(url)
		if err != nil {
			return cloneRetryResultMsg{
				origin:       retryOriginInstall,
				postCloneErr: fmt.Errorf("parsing source: %w", err),
				skillName:    skill.Skill.Name,
				folder:       folder,
			}
		}

		// Preserve SubPath and SkillName from the original registry source.
		// When the user edits the URL (e.g. HTTPS → SSH), ParseSource on
		// the raw URL loses the subpath context that the registry manifest
		// source string (e.g. "owner/repo/path/to/skill") originally had.
		if skill.Skill.Source != "" {
			orig, origErr := core.ParseSource(skill.Skill.Source)
			if origErr == nil {
				if source.SubPath == "" && orig.SubPath != "" {
					source.SubPath = orig.SubPath
				}
				if source.SkillName == "" && orig.SkillName != "" {
					source.SkillName = orig.SkillName
				}
				// If the user-edited URL produced no Owner/Repo (e.g. a bare
				// SSH URL), recover them from the original source so RepoKey
				// works for saving the override.
				if source.Owner == "" && orig.Owner != "" {
					source.Owner = orig.Owner
				}
				if source.Repo == "" && orig.Repo != "" {
					source.Repo = orig.Repo
				}
			}
		}

		installer := core.NewInstaller(app.agents)
		result, err := installer.InstallFromSource(source, core.InstallOptions{
			TargetDir:    folder,
			IsInternal:   true,
			TargetAgents: targetAgents,
		})
		if err != nil {
			// Check if this is a clone error (clone itself failed).
			if ce, ok := core.IsCloneError(err); ok {
				return cloneRetryResultMsg{
					origin:    retryOriginInstall,
					cloneErr:  ce,
					retryURL:  url,
					skillName: skill.Skill.Name,
					folder:    folder,
				}
			}
			// Clone succeeded but something after failed.
			return cloneRetryResultMsg{
				origin:       retryOriginInstall,
				postCloneErr: err,
				retryURL:     url,
				skillName:    skill.Skill.Name,
				folder:       folder,
			}
		}

		// Write lock file entries for installed skills (TUI always locks).
		for _, s := range result.InstalledSkills {
			if s.Commit != "" {
				entry := core.LockedSkill{
					Name:   s.Name,
					Source: s.Source,
					Commit: s.Commit,
					Ref:    s.Ref,
				}
				_ = core.AddOrUpdateLockEntry(folder, entry)
			}
		}

		// Full success — save clone URL override if the URL differs from
		// what ParseSource would normally produce for this repo.
		saveCloneURLOverride(app, source, url)

		return cloneRetryResultMsg{
			origin:    retryOriginInstall,
			retryURL:  url,
			skillName: skill.Skill.Name,
			folder:    folder,
		}
	}
}

func (m cloneErrorModel) retryRegistryAddCmd(app *App, url string) tea.Cmd {
	return func() tea.Msg {
		regMgr := core.NewRegistryManager(app.config.RegistriesDir())
		manifest, err := regMgr.Add(url)
		if err != nil {
			if ce, ok := core.IsCloneError(err); ok {
				return cloneRetryResultMsg{
					origin:   retryOriginRegistryAdd,
					cloneErr: ce,
					retryURL: url,
				}
			}
			return cloneRetryResultMsg{
				origin:       retryOriginRegistryAdd,
				postCloneErr: err,
				retryURL:     url,
			}
		}

		// Save registry to config.
		cfg, err := app.config.Load()
		if err != nil {
			return cloneRetryResultMsg{
				origin:       retryOriginRegistryAdd,
				postCloneErr: err,
				retryURL:     url,
			}
		}
		cfg.Registries = append(cfg.Registries, core.Registry{
			Name: manifest.Name,
			Repo: url,
		})
		if err := app.config.Save(cfg); err != nil {
			return cloneRetryResultMsg{
				origin:       retryOriginRegistryAdd,
				postCloneErr: err,
				retryURL:     url,
			}
		}

		// Save clone URL override — the repo URL from the registry entry
		// tells us the owner/repo. Parse it to get the RepoKey.
		source, parseErr := core.ParseSource(url)
		if parseErr == nil {
			saveCloneURLOverride(app, source, url)
		}

		return cloneRetryResultMsg{
			origin:       retryOriginRegistryAdd,
			retryURL:     url,
			registryName: manifest.Name,
		}
	}
}

// saveCloneURLOverride persists a clone URL override if the URL used for a
// successful clone differs from what ParseSource would normally produce.
// This is called after a successful retry with an edited URL so that future
// installs from the same repo use the working URL automatically.
func saveCloneURLOverride(app *App, source *core.ParsedSource, usedURL string) {
	repoKey := source.RepoKey()
	if repoKey == "" {
		return
	}

	// Only save if the URL is actually different from the default.
	// Re-parse the original source shorthand to see what the default would be.
	defaultURL := source.CloneURL
	if source.Owner != "" && source.Repo != "" {
		defaultSource, err := core.ParseSource(source.Owner + "/" + source.Repo)
		if err == nil {
			defaultURL = defaultSource.CloneURL
		}
	}

	if usedURL == defaultURL {
		return // No override needed — using the default URL.
	}

	// Best-effort save — don't fail the operation if this fails.
	_ = app.config.SaveCloneURLOverride(repoKey, usedURL)
}

func (m cloneErrorModel) view() string {
	if m.cloneErr == nil {
		return ""
	}

	var b strings.Builder

	// --- Retrying state: spinner + URL ---
	if m.retrying {
		b.WriteString("  ")
		b.WriteString(m.spinner.View())
		b.WriteString(" Cloning ")
		b.WriteString(normalItemStyle.Render(m.retryURL))
		b.WriteString("\n")
		return b.String()
	}

	// --- Post-clone error: clone succeeded, but something after failed ---
	if m.postCloneErr != nil {
		// Show clone success.
		b.WriteString("  ")
		b.WriteString(installedStyle.Render("Clone succeeded"))
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render("    " + m.retryURL))
		b.WriteString("\n\n")

		// Show the post-clone error.
		b.WriteString("  ")
		b.WriteString(errorStyle.Render("Post-clone error:"))
		b.WriteString("\n")
		for _, line := range strings.Split(m.postCloneErr.Error(), "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				b.WriteString("    ")
				b.WriteString(errorStyle.Render(line))
				b.WriteString("\n")
			}
		}
		b.WriteString("\n")

		// Hints for post-clone errors.
		b.WriteString(mutedStyle.Render("  Suggestions:"))
		b.WriteString("\n")
		b.WriteString("    ")
		b.WriteString(hintBulletStyle.Render("*"))
		b.WriteString(" ")
		b.WriteString(normalItemStyle.Render("Check that the repository contains SKILL.md files"))
		b.WriteString("\n")
		b.WriteString("    ")
		b.WriteString(hintBulletStyle.Render("*"))
		b.WriteString(" ")
		b.WriteString(normalItemStyle.Render("Skills must be at the repo root or in immediate subdirectories"))
		b.WriteString("\n")

		// Inline actions.
		b.WriteString("\n")
		b.WriteString(renderCloneErrorActions())

		return m.applyScroll(b.String())
	}

	// --- Clone error view: show error details + hints ---
	ce := m.cloneErr

	// Error kind.
	b.WriteString("  ")
	b.WriteString(errorStyle.Render(ce.Kind.String()))
	b.WriteString("\n\n")

	// Command that was run.
	b.WriteString(mutedStyle.Render("  Command:"))
	b.WriteString("\n")
	b.WriteString("    ")
	b.WriteString(normalItemStyle.Render(ce.Command))
	b.WriteString("\n\n")

	// Raw error output (may be multi-line).
	b.WriteString(mutedStyle.Render("  Error:"))
	b.WriteString("\n")
	for _, line := range strings.Split(ce.RawOutput, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			b.WriteString("    ")
			b.WriteString(errorStyle.Render(line))
			b.WriteString("\n")
		}
	}
	b.WriteString("\n")

	// Hints.
	if len(ce.Hints) > 0 {
		b.WriteString(mutedStyle.Render("  Suggestions:"))
		b.WriteString("\n")
		for _, hint := range ce.Hints {
			b.WriteString("    ")
			b.WriteString(hintBulletStyle.Render("*"))
			b.WriteString(" ")
			b.WriteString(normalItemStyle.Render(hint))
			b.WriteString("\n")
		}
	}

	// Edit mode.
	if m.editing {
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render("  Edit the clone URL and press Enter to retry:"))
		b.WriteString("\n")
		b.WriteString("  ")
		b.WriteString(m.textInput.View())
		b.WriteString("\n")
	} else {
		// Inline actions — visible call-to-action when not editing.
		b.WriteString("\n")
		b.WriteString(renderCloneErrorActions())
	}

	return m.applyScroll(b.String())
}

// applyScroll applies the scroll offset to rendered content.
func (m cloneErrorModel) applyScroll(content string) string {
	if m.scrollOffset > 0 {
		lines := strings.Split(content, "\n")
		if m.scrollOffset < len(lines) {
			content = strings.Join(lines[m.scrollOffset:], "\n")
		}
	}
	return content
}

// --- Messages ---

// cloneRetryResultMsg is sent when a retry from the clone error overlay completes.
// It carries the outcome: clone error, post-clone error, or full success.
type cloneRetryResultMsg struct {
	origin cloneRetryOrigin

	// If clone failed again, this is set.
	cloneErr *core.CloneError

	// If clone succeeded but a later step failed, this is set.
	postCloneErr error

	// The URL that was used for the retry.
	retryURL string

	// For install retries: context for reload.
	skillName string
	folder    string

	// For registry add retries: the registry name on success.
	registryName string
}

// registryAddDoneMsg is sent when a registry add completes (from settings, not retry).
type registryAddDoneMsg struct {
	url      string
	name     string
	warnings []string
	err      error
}

// hintBulletStyle styles the bullet point for hint items.
var hintBulletStyle = lipgloss.NewStyle().
	Foreground(colorWarning)

// hintKeyStyle styles inline key hints (e.g. "[e]") in the clone error view.
var hintKeyStyle = lipgloss.NewStyle().
	Foreground(colorSecondary).
	Bold(true)

// renderCloneErrorActions renders the inline call-to-action block.
func renderCloneErrorActions() string {
	var b strings.Builder
	b.WriteString("  ")
	b.WriteString(hintKeyStyle.Render("[e]"))
	b.WriteString(" ")
	b.WriteString(normalItemStyle.Render("Edit URL"))
	b.WriteString("   ")
	b.WriteString(hintKeyStyle.Render("[r]"))
	b.WriteString(" ")
	b.WriteString(normalItemStyle.Render("Retry"))
	b.WriteString("   ")
	b.WriteString(hintKeyStyle.Render("[esc]"))
	b.WriteString(" ")
	b.WriteString(normalItemStyle.Render("Back"))
	b.WriteString("\n")
	return b.String()
}
