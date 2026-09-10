package main

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"gpk/internal/auth"
	"gpk/internal/board"
	"gpk/internal/gh"
	"gpk/internal/tui"
)

// TestToBoardItemsCopiesEveryField locks down the gh.Item -> board.Item
// mapping. The ContentID omission caused "global id of ”" GraphQL errors:
// the query returned the id but the mapping dropped it.
func TestToBoardItemsCopiesEveryField(t *testing.T) {
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

	out := board.FromGhItems(in)
	if len(out) != 2 {
		t.Fatalf("got %d items, want 2", len(out))
	}
	got := out[0]
	want := board.Item{
		ID: "PVTI_item1", Title: "a title", Type: "Issue", Number: 42,
		URL: "https://github.com/o/r/issues/42", Assignee: "someone",
		OptionID: "opt-status", Body: "the body", Repo: "o/r", ContentID: "I_issue1",
	}
	if got != want {
		t.Errorf("field dropped or changed:\n got %+v\nwant %+v", got, want)
	}
	d := out[1]
	if d.ContentID != "" {
		t.Errorf("draft should carry empty ContentID, got %q", d.ContentID)
	}
	if d.Title != "a draft" || d.Type != "DraftIssue" {
		t.Errorf("draft fields wrong: %+v", d)
	}
}

// TestLiveDraftTitleEdit is a functional test of the app's data path against
// the live GitHub API, entirely on the user's designated sandbox board:
// project "test" (PVT_kwHODQUll84BjEtK). It creates a draft item, edits its
// title through the real model (keys -> cmd -> mutation), reads the change
// back from GitHub, and deletes the draft. Skipped unless GPIGO_LIVE_TEST=1.
func TestLiveDraftTitleEdit(t *testing.T) {
	if os.Getenv("GPIGO_LIVE_TEST") != "1" {
		t.Skip("set GPIGO_LIVE_TEST=1 to run against the live API")
	}
	src, err := auth.DefaultSource()
	if err != nil {
		t.Skipf("no token source: %v", err)
	}
	token, err := src.GetToken()
	if err != nil {
		t.Skipf("no token: %v", err)
	}
	ctx := context.Background()
	client := gh.NewClient(token)

	const testProjectID = "PVT_kwHODQUll84BjEtK"

	// locate the project (by id, but fetch its real record)
	projects, _, err := client.ListViewerProjects(ctx, 20, "")
	if err != nil {
		t.Fatalf("list projects: %v", err)
	}
	var project *gh.Project
	for i := range projects {
		if projects[i].ID == testProjectID {
			project = &projects[i]
		}
	}
	if project == nil {
		t.Skip("test project not visible to this token")
	}

	fields, err := client.GetProjectFields(ctx, project.ID)
	if err != nil {
		t.Fatalf("fields: %v", err)
	}
	status, ok := gh.StatusField(fields)
	if !ok {
		t.Fatal("no status field on test project")
	}
	titleField, ok := gh.TitleField(fields)
	if !ok {
		t.Fatal("no Title field on test project")
	}

	// create a draft item on the sandbox board (board-only, no repo touched)
	d := gqlQuery(t, client, `mutation($p: ID!, $t: String!) {
		addProjectV2DraftIssue(input: {projectId: $p, title: $t}) {
			projectItem { id }
		}
	}`, map[string]any{"p": project.ID, "t": "gpk live draft"})
	draftID := d["addProjectV2DraftIssue"].(map[string]any)["projectItem"].(map[string]any)["id"].(string)
	defer func() {
		gqlQuery(t, client, `mutation($p: ID!, $i: ID!) {
			deleteProjectV2Item(input: {projectId: $p, itemId: $i}) { deletedItemId }
		}`, map[string]any{"p": project.ID, "i": draftID})
		t.Log("draft item deleted; sandbox board restored")
	}()

	// GitHub's items connection is eventually consistent: a new item takes
	// ~1-2s to appear. Poll until it does, then run the app's exact pipeline.
	var items []gh.Item
	waitFor := func(match func(gh.Item) bool) error {
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			items, err = client.FetchAllItems(ctx, project.ID, status.ID)
			if err != nil {
				return err
			}
			for _, it := range items {
				if match(it) {
					return nil
				}
			}
			time.Sleep(500 * time.Millisecond)
		}
		return fmt.Errorf("item never appeared in items connection")
	}
	if err := waitFor(func(it gh.Item) bool { return it.ID == draftID }); err != nil {
		t.Fatal(err)
	}
	cols := board.Build(status, board.FromGhItems(items))

	var card *board.Card
	var colIdx, cardIdx int
search:
	for i := range cols {
		for j := range cols[i].Cards {
			if cols[i].Cards[j].ID == draftID {
				card = &cols[i].Cards[j]
				colIdx, cardIdx = i, j
				break search
			}
		}
	}
	if card == nil {
		t.Fatal("draft not found through app pipeline")
	}
	if card.Type != "DraftIssue" || card.ContentID != "" {
		t.Fatalf("draft card wrong: %+v", card)
	}

	// drive the real model with real keys
	bm := tui.NewBoardModel("sandbox", client, project.ID, status.ID, titleField.ID, status, nil)
	bm.SetColumns(cols)
	press := func(k string) tea.Cmd {
		m := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		if k == "enter" {
			m = tea.KeyMsg{Type: tea.KeyEnter}
		}
		up, cmd := bm.Update(m)
		bm = up.(tui.BoardModel)
		return cmd
	}
	pressKey := func(ty tea.KeyType) {
		up, _ := bm.Update(tea.KeyMsg{Type: ty})
		bm = up.(tui.BoardModel)
	}
	for i := 0; i < colIdx; i++ {
		press("l")
	}
	for i := 0; i < cardIdx; i++ {
		press("j")
	}
	press("enter") // detail
	press("e")     // editor
	if !bm.Editing() {
		t.Fatal("editor did not open")
	}
	if got := bm.InputValue(); got != "gpk live draft" {
		t.Fatalf("prefill = %q, want %q", got, "gpk live draft")
	}

	const newTitle = "gpk live draft (renamed through the app)"
	for range "gpk live draft" {
		pressKey(tea.KeyBackspace)
	}
	for _, r := range newTitle {
		up, _ := bm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		bm = up.(tui.BoardModel)
	}
	if got := bm.InputValue(); got != newTitle {
		t.Fatalf("input = %q, want %q", got, newTitle)
	}

	cmd := press("enter") // save -> real mutation cmd
	if cmd == nil {
		t.Fatal("save returned no cmd")
	}
	msg := cmd()
	up2, _ := bm.Update(msg)
	fm := up2.(tui.BoardModel)
	if fm.Editing() {
		t.Fatal("editor should close after success")
	}

	// read back from GitHub through the app's own query; the title write is
	// also eventually consistent, so poll until it shows
	found := false
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		items, err = client.FetchAllItems(ctx, project.ID, status.ID)
		if err != nil {
			t.Fatal(err)
		}
		for _, it := range items {
			if it.ID == draftID {
				if it.Title == newTitle {
					found = true
				}
				break
			}
		}
		if found {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if !found {
		t.Fatal("edited title never appeared on GitHub")
	}
	t.Logf("live draft title edit through app path OK (%q -> %q)", "gpk live draft", newTitle)
}

// gqlQuery is a small GraphQL helper for tests that need mutations the
// production client does not expose (draft add/delete).
func gqlQuery(t *testing.T, client *gh.Client, q string, vars map[string]any) map[string]any {
	t.Helper()
	var out map[string]any
	if err := client.Query(context.Background(), q, vars, &out); err != nil {
		t.Fatalf("graphql: %v", err)
	}
	return out
}
