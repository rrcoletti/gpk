// AppModel is the root Bubble Tea model: one long-lived program hosting the
// picker and board screens. Screens switch in-model, so transitions don't
// tear down the app, don't flicker, and don't re-fetch the project list.
package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"gpk/internal/board"
	"gpk/internal/gh"
)

type AppScreen int

const (
	ScreenPicker AppScreen = iota
	ScreenBoard
)

// AppModel owns the picker, the currently shown board, and a cache of opened
// boards so re-entering a project is instant (stale data shown, then
// background-refreshed).
type AppModel struct {
	client       *gh.Client
	screen       AppScreen
	picker       PickerModel
	board        *BoardModel
	boards       map[string]*BoardModel
	project      gh.Project
	err          error
	lastW, lastH int // last known terminal size, for boards created later
}

// NewAppModel creates the shell; the picker starts fetching right away.
func NewAppModel(client *gh.Client) AppModel {
	return AppModel{
		client: client,
		screen: ScreenPicker,
		picker: NewPickerModel(client),
		boards: map[string]*BoardModel{},
	}
}

func (m AppModel) Init() tea.Cmd {
	return m.picker.Init()
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.lastW, m.lastH = msg.Width, msg.Height
		var cmds []tea.Cmd
		up, c := m.picker.Update(msg)
		m.picker = up.(PickerModel)
		cmds = append(cmds, c)
		if m.board != nil {
			up2, c2 := m.board.Update(msg)
			bm := up2.(BoardModel)
			m.board = &bm
			cmds = append(cmds, c2)
		}
		return m, tea.Batch(cmds...)

	case tea.KeyMsg:
		if m.screen == ScreenPicker {
			up, c := m.picker.Update(msg)
			m.picker = up.(PickerModel)
			return m, c
		}
		if m.board != nil {
			up, c := m.board.Update(msg)
			bm := up.(BoardModel)
			m.board = &bm
			return m, c
		}
		return m, nil

	case ProjectPickedMsg:
		m.project = msg.Project
		if b, ok := m.boards[msg.Project.ID]; ok {
			// instant re-entry: cached board, data refreshes in background
			m.board = b
			m.screen = ScreenBoard
			return m, tea.Batch(b.doRefetch(), b.refreshTick())
		}
		b := NewBoardModel(msg.Project.Title,
			m.client, msg.Project.ID, "", "", gh.FieldDef{}, nil)
		b.appMode = true
		b.loading = true
		b.applySize(m.lastW, m.lastH)
		m.board = &b
		m.screen = ScreenBoard
		return m, tea.Batch(b.spinner.Tick, m.loadBoardData(msg.Project))

	case backMsg:
		m.screen = ScreenPicker
		return m, nil

	case boardDataMsg:
		b := NewBoardModel(m.project.Title,
			m.client, m.project.ID, msg.status.ID, msg.titleField.ID, msg.status, m.refetchFor(m.project, msg.status))
		b.appMode = true
		b.projectDefaultRepo = msg.defaultRepo
		b.applySize(m.lastW, m.lastH)
		b.SetColumns(board.Build(msg.status, msg.items))
		m.boards[m.project.ID] = &b
		m.board = &b
		return m, nil

	case boardDataErrMsg:
		m.err = msg.err
		m.screen = ScreenPicker
		m.picker.err = msg.err // surfaced in the picker's error state
		return m, nil
	}

	// everything else goes to the active screen
	if m.screen == ScreenBoard && m.board != nil {
		up, c := m.board.Update(msg)
		bm := up.(BoardModel)
		m.board = &bm
		return m, c
	}
	up, c := m.picker.Update(msg)
	m.picker = up.(PickerModel)
	return m, c
}

func (m AppModel) View() string {
	if m.screen == ScreenBoard && m.board != nil {
		return m.board.View()
	}
	return m.picker.View()
}

type boardDataMsg struct {
	status      gh.FieldDef
	titleField  gh.FieldDef
	items       []board.Item
	defaultRepo string // project settings' default repository, "" = none
}

type boardDataErrMsg struct{ err error }

// loadBoardData fetches everything a board needs, off the UI loop.
func (m AppModel) loadBoardData(p gh.Project) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		fields, err := client.GetProjectFields(context.Background(), p.ID)
		if err != nil {
			return boardDataErrMsg{err}
		}
		status, ok := gh.StatusField(fields)
		if !ok {
			return boardDataErrMsg{fmt.Errorf("project %q has no single-select field to use as columns", p.Title)}
		}
		titleField, _ := gh.TitleField(fields)
		repos, err := client.ProjectRepositories(context.Background(), p.ID)
		if err != nil {
			return boardDataErrMsg{err}
		}
		items, err := client.FetchAllItems(context.Background(), p.ID, status.ID)
		if err != nil {
			return boardDataErrMsg{err}
		}
		defRepo := ""
		if len(repos) > 0 {
			defRepo = repos[0] // the project settings' default repository
		}
		return boardDataMsg{status: status, titleField: titleField,
			items: board.FromGhItems(items), defaultRepo: defRepo}
	}
}

// refetchFor returns a closure the board uses for background refresh.
func (m AppModel) refetchFor(p gh.Project, status gh.FieldDef) func() ([]board.Item, error) {
	client := m.client
	return func() ([]board.Item, error) {
		items, err := client.FetchAllItems(context.Background(), p.ID, status.ID)
		if err != nil {
			return nil, err
		}
		return board.FromGhItems(items), nil
	}
}

// RunApp runs the full picker+board application in one Bubble Tea program.
func RunApp(ctx context.Context, client *gh.Client) error {
	p := tea.NewProgram(NewAppModel(client))
	_, err := p.Run()
	if err != nil {
		return fmt.Errorf("app: %w", err)
	}
	return nil
}
