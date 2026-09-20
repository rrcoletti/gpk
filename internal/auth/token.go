// gpk - read-only TUI for GitHub Projects v2 kanban boards
// Copyright (C) 2026  Rafael Coletti
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.
//
// Package auth provides GitHub token acquisition and storage.
//
// Token resolution order: GITHUB_TOKEN environment variable (override),
// then the token file (~/.config/gpk/env), then the device flow.
// All sources implement TokenSource so PAT or keyring providers can be
// added later without touching callers.
package auth

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrNoToken is returned when no token is available from a source.
var ErrNoToken = errors.New("no github token available")

// TokenSource provides a GitHub token.
type TokenSource interface {
	GetToken() (string, error)
}

// EnvVar is the environment variable consulted first.
const EnvVar = "GITHUB_TOKEN"

// EnvSource reads the token from the environment.
type EnvSource struct{}

func (EnvSource) GetToken() (string, error) {
	t := os.Getenv(EnvVar)
	if t == "" {
		return "", ErrNoToken
	}
	return t, nil
}

// FileStore reads and writes the token file. The file holds shell-style
// KEY=VALUE lines (only GITHUB_TOKEN is currently meaningful) and is
// created with 0600 permissions inside a 0700 directory.
type FileStore struct {
	Path string
}

// DefaultPath returns ~/.config/gpk/env, honoring XDG_CONFIG_HOME.
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve config dir: %w", err)
	}
	return filepath.Join(dir, "gpk", "env"), nil
}

// NewFileStore returns a FileStore at the default path.
func NewFileStore() (FileStore, error) {
	p, err := DefaultPath()
	if err != nil {
		return FileStore{}, err
	}
	return FileStore{Path: p}, nil
}

// GetToken reads GITHUB_TOKEN from the token file.
func (s FileStore) GetToken() (string, error) {
	data, err := os.ReadFile(s.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", ErrNoToken
		}
		return "", err
	}
	token, err := parseEnvFile(string(data))
	if err != nil {
		return "", fmt.Errorf("%s: %w", s.Path, err)
	}
	return token, nil
}

// Save writes the token to the file, creating the directory if needed.
// Existing file permissions are preserved so a user-chosen mode sticks.
func (s FileStore) Save(token string) error {
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return err
	}
	mode := os.FileMode(0o600)
	if fi, err := os.Stat(s.Path); err == nil {
		mode = fi.Mode().Perm()
	}
	return os.WriteFile(s.Path, []byte(EnvVar+"="+token+"\n"), mode)
}

// parseEnvFile extracts GITHUB_TOKEN from KEY=VALUE lines.
// Blank lines and # comments are ignored.
func parseEnvFile(data string) (string, error) {
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if strings.TrimSpace(k) == EnvVar {
			v = strings.TrimSpace(v)
			if v == "" {
				return "", errors.New("empty GITHUB_TOKEN")
			}
			return v, nil
		}
	}
	return "", ErrNoToken
}

// Chain tries each source in order and returns the first success.
type Chain struct {
	Sources []TokenSource
}

func (c Chain) GetToken() (string, error) {
	for _, src := range c.Sources {
		t, err := src.GetToken()
		if err == nil {
			return t, nil
		}
		if !errors.Is(err, ErrNoToken) {
			return "", err
		}
	}
	return "", ErrNoToken
}

// DefaultSource resolves: env override, then token file.
func DefaultSource() (TokenSource, error) {
	store, err := NewFileStore()
	if err != nil {
		return nil, err
	}
	return Chain{Sources: []TokenSource{EnvSource{}, store}}, nil
}
