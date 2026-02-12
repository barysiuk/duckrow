package tui

import "github.com/charmbracelet/lipgloss"

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

	// Skill name in content.
	skillNameStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#F3F4F6"))

	// Skill version.
	skillVersionStyle = lipgloss.NewStyle().
				Foreground(colorMuted)

	// Viewport overlay (SKILL.md preview).
	viewportTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(colorPrimary).
				Padding(0, 1)

	// Spinner style.
	spinnerStyle = lipgloss.NewStyle().
			Foreground(colorSecondary)

	// Section header rule (the ─── line after the label).
	sectionRuleStyle = lipgloss.NewStyle().
				Foreground(colorBorder)
)

// renderSectionHeader renders a section label with short rules on both sides:
// "  ── SKILLS ──────"
func renderSectionHeader(label string, _ int) string {
	rule := sectionRuleStyle.Render("──")
	text := sectionHeaderStyle.Render(" " + label + " ")
	return "  " + rule + text + rule
}
