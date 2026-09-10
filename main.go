// Command gpk is a read-only TUI for GitHub Projects v2 kanban boards.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"gpk/internal/auth"
	"gpk/internal/board"
	"gpk/internal/gh"
	"gpk/internal/tui"
)

var version = "0.0.1-dev"

func main() {
	whoami := flag.Bool("whoami", false, "verify auth: print the GitHub login the stored token resolves to")
	mock := flag.Bool("mock", false, "show the board with sample data (no API calls beyond auth)")
	showVersion := flag.Bool("version", false, "print version")
	flag.Parse()

	if *showVersion {
		fmt.Println("gpk", version)
		return
	}

	ctx := context.Background()
	token, err := obtainToken(ctx)
	if err != nil {
		fatal(err)
	}

	login, err := gh.NewClient(token).ViewerLogin(ctx)
	if err != nil {
		fatal(err)
	}

	if *whoami {
		fmt.Println("authenticated as", login)
		return
	}

	if *mock {
		if err := tui.RunMockBoard(login); err != nil {
			fatal(err)
		}
		return
	}

	if err := runPicker(ctx, token); err != nil {
		fatal(err)
	}
}

// runPicker shows the project picker, then fetches the project's items and
// renders the board.
func runPicker(ctx context.Context, token string) error {
	client := gh.NewClient(token)

	prog := tea.NewProgram(tui.NewPickerModel(client))
	final, err := prog.Run()
	if err != nil {
		return fmt.Errorf("project picker: %w", err)
	}
	pm, ok := final.(tui.PickerModel)
	if !ok {
		return fmt.Errorf("unexpected picker model type %T", final)
	}
	project, ok := pm.Selected()
	if !ok {
		return nil // user quit without selecting
	}

	fields, err := client.GetProjectFields(ctx, project.ID)
	if err != nil {
		return err
	}
	status, ok := gh.StatusField(fields)
	if !ok {
		return fmt.Errorf("project %q has no single-select field to use as columns", project.Title)
	}

	items, err := fetchAllItems(ctx, client, project.ID, status.ID)
	if err != nil {
		return err
	}

	bm := tui.NewBoardModel(fmt.Sprintf("%s #%d — %d items", project.Title, project.Number, len(items)))
	bm.SetColumns(board.Build(status, toBoardItems(items)))
	_, err = tea.NewProgram(bm).Run()
	return err
}

// fetchAllItems pages through the project's items.
func fetchAllItems(ctx context.Context, client *gh.Client, projectID, statusFieldID string) ([]gh.Item, error) {
	var all []gh.Item
	cursor := ""
	for {
		page, next, err := client.GetProjectItems(ctx, projectID, statusFieldID, 100, cursor)
		if err != nil {
			return nil, err
		}
		all = append(all, page...)
		if next == "" {
			return all, nil
		}
		cursor = next
	}
}

// toBoardItems converts API items to board items (currently 1:1).
func toBoardItems(items []gh.Item) []board.Item {
	out := make([]board.Item, len(items))
	for i, it := range items {
		out[i] = board.Item{
			ID:       it.ID,
			Title:    it.Title,
			Type:     it.Type,
			Number:   it.Number,
			URL:      it.URL,
			Assignee: it.Assignee,
			OptionID: it.OptionID,
		}
	}
	return out
}

// printLinkOptions renders the URL five ways because terminal chains
// (terminal + ssh + multiplexer) differ in which link format they honor.
// The user clicks whichever one their setup renders as a link.
func printLinkOptions(url string) {
	fmt.Println("Verification URL (pick the first one that is clickable):")
	fmt.Println("  [1] plain:    " + url)
	fmt.Println("  [2] link:     " + auth.Hyperlink(url, url, "", false))
	fmt.Println("  [3] link:     " + auth.Hyperlink(url, url, "", true))
	fmt.Println("  [4] link:     " + auth.Hyperlink(url, "open in browser", "gpk", false))
	fmt.Println("  [5] link:     " + auth.Hyperlink(url, "open in browser", "gpk", true))
}

// obtainToken resolves the token: env var, then token file, then the
// interactive device flow (result persisted to the token file).
func obtainToken(ctx context.Context) (string, error) {
	src, err := auth.DefaultSource()
	if err != nil {
		return "", err
	}
	token, err := src.GetToken()
	if err == nil {
		return token, nil
	}
	if err != auth.ErrNoToken {
		return "", err
	}

	fmt.Println("No GitHub token found. Starting device flow.")
	fmt.Println()

	hc := &http.Client{Timeout: 30 * time.Second}
	dc, err := auth.RequestDeviceCode(ctx, hc)
	if err != nil {
		return "", err
	}

	fmt.Printf("Open the verification URL in your browser and enter code: %s\n", dc.UserCode)
	printLinkOptions(dc.VerificationURL)
	fmt.Println("Waiting for approval...")

	token, err = auth.PollToken(ctx, hc, dc, nil)
	if err != nil {
		return "", err
	}

	store, err := auth.NewFileStore()
	if err != nil {
		return "", err
	}
	if err := store.Save(token); err != nil {
		return "", fmt.Errorf("token obtained but could not be saved: %w", err)
	}
	fmt.Printf("Token saved to %s\n", store.Path)
	return token, nil
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "gpk:", err)
	os.Exit(1)
}
