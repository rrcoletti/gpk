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
