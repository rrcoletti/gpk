package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"gpk/internal/board"
	"gpk/internal/gh"
)

// RunMockBoard shows the board UI with sample data. Lets the layout and
// keybindings be reviewed before wiring live data (milestone 4).
func RunMockBoard(user string) error {
	status := gh.FieldDef{
		Name: "Status",
		Options: []gh.SelectOption{
			{ID: "o1", Name: "Backlog", Color: "GRAY"},
			{ID: "o2", Name: "Todo", Color: "YELLOW"},
			{ID: "o3", Name: "In Progress", Color: "BLUE"},
			{ID: "o4", Name: "In Review", Color: "PURPLE"},
			{ID: "o5", Name: "Done", Color: "GREEN"},
		},
	}
	items := []board.Item{
		{ID: "i1", Title: "Fix flaky test in CI pipeline", Type: "Issue", Number: 142, Assignee: "rrcoletti", OptionID: "o3"},
		{ID: "i2", Title: "Add pagination to the items GraphQL query", Type: "Issue", Number: 138, OptionID: "o2"},
		{ID: "i3", Title: "board: horizontal scroll jumps past last column", Type: "Issue", Number: 137, Assignee: "rrcoletti", OptionID: "o4"},
		{ID: "i4", Title: "Investigate token refresh edge case", Type: "Issue", Number: 129, OptionID: "o1"},
		{ID: "i5", Title: "Document keybindings in README", Type: "Issue", Number: 121, OptionID: "o5"},
		{ID: "i6", Title: "Refactor styles into a theme file", Type: "Issue", Number: 118, OptionID: "o1"},
		{ID: "i7", Title: "Support org projects (v1)", Type: "DraftIssue", OptionID: "o2"},
		{ID: "i8", Title: "Raw item with no status yet", Type: "Issue", Number: 99},
		{ID: "i8b", Title: "Unknown status option item", Type: "Issue", Number: 98, OptionID: "zzz"},
		{ID: "i9", Title: "Draft: spike keyboard-driven column reordering", Type: "DraftIssue"},
	}

	m := NewBoardModel(user + " — MOCK DATA (no live items yet)")
	m.SetColumns(board.Build(status, items))
	_, err := tea.NewProgram(m).Run()
	return err
}
