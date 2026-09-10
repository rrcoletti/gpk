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
		{ID: "i1", Title: "Fix flaky test in CI pipeline", Type: "Issue", Number: 142, Assignee: "rrcoletti", OptionID: "o3",
			Repo: "rrcoletti/gpk", URL: "https://github.com/rrcoletti/gpk/issues/142",
			Body: "The pipeline fails intermittently on the arm runner when the cache is cold.\n\nReproduces about one in five runs. Steps:\n\n1. Clear the runner cache\n2. Run the full test suite\n3. Watch the artifact upload step time out\n\nSuspect a race between the upload and the teardown hook."},
		{ID: "i2", Title: "Add pagination to the items GraphQL query", Type: "Issue", Number: 138, OptionID: "o2",
			Repo: "rrcoletti/other", URL: "https://github.com/rrcoletti/other/issues/138",
			Body: "Large projects only show their first page of items. The query already accepts a cursor; wire it through and load more when the user reaches the end of a column."},
		{ID: "i3", Title: "board: horizontal scroll jumps past last column", Type: "Issue", Number: 137, Assignee: "rrcoletti", OptionID: "o4",
			Repo: "rrcoletti/gpk", URL: "https://github.com/rrcoletti/gpk/issues/137",
			Body: "Pressing L repeatedly scrolls the viewport so the last column sits on the far left with empty space on the right. The clamp should stop at the last column."},
		{ID: "i4", Title: "Investigate token refresh edge case", Type: "Issue", Number: 129, OptionID: "o1",
			Repo: "rrcoletti/gpk", URL: "https://github.com/rrcoletti/gpk/issues/129",
			Body: "Device flow tokens do not expire, but a revoked token fails silently until the next API call. Consider probing auth on startup."},
		{ID: "i5", Title: "Document keybindings in README", Type: "Issue", Number: 121, OptionID: "o5",
			Repo: "rrcoletti/gpk", URL: "https://github.com/rrcoletti/gpk/issues/121", Body: "Done as part of the first documentation pass."},
		{ID: "i6", Title: "Refactor styles into a theme file", Type: "Issue", Number: 118, OptionID: "o1",
			Repo: "rrcoletti/gpk", URL: "https://github.com/rrcoletti/gpk/issues/118",
			Body: "Colors are scattered across picker and board files. Collect them in one theme.go so a future config file can override them."},
		{ID: "i7", Title: "Support org projects (v1)", Type: "DraftIssue", OptionID: "o2",
			Body: "The owner resolution already separates viewer from orgs. Expose org projects in the picker once permissions are clear."},
		{ID: "i8", Title: "Raw item with no status yet", Type: "Issue", Number: 99,
			Repo: "rrcoletti/gpk", Body: "Just added to the project, nobody set a status yet."},
		{ID: "i8b", Title: "Unknown status option item", Type: "Issue", Number: 98, OptionID: "zzz",
			Repo: "rrcoletti/gpk", Body: "Its option was deleted from the Status field; the board parks it in No Status."},
		{ID: "i9", Title: "Draft: spike keyboard-driven column reordering", Type: "DraftIssue",
			Body: "Ideas: shift+H/L to move whole columns. Unclear if the GraphQL API supports field option ordering; needs a spike."},
	}

	m := NewBoardModel(user+" — MOCK DATA (moves and edits are local only)", nil, "", "", "", gh.FieldDef{}, nil)
	m.SetColumns(board.Build(status, items))
	_, err := tea.NewProgram(m).Run()
	return err
}
