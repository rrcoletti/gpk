package gh

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// SetItemStatus moves an item to the column identified by optionID.
// optionID "" clears the field value (the "No Status" column).
func (c *Client) SetItemStatus(ctx context.Context, projectID, itemID, fieldID, optionID string) error {
	if optionID == "" {
		return c.clearItemStatus(ctx, projectID, itemID, fieldID)
	}
	m := `
		mutation($project: ID!, $item: ID!, $field: ID!, $option: String!) {
			updateProjectV2ItemFieldValue(input: {
				projectId: $project
				itemId: $item
				fieldId: $field
				value: {singleSelectOptionId: $option}
			}) {
				projectV2Item { id }
			}
		}`
	var out struct {
		UpdateProjectV2ItemFieldValue struct {
			ProjectV2Item struct {
				ID string `json:"id"`
			} `json:"projectV2Item"`
		} `json:"updateProjectV2ItemFieldValue"`
	}
	err := c.Query(ctx, m, map[string]any{
		"project": projectID, "item": itemID, "field": fieldID, "option": optionID,
	}, &out)
	if err != nil {
		return scopeHint(err)
	}
	return nil
}

// clearItemStatus removes the single-select value (moving to "No Status").
func (c *Client) clearItemStatus(ctx context.Context, projectID, itemID, fieldID string) error {
	m := `
		mutation($project: ID!, $item: ID!, $field: ID!) {
			clearProjectV2ItemFieldValue(input: {
				projectId: $project
				itemId: $item
				fieldId: $field
			}) {
				projectV2Item { id }
			}
		}`
	var out struct {
		ClearProjectV2ItemFieldValue struct {
			ProjectV2Item struct {
				ID string `json:"id"`
			} `json:"projectV2Item"`
		} `json:"clearProjectV2ItemFieldValue"`
	}
	err := c.Query(ctx, m, map[string]any{
		"project": projectID, "item": itemID, "field": fieldID,
	}, &out)
	return scopeHint(err)
}

// SetItemTitle sets an item's title. Issues are edited via updateIssue,
// pull requests via updatePullRequest (the project Title field is read-only
// for both); draft items are edited via the project's built-in Title field.
func (c *Client) SetItemTitle(ctx context.Context, itemType, contentID, projectID, itemID, titleFieldID, title string) error {
	switch itemType {
	case "DraftIssue":
		return c.setDraftItemTitle(ctx, projectID, itemID, titleFieldID, title)
	case "PullRequest":
		m := `
			mutation($id: ID!, $title: String!) {
				updatePullRequest(input: {pullRequestId: $id, title: $title}) {
					pullRequest { id }
				}
			}`
		var out struct {
			UpdatePullRequest struct {
				PullRequest struct {
					ID string `json:"id"`
				} `json:"pullRequest"`
			} `json:"updatePullRequest"`
		}
		return scopeHint(c.Query(ctx, m, map[string]any{"id": contentID, "title": title}, &out))
	default: // Issue
		m := `
			mutation($id: ID!, $title: String!) {
				updateIssue(input: {id: $id, title: $title}) {
					issue { id }
				}
			}`
		var out struct {
			UpdateIssue struct {
				Issue struct {
					ID string `json:"id"`
				} `json:"issue"`
			} `json:"updateIssue"`
		}
		return scopeHint(c.Query(ctx, m, map[string]any{"id": contentID, "title": title}, &out))
	}
}

// setDraftItemTitle writes the built-in Title text field of a draft item.
func (c *Client) setDraftItemTitle(ctx context.Context, projectID, itemID, titleFieldID, title string) error {
	m := `
		mutation($project: ID!, $item: ID!, $field: ID!, $value: String!) {
			updateProjectV2ItemFieldValue(input: {
				projectId: $project
				itemId: $item
				fieldId: $field
				value: {text: $value}
			}) {
				projectV2Item { id }
			}
		}`
	var out struct {
		UpdateProjectV2ItemFieldValue struct {
			ProjectV2Item struct {
				ID string `json:"id"`
			} `json:"projectV2Item"`
		} `json:"updateProjectV2ItemFieldValue"`
	}
	return scopeHint(c.Query(ctx, m, map[string]any{
		"project": projectID, "item": itemID, "field": titleFieldID, "value": title,
	}, &out))
}

// scopeHint annotates scope-related failures with a fix hint.
func scopeHint(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if strings.Contains(msg, "scopes") || strings.Contains(msg, "Scope") ||
		strings.Contains(msg, "forbidden") || strings.Contains(msg, "forbidden") {
		return fmt.Errorf("%w\nre-auth with write access: remove the token file (~/.config/gpk/env) and run gpk again", err)
	}
	return err
}

// AddDraftItem creates a draft item on the board and, when optionID is
// non-empty, moves it into that status column. Returns the item id.
func (c *Client) AddDraftItem(ctx context.Context, projectID, title, statusFieldID, optionID string) (string, error) {
	m := `
		mutation($p: ID!, $t: String!) {
			addProjectV2DraftIssue(input: {projectId: $p, title: $t}) {
				projectItem { id }
			}
		}`
	var out struct {
		AddProjectV2DraftIssue struct {
			ProjectItem struct {
				ID string `json:"id"`
			} `json:"projectItem"`
		} `json:"addProjectV2DraftIssue"`
	}
	if err := c.Query(ctx, m, map[string]any{"p": projectID, "t": title}, &out); err != nil {
		return "", scopeHint(err)
	}
	id := out.AddProjectV2DraftIssue.ProjectItem.ID
	if id == "" {
		return "", errors.New("addProjectV2DraftIssue returned no item id")
	}
	if optionID != "" {
		if err := c.SetItemStatus(ctx, projectID, id, statusFieldID, optionID); err != nil {
			return id, err
		}
	}
	return id, nil
}

// DeleteItem removes an item from the project. For draft items this deletes
// the draft itself; for linked issues/PRs it only removes them from the
// board (same as GitHub's web UI "Remove from project").
func (c *Client) DeleteItem(ctx context.Context, projectID, itemID string) error {
	m := `
		mutation($p: ID!, $i: ID!) {
			deleteProjectV2Item(input: {projectId: $p, itemId: $i}) {
				deletedItemId
			}
		}`
	var out struct {
		DeleteProjectV2Item struct {
			DeletedItemID string `json:"deletedItemId"`
		} `json:"deleteProjectV2Item"`
	}
	return scopeHint(c.Query(ctx, m, map[string]any{"p": projectID, "i": itemID}, &out))
}

// CreatedIssue describes a newly created issue added to a project.
type CreatedIssue struct {
	ItemID  string // project item id
	IssueID string // issue node id
	Number  int
	URL     string
	Title   string
	Repo    string
}

// CreateIssueInRepo creates a real issue in owner/name, adds it to the
// project and sets its status. repoWithOwner is "owner/name".
func (c *Client) CreateIssueInRepo(ctx context.Context, projectID, repoWithOwner, title, statusFieldID, optionID string) (CreatedIssue, error) {
	owner, name, found := strings.Cut(repoWithOwner, "/")
	if !found {
		return CreatedIssue{}, fmt.Errorf("invalid repository %q (want owner/name)", repoWithOwner)
	}

	// 1. resolve the repository node id
	var repoOut struct {
		Repository struct {
			ID string `json:"id"`
		} `json:"repository"`
	}
	if err := c.Query(ctx, `query($o: String!, $n: String!) {
		repository(owner: $o, name: $n) { id }
	}`, map[string]any{"o": owner, "n": name}, &repoOut); err != nil {
		return CreatedIssue{}, err
	}
	repoID := repoOut.Repository.ID
	if repoID == "" {
		return CreatedIssue{}, fmt.Errorf("repository %s not found", repoWithOwner)
	}

	// 2. create the issue
	var issOut struct {
		CreateIssue struct {
			Issue struct {
				ID     string `json:"id"`
				Number int    `json:"number"`
				URL    string `json:"url"`
				Title  string `json:"title"`
			} `json:"issue"`
		} `json:"createIssue"`
	}
	if err := c.Query(ctx, `mutation($repo: ID!, $title: String!) {
		createIssue(input: {repositoryId: $repo, title: $title}) {
			issue { id number url title }
		}
	}`, map[string]any{"repo": repoID, "title": title}, &issOut); err != nil {
		return CreatedIssue{}, scopeHint(err)
	}
	iss := issOut.CreateIssue.Issue
	if iss.ID == "" {
		return CreatedIssue{}, errors.New("createIssue returned no issue id")
	}

	// 3. add it to the project board
	var addOut struct {
		AddProjectV2ItemById struct {
			Item struct {
				ID string `json:"id"`
			} `json:"item"`
		} `json:"addProjectV2ItemById"`
	}
	if err := c.Query(ctx, `mutation($p: ID!, $c: ID!) {
		addProjectV2ItemById(input: {projectId: $p, contentId: $c}) {
			item { id }
		}
	}`, map[string]any{"p": projectID, "c": iss.ID}, &addOut); err != nil {
		return CreatedIssue{}, scopeHint(err)
	}
	itemID := addOut.AddProjectV2ItemById.Item.ID

	// 4. set the status column
	if optionID != "" {
		if err := c.SetItemStatus(ctx, projectID, itemID, statusFieldID, optionID); err != nil {
			return CreatedIssue{}, err
		}
	}

	return CreatedIssue{
		ItemID: itemID, IssueID: iss.ID, Number: iss.Number,
		URL: iss.URL, Title: iss.Title, Repo: repoWithOwner,
	}, nil
}
