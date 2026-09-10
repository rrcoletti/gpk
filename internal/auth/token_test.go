package auth

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestParseEnvFile(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		want    string
		wantErr error
	}{
		{"simple", "GITHUB_TOKEN=ghp_abc\n", "ghp_abc", nil},
		{"with comments and blanks", "# comment\n\nGITHUB_TOKEN=ghp_x\n", "ghp_x", nil},
		{"spaces around", "GITHUB_TOKEN =  ghp_y \n", "ghp_y", nil},
		{"no trailing newline", "GITHUB_TOKEN=ghp_z", "ghp_z", nil},
		{"other vars first", "EDITOR=vim\nGITHUB_TOKEN=ghp_w\n", "ghp_w", nil},
		{"missing", "EDITOR=vim\n", "", ErrNoToken},
		{"empty value", "GITHUB_TOKEN=\n", "", errors.New("empty")},
		{"malformed line skipped", "notapair\nGITHUB_TOKEN=ghp_v\n", "ghp_v", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseEnvFile(tt.data)
			if tt.wantErr == ErrNoToken {
				if !errors.Is(err, ErrNoToken) {
					t.Fatalf("want ErrNoToken, got %v", err)
				}
				return
			}
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("want error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFileStoreSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gpk", "env")
	store := FileStore{Path: path}

	if _, err := store.GetToken(); !errors.Is(err, ErrNoToken) {
		t.Fatalf("missing file should yield ErrNoToken, got %v", err)
	}

	if err := store.Save("ghp_tok123"); err != nil {
		t.Fatal(err)
	}

	got, err := store.GetToken()
	if err != nil || got != "ghp_tok123" {
		t.Fatalf("got %q, %v", got, err)
	}

	// Permissions: file 0600, directory 0700.
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Errorf("file perm = %o, want 600", perm)
	}
	di, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if perm := di.Mode().Perm(); perm != 0o700 {
		t.Errorf("dir perm = %o, want 700", perm)
	}
}

func TestChainPrecedence(t *testing.T) {
	t.Setenv(EnvVar, "from_env")
	store := FileStore{Path: filepath.Join(t.TempDir(), "nonexistent")}
	chain := Chain{Sources: []TokenSource{EnvSource{}, store}}
	got, err := chain.GetToken()
	if err != nil || got != "from_env" {
		t.Fatalf("env should win: got %q, %v", got, err)
	}
}

func TestChainFallsThroughToStore(t *testing.T) {
	t.Setenv(EnvVar, "")
	path := filepath.Join(t.TempDir(), "env")
	if err := os.WriteFile(path, []byte("GITHUB_TOKEN=from_file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	chain := Chain{Sources: []TokenSource{EnvSource{}, FileStore{Path: path}}}
	got, err := chain.GetToken()
	if err != nil || got != "from_file" {
		t.Fatalf("file should be used: got %q, %v", got, err)
	}
}

func TestFormatUserCode(t *testing.T) {
	for in, want := range map[string]string{
		"ABCD1234": "ABCD-1234",
		"AB12":     "AB12",
		"A-BC1234": "A-BC1234",
	} {
		if got := FormatUserCode(in); got != want {
			t.Errorf("FormatUserCode(%q) = %q, want %q", in, got, want)
		}
	}
}
