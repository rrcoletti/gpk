package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"gpk/internal/board"
	"gpk/internal/gh"
)

// key builds a KeyMsg the way Update expects to receive it.
func key(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEscape}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func mockBoard(t *testing.T) BoardModel {
	t.Helper()
	status := gh.FieldDef{Name: "Status", Options: []gh.SelectOption{
		{ID: "o1", Name: "Todo"},
		{ID: "o2", Name: "Done"},
	}}
	items := []board.Item{
		{ID: "i0", Title: "no status card", Type: "Issue"}, // lands in "No Status"
		{ID: "i1", Title: "first card", Type: "Issue", Number: 4, OptionID: "o1"},
		{ID: "i2", Title: "second card", Type: "Issue", OptionID: "o2"},
	}
	m := NewBoardModel("test board", nil, "", "", "", status, nil)
	m.SetColumns(board.Build(status, items))
	return m
}

// step sends one key through Update and, when a cmd comes back, executes it
// and feeds the resulting msg back in (mimicking the bubbletea loop, so the
// test sees exactly what the real app would do).
func step(t *testing.T, m BoardModel, k string) BoardModel {
	t.Helper()
	up, cmd := m.Update(key(k))
	next := up.(BoardModel)
	if cmd == nil {
		return next
	}
	msg := cmd()
	if msg == nil {
		return next
	}
	up2, _ := next.Update(msg)
	return up2.(BoardModel)
}

func TestEscOnBoardRequestsBack(t *testing.T) {
	m := mockBoard(t)
	final := step(t, m, "esc")
	// esc quits the program; the Back flag must be set on the returned model
	if !final.Back() {
		t.Error("esc on board should set Back()=true")
	}
}

func TestQOnBoardDoesNotRequestBack(t *testing.T) {
	m := mockBoard(t)
	final := step(t, m, "q")
	if final.Back() {
		t.Error("q on board should NOT set Back()=true")
	}
}

func TestEditTitlePrefillHasNoExtraChar(t *testing.T) {
	// Regression: pressing 'e' used to append an 'e' to the pre-filled input.
	m := mockBoard(t)
	m.detail = true
	up, _ := m.Update(key("e"))
	m = up.(BoardModel)
	if !m.editing {
		t.Fatal("e should open the editor in detail view")
	}
	if got := m.input.Value(); got != "no status card" {
		t.Errorf("input = %q, want %q (no appended key echo)", got, "no status card")
	}
}

func TestEditSavesCleanTitle(t *testing.T) {
	// Saving the untouched prefill must not append "#4"-style decorations.
	m := mockBoard(t)
	m.detail = true
	up, _ := m.Update(key("e"))
	m = up.(BoardModel)
	if got := m.input.Value(); got != "no status card" {
		t.Fatalf("prefill = %q, want %q", got, "no status card")
	}
	m = step(t, m, "enter") // save unchanged: Update -> cmd -> titleEditedMsg
	if m.editing {
		t.Fatal("enter should close the editor")
	}
	card, ok := m.selectedCard()
	if !ok || card.Title != "no status card" {
		t.Fatalf("title after save = %+v", card)
	}
}

func TestMoveCardViaKeys(t *testing.T) {
	m := mockBoard(t)
	before := len(m.cards(1))
	m = step(t, m, "L") // move the no-status card right, into Todo
	if len(m.cards(0)) != 0 {
		t.Fatalf("source should be empty, got %v", m.cards(0))
	}
	if n := len(m.cards(1)); n != 2 {
		t.Fatalf("dest should hold 2 cards, got %d", n)
	}
	if got := m.cards(1)[1].ID; got != "i0" {
		t.Errorf("moved card should be at end of dest, got %s", got)
	}
	if m.colSelected != 1 {
		t.Fatalf("selection should follow the card, got col %d", m.colSelected)
	}
	_ = before
}

func TestMoveSingleKeyMovesOnce(t *testing.T) {
	// Regression: mock mode used to apply the move twice per keypress.
	m := mockBoard(t)
	m = step(t, m, "L") // i0: No Status -> Todo
	if got := len(m.cards(2)); got != 1 {
		t.Fatalf("Done column changed by a move into Todo: %d cards", got)
	}
	if len(m.cards(0)) != 0 || len(m.cards(1)) != 2 {
		t.Fatalf("unexpected distribution: %d/%d/%d",
			len(m.cards(0)), len(m.cards(1)), len(m.cards(2)))
	}
}

func TestClampOffsetScrollsColumns(t *testing.T) {
	m := mockBoard(t)
	up, _ := m.Update(tea.WindowSizeMsg{Width: 50, Height: 24})
	m = up.(BoardModel)
	for i := 0; i < 5; i++ {
		up, _ = m.Update(key("l"))
		m = up.(BoardModel)
	}
	if m.colSelected != 2 { // 3 columns total
		t.Fatalf("colSelected = %d, want 2", m.colSelected)
	}
	if m.colOffset > m.colSelected {
		t.Fatalf("offset %d beyond selected %d", m.colOffset, m.colSelected)
	}
}

func TestDetailOpenClose(t *testing.T) {
	m := mockBoard(t)
	up, _ := m.Update(key("enter"))
	m = up.(BoardModel)
	if !m.detail {
		t.Fatal("enter should open the detail view")
	}
	up, _ = m.Update(key("esc"))
	m = up.(BoardModel)
	if m.detail {
		t.Fatal("esc should close the detail view")
	}
	if m.Back() {
		t.Fatal("esc in detail must not trigger board-back")
	}
}

func TestColumnWidthFillsTerminal(t *testing.T) {
	// user rule: terminal width divided by column count; all columns visible
	// when each gets >= minColWidth, else show fewer with a peek sliver.
	status := gh.FieldDef{Name: "Status", Options: []gh.SelectOption{
		{ID: "o1", Name: "A"}, {ID: "o2", Name: "B"}, {ID: "o3", Name: "C"},
	}}
	items := []board.Item{{ID: "i1", Title: "a", OptionID: "o1"}}
	m := NewBoardModel("t", nil, "", "", "", status, nil)
	m.SetColumns(board.Build(status, items)) // 3 columns (No Status hidden: all items have a status)

	// border cost: each column box adds 2 cells, so usable = width - 2*n
	up, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 40})
	m = up.(BoardModel)
	if got := m.visibleColumnCount(); got != 3 {
		t.Fatalf("200: (200-6)/3 = 64 >= 22: all 3 should fit, got %d", got)
	}
	if got := m.columnWidth(); got != 64 {
		t.Fatalf("columnWidth = %d, want 64 ((200-6)/3)", got)
	}

	up, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m = up.(BoardModel)
	if got := m.visibleColumnCount(); got != 3 {
		t.Fatalf("100: (100-6)/3 = 31 >= 22: all 3 should fit, got %d", got)
	}
	if got := m.columnWidth(); got != 31 { // (100-6)/3
		t.Fatalf("columnWidth = %d, want 31", got)
	}

	// 72 cells: (72-6)/3 = 22 exactly: still fits
	up, _ = m.Update(tea.WindowSizeMsg{Width: 72, Height: 40})
	m = up.(BoardModel)
	if got := m.visibleColumnCount(); got != 3 {
		t.Fatalf("(72-6)/3 = 22: should still fit, got %d", got)
	}

	// 71 cells: 21 < 22 -> 2 visible, reserve 5 peek: (71-5-4)/2 = 31
	up, _ = m.Update(tea.WindowSizeMsg{Width: 71, Height: 40})
	m = up.(BoardModel)
	if got := m.visibleColumnCount(); got != 2 {
		t.Fatalf("want 2 visible at 71 cells, got %d", got)
	}
	if got := m.columnWidth(); got != 31 { // (71-5-4)/2
		t.Fatalf("columnWidth = %d, want 31", got)
	}
	v := m.View()
	if !strings.Contains(v, "← 1/3 →") {
		t.Errorf("scroll indicator missing while 1 of 3 columns shown:\n%s", v)
	}
}

func TestPeekSliverShown(t *testing.T) {
	status := gh.FieldDef{Name: "Status", Options: []gh.SelectOption{
		{ID: "o1", Name: "A"}, {ID: "o2", Name: "B"}, {ID: "o3", Name: "C"},
	}}
	items := []board.Item{{ID: "i1", Title: "a", OptionID: "o1"}}
	m := NewBoardModel("t", nil, "", "", "", status, nil)
	m.SetColumns(board.Build(status, items)) // 3 columns

	up, _ := m.Update(tea.WindowSizeMsg{Width: 50, Height: 40})
	m = up.(BoardModel)
	// (50-6)/3 = 14 < 22 -> 2 visible: (50-5-4)/2 = 20 < 22
	// -> 1 visible at (50-5-2) = 43 + 5 peek
	if got := m.visibleColumnCount(); got != 1 {
		t.Fatalf("want 1 visible, got %d", got)
	}
	v := m.View()
	if !strings.Contains(v, "← 1/3 →") {
		t.Errorf("expected 1/3 scroll indicator with peek:\n%s", v)
	}
	// the peek sliver must be exactly peekWidth wide: each row of the sliver
	// is 4 cells (first column line in view)
	for _, line := range strings.Split(v, "\n") {
		if strings.Contains(line, "╭") && !strings.Contains(line, "╮") {
			// a border line ending without a right corner = the peek column
			idx := strings.LastIndex(line, "╭")
			tail := line[idx:]
			if len([]rune(tail)) > peekWidth {
				t.Errorf("peek sliver is %d cells, want %d: %q", len([]rune(tail)), peekWidth, tail)
			}
			break
		}
	}
}

func TestAddItemFlow(t *testing.T) {
	m := mockBoard(t)
	// '+': open the add prompt
	up, _ := m.Update(key("+"))
	m = up.(BoardModel)
	if !m.adding {
		t.Fatal("+ should open the add prompt")
	}
	// type a title
	for _, r := range "new item" {
		up, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = up.(BoardModel)
	}
	if got := m.input.Value(); got != "new item" {
		t.Fatalf("input = %q", got)
	}
	// enter -> cmd -> itemAddedMsg applied by the loop
	up, cmd := m.Update(key("enter"))
	m = up.(BoardModel)
	if cmd == nil {
		t.Fatal("add should return a cmd")
	}
	msg := cmd()
	if msg == nil {
		t.Fatal("mock add cmd must produce a msg")
	}
	up2, _ := m.Update(msg)
	m = up2.(BoardModel)
	if m.adding {
		t.Fatal("add mode should close after success")
	}
	// new card appended to column 0 and selected
	if n := len(m.cards(0)); n != 2 {
		t.Fatalf("col 0 should have 2 cards, got %d", n)
	}
	if c := m.cards(0)[1]; c.Title != "new item" || c.Type != "DraftIssue" {
		t.Fatalf("added card wrong: %+v", c)
	}
	if m.cardSel[0] != 1 {
		t.Fatalf("new card should be selected, got %d", m.cardSel[0])
	}
}

func TestAddItemToNamedColumn(t *testing.T) {
	m := mockBoard(t)
	up, _ := m.Update(key("l")) // move to Todo column
	m = up.(BoardModel)
	up, _ = m.Update(key("+"))
	m = up.(BoardModel)
	for _, r := range "todo item" {
		up, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = up.(BoardModel)
	}
	up, cmd := m.Update(key("enter"))
	m = up.(BoardModel)
	msg := cmd()
	up2, _ := m.Update(msg)
	m = up2.(BoardModel)
	if n := len(m.cards(1)); n != 2 {
		t.Fatalf("Todo should have 2 cards, got %d", n)
	}
	if c := m.cards(1)[1]; c.Title != "todo item" {
		t.Fatalf("added card wrong: %+v", c)
	}
	if m.colSelected != 1 {
		t.Fatal("selection should follow the new card")
	}
}

func TestAddItemEmptyTitleIgnored(t *testing.T) {
	m := mockBoard(t)
	up, _ := m.Update(key("+"))
	m = up.(BoardModel)
	up, cmd := m.Update(key("enter"))
	m = up.(BoardModel)
	if m.adding != true || cmd != nil {
		// enter with empty input: cmd may be nil, mode stays until esc
		if cmd != nil {
			t.Fatal("empty title must not produce a cmd")
		}
		if len(m.cards(0)) != 1 {
			t.Fatal("no card should be added")
		}
	}
}

func TestDeleteItemConfirmFlow(t *testing.T) {
	m := mockBoard(t)
	// '-': opens confirmation, does NOT delete yet
	up, _ := m.Update(key("-"))
	m = up.(BoardModel)
	if !m.confirming {
		t.Fatal("- should open the delete confirmation")
	}
	if len(m.cards(0)) != 1 {
		t.Fatal("no deletion before confirmation")
	}
	// esc cancels
	up, _ = m.Update(key("esc"))
	m = up.(BoardModel)
	if m.confirming {
		t.Fatal("esc should cancel the confirmation")
	}
	if len(m.cards(0)) != 1 {
		t.Fatal("cancelled delete must not remove the card")
	}
	// now delete for real
	up, _ = m.Update(key("-"))
	m = up.(BoardModel)
	up, cmd := m.Update(key("enter"))
	m = up.(BoardModel)
	if cmd == nil {
		t.Fatal("confirm should return a cmd")
	}
	msg := cmd()
	if msg == nil {
		t.Fatal("mock delete cmd must produce a msg")
	}
	up2, _ := m.Update(msg)
	m = up2.(BoardModel)
	if m.confirming {
		t.Fatal("confirmation should clear after delete")
	}
	if len(m.cards(0)) != 0 {
		t.Fatalf("card should be deleted, got %d", len(m.cards(0)))
	}
}

func TestDeleteKeepsSaneSelection(t *testing.T) {
	m := mockBoard(t)
	// column 1 (Todo) holds one card, i1. Delete it; selection must clamp.
	up, _ := m.Update(key("l")) // to Todo
	m = up.(BoardModel)
	up, _ = m.Update(key("-")) // confirm opens
	m = up.(BoardModel)
	up, cmd := m.Update(key("enter")) // confirm
	m = up.(BoardModel)
	msg := cmd()
	up2, _ := m.Update(msg)
	m = up2.(BoardModel)
	if len(m.cards(1)) != 0 {
		t.Fatalf("Todo should be empty, got %d", len(m.cards(1)))
	}
	if m.cardSel[1] != 0 {
		t.Fatalf("selection should clamp to 0, got %d", m.cardSel[1])
	}
	if m.confirming {
		t.Fatal("confirming should be cleared")
	}
	// navigation still works after an empty column
	up, _ = m.Update(key("l"))
	m = up.(BoardModel)
	if m.colSelected != 2 || m.cardSel[2] != 0 {
		t.Fatalf("nav after empty column: col=%d sel=%d", m.colSelected, m.cardSel[2])
	}
}

func TestAddScreenDraftVsRepo(t *testing.T) {
	// board with no repos -> draft item
	status := gh.FieldDef{Name: "Status", Options: []gh.SelectOption{{ID: "o1", Name: "A"}}}
	items := []board.Item{{ID: "i1", Title: "a", Type: "Issue", OptionID: "o1"}}
	m := NewBoardModel("t", nil, "", "", "", status, nil)
	m.SetColumns(board.Build(status, items))
	up, _ := m.Update(key("+"))
	m = up.(BoardModel)
	if !m.adding || m.addRepo != "" {
		t.Fatalf("no repos on board: addRepo=%q adding=%v", m.addRepo, m.adding)
	}
	v := m.View()
	if !strings.Contains(v, "draft item") {
		t.Errorf("add screen should announce a draft:\n%s", v)
	}

	// board whose items all come from one repo -> real issue in that repo
	status2 := gh.FieldDef{Name: "Status", Options: []gh.SelectOption{{ID: "o1", Name: "A"}}}
	items2 := []board.Item{
		{ID: "i1", Title: "a", Type: "Issue", Repo: "rrcoletti/ca-go", OptionID: "o1"},
		{ID: "i2", Title: "b", Type: "Issue", Repo: "rrcoletti/ca-go", OptionID: "o1"},
	}
	m2 := NewBoardModel("t", nil, "", "", "", status2, nil)
	m2.SetColumns(board.Build(status2, items2))
	up2, _ := m2.Update(key("+"))
	m2 = up2.(BoardModel)
	if m2.addRepo != "rrcoletti/ca-go" {
		t.Fatalf("addRepo = %q, want rrcoletti/ca-go", m2.addRepo)
	}
	v2 := m2.View()
	if !strings.Contains(v2, "issue in rrcoletti/ca-go") {
		t.Errorf("add screen should announce the default repo:\n%s", v2)
	}

	// type a title, then create (mock mode, so locally)
	for _, r := range "new issue" {
		up2, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m2 = up2.(BoardModel)
	}
	up2, cmd2 := m2.Update(key("enter"))
	m2 = up2.(BoardModel)
	if cmd2 == nil {
		t.Fatal("create with a title should return a cmd")
	}
	msg := cmd2()
	up3, _ := m2.Update(msg)
	m2 = up3.(BoardModel)
	card := m2.cards(0)[len(m2.cards(0))-1]
	if card.Type != "Issue" || card.Repo != "rrcoletti/ca-go" {
		t.Fatalf("created card should be an issue in the default repo: %+v", card)
	}

	// mixed repos -> no default, back to draft
	items3 := []board.Item{
		{ID: "i1", Title: "a", Type: "Issue", Repo: "o/r1", OptionID: "o1"},
		{ID: "i2", Title: "b", Type: "Issue", Repo: "o/r2", OptionID: "o1"},
	}
	m3 := NewBoardModel("t", nil, "", "", "", status2, nil)
	m3.SetColumns(board.Build(status2, items3))
	up4, _ := m3.Update(key("+"))
	m3 = up4.(BoardModel)
	if m3.addRepo != "" {
		t.Fatalf("mixed repos must not yield a default, got %q", m3.addRepo)
	}
}

func TestAddRepoMenu(t *testing.T) {
	status := gh.FieldDef{Name: "Status", Options: []gh.SelectOption{{ID: "o1", Name: "A"}}}
	items := []board.Item{
		{ID: "i1", Title: "a", Type: "Issue", Repo: "rrcoletti/mac", OptionID: "o1"},
		{ID: "i2", Title: "b", Type: "Issue", Repo: "rrcoletti/other", OptionID: "o1"},
	}
	m := NewBoardModel("t", nil, "", "", "", status, nil)
	m.SetColumns(board.Build(status, items))
	up, _ := m.Update(key("+"))
	m = up.(BoardModel)
	if !m.ChoosingRepo() {
		t.Fatal("mixed-repo board should open the repo menu")
	}
	v := m.View()
	if !strings.Contains(v, "rrcoletti/mac") || !strings.Contains(v, "rrcoletti/other") {
		t.Errorf("repo menu should list the board's repos:\n%s", v)
	}
	// pick the second repo
	up, _ = m.Update(key("j"))
	m = up.(BoardModel)
	up, _ = m.Update(key("enter"))
	m = up.(BoardModel)
	if m.ChoosingRepo() {
		t.Fatal("enter should close the repo menu")
	}
	if m.DefaultRepo() != "rrcoletti/other" {
		t.Fatalf("chosen repo = %q", m.DefaultRepo())
	}
	if !m.Adding() {
		t.Fatal("input should be focused after choosing a repo")
	}
	// type title, create -> real issue in the chosen repo
	for _, r := range "created in other" {
		up, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = up.(BoardModel)
	}
	up, cmd := m.Update(key("enter"))
	m = up.(BoardModel)
	if cmd == nil {
		t.Fatal("create should return a cmd")
	}
	msg := cmd()
	up2, _ := m.Update(msg)
	m = up2.(BoardModel)
	card := m.cards(0)[len(m.cards(0))-1]
	if card.Type != "Issue" || card.Repo != "rrcoletti/other" {
		t.Fatalf("created card wrong: %+v", card)
	}
}

func TestAddSingleRepoSkipsMenu(t *testing.T) {
	// single-repo board: no menu, straight to the input
	status := gh.FieldDef{Name: "Status", Options: []gh.SelectOption{{ID: "o1", Name: "A"}}}
	items := []board.Item{
		{ID: "i1", Title: "a", Type: "Issue", Repo: "rrcoletti/mac", OptionID: "o1"},
	}
	m := NewBoardModel("t", nil, "", "", "", status, nil)
	m.SetColumns(board.Build(status, items))
	up, _ := m.Update(key("+"))
	m = up.(BoardModel)
	if m.ChoosingRepo() {
		t.Fatal("single-repo board must not show the repo menu")
	}
	if m.DefaultRepo() != "rrcoletti/mac" {
		t.Fatalf("inferred repo = %q", m.DefaultRepo())
	}
}

func TestHelpOverlay(t *testing.T) {
	m := mockBoard(t)
	up, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 25})
	m = up.(BoardModel)

	up, cmd := m.Update(key("?"))
	m = up.(BoardModel)
	if cmd != nil {
		t.Fatalf("? should not return a cmd")
	}
	v := m.View()
	if !strings.Contains(v, "Commands") || !strings.Contains(v, "move in column") {
		t.Errorf("help overlay not shown:\n%s", v)
	}
	// overlay eats navigation keys
	up, _ = m.Update(key("l"))
	m = up.(BoardModel)
	if !strings.Contains(m.View(), "Commands") {
		t.Error("overlay should stay open on other keys")
	}

	// esc closes; in app mode it must not emit backMsg
	up, cmd = m.Update(key("esc"))
	m = up.(BoardModel)
	if cmd != nil {
		t.Errorf("esc while helping must not emit backMsg, got %v", cmd)
	}
	if strings.Contains(m.View(), "Commands") {
		t.Error("overlay should be closed after esc")
	}
}
