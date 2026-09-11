// Package tui holds the Bubble Tea screens: project picker now, board later.
package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"gpk/internal/gh"
)

var (
	pickerTitleStyle = themeTitle
	pickerDimStyle   = themeDim
	pickerErrStyle   = themeErr
	pickerListStyle  = themeList
	pickerNumStyle   = themeNum
)

// ProjectPickedMsg is emitted when the user selects a project.
type ProjectPickedMsg struct {
	Project gh.Project
}

// ProjectsLoadedMsg carries the fetched project list.
type ProjectsLoadedMsg struct {
	Projects []gh.Project
	Cursor   string // next page cursor, "" if no more
	HasMore  bool
}

// projectsLoadedMsg wraps an error from the fetch command.
type projectsLoadErrMsg struct{ err error }

// PickerModel lets the user choose one of the viewer's projects.
type PickerModel struct {
	client *gh.Client

	spinner  spinner.Model
	projects []gh.Project
	cursor   string // next page cursor, "" when exhausted
	selected int    // index into projects
	top      int    // first visible row (scrolling viewport)
	helping  bool   // command overlay open (toggled with ?)
	loading  bool
	err      error

	width  int
	height int
}

// NewPickerModel creates the picker and kicks off the first page fetch.
func NewPickerModel(client *gh.Client) PickerModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = pickerTitleStyle
	return PickerModel{client: client, spinner: sp, loading: true}
}

// Init starts the project fetch.
func (m PickerModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.fetchPage(""))
}

func (m PickerModel) fetchPage(cursor string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		projects, next, err := client.ListViewerProjects(context.Background(), 50, cursor)
		if err != nil {
			return projectsLoadErrMsg{err}
		}
		return ProjectsLoadedMsg{Projects: projects, Cursor: next, HasMore: next != ""}
	}
}

func (m PickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case tea.KeyMsg:
		if m.helping {
			// overlay eats everything except close/quit
			switch msg.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "esc", "?":
				m.helping = false
			}
			return m, nil
		}
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit
		case "?":
			m.helping = true
		case "up", "k":
			if m.selected > 0 {
				m.selected--
			}
			m.clamp()
		case "down", "j":
			if m.selected < len(m.projects)-1 {
				m.selected++
			}
			m.clamp()
		case "home", "g":
			m.selected = 0
			m.clamp()
		case "end", "G":
			m.selected = len(m.projects) - 1
			m.clamp()
		case "enter":
			if !m.loading && m.err == nil && m.selected < len(m.projects) {
				return m, func() tea.Msg {
					return ProjectPickedMsg{Project: m.projects[m.selected]}
				}
			}
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case ProjectsLoadedMsg:
		m.loading = false
		m.projects = append(m.projects, msg.Projects...)
		m.cursor = msg.Cursor

	case projectsLoadErrMsg:
		m.loading = false
		m.err = msg.err
	}

	return m, nil
}

func (m PickerModel) View() string {
	if m.err != nil {
		return m.renderError()
	}
	if m.loading && len(m.projects) == 0 {
		return m.renderShell("\n " + m.spinner.View() + " loading projects...")
	}
	if len(m.projects) == 0 {
		return m.renderShell(pickerDimStyle.Render("no projects found for your account"))
	}

	inner := m.listHeight()
	if m.selected < m.top {
		m.top = m.selected
	}
	if m.selected >= m.top+inner {
		m.top = m.selected - inner + 1
	}
	if m.top < 0 {
		m.top = 0
	}
	end := m.top + inner
	if end > len(m.projects) {
		end = len(m.projects)
	}

	var rows []string
	for i := m.top; i < end; i++ {
		rows = append(rows, m.renderRow(i))
	}

	scroll := ""
	if m.top > 0 || end < len(m.projects) {
		scroll = pickerDimStyle.Render(fmt.Sprintf(" \u2191 %d-%d of %d \u2193", m.top+1, end, len(m.projects)))
	}

	var content string
	if pad := inner - (end - m.top); pad > 0 {
		content = strings.Join(rows, "\n") + strings.Repeat("\n", pad)
	} else {
		content = strings.Join(rows, "\n")
	}

	list := pickerListStyle.Width(m.innerWidth()).Height(inner).Render(content)
	foot := pickerDimStyle.Render(" ?: commands · q or Esc: quit")
	view := m.header() + "\n" + scroll + "\n" + list + "\n" + foot
	if m.helping {
		return overlayCenter(view, helpBox([][2]string{
			{"k / ↑", "previous project"},
			{"j / ↓", "next project"},
			{"g", "first project"},
			{"G", "last project"},
			{},
			{"Enter", "open project board"},
		}), m.width)
	}
	return view
}

// renderRow draws one project row, padded to the full pane width,
// highlighted when selected.
func (m PickerModel) renderRow(i int) string {
	p := m.projects[i]
	prefix := "  " // aligns non-selected rows with the selected row's "> "
	if i == m.selected {
		prefix = "> "
	}
	line := prefix + p.Title
	if p.Closed {
		line += "  (closed)"
	}
	w := m.innerWidth() - 2
	for lipgloss.Width(line) < w {
		line += " "
	}
	if i == m.selected {
		return pickerSelStyle.Render(line)
	}
	if p.Closed {
		return pickerDimStyle.Render(line)
	}
	return line
}

// clamp keeps the selected row inside the visible viewport.
func (m *PickerModel) clamp() {
	inner := m.listHeight()
	if m.selected < m.top {
		m.top = m.selected
	}
	if m.selected >= m.top+inner {
		m.top = m.selected - inner + 1
	}
	if m.top < 0 {
		m.top = 0
	}
	if m.top+inner > len(m.projects) {
		m.top = len(m.projects) - inner
	}
	if m.top < 0 {
		m.top = 0
	}
}

// header is the top bar: app name, project count, pagination hint.
func (m PickerModel) header() string {
	t := fmt.Sprintf("%s's GitHub · %d project(s)", User, len(m.projects))
	if m.cursor != "" {
		t += " · more available"
	}
	return themeWhite.Render(fmt.Sprintf(" gpk %s", Version)) + pickerTitleStyle.Render(" · "+t)
}

// listHeight is the number of visible rows for the current terminal size.
func (m PickerModel) listHeight() int {
	if m.height == 0 {
		return 10
	}
	h := m.height - 5 // header + scroll + borders + footer
	if h < 3 {
		h = 3
	}
	return h
}

func (m PickerModel) innerWidth() int {
	w := m.width - 2 // borders
	if m.width == 0 {
		w = 76
	}
	if w < 20 {
		w = 20
	}
	return w
}

// renderShell is the full-window frame around arbitrary content.
func (m PickerModel) renderShell(inner string) string {
	list := pickerListStyle.Width(m.innerWidth()).Height(m.listHeight()).Render(inner)
	foot := pickerDimStyle.Render(" q or Esc: quit")
	return m.header() + "\n\n" + list + "\n" + foot
}

func (m PickerModel) renderError() string {
	return m.renderShell("\n " + pickerErrStyle.Render("Error: "+m.err.Error()))
}
