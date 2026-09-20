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
package gh

import (
	"context"
	"fmt"
)

// ProjectItem is one item on a project board with its status resolution.
type Item struct {
	ID        string
	Title     string
	Type      string // "Issue", "PullRequest", "DraftIssue"
	Number    int    // 0 for drafts
	URL       string
	Assignee  string // first assignee login, "" if none
	OptionID  string // status option id; "" = no status set
	Body      string // markdown body, may be empty
	Repo      string // "owner/name", empty for drafts
	ContentID string // node id of the Issue/PullRequest, "" for drafts
	// Fields holds the item's other single-select field values (Priority,
	// Size, ...) in field order, status excluded.
	Fields []ItemField
}

// ItemField is one non-status single-select field value.
type ItemField struct {
	Field string
	Value string
}

// GetProjectItems fetches one page of a project's items, resolving each
// item's status option against statusFieldID. Returns the page plus the
// next cursor ("" when exhausted).
func (c *Client) GetProjectItems(ctx context.Context, projectID, statusFieldID string, first int, after string) ([]Item, string, error) {
	if first <= 0 || first > 100 {
		first = 50
	}
	q := `
		query($id: ID!, $first: Int!, $after: String) {
			node(id: $id) {
				... on ProjectV2 {
					items(first: $first, after: $after) {
						pageInfo { hasNextPage endCursor }
						nodes {
							id
							type
							fieldValues(first: 30) {
								nodes {
									__typename
									... on ProjectV2ItemFieldSingleSelectValue {
										optionId
										name
										field {
											... on ProjectV2SingleSelectField { id name }
										}
									}
								}
							}
							content {
								__typename
								... on Issue {
									id
									title
									number
									url
									body
									repository { nameWithOwner }
									assignees(first: 5) { nodes { login } }
								}
								... on PullRequest {
									id
									title
									number
									url
									body
									repository { nameWithOwner }
									assignees(first: 5) { nodes { login } }
								}
								... on DraftIssue { title body }
							}
						}
					}
				}
			}
		}`
	vars := map[string]any{"id": projectID, "first": first}
	if after != "" {
		vars["after"] = after
	}

	var out struct {
		Node struct {
			Items struct {
				PageInfo struct {
					HasNextPage bool   `json:"hasNextPage"`
					EndCursor   string `json:"endCursor"`
				} `json:"pageInfo"`
				Nodes []struct {
					ID          string `json:"id"`
					Type        string `json:"type"`
					FieldValues struct {
						Nodes []struct {
							Typename string `json:"__typename"`
							OptionID string `json:"optionId"`
							Name     string `json:"name"`
							Field    struct {
								ID   string `json:"id"`
								Name string `json:"name"`
							} `json:"field"`
						} `json:"nodes"`
					} `json:"fieldValues"`
					Content struct {
						ID         string `json:"id"`
						Typename   string `json:"__typename"`
						Title      string `json:"title"`
						Number     int    `json:"number"`
						URL        string `json:"url"`
						Body       string `json:"body"`
						Repository struct {
							NameWithOwner string `json:"nameWithOwner"`
						} `json:"repository"`
						Assignees struct {
							Nodes []struct {
								Login string `json:"login"`
							} `json:"nodes"`
						} `json:"assignees"`
					} `json:"content"`
				} `json:"nodes"`
			} `json:"items"`
		} `json:"node"`
	}
	if err := c.Query(ctx, q, vars, &out); err != nil {
		return nil, "", fmt.Errorf("get project items: %w", err)
	}

	items := make([]Item, 0, len(out.Node.Items.Nodes))
	for _, n := range out.Node.Items.Nodes {
		it := Item{
			ID:    n.ID,
			Title: n.Content.Title,
			Type:  n.Content.Typename,
		}
		if n.Content.Typename != "DraftIssue" {
			it.Number = n.Content.Number
			it.URL = n.Content.URL
			it.Repo = n.Content.Repository.NameWithOwner
			it.ContentID = n.Content.ID
		}
		it.Body = n.Content.Body
		if len(n.Content.Assignees.Nodes) > 0 {
			it.Assignee = n.Content.Assignees.Nodes[0].Login
		}
		for _, fv := range n.FieldValues.Nodes {
			if fv.Typename != "ProjectV2ItemFieldSingleSelectValue" {
				continue
			}
			if fv.Field.ID == statusFieldID {
				it.OptionID = fv.OptionID
				continue
			}
			if fv.Field.Name != "" && fv.Name != "" {
				it.Fields = append(it.Fields, ItemField{Field: fv.Field.Name, Value: fv.Name})
			}
		}
		items = append(items, it)
	}

	cursor := ""
	if out.Node.Items.PageInfo.HasNextPage {
		cursor = out.Node.Items.PageInfo.EndCursor
	}
	return items, cursor, nil
}

// FetchAllItems pages through all items of a project.
func (c *Client) FetchAllItems(ctx context.Context, projectID, statusFieldID string) ([]Item, error) {
	var all []Item
	cursor := ""
	for {
		page, next, err := c.GetProjectItems(ctx, projectID, statusFieldID, 100, cursor)
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
