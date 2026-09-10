package board

// Item is the raw per-item data the board needs, decoupled from GraphQL.
type Item struct {
	ID        string
	Title     string
	Type      string
	Number    int
	URL       string
	Assignee  string
	OptionID  string // status option id; "" = no status
	Body      string
	Repo      string
	ContentID string
}
