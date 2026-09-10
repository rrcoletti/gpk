// Command gpk is a read-only TUI for GitHub Projects v2 kanban boards.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"gpk/internal/auth"
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

	client := gh.NewClient(token)
	login, err := client.ViewerLogin(ctx)
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

	if err := tui.RunApp(ctx, client); err != nil {
		fatal(err)
	}
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
