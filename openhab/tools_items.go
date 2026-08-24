package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	defaultPageSize = 50
	maxPageSize     = 200
)

// ItemSummary is the compact form returned by listings. get_item returns the
// full Item when more is needed.
type ItemSummary struct {
	Name  string `json:"name" jsonschema:"the item name, used by every other tool"`
	Type  string `json:"type" jsonschema:"item type, for example Switch, Number, String or Group"`
	State string `json:"state" jsonschema:"current state as openHAB reports it"`
	Label string `json:"label" jsonschema:"human-readable label, often in the user's own language"`
}

type listItemsInput struct {
	FilterName  string `json:"filter_name,omitempty" jsonschema:"only items whose name contains this text, case-insensitive"`
	FilterLabel string `json:"filter_label,omitempty" jsonschema:"only items whose label contains this text, case-insensitive"`
	FilterType  string `json:"filter_type,omitempty" jsonschema:"only items of this type, for example Switch or Number"`
	FilterTag   string `json:"filter_tag,omitempty" jsonschema:"only items carrying this tag, for example Switchable"`
	Page        int    `json:"page,omitempty" jsonschema:"1-based page number, defaults to 1"`
	PageSize    int    `json:"page_size,omitempty" jsonschema:"items per page, defaults to 50 and is capped at 200"`
}

type listItemsOutput struct {
	Items    []ItemSummary `json:"items" jsonschema:"the items on this page"`
	Total    int           `json:"total" jsonschema:"how many items matched the filters, before pagination"`
	Page     int           `json:"page" jsonschema:"the page that was returned"`
	PageSize int           `json:"page_size" jsonschema:"the page size that was applied"`
	Success  bool          `json:"success" jsonschema:"whether the operation succeeded"`
	Error    string        `json:"error,omitempty" jsonschema:"error message if it failed"`
}

// listItems returns a compact view of the items, filtered and paginated here
// because openHAB's REST API returns the whole collection.
func (s *server) listItems(ctx context.Context, _ *mcp.CallToolRequest, input listItemsInput) (
	*mcp.CallToolResult,
	listItemsOutput,
	error,
) {
	page, pageSize, err := paginationOf(input.Page, input.PageSize)
	if err != nil {
		return nil, listItemsOutput{Error: err.Error()}, nil
	}

	items, err := s.client.Items(ctx)
	if err != nil {
		return nil, listItemsOutput{Error: err.Error()}, nil
	}

	matched := make([]ItemSummary, 0, len(items))
	for _, item := range items {
		if !contains(item.Name, input.FilterName) ||
			!contains(item.Label, input.FilterLabel) ||
			!equalFold(item.Type, input.FilterType) ||
			!hasTag(item.Tags, input.FilterTag) {
			continue
		}
		matched = append(matched, ItemSummary{
			Name:  item.Name,
			Type:  item.Type,
			State: item.State,
			Label: item.Label,
		})
	}

	return nil, listItemsOutput{
		Items:    pageOf(matched, page, pageSize),
		Total:    len(matched),
		Page:     page,
		PageSize: pageSize,
		Success:  true,
	}, nil
}

type getItemInput struct {
	Name         string `json:"name" jsonschema:"the item name, exactly as list_items reports it"`
	WithMetadata bool   `json:"with_metadata,omitempty" jsonschema:"include the item's metadata namespaces"`
}

type getItemOutput struct {
	Item    Item   `json:"item" jsonschema:"the full item, including tags and group membership"`
	Success bool   `json:"success" jsonschema:"whether the operation succeeded"`
	Error   string `json:"error,omitempty" jsonschema:"error message if it failed"`
}

// getItem returns one item in full.
func (s *server) getItem(ctx context.Context, _ *mcp.CallToolRequest, input getItemInput) (
	*mcp.CallToolResult,
	getItemOutput,
	error,
) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, getItemOutput{Error: "name is required (the item name, as reported by list_items)"}, nil
	}

	item, err := s.client.Item(ctx, name, input.WithMetadata)
	if err != nil {
		return nil, getItemOutput{Error: describeItemError(name, err)}, nil
	}

	return nil, getItemOutput{Item: item, Success: true}, nil
}

// describeItemError turns a 404 into language a model can act on.
func describeItemError(name string, err error) string {
	if errors.Is(err, errNotFound) {
		return fmt.Sprintf("no item named %q; use list_items to find the exact name", name)
	}
	return err.Error()
}

// paginationOf validates and defaults the paging inputs.
func paginationOf(page, pageSize int) (int, int, error) {
	if page < 0 {
		return 0, 0, fmt.Errorf("page must be 1 or greater, got %d", page)
	}
	if pageSize < 0 {
		return 0, 0, fmt.Errorf("page_size must be 1 or greater, got %d", pageSize)
	}
	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	return page, pageSize, nil
}

// pageOf slices one page out of rows, returning empty past the end.
func pageOf[T any](rows []T, page, pageSize int) []T {
	start := (page - 1) * pageSize
	if start >= len(rows) {
		return []T{}
	}
	end := start + pageSize
	if end > len(rows) {
		end = len(rows)
	}
	return rows[start:end]
}

// contains reports whether value holds needle, case-insensitively. An empty
// needle matches everything.
func contains(value, needle string) bool {
	if needle == "" {
		return true
	}
	return strings.Contains(strings.ToLower(value), strings.ToLower(needle))
}

// equalFold compares case-insensitively. An empty want matches everything.
func equalFold(value, want string) bool {
	if want == "" {
		return true
	}
	return strings.EqualFold(value, want)
}

// hasTag reports whether tags holds want. An empty want matches everything.
func hasTag(tags []string, want string) bool {
	if want == "" {
		return true
	}
	for _, tag := range tags {
		if strings.EqualFold(tag, want) {
			return true
		}
	}
	return false
}
