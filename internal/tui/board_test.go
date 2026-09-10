package tui

import (
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
