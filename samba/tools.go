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
	prefix := s.cfg.ToolPrefix

	specs := []toolSpec{
		spec(prefix, "list", listDescription, s.list),
		spec(prefix, "search", searchDescription, s.search),
		spec(prefix, "read", readDescription, s.read),
	}

	if s.cfg.ReadOnly {
		return specs
	}

	specs = append(specs,
		spec(prefix, "write", writeDescription, s.write),
		spec(prefix, "move", moveDescription, s.move),
	)

	// SMB_DISABLE_DELETE hides the tool outright; the handlers of the tools
	// above independently refuse their destructive variants, so the guarantee
	// does not rest on registration alone.
	if s.cfg.DisableDelete {
		return specs
	}

	return append(specs, spec(prefix, "delete", deleteDescription, s.remove))
}

// spec captures one tool's registration, deferring the AddTool call so the
// list can be inspected without an MCP server in hand. The configured prefix
// is applied here, so a tool is advertised and registered under one name.
func spec[In, Out any](
	prefix, name, description string,
	handler func(context.Context, *mcp.CallToolRequest, In) (*mcp.CallToolResult, Out, error),
) toolSpec {
	prefixed := prefix + name
	return toolSpec{
		name:        prefixed,
		description: description,
		add: func(server *mcp.Server) {
			mcp.AddTool(server, &mcp.Tool{Name: prefixed, Description: description}, handler)
		},
	}
}

const (
	listDescription   = "List the files and directories directly inside a directory on the SMB share. Paths are relative to the root of the share; leave path empty for the share root."
	searchDescription = "Find files and directories on the SMB share by name, matching a glob such as *.gguf or plain text as a substring. Only names are matched, never file contents."
	readDescription   = "Read a text file from the SMB share, returning its content with line numbers. Refuses binary files and files above the configured size limit."
	writeDescription  = "Write content to a file on the SMB share, creating parent directories as needed and replacing any existing file."
	moveDescription   = "Move or rename a file or directory within the SMB share. Refuses to replace an existing destination unless overwrite is set."
	deleteDescription = "Delete a file or directory from the SMB share. A directory with contents is only removed when recursive is set."
)
