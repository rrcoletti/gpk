package board

import (
	"testing"

	"gpk/internal/gh"
)

// TestFromGhItemsCopiesEveryField locks down the gh.Item -> board.Item
// mapping. The ContentID omission caused "global id of ”" GraphQL errors:
// the query returned the id but the mapping dropped it.
func TestFromGhItemsCopiesEveryField(t *testing.T) {
	in := []gh.Item{{
		ID:        "PVTI_item1",
		Title:     "a title",
		Type:      "Issue",
		Number:    42,
		URL:       "https://github.com/o/r/issues/42",
		Assignee:  "someone",
		OptionID:  "opt-status",
		Body:      "the body",
		Repo:      "o/r",
		ContentID: "I_issue1",
	}, {
		ID:    "PVTI_draft1",
		Title: "a draft",
		Type:  "DraftIssue",
	}}

	out := FromGhItems(in)
	if len(out) != 2 {
		t.Fatalf("got %d items, want 2", len(out))
	}
	want := Item{
		ID: "PVTI_item1", Title: "a title", Type: "Issue", Number: 42,
		URL: "https://github.com/o/r/issues/42", Assignee: "someone",
		OptionID: "opt-status", Body: "the body", Repo: "o/r", ContentID: "I_issue1",
	}
	if out[0].ID != want.ID || out[0].Title != want.Title || out[0].Type != want.Type ||
		out[0].Number != want.Number || out[0].URL != want.URL ||
		out[0].Assignee != want.Assignee || out[0].OptionID != want.OptionID ||
		out[0].Body != want.Body || out[0].Repo != want.Repo ||
		out[0].ContentID != want.ContentID || len(out[0].Fields) != 0 {
		t.Errorf("field dropped or changed:\n got %+v\nwant %+v", out[0], want)
	}
	if out[1].ContentID != "" || out[1].Title != "a draft" || out[1].Type != "DraftIssue" {
		t.Errorf("draft fields wrong: %+v", out[1])
	}
}
