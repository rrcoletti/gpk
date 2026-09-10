package gh

import (
	"context"
	"fmt"
	"strings"
)

// Project is a GitHub Projects v2 project.
type Project struct {
	ID     string
	Number int
	Title  string
	Closed bool
	URL    string
}

// SelectOption is one option of a single-select field (e.g. a Status column).
type SelectOption struct {
	ID    string
	Name  string
	Color string // GitHub color name, e.g. GREEN; empty for NONE
}

// FieldDef is a project field definition.
type FieldDef struct {
	ID      string
	Name    string
	Options []SelectOption // non-nil for single-select fields
}

// IsSingleSelect reports whether the field is a single-select (has options).
func (f FieldDef) IsSingleSelect() bool { return f.Options != nil }

// ListViewerProjects fetches the viewer's projects, most recently updated
// first. pageSize caps the page; cursor "" fetches the first page. The
// returned cursor is "" when there are no more pages.
func (c *Client) ListViewerProjects(ctx context.Context, pageSize int, cursor string) ([]Project, string, error) {
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 50
	}
	q := `
		query($first: Int!, $after: String) {
			viewer {
				projectsV2(first: $first, after: $after,
					orderBy: {field: UPDATED_AT, direction: DESC}) {
					pageInfo { hasNextPage endCursor }
					nodes {
						id
						number
						title
						closed
						url
					}
				}
			}
		}`
	vars := map[string]any{"first": pageSize}
	if cursor != "" {
		vars["after"] = cursor
	}

	var out struct {
		Viewer struct {
			ProjectsV2 struct {
				PageInfo struct {
					HasNextPage bool   `json:"hasNextPage"`
					EndCursor   string `json:"endCursor"`
				} `json:"pageInfo"`
				Nodes []struct {
					ID     string `json:"id"`
					Number int    `json:"number"`
					Title  string `json:"title"`
					Closed bool   `json:"closed"`
					URL    string `json:"url"`
				} `json:"nodes"`
			} `json:"projectsV2"`
		} `json:"viewer"`
	}
	if err := c.Query(ctx, q, vars, &out); err != nil {
		return nil, "", fmt.Errorf("list projects: %w", err)
	}

	projects := make([]Project, 0, len(out.Viewer.ProjectsV2.Nodes))
	for _, n := range out.Viewer.ProjectsV2.Nodes {
		projects = append(projects, Project{
			ID:     n.ID,
			Number: n.Number,
			Title:  n.Title,
			Closed: n.Closed,
			URL:    n.URL,
		})
	}
	end := ""
	if out.Viewer.ProjectsV2.PageInfo.HasNextPage {
		end = out.Viewer.ProjectsV2.PageInfo.EndCursor
	}
	return projects, end, nil
}

// GetProjectFields fetches all field definitions of a project.
func (c *Client) GetProjectFields(ctx context.Context, projectID string) ([]FieldDef, error) {
	q := `
		query($id: ID!) {
			node(id: $id) {
				... on ProjectV2 {
					fields(first: 100) {
						nodes {
							__typename
							... on ProjectV2SingleSelectField {
								id
								name
								options { id name color }
							}
							... on ProjectV2Field { id name }
							... on ProjectV2IterationField { id name }
						}
					}
				}
			}
		}`
	vars := map[string]any{"id": projectID}

	var out struct {
		Node struct {
			Fields struct {
				Nodes []struct {
					Typename string `json:"__typename"`
					ID       string `json:"id"`
					Name     string `json:"name"`
					Options  []struct {
						ID    string `json:"id"`
						Name  string `json:"name"`
						Color string `json:"color"`
					} `json:"options"`
				} `json:"nodes"`
			} `json:"fields"`
		} `json:"node"`
	}
	if err := c.Query(ctx, q, vars, &out); err != nil {
		return nil, fmt.Errorf("get project fields: %w", err)
	}

	fields := make([]FieldDef, 0, len(out.Node.Fields.Nodes))
	for _, n := range out.Node.Fields.Nodes {
		f := FieldDef{ID: n.ID, Name: n.Name}
		if n.Typename == "ProjectV2SingleSelectField" {
			f.Options = make([]SelectOption, 0, len(n.Options))
			for _, o := range n.Options {
				f.Options = append(f.Options, SelectOption{ID: o.ID, Name: o.Name, Color: o.Color})
			}
		}
		fields = append(fields, f)
	}
	return fields, nil
}

// TitleField finds the project's built-in Title field (a text field),
// used to edit draft issue titles. Returns ok=false if absent.
func TitleField(fields []FieldDef) (FieldDef, bool) {
	for _, f := range fields {
		if !f.IsSingleSelect() && strings.EqualFold(f.Name, "Title") {
			return f, true
		}
	}
	return FieldDef{}, false
}

// StatusField picks the column field from a project's fields: the
// single-select field named "Status" (case-insensitive) if present, else the
// first single-select field. Returns ok=false if the project has none.
func StatusField(fields []FieldDef) (FieldDef, bool) {
	for _, f := range fields {
		if f.IsSingleSelect() && strings.EqualFold(f.Name, "Status") {
			return f, true
		}
	}
	for _, f := range fields {
		if f.IsSingleSelect() {
			return f, true
		}
	}
	return FieldDef{}, false
}

// ProjectRepositories returns the repositories linked to a project
// (the project settings' default repository, used for draft-issue
// conversion and workflows). Usually one; may be empty.
func (c *Client) ProjectRepositories(ctx context.Context, projectID string) ([]string, error) {
	q := `
		query($id: ID!) {
			node(id: $id) {
				... on ProjectV2 {
					repositories(first: 10) { nodes { nameWithOwner } }
				}
			}
		}`
	vars := map[string]any{"id": projectID}
	var out struct {
		Node struct {
			Repositories struct {
				Nodes []struct {
					NameWithOwner string `json:"nameWithOwner"`
				} `json:"nodes"`
			} `json:"repositories"`
		} `json:"node"`
	}
	if err := c.Query(ctx, q, vars, &out); err != nil {
		return nil, fmt.Errorf("project repositories: %w", err)
	}
	repos := make([]string, 0, len(out.Node.Repositories.Nodes))
	for _, n := range out.Node.Repositories.Nodes {
		repos = append(repos, n.NameWithOwner)
	}
	return repos, nil
}
