package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"gpk/internal/gh"
)

func init() { User = "user" }

func TestPickerFullWindowLayout(t *testing.T) {
	m := NewPickerModel(nil)
	var ps []gh.Project
	for i := 1; i <= 60; i++ {
		ps = append(ps, gh.Project{ID: "p", Number: i, Title: "project"})
	}
	m.projects = ps
	m.loading = false
	up, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = up.(PickerModel)

	inner := m.listHeight()
	if inner >= 60 {
		t.Fatalf("listHeight %d should be smaller than 60 items", inner)
	}

	// scroll down past the viewport: viewport must follow
	for i := 0; i < 30; i++ {
		up, _ = m.Update(key("j"))
		m = up.(PickerModel)
	}
	if m.selected != 30 {
		t.Fatalf("selected=%d", m.selected)
	}
	if m.selected < m.top || m.selected >= m.top+inner {
		t.Fatalf("selection %d outside viewport [%d,%d)", m.selected, m.top, m.top+inner)
	}
	v := m.View()
	if !strings.Contains(v, "↑ ") || !strings.Contains(v, "of 60") {
		t.Errorf("scroll indicator wrong:\n%s", v)
	}
	// full-window frame must be present
	if !strings.Contains(v, " gpk dev · user's GitHub · 60 project(s)") {
		t.Error("missing header")
	}
	if !strings.Contains(v, "q or Esc: quit") {
		t.Error("missing footer")
	}
}

func TestPickerSmallTerminal(t *testing.T) {
	m := NewPickerModel(nil)
	m.projects = []gh.Project{{ID: "p", Number: 1, Title: "one"}}
	m.loading = false
	up, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 8})
	m = up.(PickerModel)
	v := m.View()
	if !strings.Contains(v, " gpk dev · user's GitHub · 1 project(s)") || !strings.Contains(v, "one") {
		t.Errorf("small-terminal view broken:\n%s", v)
	}
}
