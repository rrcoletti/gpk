package tui

// Shared visual language for every screen: one source of truth for colors,
// borders, header/footer bars and selection styling.

import "github.com/charmbracelet/lipgloss"

// Accent and neutral palette (256-color).
const (
	colorAccent = "205" // selection / focus
	colorDim    = "241"
	colorBorder = "238"
	colorError  = "196"
	colorInfo   = "39" // numbers, links
	colorRow    = "250"
	colorSelBg  = "236"
)

var (
	// themeTitle is the header/title bar style, shared by picker and board.
	themeTitle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorAccent))

	// themeDim is secondary info: footers, counts, hints.
	themeDim = lipgloss.NewStyle().Foreground(lipgloss.Color(colorDim))

	// themeErr is error toasts/messages.
	themeErr = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorError))

	// themeList is the rounded bordered pane both screens use.
	themeList = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colorBorder))

	// themeSelList is the focused pane's border.
	themeSelList = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colorAccent))

	// themeSelRow highlights the selected row across the full width.
	themeSelRow = lipgloss.NewStyle().Bold(true).
			Foreground(lipgloss.Color(colorAccent)).
			Background(lipgloss.Color(colorSelBg))

	// themeNum tints numbers (issue/project numbers, URLs).
	themeNum = lipgloss.NewStyle().Foreground(lipgloss.Color(colorInfo))

	// themeCard is normal card text.
	themeCard = lipgloss.NewStyle().Foreground(lipgloss.Color(colorRow))
)
