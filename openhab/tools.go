package main

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// toolSpec pairs a tool's advertised identity with the call that registers it,
// so the set of tools a configuration exposes is one list rather than a
// scattering of conditionals in main.
type toolSpec struct {
	name        string
	description string
	add         func(*mcp.Server)
}

// toolSpecs returns the tools this configuration exposes. Restricted tools are
// never registered rather than registered-and-refusing, so a model does not
// waste a call discovering it is not allowed.
func (s *server) toolSpecs() []toolSpec {
	specs := []toolSpec{
		spec("list_items", listItemsDescription, s.listItems),
		spec("get_item", getItemDescription, s.getItem),
		spec("list_things", listThingsDescription, s.listThings),
		spec("get_thing_status", getThingStatusDescription, s.getThingStatus),
		spec("list_rules", listRulesDescription, s.listRules),
	}

	if s.cfg.ReadOnly {
		return specs
	}

	return append(specs,
		spec("send_command", sendCommandDescription, s.sendCommand),
		spec("update_item_state", updateStateDescription, s.updateState),
		spec("run_rule_now", runRuleDescription, s.runRule),
	)
}

// spec captures one tool's registration, deferring the AddTool call so the
// list can be inspected without an MCP server in hand.
func spec[In, Out any](
	name, description string,
	handler func(context.Context, *mcp.CallToolRequest, In) (*mcp.CallToolResult, Out, error),
) toolSpec {
	return toolSpec{
		name:        name,
		description: description,
		add: func(server *mcp.Server) {
			mcp.AddTool(server, &mcp.Tool{Name: name, Description: description}, handler)
		},
	}
}

const (
	listItemsDescription      = "List openHAB items with their current state. Items are the things you read and control: switches, sensors, setpoints. Filter by name, label, type or tag, and page through the results. Always find the exact item name here before acting on it."
	getItemDescription        = "Get one openHAB item in full, including its tags and the groups it belongs to. Use list_items first to find the exact name."
	sendCommandDescription    = "Send a command to an openHAB item, for example ON, OFF, UP, DOWN, TOGGLE or a number. This is how you act on the house: the command travels through rules and bindings to the device."
	updateStateDescription    = "Set an openHAB item's state directly, without triggering rules. Use this to record a value; use send_command to act on a device."
	listThingsDescription     = "List openHAB things with their status. Things are the physical devices and bindings behind the items. Use this to find out whether something is ONLINE before blaming a rule."
	getThingStatusDescription = "Get one openHAB thing's status, including the detail and description a binding reports when it is offline."
	listRulesDescription      = "List openHAB rules with their status, optionally filtered by tag. Rules are the automations that run on triggers."
	runRuleDescription        = "Run an openHAB rule immediately, as the 'run now' button in the openHAB UI does. Use list_rules to find the exact UID."
)
