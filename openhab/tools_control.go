package main

import (
	"context"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// commandOutput is the shared result of the two acting tools.
type commandOutput struct {
	Name    string `json:"name" jsonschema:"the item that was addressed"`
	Sent    string `json:"sent" jsonschema:"the command or state that was sent"`
	Success bool   `json:"success" jsonschema:"whether the operation succeeded"`
	Error   string `json:"error,omitempty" jsonschema:"error message if it failed"`
}

type sendCommandInput struct {
	Name    string `json:"name" jsonschema:"the item name, exactly as the item listing reports it"`
	Command string `json:"command" jsonschema:"the command to send, for example ON, OFF, UP, DOWN, TOGGLE or a number"`
}

// sendCommand posts a command to an item. openHAB propagates a command through
// rules and bindings, which is what turning a device on means.
func (s *server) sendCommand(ctx context.Context, _ *mcp.CallToolRequest, input sendCommandInput) (
	*mcp.CallToolResult,
	commandOutput,
	error,
) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, commandOutput{Error: s.requireItemName()}, nil
	}
	command := strings.TrimSpace(input.Command)
	if command == "" {
		return nil, commandOutput{Name: name, Error: "command is required (for example ON, OFF or a number)"}, nil
	}

	if err := s.client.SendCommand(ctx, name, command); err != nil {
		return nil, commandOutput{Name: name, Sent: command, Error: s.describeItemError(name, err)}, nil
	}

	return nil, commandOutput{Name: name, Sent: command, Success: true}, nil
}

type updateStateInput struct {
	Name  string `json:"name" jsonschema:"the item name, exactly as the item listing reports it"`
	State string `json:"state" jsonschema:"the state to set, for example ON, OFF or a number"`
}

// updateState sets an item's state without triggering rules. Sending a command
// acts on a device; this records a value.
func (s *server) updateState(ctx context.Context, _ *mcp.CallToolRequest, input updateStateInput) (
	*mcp.CallToolResult,
	commandOutput,
	error,
) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, commandOutput{Error: s.requireItemName()}, nil
	}
	state := strings.TrimSpace(input.State)
	if state == "" {
		return nil, commandOutput{Name: name, Error: "state is required (for example ON, OFF or a number)"}, nil
	}

	if err := s.client.UpdateState(ctx, name, state); err != nil {
		return nil, commandOutput{Name: name, Sent: state, Error: s.describeItemError(name, err)}, nil
	}

	return nil, commandOutput{Name: name, Sent: state, Success: true}, nil
}
