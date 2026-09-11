package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"gpk/internal/auth"
	"gpk/internal/board"
	"gpk/internal/gh"
)

const (
	minColWidth = 22
	peekWidth   = 5 // sliver of the next column: 1 border cell + 4 chars,
	// peekSpace is the gap kept right of the peek sliver; the layout
	// reserves peekWidth+peekSpace and the sliver absorbs the division
	// remainder on top of peekWidth.
	peekSpace = 1
	// enough to show a 4-letter column name like "Done"
	cardMaxLines = 4
)

// GitHub status colors -> lipgloss colors (web UI palette).
var optionColors = map[string]string{
	"GRAY": "241", "PURPLE": "135", "BLUE": "39", "GREEN": "36",
	"YELLOW": "178", "ORANGE": "208", "RED": "196", "PINK": "204",
}

var (
	bTitleStyle   = themeTitle
	bDimStyle     = themeDim
	bColHeadStyle = lipgloss.NewStyle().Bold(true)
	bColCount     = themeDim
	bCardStyle    = themeCard
	bSelStyle     = themeSelRow
	bColStyle     = themeList
	bSelColStyle  = themeSelList
)

// refreshInterval is how often the board re-fetches items so changes made
// elsewhere (e.g. the browser) appear without user action. 5s keeps the
// board near-live; GraphQL polling is cheap and this project's usage is
// well under API rate limits.
const refreshInterval = 5 * time.Second

// BoardModel renders the kanban board (read-only in v0).
type BoardModel struct {
	titleBase string // project name + number, count appended dynamically
	columns   []board.Column

	// write access; client == nil means mock mode (moves stay local)
	client    *gh.Client
	projectID string
	fieldID   string

	// refetch, when non-nil, re-pulls items for auto-refresh
	refetch     func() ([]board.Item, error)
	statusField gh.FieldDef
	refreshing  bool

	colSelected int         // index into columns
	cardSel     map[int]int // column index -> selected card
	visibleCols int         // columns that fit
	colOffset   int         // first visible column index
	width       int
	height      int
	spinner     spinner.Model
	loading     bool
	errToast    string
	moving      bool // in-flight API call; ignore extra H/L

	detail       bool // detail view open for the selected card
	detailScroll int
	back         bool // mock mode: esc quits with Back()=true
	appMode      bool // runs inside AppModel: esc emits backMsg instead

	titleFieldID string // project's built-in Title field, for drafts
	editing      bool   // editing the card title in detail view
	input        textinput.Model
	adding       bool   // add screen open for the selected column
	addRepo      string // repo new issues go to; "" = draft
	// projectDefaultRepo is the repository linked in the project settings;
	// it overrides the single-repo inference from board items.
	projectDefaultRepo string
	// repo menu: shown on the add screen when the board mixes repositories
	choosingRepo   bool
	repoCandidates []string
	repoSel        int
	repoTop        int
	confirming     bool // delete confirmation pending
	helping        bool // command overlay open (toggled with ?)
}

// NewBoardModel creates the board; cards are set later via SetColumns.
// client/projectID/fieldID enable card moves, titleFieldID enables draft
// title edits; refetch enables background auto-refresh. Pass zero values
// for mock mode.
func NewBoardModel(titleBase string, client *gh.Client, projectID, fieldID, titleFieldID string, statusField gh.FieldDef, refetch func() ([]board.Item, error)) BoardModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = bTitleStyle
	ti := textinput.New()
	ti.Placeholder = "item title"
	ti.Width = 64 // without a width, the placeholder renders as just "t"
	ti.CharLimit = 256
	return BoardModel{
		titleBase:    titleBase,
		client:       client,
		projectID:    projectID,
		fieldID:      fieldID,
		titleFieldID: titleFieldID,
		statusField:  statusField,
		refetch:      refetch,
		spinner:      sp,
		loading:      true,
		cardSel:      map[int]int{},
		input:        ti,
	}
}

// SetColumns swaps in (mock or real) data and turns off the loading state.
func (m *BoardModel) SetColumns(cols []board.Column) {
	m.columns = cols
	m.loading = false
	if m.colSelected >= len(m.columns) {
		m.colSelected = 0
	}
}
func (m BoardModel) Init() tea.Cmd {
	cmds := []tea.Cmd{}
	if m.loading {
		cmds = append(cmds, m.spinner.Tick)
	}
	if m.refetch != nil {
		cmds = append(cmds, m.refreshTick())
	}
	return tea.Batch(cmds...)
}

// refreshTick schedules the next background refresh.
func (m BoardModel) refreshTick() tea.Cmd {
	return tea.Tick(refreshInterval, func(time.Time) tea.Msg { return refreshTickMsg{} })
}

func (m BoardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.clampOffset()

	case tea.KeyMsg:
		if m.detail {
			if m.editing {
				// the editor owns the keyboard; esc cancels back to detail
				switch msg.String() {
				case "ctrl+c":
					return m, tea.Quit
				case "esc":
					m.editing = false
					m.input.Blur()
				case "enter":
					title := strings.TrimSpace(m.input.Value())
					if title == "" {
						return m, nil
					}
					if card, ok := m.selectedCard(); ok {
						return m, m.setTitleCmd(card, title)
					}
				default:
					var cmd tea.Cmd
					m.input, cmd = m.input.Update(msg)
					return m, cmd
				}
				return m, nil
			}
			switch msg.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "esc", "enter", "q":
				m.detail = false
				m.detailScroll = 0
			case "up", "k":
				if m.detailScroll > 0 {
					m.detailScroll--
				}
			case "down", "j":
				if m.detailScroll < m.detailMaxScroll() {
					m.detailScroll++
				}
			case "e":
				if card, ok := m.selectedCard(); ok {
					m.input.SetValue(card.Title) // raw title only, no "#number"
					m.input.CursorEnd()
					m.input.Focus()
					m.editing = true
					return m, nil // don't feed the opening keypress to the input
				}
			}
			return m, nil
		}
		if m.choosingRepo {
			switch msg.String() {
			case "esc":
				m.choosingRepo = false
				m.adding = false
			case "up", "k":
				if m.repoSel > 0 {
					m.repoSel--
				}
				m.clampRepoView()
			case "down", "j":
				if m.repoSel < len(m.repoCandidates)-1 {
					m.repoSel++
				}
				m.clampRepoView()
			case "enter":
				m.addRepo = m.repoCandidates[m.repoSel]
				m.choosingRepo = false
				m.input.SetValue("")
				m.input.Focus()
			}
			return m, nil
		}
		if m.adding {
			switch msg.String() {
			case "esc":
				m.adding = false
				m.input.Blur()
			case "enter":
				title := strings.TrimSpace(m.input.Value())
				if title == "" {
					return m, nil
				}
				return m, m.addItemCmd(title)
			default:
				var cmd tea.Cmd
				m.input, cmd = m.input.Update(msg)
				return m, cmd
			}
			return m, nil
		}
		if m.confirming {
			switch msg.String() {
			case "esc", "n":
				m.confirming = false
			case "enter", "y":
				return m, m.deleteItemCmd()
			}
			return m, nil
		}
		if m.helping {
			// overlay eats everything except close/quit
			switch msg.String() {
			case "ctrl+c", "q":
				return m, tea.Quit
			case "esc", "?":
				m.helping = false
			}
			return m, nil
		}
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "?":
			m.helping = true
		case "+":
			if !m.adding && !m.confirming {
				m.addRepo = m.projectDefaultRepo
				if m.addRepo == "" {
					m.addRepo = board.DefaultRepo(m.columns) // single-repo boards
				}
				if m.addRepo == "" {
					if cands := board.ItemRepos(m.columns); len(cands) > 1 {
						// mixed repos and no configured default: menu
						m.repoCandidates = cands
						m.repoSel = 0
						m.repoTop = 0
						m.choosingRepo = true
						m.adding = true
						return m, nil
					}
				}
				m.input.SetValue("")
				m.input.Focus()
				m.adding = true
				return m, nil
			}
		case "-":
			if !m.adding && !m.confirming {
				if _, ok := m.selectedCard(); ok {
					m.confirming = true
					return m, nil
				}
			}
		case "esc":
			if m.appMode {
				return m, func() tea.Msg { return backMsg{} }
			}
			m.back = true
			return m, tea.Quit
		case "enter":
			if _, ok := m.selectedCard(); ok {
				m.detail = true
				m.detailScroll = 0
			}
		case "r":
			if m.refetch != nil && !m.moving && !m.refreshing && !m.loading {
				m.refreshing = true
				return m, m.doRefetch()
			}
		case "left", "h":
			if m.colSelected > 0 {
				m.colSelected--
				m.clampOffset()
			}
		case "right", "l":
			if m.colSelected < len(m.columns)-1 {
				m.colSelected++
				m.clampOffset()
			}
		case "up", "k":
			if n := len(m.cards(m.colSelected)); n > 0 {
				i := m.cardSel[m.colSelected]
				if i > 0 {
					m.cardSel[m.colSelected] = i - 1
				}
			}
		case "down", "j":
			if n := len(m.cards(m.colSelected)); n > 0 {
				i := m.cardSel[m.colSelected]
				if i < n-1 {
					m.cardSel[m.colSelected] = i + 1
				}
			}
		case "H":
			return m, m.moveCardCmd(-1)
		case "L":
			return m, m.moveCardCmd(+1)
		case "home", "g":
			m.cardSel[m.colSelected] = 0
		case "end", "G":
			if n := len(m.cards(m.colSelected)); n > 0 {
				m.cardSel[m.colSelected] = n - 1
			}
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case itemMovedMsg:
		m.applyMove(msg.fromCol, msg.cardIdx, msg.toCol)
		m.moving = false

	case titleEditedMsg:
		m.applyTitle(msg.title)

	case titleErrMsg:
		m.errToast = msg.err.Error()
		m.editing = false
		m.input.Blur()

	case itemAddedMsg:
		m.applyAdd(msg)

	case itemDeletedMsg:
		m.applyDelete(msg.col, msg.cardIdx)

	case errMsg:
		m.errToast = msg.err.Error()
		m.moving = false
		m.adding = false
		m.confirming = false
		m.input.Blur()

	case refreshTickMsg:
		if m.refetch == nil || m.moving || m.refreshing || m.loading || m.editing {
			return m, m.refreshTick() // retry on next interval
		}
		m.refreshing = true
		return m, tea.Batch(m.doRefetch(), m.refreshTick())

	case itemsRefreshedMsg:
		m.refreshing = false
		m.applyRefresh(msg.items)

	case refreshErrMsg:
		m.refreshing = false
		// silent: keep showing the last good board; transient network
		// hiccups shouldn't spam toasts
	}
	return m, nil
}

type refreshTickMsg struct{}
type itemsRefreshedMsg struct{ items []board.Item }
type refreshErrMsg struct{ err error }

// doRefetch pulls fresh items in the background.
func (m BoardModel) doRefetch() tea.Cmd {
	refetch := m.refetch
	return func() tea.Msg {
		items, err := refetch()
		if err != nil {
			return refreshErrMsg{err}
		}
		return itemsRefreshedMsg{items}
	}
}

// applyRefresh rebuilds columns from fresh data, keeping the selected card
// (matched by ID) selected.
func (m *BoardModel) applyRefresh(items []board.Item) {
	var selID string
	if cards := m.cards(m.colSelected); len(cards) > 0 && m.cardSel[m.colSelected] < len(cards) {
		selID = cards[m.cardSel[m.colSelected]].ID
	}
	m.SetColumns(board.Build(m.statusField, items))
	if selID != "" && m.colSelected < len(m.columns) {
		for j, c := range m.columns[m.colSelected].Cards {
			if c.ID == selID {
				m.cardSel[m.colSelected] = j
				break
			}
		}
	}
}

// applySize feeds a terminal size into the board (used by the app shell
// when creating boards after the initial WindowSizeMsg).
func (m *BoardModel) applySize(w, h int) {
	m.width, m.height = w, h
	m.clampOffset()
}

// Back reports whether esc quit the board in mock mode. In appMode esc
// never quits; it emits backMsg for the AppModel to switch screens.
func (m BoardModel) Back() bool { return m.back }

// backMsg asks the AppModel to return to the project picker.
type backMsg struct{}

// Editing reports whether the title editor is open (used by tests).
func (m BoardModel) Editing() bool { return m.editing }

// Adding reports whether the add prompt is open (used by tests).
func (m BoardModel) Adding() bool { return m.adding }

// Confirming reports whether the delete confirmation is open (tests).
func (m BoardModel) Confirming() bool { return m.confirming }

// ChoosingRepo reports whether the repo menu is open (tests).
func (m BoardModel) ChoosingRepo() bool { return m.choosingRepo }

// DefaultRepo is the repo new issues would be created in ("" = drafts).
func (m BoardModel) DefaultRepo() string { return m.addRepo }

// Board exposes the active board model (tests and shell consumers).
func (m AppModel) Board() *BoardModel { return m.board }

// ProjectID, FieldID, Loading, Columns are read accessors for tests.
func (m BoardModel) ProjectID() string { return m.projectID }
func (m BoardModel) FieldID() string   { return m.fieldID }
func (m BoardModel) Loading() bool     { return m.loading }
func (m BoardModel) Columns() int      { return len(m.columns) }

// Cards returns the cards of column i (tests).
func (m BoardModel) Cards(col int) []board.Card { return m.cards(col) }

// InputValue is the current editor text (used by tests).
func (m BoardModel) InputValue() string { return m.input.Value() }

// header is the title line: app name + version, project, live item count.
func (m BoardModel) header() string {
	n := 0
	for _, c := range m.columns {
		n += len(c.Cards)
	}
	return fmt.Sprintf("Project %s · %d item(s)", m.titleBase, n)
}

// itemMovedMsg is sent after a successful (or mock) card move.
type itemMovedMsg struct {
	fromCol, cardIdx, toCol int
}

type errMsg struct{ err error }

// Error makes errMsg readable in test failures and logs.
func (e errMsg) Error() string { return e.err.Error() }

// ErrorMsg is the exported view of errMsg for tests and shell users.
type ErrorMsg = errMsg

// titleEditedMsg carries the new title for local state update.
type titleEditedMsg struct {
	title string
}

type titleErrMsg struct{ err error }

// setTitleCmd persists the new title (mock mode: local only).
func (m BoardModel) setTitleCmd(card board.Card, title string) tea.Cmd {
	if m.client == nil {
		return func() tea.Msg { return titleEditedMsg{title} }
	}
	if card.Type != "DraftIssue" && card.ContentID == "" {
		return func() tea.Msg {
			return titleErrMsg{fmt.Errorf("item has no content id (deleted issue/PR?); refresh the board")}
		}
	}
	client, projectID, fieldID := m.client, m.projectID, m.titleFieldID
	ctype, contentID := card.Type, card.ContentID
	itemID := card.ID // project item id, used by the draft path
	return func() tea.Msg {
		if err := client.SetItemTitle(context.Background(), ctype, contentID, projectID, itemID, fieldID, title); err != nil {
			return titleErrMsg{err}
		}
		return titleEditedMsg{title}
	}
}

// applyTitle sets the selected card's title locally after success.
func (m *BoardModel) applyTitle(title string) {
	if cards := m.cards(m.colSelected); m.cardSel[m.colSelected] < len(cards) {
		cards[m.cardSel[m.colSelected]].Title = title
	}
	m.editing = false
	m.input.Blur()
	m.errToast = ""
}

// itemAddedMsg carries a freshly created card for local insertion.
type itemAddedMsg struct {
	col  int
	card board.Card
}

// itemDeletedMsg carries the removed card's position.
type itemDeletedMsg struct {
	col     int
	cardIdx int
}

// addItemCmd creates a draft item in the selected column (mock: local only).
func (m BoardModel) addItemCmd(title string) tea.Cmd {
	col := m.colSelected
	optionID := ""
	if col >= 0 && col < len(m.columns) {
		optionID = m.columns[col].OptionID // "" -> No Status, skip field write
	}
	if m.client == nil {
		card := board.Card{ID: fmt.Sprintf("mock-%d", time.Now().UnixNano()), Title: title, Type: "DraftIssue"}
		if m.addRepo != "" {
			card.Type = "Issue"
			card.Repo = m.addRepo
			card.Number = int(time.Now().Unix() % 1000)
		}
		return func() tea.Msg { return itemAddedMsg{col: col, card: card} }
	}
	if m.projectID == "" || m.fieldID == "" {
		return func() tea.Msg { return errMsg{fmt.Errorf("cannot add: project or status field unknown")} }
	}
	client, projectID, fieldID, repo := m.client, m.projectID, m.fieldID, m.addRepo
	return func() tea.Msg {
		if repo != "" {
			created, err := client.CreateIssueInRepo(context.Background(), projectID, repo, title, fieldID, optionID)
			if err != nil {
				return errMsg{err}
			}
			return itemAddedMsg{col: col, card: board.Card{
				ID: created.ItemID, Title: created.Title, Type: "Issue",
				Number: created.Number, URL: created.URL, Repo: created.Repo,
				ContentID: created.IssueID,
			}}
		}
		id, err := client.AddDraftItem(context.Background(), projectID, title, fieldID, optionID)
		if err != nil {
			return errMsg{err}
		}
		return itemAddedMsg{col: col, card: board.Card{ID: id, Title: title, Type: "DraftIssue"}}
	}
}

// deleteItemCmd removes the selected item from the board (mock: local only).
func (m BoardModel) deleteItemCmd() tea.Cmd {
	card, ok := m.selectedCard()
	if !ok {
		return nil
	}
	col, idx := m.colSelected, m.cardSel[m.colSelected]
	if m.client == nil {
		return func() tea.Msg { return itemDeletedMsg{col: col, cardIdx: idx} }
	}
	client, projectID, itemID := m.client, m.projectID, card.ID
	return func() tea.Msg {
		if err := client.DeleteItem(context.Background(), projectID, itemID); err != nil {
			return errMsg{err}
		}
		return itemDeletedMsg{col: col, cardIdx: idx}
	}
}

// applyAdd inserts the new card at the end of its column and selects it.
func (m *BoardModel) applyAdd(msg itemAddedMsg) {
	if msg.col < 0 || msg.col >= len(m.columns) {
		return
	}
	m.columns[msg.col].Cards = append(m.columns[msg.col].Cards, msg.card)
	m.colSelected = msg.col
	m.cardSel[msg.col] = len(m.columns[msg.col].Cards) - 1
	m.clampOffset()
	m.adding = false
	m.input.Blur()
	m.errToast = ""
}

// applyDelete removes the card and keeps a sane selection.
func (m *BoardModel) applyDelete(col, cardIdx int) {
	if col < 0 || col >= len(m.columns) || cardIdx >= len(m.columns[col].Cards) {
		return
	}
	cards := m.columns[col].Cards
	m.columns[col].Cards = append(cards[:cardIdx], cards[cardIdx+1:]...)
	if n := len(m.columns[col].Cards); n > 0 {
		if cardIdx >= n {
			cardIdx = n - 1
		}
		m.cardSel[col] = cardIdx
	} else {
		m.cardSel[col] = 0
	}
	m.confirming = false
	m.errToast = ""
}

// moveCardCmd moves the selected card one column left (dir=-1) or right
// (dir=+1). In mock mode (no client) the move is applied locally.
func (m BoardModel) moveCardCmd(dir int) tea.Cmd {
	if m.moving || m.loading {
		return nil
	}
	to := m.colSelected + dir
	if to < 0 || to >= len(m.columns) {
		return nil
	}
	cardIdx := m.cardSel[m.colSelected]
	if cardIdx >= len(m.cards(m.colSelected)) {
		return nil
	}
	from := m.colSelected

	if m.client == nil {
		// mock mode: no local mutation here — applyMove runs on the
		// itemMovedMsg, same as the API path, so the move applies once
		return func() tea.Msg { return itemMovedMsg{from, cardIdx, to} }
	}

	card := m.columns[from].Cards[cardIdx]
	targetOptionID := m.columns[to].OptionID // "" = No Status -> clear value
	client, projectID, fieldID := m.client, m.projectID, m.fieldID
	return func() tea.Msg {
		if err := client.SetItemStatus(context.Background(), projectID, card.ID, fieldID, targetOptionID); err != nil {
			return errMsg{err}
		}
		return itemMovedMsg{from, cardIdx, to}
	}
}

// applyMove mutates local state after a successful move.
func (m *BoardModel) applyMove(from, cardIdx, to int) {
	newIdx, err := board.MoveCard(m.columns, from, cardIdx, to)
	if err != nil {
		m.errToast = err.Error()
		return
	}
	m.colSelected = to
	m.cardSel[to] = newIdx
	m.clampOffset()
	m.errToast = ""
}

func (m BoardModel) cards(col int) []board.Card {
	if col < 0 || col >= len(m.columns) {
		return nil
	}
	return m.columns[col].Cards
}

// selectedCard returns the highlighted card, if any.
func (m BoardModel) selectedCard() (board.Card, bool) {
	cards := m.cards(m.colSelected)
	idx := m.cardSel[m.colSelected]
	if idx < 0 || idx >= len(cards) {
		return board.Card{}, false
	}
	return cards[idx], true
}

// detailLayout builds the fixed header lines and the scrollable body lines
// of the detail view. Both Update and View use it so scrolling stays
// consistent.
func (m BoardModel) detailLayout() (head, body []string) {
	card, _ := m.selectedCard()
	inner := m.modalInnerWidth()

	meta := card.Type
	if meta == "" {
		meta = "Item"
	}
	if m.colSelected >= 0 && m.colSelected < len(m.columns) {
		meta += ": " + m.columns[m.colSelected].Option.Name
	}
	head = []string{bTitleStyle.Render(cardTitle(card)), "", themeCard.Render(meta)}
	if card.Assignee != "" {
		head = append(head, themeCard.Render("Assignee: @"+card.Assignee))
	}
	if card.Repo != "" {
		head = append(head, themeCard.Render("Repository: "+card.Repo))
	}
	for _, f := range card.Fields {
		head = append(head, themeCard.Render(f.Field+": "+f.Value))
	}
	if card.URL != "" {
		// OSC 8 makes the URL clickable in terminals that support it
		// (ghostty does); elsewhere the plain URL text remains for
		// copy/paste and terminal URL detection.
		head = append(head, themeCard.Render(auth.Hyperlink(card.URL, card.URL, "", false)))
	}
	head = append(head, "")

	if strings.TrimSpace(card.Body) == "" {
		body = []string{themeCard.Render("(no description)")}
	} else {
		body = wrap(strings.TrimSpace(card.Body), inner)
	}
	return head, body
}

// cardTitle is the title with number, or just the title for drafts.
func cardTitle(c board.Card) string {
	if c.Number > 0 {
		return fmt.Sprintf("%s #%d", c.Title, c.Number)
	}
	return c.Title
}

// detailMaxScroll is the largest detailScroll that still shows content.
func (m BoardModel) detailMaxScroll() int {
	_, body := m.detailLayout()
	max := len(body) - m.detailVis()
	if max < 0 {
		max = 0
	}
	return max
}

// renderDetail shows one card: title, metadata, and its body, scrollable,
// as a box overlaid on the board.
func (m BoardModel) renderDetail(bg string) string {
	if _, ok := m.selectedCard(); !ok {
		return bg
	}
	head, body := m.detailLayout()

	if m.editing {
		body := m.input.View() + "\n\n" + strings.Join(head[2:], "\n")
		foot := pickerErrStyle.Render("Enter: save") + modalFoot(" · Esc: cancel")
		return overlayCenter(bg, modalBox(head[0], body, foot, 0), m.width)
	}

	vis := m.detailVis()
	scroll := m.detailScroll
	if max := len(body) - vis; scroll > max {
		scroll = max
	}
	if scroll < 0 {
		scroll = 0
	}
	end := scroll + vis
	if end > len(body) {
		end = len(body)
	}
	shown := body[scroll:end]
	foot := modalFoot("↑/↓ or j/k: scroll · e: edit title · Esc: back")
	content := strings.Join(head[2:], "\n") + "\n" + strings.Join(shown, "\n")
	return overlayCenter(bg, modalBox(head[0], content, foot, m.modalInnerWidth()), m.width)
}

// clampRepoView keeps the selected repo row visible.
func (m *BoardModel) clampRepoView() {
	inner := m.height - 8
	if m.height == 0 {
		inner = 10
	}
	if inner < 1 {
		inner = 1
	}
	if m.repoSel < m.repoTop {
		m.repoTop = m.repoSel
	}
	if m.repoSel >= m.repoTop+inner {
		m.repoTop = m.repoSel - inner + 1
	}
	if m.repoTop < 0 {
		m.repoTop = 0
	}
}

// renderAdd is the add screen: same full-screen treatment as title editing.
// Shows where the item will be created (default repo or draft).
func (m BoardModel) renderAdd(bg string) string {
	colName := "No Status"
	if m.colSelected >= 0 && m.colSelected < len(m.columns) {
		colName = m.columns[m.colSelected].Option.Name
	}
	title := modalTitle("New item — " + colName)

	if m.choosingRepo {
		// repo menu: the board's items come from several repositories
		inner := m.height - 12
		if m.height == 0 {
			inner = 8
		}
		if inner < 1 {
			inner = 1
		}
		var lines []string
		for _, l := range wrap("The items on this board come from several repositories. Pick one:", m.modalInnerWidth()) {
			lines = append(lines, themeCard.Render(l))
		}
		lines = append(lines, "")
		var rows []string
		for i, r := range m.repoCandidates {
			line := "  " + r
			if i == m.repoSel {
				line = "> " + r
			}
			if i == m.repoSel {
				rows = append(rows, themeSelRow.Render(line))
			} else {
				rows = append(rows, line)
			}
		}
		content := strings.Join(rows[m.repoTop:min(m.repoTop+inner, len(rows))], "\n")
		if m.repoTop > 0 || m.repoTop+inner < len(m.repoCandidates) {
			content += "\n" + themeDim.Render(fmt.Sprintf("  ↑ %d-%d of %d ↓",
				m.repoTop+1, min(m.repoTop+inner, len(m.repoCandidates)), len(m.repoCandidates)))
		}
		foot := modalFoot("↑/↓ or j/k: move · Enter: select · Esc: cancel")
		return overlayCenter(bg, modalBox(title, strings.Join(lines, "\n")+"\n"+content, foot, 0), m.width)
	}

	body := m.input.View() + "\n\n"
	if m.addRepo != "" {
		body += themeCard.Render("Note: item will be created as an issue in " + m.addRepo + ".")
	} else {
		body += themeCard.Render("Note: item will be created as a draft (no repositories to create an issue in).")
	}
	foot := modalFoot("Enter: create · Esc: cancel")
	return overlayCenter(bg, modalBox(title, body, foot, 0), m.width)
}

// renderConfirm is the delete confirmation overlay.
func (m BoardModel) renderConfirm(bg string) string {
	card, ok := m.selectedCard()
	if !ok {
		return bg
	}
	var note []string
	for _, l := range wrap("Note: drafts are deleted; linked issues and pull requests stay in their repository.", m.modalInnerWidth()) {
		note = append(note, themeCard.Render(l))
	}
	body := themeCard.Render(cardTitle(card)) + "\n\n" +
		themeErr.Render("This removes the item from the board.") + "\n\n" + strings.Join(note, "\n")
	foot := pickerErrStyle.Render("Enter: confirm delete") + modalFoot(" · Esc: cancel")
	return overlayCenter(bg, modalBox(modalTitle("Delete item"), body, foot, 0), m.width)
}

// layout computes how many columns are visible and their equal width.
//
// All columns share the terminal width when each box lands at least
// minColWidth wide. A column's border costs 2 cells, so the usable width is
// (width - 2*n) / n. If that falls below 22, one column is dropped and the
// rest share (width - peekWidth - peekSpace - 2*n) / n. The scroll state
// leaves room for a peekWidth sliver (border + 4 chars) of the next column
// plus one trailing space; the division remainder widens the sliver.
func (m BoardModel) layout() (visible, colW int) {
	total := len(m.columns)
	if m.width == 0 || total == 0 {
		return 1, minColWidth
	}
	if w := (m.width - 2*total) / total; w >= minColWidth {
		return total, w
	}
	for n := total - 1; n >= 1; n-- {
		if w := (m.width - peekWidth - peekSpace - 2*n) / n; w >= minColWidth {
			return n, w
		}
	}
	// single column always fits the remaining width
	w := m.width - peekWidth - peekSpace - 2
	if w < minColWidth {
		w = minColWidth
	}
	return 1, w
}

// columnWidth is the equal width every visible column gets.
func (m BoardModel) columnWidth() int {
	_, colW := m.layout()
	return colW
}

// visibleColumnCount is how many columns are shown at once.
func (m BoardModel) visibleColumnCount() int {
	n, _ := m.layout()
	return n
}

// clampOffset keeps the selected column visible when scrolling horizontally.
func (m *BoardModel) clampOffset() {
	n := m.visibleColumnCount()
	if m.colSelected < m.colOffset {
		m.colOffset = m.colSelected
	}
	if m.colSelected >= m.colOffset+n {
		m.colOffset = m.colSelected - n + 1
	}
	if m.colOffset < 0 {
		m.colOffset = 0
	}
}

func (m BoardModel) View() string {
	if m.loading {
		return "\n" + m.spinner.View() + " loading board...\n"
	}
	if len(m.columns) == 0 {
		return "\nNo columns found for this project.\n"
	}
	base := m.renderBoard()
	if m.adding {
		return m.renderAdd(base)
	}
	if m.confirming {
		return m.renderConfirm(base)
	}
	if m.detail {
		return m.renderDetail(base)
	}
	if m.helping {
		return m.renderHelp(base)
	}
	return base
}

// modalInnerWidth is the content width of the modal overlay boxes.
func (m BoardModel) modalInnerWidth() int {
	w := m.width - 8
	if w > 70 {
		w = 70
	}
	if w < 20 {
		w = 20
	}
	return w
}

// detailVis is the number of scrollable description rows that fit in the
// detail box: terminal height minus the box overhead (title, blank, header
// lines, footer, border).
func (m BoardModel) detailVis() int {
	head, _ := m.detailLayout()
	vis := m.height - len(head) - 8 // never approach the terminal height
	if vis > 12 {
		vis = 12
	}
	if vis < 3 {
		vis = 3
	}
	return vis
}

// renderBoard draws the plain board: header, columns, footer, toast.
func (m BoardModel) renderBoard() string {
	colW := m.columnWidth()
	n := m.visibleColumnCount()
	end := m.colOffset + n
	if end > len(m.columns) {
		end = len(m.columns)
	}

	var cols []string
	for i := m.colOffset; i < end; i++ {
		cols = append(cols, m.renderColumn(i, colW))
	}
	row := lipgloss.JoinHorizontal(lipgloss.Top, cols...)
	if end < len(m.columns) {
		// peek: 5-cell sliver (border + 4 chars) of the next column.
		// Truncation must be ANSI-aware: styled lines start with escape
		// sequences and naive rune cutting turns them into garbage the
		// terminal swallows, hiding the sliver entirely.
		// sliver width: reserved peekWidth + the division remainder, keeping
		// one cell of space at the terminal's right edge
		sliverW := m.width - (end-m.colOffset)*(colW+2) - peekSpace
		lines := strings.Split(m.renderColumn(end, colW), "\n")
		for i, ln := range lines {
			lines[i] = ansi.Truncate(ln, sliverW, "")
		}
		sliver := strings.Join(lines, "\n")
		row = lipgloss.JoinHorizontal(lipgloss.Top, row, sliver)
	}

	head := themeWhite.Render(" gpk "+Version) +
		bTitleStyle.Render(" · "+m.header())

	toast := ""
	if m.errToast != "" {
		toast = "\n" + pickerErrStyle.Render(m.errToast)
	} else if m.moving {
		toast = "\n" + bDimStyle.Render(m.spinner.View()+" moving...")
	}

	hints := bDimStyle.Render(" ?: commands · Esc: back · q: quit")
	hintsW := lipgloss.Width(" ?: commands · Esc: back · q: quit")
	nav := fmt.Sprintf("[← %d/%d →]", m.colSelected+1, len(m.columns))
	gap := m.width - 1 - hintsW - lipgloss.Width(nav)
	if gap < 1 {
		gap = 1
	}
	foot := hints + strings.Repeat(" ", gap) + bDimStyle.Render(nav)
	return head + "\n\n" + row + "\n" + foot + toast
}

// renderHelp overlays the centered command list on top of the board view.
func (m BoardModel) renderHelp(bg string) string {
	rows := [][2]string{
		{"h / ←", "previous column"},
		{"l / →", "next column"},
		{"j / ↓", "next item"},
		{"k / ↑", "previous item"},
		{"g", "first item"},
		{"G", "last item"},
		{},
		{"Enter", "open item detail"},
		{},
		{"+", "add item to the selected column"},
		{"-", "delete the selected item"},
		{},
		{"H", "move item to previous column"},
		{"L", "move item to next column"},
		{},
		{"r", "refresh board"},
	}
	return overlayCenter(bg, helpBox(rows), m.width)
}

// renderColumn draws one column with header (colored, with count) and cards.
func (m BoardModel) renderColumn(i, colW int) string {
	c := m.columns[i]
	selected := i == m.colSelected

	color, ok := optionColors[c.Option.Color]
	if !ok {
		color = "241"
	}
	head := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(color)).
		Render(truncate(c.Option.Name, colW-4))
	count := bColCount.Render(fmt.Sprintf(" (%d)", len(c.Cards)))
	header := head + count

	var lines []string
	lines = append(lines, header, "")

	if len(c.Cards) == 0 {
		lines = append(lines, bDimStyle.Render("· empty"))
	}
	sel := m.cardSel[i]
	for j, card := range c.Cards {
		for k, ln := range cardLines(card, colW-4) {
			prefix := "  "
			if k == 0 {
				if j == sel && selected {
					prefix = "> "
				} else {
					prefix = "  "
				}
			}
			style := bCardStyle
			if j == sel && selected {
				style = bSelStyle
			}
			lines = append(lines, style.Render(prefix+ln))
		}
	}

	style := bColStyle
	if selected {
		style = bSelColStyle
	}
	return style.Width(colW).Height(m.bodyHeight()).Render(strings.Join(lines, "\n"))
}

// cardLines wraps a card into at most cardMaxLines view lines.
func cardLines(c board.Card, width int) []string {
	wrapped := wrap(c.Title, width)
	if len(wrapped) > cardMaxLines {
		wrapped = wrapped[:cardMaxLines]
		wrapped[len(wrapped)-1] = truncate(wrapped[len(wrapped)-1], width-1) + "…"
	}
	return wrapped
}

func (m BoardModel) bodyHeight() int {
	if m.height == 0 {
		return 12
	}
	h := m.height - 5
	if h < 6 {
		h = 6
	}
	return h
}

// wrap breaks s into lines of at most width display cells, on spaces when
// possible.
func wrap(s string, width int) []string {
	if width <= 0 {
		return []string{s}
	}
	var out []string
	for _, para := range strings.Split(s, "\n") {
		words := strings.Fields(para)
		if len(words) == 0 {
			out = append(out, "")
			continue
		}
		line := ""
		for _, w := range words {
			switch {
			case line == "":
				line = w
			case lipgloss.Width(line+" "+w) <= width:
				line += " " + w
			default:
				out = append(out, line)
				line = w
			}
		}
		out = append(out, line)
	}
	return out
}

// truncate cuts s to at most width display cells.
func truncate(s string, width int) string {
	if lipgloss.Width(s) <= width {
		return s
	}
	if width <= 1 {
		return "…"
	}
	runes := []rune(s)
	for lipgloss.Width(string(runes)) > width-1 {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "…"
}
