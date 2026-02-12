package tui

import (
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
)

// Color palette.
var (
	colorPrimary   = lipgloss.Color("#7C3AED") // Purple
	colorSecondary = lipgloss.Color("#A78BFA") // Light purple
	colorSuccess   = lipgloss.Color("#10B981") // Green (installed)
	colorDanger    = lipgloss.Color("#EF4444") // Red (errors)
	colorMuted     = lipgloss.Color("#6B7280") // Gray
	colorBorder    = lipgloss.Color("#374151") // Dark gray
	colorWarning   = lipgloss.Color("#F59E0B") // Amber
)

// Shared styles used across TUI views.
var (
	// Header bar: "DuckRow  ~/code/my-app"
	logoStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(colorPrimary).
			Padding(0, 1)

	headerPathStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#F3F4F6")).
			Padding(0, 1)

	headerHintStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	// Main content area.
	contentStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(1, 2)

	// Section header within a panel (e.g. "INSTALLED", "AVAILABLE").
	// NOTE: No MarginBottom — use explicit \n in view functions for predictable height.
	sectionHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorMuted)

	// Selected item in a list.
	selectedItemStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorPrimary)

	// Normal (unselected) item in a list.
	normalItemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#D1D5DB"))

	// Muted text (descriptions, secondary info).
	mutedStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	// Badge for skill counts.
	badgeStyle = lipgloss.NewStyle().
			Foreground(colorSecondary)

	// Installed / success indicator.
	installedStyle = lipgloss.NewStyle().
			Foreground(colorSuccess)

	// Error text.
	errorStyle = lipgloss.NewStyle().
			Foreground(colorDanger)

	// Warning / banner text.
	warningStyle = lipgloss.NewStyle().
			Foreground(colorWarning)

	// Help text at the bottom.
	helpStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	// Skill version (used in registry skill items).
	skillVersionStyle = lipgloss.NewStyle().
				Foreground(colorMuted)

	// Viewport overlay (SKILL.md preview).
	viewportTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#D1D5DB")).
				Background(colorBorder).
				Padding(0, 1)

	// Preview scroll percentage badge.
	previewPctStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#D1D5DB")).
			Background(colorBorder)

	// Spinner style.
	spinnerStyle = lipgloss.NewStyle().
			Foreground(colorSecondary)

	// Section header rule (the ─── line after the label).
	sectionRuleStyle = lipgloss.NewStyle().
				Foreground(colorBorder)

	// Confirmation dialog.
	dialogBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorPrimary).
			Padding(1, 2)

	dialogButtonStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFF7DB")).
				Background(colorMuted).
				Padding(0, 2)

	dialogActiveButtonStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFF7DB")).
				Background(colorDanger).
				Padding(0, 2).
				Bold(true)
)

// renderSectionHeader renders a section label with short rules on both sides:
// "  ── SKILLS ──────"
func renderSectionHeader(label string, _ int) string {
	rule := sectionRuleStyle.Render("──")
	text := sectionHeaderStyle.Render(" " + label + " ")
	return "  " + rule + text + rule
}

// newSkillDelegate creates a DefaultDelegate styled to match the DuckRow theme.
// Uses the fancy list pattern: vertical bar for selection, title + description
// on two lines, filter match highlighting.
func newSkillDelegate() list.DefaultDelegate {
	d := list.NewDefaultDelegate()

	d.Styles.NormalTitle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#F3F4F6")).
		Padding(0, 0, 0, 2)

	d.Styles.NormalDesc = lipgloss.NewStyle().
		Foreground(colorMuted).
		Padding(0, 0, 0, 2)

	d.Styles.SelectedTitle = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(colorPrimary).
		Foreground(colorSecondary).
		Bold(true).
		Padding(0, 0, 0, 1)

	d.Styles.SelectedDesc = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(colorPrimary).
		Foreground(colorMuted).
		Padding(0, 0, 0, 1)

	d.Styles.DimmedTitle = lipgloss.NewStyle().
		Bold(true).
		Foreground(colorMuted).
		Padding(0, 0, 0, 2)

	d.Styles.DimmedDesc = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#4D4D4D")).
		Padding(0, 0, 0, 2)

	d.SetSpacing(1)

	return d
}
