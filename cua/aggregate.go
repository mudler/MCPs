package main

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// upstream is a connected client session to one of nib's in-process MCP
// servers, together with a label used in error messages.
type upstream struct {
	name    string
	session *mcp.ClientSession
}

// connectUpstream dials an in-memory transport and completes the MCP
// handshake, returning a session ready for ListTools and CallTool.
func connectUpstream(ctx context.Context, name string, t mcp.Transport) (*upstream, error) {
	c := mcp.NewClient(&mcp.Implementation{Name: "cua-aggregator", Version: version}, nil)
	sess, err := c.Connect(ctx, t, nil)
	if err != nil {
		return nil, fmt.Errorf("connect to %s server: %w", name, err)
	}
	return &upstream{name: name, session: sess}, nil
}

// toolAllowed reports whether name passes the allowlist. An empty allowlist
// means no filtering.
func toolAllowed(allow map[string]bool, name string) bool {
	if len(allow) == 0 {
		return true
	}
	return allow[name]
}

// aggregate registers every tool exported by ups onto srv, forwarding calls to
// the owning upstream. Results are passed through verbatim so that image
// content survives intact.
func aggregate(ctx context.Context, srv *mcp.Server, ups []*upstream, allow map[string]bool) error {
	owners := map[string]string{}

	for _, u := range ups {
		res, err := u.session.ListTools(ctx, nil)
		if err != nil {
			return fmt.Errorf("list tools from %s server: %w", u.name, err)
		}

		for _, tool := range res.Tools {
			if !toolAllowed(allow, tool.Name) {
				continue
			}
			if owner, dup := owners[tool.Name]; dup {
				return fmt.Errorf("tool %q is exported by both the %s and %s servers", tool.Name, owner, u.name)
			}
			owners[tool.Name] = u.name

			session := u.session // capture per iteration
			srv.AddTool(tool, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				// Arguments is a json.RawMessage, which marshals verbatim.
				// Forwarding the raw bytes keeps the request direction as
				// lossless as the response direction.
				return session.CallTool(ctx, &mcp.CallToolParams{
					Name:      req.Params.Name,
					Arguments: req.Params.Arguments,
				})
			})
		}
	}
	return nil
}
