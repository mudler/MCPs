package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ThingSummary is the compact form returned by the thing listing. openHAB's
// own payload carries every channel, which dwarfs the part that matters.
type ThingSummary struct {
	UID          string `json:"uid" jsonschema:"the thing UID, used to ask for one thing's status"`
	ThingTypeUID string `json:"thing_type_uid" jsonschema:"the binding and thing type, for example zwave:device"`
	Label        string `json:"label" jsonschema:"human-readable label"`
	Status       string `json:"status" jsonschema:"ONLINE, OFFLINE, UNINITIALIZED or similar"`
	StatusDetail string `json:"status_detail,omitempty" jsonschema:"why it is in that status, for example COMMUNICATION_ERROR"`
}

type listThingsInput struct {
	FilterUID   string `json:"filter_uid,omitempty" jsonschema:"only things whose UID contains this text, case-insensitive"`
	FilterLabel string `json:"filter_label,omitempty" jsonschema:"only things whose label contains this text, case-insensitive"`
	Page        int    `json:"page,omitempty" jsonschema:"1-based page number, defaults to 1"`
	PageSize    int    `json:"page_size,omitempty" jsonschema:"things per page, defaults to 50 and is capped at 200"`
}

type listThingsOutput struct {
	Things   []ThingSummary `json:"things" jsonschema:"the things on this page"`
	Total    int            `json:"total" jsonschema:"how many things matched the filters, before pagination"`
	Page     int            `json:"page" jsonschema:"the page that was returned"`
	PageSize int            `json:"page_size" jsonschema:"the page size that was applied"`
	Success  bool           `json:"success" jsonschema:"whether the operation succeeded"`
	Error    string         `json:"error,omitempty" jsonschema:"error message if it failed"`
}

// listThings reports the bindings' things and whether they are healthy.
func (s *server) listThings(ctx context.Context, _ *mcp.CallToolRequest, input listThingsInput) (
	*mcp.CallToolResult,
	listThingsOutput,
	error,
) {
	page, pageSize, err := paginationOf(input.Page, input.PageSize)
	if err != nil {
		return nil, listThingsOutput{Error: err.Error()}, nil
	}

	things, err := s.client.Things(ctx)
	if err != nil {
		return nil, listThingsOutput{Error: err.Error()}, nil
	}

	matched := make([]ThingSummary, 0, len(things))
	for _, thing := range things {
		if !contains(thing.UID, input.FilterUID) || !contains(thing.Label, input.FilterLabel) {
			continue
		}
		matched = append(matched, ThingSummary{
			UID:          thing.UID,
			ThingTypeUID: thing.ThingTypeUID,
			Label:        thing.Label,
			Status:       thing.StatusInfo.Status,
			StatusDetail: thing.StatusInfo.StatusDetail,
		})
	}

	return nil, listThingsOutput{
		Things:   pageOf(matched, page, pageSize),
		Total:    len(matched),
		Page:     page,
		PageSize: pageSize,
		Success:  true,
	}, nil
}

type getThingStatusInput struct {
	UID string `json:"uid" jsonschema:"the thing UID, exactly as the thing listing reports it"`
}

type getThingStatusOutput struct {
	UID     string      `json:"uid" jsonschema:"the thing that was queried"`
	Status  ThingStatus `json:"status" jsonschema:"status, detail and description"`
	Success bool        `json:"success" jsonschema:"whether the operation succeeded"`
	Error   string      `json:"error,omitempty" jsonschema:"error message if it failed"`
}

// getThingStatus reports one thing's health, including the description a
// binding attaches when something is wrong.
func (s *server) getThingStatus(ctx context.Context, _ *mcp.CallToolRequest, input getThingStatusInput) (
	*mcp.CallToolResult,
	getThingStatusOutput,
	error,
) {
	uid := strings.TrimSpace(input.UID)
	if uid == "" {
		return nil, getThingStatusOutput{
			Error: fmt.Sprintf("uid is required (the thing UID, as reported by %s)", s.tool("list_things")),
		}, nil
	}

	status, err := s.client.ThingStatus(ctx, uid)
	if err != nil {
		return nil, getThingStatusOutput{UID: uid, Error: err.Error()}, nil
	}

	return nil, getThingStatusOutput{UID: uid, Status: status, Success: true}, nil
}

// RuleSummary is the compact form returned by the rule listing.
type RuleSummary struct {
	UID         string   `json:"uid" jsonschema:"the rule UID, used to run the rule now"`
	Name        string   `json:"name" jsonschema:"human-readable rule name"`
	Description string   `json:"description,omitempty" jsonschema:"what the rule does, when the author wrote it down"`
	Tags        []string `json:"tags,omitempty" jsonschema:"tags on the rule"`
	Status      string   `json:"status" jsonschema:"IDLE, RUNNING or UNINITIALIZED"`
}

type listRulesInput struct {
	FilterTag string `json:"filter_tag,omitempty" jsonschema:"only rules carrying this tag; openHAB applies this filter itself"`
	Page      int    `json:"page,omitempty" jsonschema:"1-based page number, defaults to 1"`
	PageSize  int    `json:"page_size,omitempty" jsonschema:"rules per page, defaults to 50 and is capped at 200"`
}

type listRulesOutput struct {
	Rules    []RuleSummary `json:"rules" jsonschema:"the rules on this page"`
	Total    int           `json:"total" jsonschema:"how many rules matched, before pagination"`
	Page     int           `json:"page" jsonschema:"the page that was returned"`
	PageSize int           `json:"page_size" jsonschema:"the page size that was applied"`
	Success  bool          `json:"success" jsonschema:"whether the operation succeeded"`
	Error    string        `json:"error,omitempty" jsonschema:"error message if it failed"`
}

// listRules reports the rules openHAB knows about. The tag filter is openHAB's
// own, so it happens server-side.
func (s *server) listRules(ctx context.Context, _ *mcp.CallToolRequest, input listRulesInput) (
	*mcp.CallToolResult,
	listRulesOutput,
	error,
) {
	page, pageSize, err := paginationOf(input.Page, input.PageSize)
	if err != nil {
		return nil, listRulesOutput{Error: err.Error()}, nil
	}

	rules, err := s.client.Rules(ctx, strings.TrimSpace(input.FilterTag))
	if err != nil {
		return nil, listRulesOutput{Error: err.Error()}, nil
	}

	summaries := make([]RuleSummary, 0, len(rules))
	for _, rule := range rules {
		summaries = append(summaries, RuleSummary{
			UID:         rule.UID,
			Name:        rule.Name,
			Description: rule.Description,
			Tags:        rule.Tags,
			Status:      rule.Status.Status,
		})
	}

	return nil, listRulesOutput{
		Rules:    pageOf(summaries, page, pageSize),
		Total:    len(summaries),
		Page:     page,
		PageSize: pageSize,
		Success:  true,
	}, nil
}

type runRuleInput struct {
	UID string `json:"uid" jsonschema:"the rule UID, exactly as the rule listing reports it"`
}

type runRuleOutput struct {
	UID     string `json:"uid" jsonschema:"the rule that was run"`
	Success bool   `json:"success" jsonschema:"whether the operation succeeded"`
	Error   string `json:"error,omitempty" jsonschema:"error message if it failed"`
}

// runRule triggers a rule immediately, as the openHAB UI's "run now" does.
func (s *server) runRule(ctx context.Context, _ *mcp.CallToolRequest, input runRuleInput) (
	*mcp.CallToolResult,
	runRuleOutput,
	error,
) {
	uid := strings.TrimSpace(input.UID)
	if uid == "" {
		return nil, runRuleOutput{
			Error: fmt.Sprintf("uid is required (the rule UID, as reported by %s)", s.tool("list_rules")),
		}, nil
	}

	if err := s.client.RunRule(ctx, uid); err != nil {
		return nil, runRuleOutput{UID: uid, Error: err.Error()}, nil
	}

	return nil, runRuleOutput{UID: uid, Success: true}, nil
}
