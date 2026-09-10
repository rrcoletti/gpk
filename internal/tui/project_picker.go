// Package tui holds the Bubble Tea screens: project picker now, board later.
package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"gpk/internal/gh"
)

var (
	pickerTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	pickerDimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	pickerSelStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	pickerErrStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))
)

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
	chosen   bool   // true after Enter
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

// Selected returns the chosen project; ok is false if the user quit
// without selecting anything.
func (m PickerModel) Selected() (gh.Project, bool) {
	if !m.chosen || m.err != nil || m.selected >= len(m.projects) {
		return gh.Project{}, false
	}
	return m.projects[m.selected], true
}

func (m PickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "up", "k":
			if m.selected > 0 {
				m.selected--
			}
		case "down", "j":
			if m.selected < len(m.projects)-1 {
				m.selected++
			}
		case "enter":
			if !m.loading && m.err == nil && m.selected < len(m.projects) {
				m.chosen = true
				return m, tea.Quit
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
	var b strings.Builder

	b.WriteString(pickerTitleStyle.Render("Select a project"))
	b.WriteString("\n")

	if m.err != nil {
		b.WriteString("\n")
		b.WriteString(pickerErrStyle.Render("Error: " + m.err.Error()))
		b.WriteString("\n\n")
		b.WriteString(pickerDimStyle.Render("press q to quit"))
		return b.String()
	}

	if m.loading && len(m.projects) == 0 {
		b.WriteString("\n" + m.spinner.View() + " loading projects...\n")
		return b.String()
	}

	b.WriteString("\n")
	for i, p := range m.projects {
		line := fmt.Sprintf("  #%d  %s", p.Number, p.Title)
		if p.Closed {
			line += " " + pickerDimStyle.Render("(closed)")
		}
		if i == m.selected {
			b.WriteString(pickerSelStyle.Render("> " + line))
		} else {
			b.WriteString("  " + line)
		}
		b.WriteString("\n")
	}

	if m.cursor != "" {
		b.WriteString(pickerDimStyle.Render("\n  more pages exist (pagination UI comes with the board)"))
	}

	b.WriteString("\n" + pickerDimStyle.Render("j/k move · enter select · q quit"))
	return b.String()
}
