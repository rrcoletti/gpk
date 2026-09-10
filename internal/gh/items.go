package gh

import (
	"context"
	"fmt"
)

// ProjectItem is one item on a project board with its status resolution.
type Item struct {
	ID       string
	Title    string
	Type     string // "Issue", "PullRequest", "DraftIssue"
	Number   int    // 0 for drafts
	URL      string
	Assignee string // first assignee login, "" if none
	OptionID string // status option id; "" = no status set
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
										field {
											... on ProjectV2SingleSelectField { id }
										}
									}
								}
							}
							content {
								__typename
								... on Issue {
									title
									number
									url
									assignees(first: 5) { nodes { login } }
								}
								... on PullRequest {
									title
									number
									url
									assignees(first: 5) { nodes { login } }
								}
								... on DraftIssue { title }
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
							Field    struct {
								ID string `json:"id"`
							} `json:"field"`
						} `json:"nodes"`
					} `json:"fieldValues"`
					Content struct {
						Typename  string `json:"__typename"`
						Title     string `json:"title"`
						Number    int    `json:"number"`
						URL       string `json:"url"`
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
		}
		if len(n.Content.Assignees.Nodes) > 0 {
			it.Assignee = n.Content.Assignees.Nodes[0].Login
		}
		for _, fv := range n.FieldValues.Nodes {
			if fv.Typename == "ProjectV2ItemFieldSingleSelectValue" && fv.Field.ID == statusFieldID {
				it.OptionID = fv.OptionID
				break
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
