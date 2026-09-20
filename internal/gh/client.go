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
// Package gh is a thin GraphQL client for the GitHub API.
package gh

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

const graphqlURL = "https://api.github.com/graphql"

// Client issues authenticated GraphQL queries to GitHub.
type Client struct {
	http  *http.Client
	token string
}

// NewClient creates a client using the given token.
func NewClient(token string) *Client {
	return &Client{
		http:  &http.Client{Timeout: 30 * time.Second},
		token: token,
	}
}

type graphQLRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

type graphQLError struct {
	Message string `json:"message"`
}

type graphQLResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []graphQLError  `json:"errors"`
}

// ErrUnauthorized is returned when GitHub rejects the credential (HTTP 401),
// so callers can offer re-authentication. Scope problems surface as GraphQL
// errors (403-style messages), not this.
var ErrUnauthorized = errors.New("github rejected the token (401)")

// Query executes a GraphQL query and unmarshals data into out.
func (c *Client) Query(ctx context.Context, query string, vars map[string]any, out any) error {
	body, err := json.Marshal(graphQLRequest{Query: query, Variables: vars})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, graphqlURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("graphql request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("%w: the token is invalid, expired, or was revoked", ErrUnauthorized)
	}

	var gr graphQLResponse
	if err := json.NewDecoder(resp.Body).Decode(&gr); err != nil {
		return fmt.Errorf("decode graphql response: %w", err)
	}
	if len(gr.Errors) > 0 {
		return fmt.Errorf("graphql error: %s", gr.Errors[0].Message)
	}
	if gr.Data == nil {
		return fmt.Errorf("graphql response has no data (http %d)", resp.StatusCode)
	}
	return json.Unmarshal(gr.Data, out)
}

// ViewerLogin returns the authenticated user's login.
func (c *Client) ViewerLogin(ctx context.Context) (string, error) {
	var out struct {
		Viewer struct {
			Login string `json:"login"`
		} `json:"viewer"`
	}
	err := c.Query(ctx, `query { viewer { login } }`, nil, &out)
	if err != nil {
		return "", err
	}
	return out.Viewer.Login, nil
}
