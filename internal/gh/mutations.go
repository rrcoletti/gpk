package gh

import (
	"context"
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
