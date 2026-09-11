// Command gpk is a read-only TUI for GitHub Projects v2 kanban boards.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"gpk/internal/auth"
	"gpk/internal/gh"
	"gpk/internal/tui"
)

var version = "1.0.0"

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
	tui.Version = version
	token, fromEnv, err := obtainToken(ctx)
	if err != nil {
		fatal(err)
	}

	client := gh.NewClient(token)
	login, err := client.ViewerLogin(ctx)
	if errors.Is(err, gh.ErrUnauthorized) {
		if fromEnv {
			fatal(fmt.Errorf("GitHub rejected the token in $%s: it is invalid, expired, or was revoked. Fix or unset it and run gpk again", auth.EnvVar))
		}
		login, err = reauth(ctx)
	}
	if err != nil {
		fatal(err)
	}

	tui.User = login

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

// obtainToken resolves the token: env var, then token file, then the
// interactive device flow (result persisted to the token file). fromEnv
// reports whether the token came from the environment (not deletable).
func obtainToken(ctx context.Context) (token string, fromEnv bool, err error) {
	src, err := auth.DefaultSource()
	if err != nil {
		return "", false, err
	}
	token, err = src.GetToken()
	if err == nil {
		return token, os.Getenv(auth.EnvVar) != "", nil
	}
	if !errors.Is(err, auth.ErrNoToken) {
		return "", false, err
	}
	token, err = deviceFlow(ctx, "No GitHub token found.")
	return token, false, err
}

// deviceFlow runs the interactive OAuth device flow and persists the result.
// intro explains why the flow is starting (no token / dead token).
func deviceFlow(ctx context.Context, intro string) (string, error) {
	fmt.Println(intro, "Starting device flow.")
	fmt.Println()

	hc := &http.Client{Timeout: 30 * time.Second}
	dc, err := auth.RequestDeviceCode(ctx, hc)
	if err != nil {
		return "", err
	}

	// Single link format (OSC 8, BEL terminator, URL as its own label): the
	// variant that survived testing across the user's terminal setups. The
	// plain URL also stays visible to terminals without OSC 8 support.
	fmt.Printf("Open %s in your browser and enter code: %s\n",
		auth.Hyperlink(dc.VerificationURL, dc.VerificationURL, "", true), dc.UserCode)
	fmt.Println("Waiting for approval...")

	token, err := auth.PollToken(ctx, hc, dc, nil)
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

// reauth handles a rejected stored token: ask the user, delete the token
// file, run the device flow again, and verify the new token. Returns the
// login on success.
func reauth(ctx context.Context) (string, error) {
	store, err := auth.NewFileStore()
	if err != nil {
		return "", err
	}
	fmt.Printf("\nGitHub rejected the token stored in %s: it is invalid, expired, or was revoked.\n", store.Path)
	fmt.Print("Remove it and authenticate again? [Y/n] ")

	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "", "y", "yes":
	default:
		return "", fmt.Errorf("token rejected; remove %s manually and run gpk again", store.Path)
	}
	if err := os.Remove(store.Path); err != nil && !os.IsNotExist(err) {
		return "", err
	}

	token, err := deviceFlow(ctx, "Old token removed.")
	if err != nil {
		return "", err
	}
	client := gh.NewClient(token)
	return client.ViewerLogin(ctx)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "gpk:", err)
	os.Exit(1)
}
