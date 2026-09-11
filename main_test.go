package main

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strings"
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
	if got.ID != want.ID || got.Title != want.Title || got.Type != want.Type ||
		got.Number != want.Number || got.URL != want.URL ||
		got.Assignee != want.Assignee || got.OptionID != want.OptionID ||
		got.Body != want.Body || got.Repo != want.Repo ||
		got.ContentID != want.ContentID || len(got.Fields) != 0 {
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

// pumpApp executes cmd (expanding tea.Batch sub-commands recursively) and
// feeds every produced message back into the app model, mimicking the
// bubbletea runtime loop in tests.
func pumpApp(t *testing.T, am tui.AppModel, cmd tea.Cmd) tui.AppModel {
	t.Helper()
	run := func(c tea.Cmd) {
		if c == nil {
			return
		}
		msg := c()
		if msg == nil {
			return
		}
		// tea.Batch's batchMsg is unexported; unwrap any slice of cmds
		// via reflection and run each like the runtime does.
		rv := reflect.ValueOf(msg)
		if rv.Kind() == reflect.Slice {
			for i := 0; i < rv.Len(); i++ {
				if sub, ok := rv.Index(i).Interface().(tea.Cmd); ok {
					am = pumpApp(t, am, sub)
				}
			}
			return
		}
		up, _ := am.Update(msg)
		am = up.(tui.AppModel)
	}
	run(cmd)
	return am
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

// TestLiveAddDeleteItem exercises + and - against the live API on the
// sandbox board ("test"): add a draft item to a column through the real
// model, verify it on GitHub, delete it with confirmation, verify it's gone.
// Skipped unless GPIGO_LIVE_TEST=1.
func TestLiveAddDeleteItem(t *testing.T) {
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
		t.Skip("test project not visible")
	}
	fields, err := client.GetProjectFields(ctx, project.ID)
	if err != nil {
		t.Fatalf("fields: %v", err)
	}
	status, ok := gh.StatusField(fields)
	if !ok {
		t.Fatal("no status field")
	}
	// THE REAL APP PATH: the AppModel owns the board; keys go through it.
	// This is the layer that dropped the status field id when wired directly.
	am := tui.NewAppModel(client)
	up, _ := am.Update(tea.WindowSizeMsg{Width: 200, Height: 50})
	am = up.(tui.AppModel)
	up, cmd0 := am.Update(tui.ProjectPickedMsg{Project: *project})
	am = up.(tui.AppModel)
	if cmd0 == nil {
		t.Fatal("pick should start board loading")
	}
	// run the load cmd like the bubbletea loop would (expand batches)
	am = pumpApp(t, am, cmd0)

	press := func(k string) tea.Cmd {
		m := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		if k == "enter" {
			m = tea.KeyMsg{Type: tea.KeyEnter}
		}
		up, cmd := am.Update(m)
		am = up.(tui.AppModel)
		return cmd
	}

	t.Logf("board wiring after load: projectID=%q fieldID=%q loading=%v cols=%d",
		am.Board().ProjectID(), am.Board().FieldID(), am.Board().Loading(), am.Board().Columns())

	// ADD: select column 2 (second status option), press +, type title, enter
	press("l")
	press("l")
	press("+")
	if !am.Board().Adding() {
		t.Fatal("+ should open the add prompt")
	}
	const title = "gpk live add/delete item"
	for _, r := range title {
		up, _ := am.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		am = up.(tui.AppModel)
	}
	up, cmd := am.Update(tea.KeyMsg{Type: tea.KeyEnter})
	am = up.(tui.AppModel)
	if cmd == nil {
		t.Fatal("add should return a cmd")
	}
	msg := cmd() // executes addProjectV2DraftIssue + status set
	if e, ok := msg.(tui.ErrorMsg); ok {
		t.Fatalf("add failed: %v", e.Error())
	}
	up2, _ := am.Update(msg)
	am = up2.(tui.AppModel)
	if len(am.Board().Cards(2)) != 1 {
		t.Fatalf("card not added locally to column 2: %d", len(am.Board().Cards(2)))
	}
	itemID := am.Board().Cards(2)[0].ID
	if itemID == "" || strings.HasPrefix(itemID, "mock-") {
		t.Fatalf("add did not return a real item id: %q", itemID)
	}
	t.Logf("added item %s (%q)", itemID, title)

	// verify on GitHub (eventual consistency: poll)
	found := false
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) && !found {
		items, err := client.FetchAllItems(ctx, project.ID, status.ID)
		if err != nil {
			t.Fatal(err)
		}
		for _, it := range items {
			if it.ID == itemID && it.Title == title {
				found = true
			}
		}
		if !found {
			time.Sleep(500 * time.Millisecond)
		}
	}
	if !found {
		t.Fatal("added item never appeared on GitHub")
	}
	t.Log("item verified on GitHub")

	// DELETE: select it (applyAdd selected it), press -, confirm with enter
	press("-")
	if !am.Board().Confirming() {
		t.Fatal("- should open the delete confirmation")
	}
	up, cmd = am.Update(tea.KeyMsg{Type: tea.KeyEnter})
	am = up.(tui.AppModel)
	if cmd == nil {
		t.Fatal("confirm should return a cmd")
	}
	msg = cmd() // executes deleteProjectV2Item
	up2, _ = am.Update(msg)
	am = up2.(tui.AppModel)
	if len(am.Board().Cards(2)) != 0 {
		t.Fatalf("card should be gone locally, got %d", len(am.Board().Cards(2)))
	}

	// verify on GitHub: item must disappear
	gone := false
	deadline = time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) && !gone {
		items, err := client.FetchAllItems(ctx, project.ID, status.ID)
		if err != nil {
			t.Fatal(err)
		}
		gone = true
		for _, it := range items {
			if it.ID == itemID {
				gone = false
			}
		}
		if !gone {
			time.Sleep(500 * time.Millisecond)
		}
	}
	if !gone {
		t.Fatal("deleted item still on GitHub")
	}
	t.Logf("item %s deleted and verified gone from GitHub", itemID)
	t.Log("sandbox board restored to empty")
}

// TestLiveAddDraftAndIssueInRepo covers both add paths live on the sandbox
// board: draft (no default repo) and real issue (mock default repo path is
// model-tested; here we force the repo path by pointing at the test repo).
// Skipped unless GPIGO_LIVE_TEST=1.
func TestLiveAddDraft(t *testing.T) {
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
	projects, _, err := client.ListViewerProjects(ctx, 20, "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var project *gh.Project
	for i := range projects {
		if projects[i].ID == testProjectID {
			project = &projects[i]
		}
	}
	if project == nil {
		t.Skip("test project not visible")
	}
	fields, err := client.GetProjectFields(ctx, project.ID)
	if err != nil {
		t.Fatalf("fields: %v", err)
	}
	status, ok := gh.StatusField(fields)
	if !ok {
		t.Fatal("no status field")
	}
	am := tui.NewAppModel(client)
	up, _ := am.Update(tea.WindowSizeMsg{Width: 200, Height: 50})
	am = up.(tui.AppModel)
	up, cmd0 := am.Update(tui.ProjectPickedMsg{Project: *project})
	am = up.(tui.AppModel)
	if cmd0 == nil {
		t.Fatal("pick should start board loading")
	}
	// run the load cmd like the bubbletea loop would: expand batches and
	// feed every produced message back into the model until quiet
	am = pumpApp(t, am, cmd0)

	press := func(k string) tea.Cmd {
		m := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		if k == "enter" {
			m = tea.KeyMsg{Type: tea.KeyEnter}
		}
		up, cmd := am.Update(m)
		am = up.(tui.AppModel)
		return cmd
	}

	// ADD as draft (sandbox board has no items, so no default repo)
	press("+")
	if !am.Board().Adding() {
		t.Fatal("+ should open the add screen")
	}
	if am.Board().DefaultRepo() != "" {
		t.Fatalf("empty board should have no default repo, got %q", am.Board().DefaultRepo())
	}
	const title = "gpk live add draft"
	for _, r := range title {
		up, _ := am.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		am = up.(tui.AppModel)
	}
	up, cmd := am.Update(tea.KeyMsg{Type: tea.KeyEnter})
	am = up.(tui.AppModel)
	if cmd == nil {
		t.Fatal("add should return a cmd")
	}
	msg := cmd() // addProjectV2DraftIssue; column 0 = No Status -> no field write
	up2, _ := am.Update(msg)
	am = up2.(tui.AppModel)
	if len(am.Board().Cards(0)) != 1 {
		t.Fatalf("card not added locally: %d cards in col 0", len(am.Board().Cards(0)))
	}
	itemID := am.Board().Cards(0)[0].ID
	if itemID == "" || strings.HasPrefix(itemID, "mock-") {
		t.Fatalf("no real item id: %q", itemID)
	}

	// verify on GitHub (poll: eventual consistency)
	found := false
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) && !found {
		items, err := client.FetchAllItems(ctx, project.ID, status.ID)
		if err != nil {
			t.Fatal(err)
		}
		for _, it := range items {
			if it.ID == itemID && it.Title == title && it.Type == "DraftIssue" {
				found = true
			}
		}
		if !found {
			time.Sleep(500 * time.Millisecond)
		}
	}
	if !found {
		t.Fatal("added draft never appeared on GitHub")
	}
	t.Logf("draft %s verified on GitHub", itemID)

	// cleanup: delete it through the app path
	upDel, _ := am.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'-'}})
	am = upDel.(tui.AppModel)
	if !am.Board().Confirming() {
		t.Fatal("- should open the delete confirmation")
	}
	up, cmd = am.Update(tea.KeyMsg{Type: tea.KeyEnter})
	am = up.(tui.AppModel)
	if cmd == nil {
		t.Fatal("confirm should return a cmd")
	}
	msg = cmd()
	up2, _ = am.Update(msg)
	am = up2.(tui.AppModel)
	if len(am.Board().Cards(0)) != 0 {
		t.Fatal("card should be gone locally")
	}

	// verify deletion on GitHub
	gone := false
	deadline = time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) && !gone {
		items, err := client.FetchAllItems(ctx, project.ID, status.ID)
		if err != nil {
			t.Fatal(err)
		}
		gone = true
		for _, it := range items {
			if it.ID == itemID {
				gone = false
			}
		}
		if !gone {
			time.Sleep(500 * time.Millisecond)
		}
	}
	if !gone {
		t.Fatal("draft still on GitHub after delete")
	}
	t.Log("draft deleted; sandbox board restored to empty")
}
