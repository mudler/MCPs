package main

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// fakeUpstream starts an in-process MCP server exposing the named tools, each
// returning a text part plus a PNG image part, and returns a connected
// upstream pointed at it.
func fakeUpstream(ctx context.Context, name string, png []byte, toolNames ...string) *upstream {
	return fakeUpstreamCapturing(ctx, name, png, nil, toolNames...)
}

// fakeUpstreamCapturing is fakeUpstream with an optional hook invoked with the
// raw argument bytes each tool call arrives with.
func fakeUpstreamCapturing(ctx context.Context, name string, png []byte, capture func(json.RawMessage), toolNames ...string) *upstream {
	srv := mcp.NewServer(&mcp.Implementation{Name: name, Version: "v0"}, nil)
	for _, tn := range toolNames {
		srv.AddTool(
			&mcp.Tool{Name: tn, Description: "fake " + tn, InputSchema: map[string]any{"type": "object"}},
			func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				if capture != nil {
					capture(req.Params.Arguments)
				}
				return &mcp.CallToolResult{Content: []mcp.Content{
					&mcp.TextContent{Text: "called " + req.Params.Name},
					&mcp.ImageContent{MIMEType: "image/png", Data: png},
				}}, nil
			})
	}
	serverT, clientT := mcp.NewInMemoryTransports()
	go func() { _ = srv.Run(ctx, serverT) }()

	u, err := connectUpstream(ctx, name, clientT)
	Expect(err).NotTo(HaveOccurred())
	return u
}

// connectToAggregator wires an aggregating server and returns a client session.
func connectToAggregator(ctx context.Context, ups []*upstream, allow map[string]bool) *mcp.ClientSession {
	agg := mcp.NewServer(&mcp.Implementation{Name: "cua", Version: "test"}, nil)
	Expect(aggregate(ctx, agg, ups, allow)).To(Succeed())

	serverT, clientT := mcp.NewInMemoryTransports()
	go func() { _ = agg.Run(ctx, serverT) }()

	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v0"}, nil).Connect(ctx, clientT, nil)
	Expect(err).NotTo(HaveOccurred())
	return cs
}

var _ = Describe("aggregate", func() {
	var (
		ctx context.Context
		png []byte
	)

	BeforeEach(func() {
		ctx = context.Background()
		png = []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0xde, 0xad}
	})

	It("merges the tool lists of every upstream", func() {
		ups := []*upstream{
			fakeUpstream(ctx, "computer", png, "computer_use"),
			fakeUpstream(ctx, "browser", png, "browser_navigate", "browser_click"),
		}
		cs := connectToAggregator(ctx, ups, nil)

		res, err := cs.ListTools(ctx, nil)
		Expect(err).NotTo(HaveOccurred())

		names := []string{}
		for _, t := range res.Tools {
			names = append(names, t.Name)
		}
		Expect(names).To(ConsistOf("computer_use", "browser_navigate", "browser_click"))
	})

	// This is the regression guard for the failure mode that would quietly
	// gut the server: screenshots are the primary output of both toolsets.
	It("forwards image content byte-for-byte", func() {
		ups := []*upstream{fakeUpstream(ctx, "computer", png, "computer_use")}
		cs := connectToAggregator(ctx, ups, nil)

		res, err := cs.CallTool(ctx, &mcp.CallToolParams{
			Name:      "computer_use",
			Arguments: map[string]any{"action": "capture"},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Content).To(HaveLen(2))

		img, ok := res.Content[1].(*mcp.ImageContent)
		Expect(ok).To(BeTrue(), "second content part should be *mcp.ImageContent")
		Expect(img.MIMEType).To(Equal("image/png"))
		Expect(bytes.Equal(img.Data, png)).To(BeTrue(), "image bytes must survive forwarding intact")
	})

	It("dispatches to the upstream that owns the tool", func() {
		ups := []*upstream{
			fakeUpstream(ctx, "computer", png, "computer_use"),
			fakeUpstream(ctx, "browser", png, "browser_navigate"),
		}
		cs := connectToAggregator(ctx, ups, nil)

		res, err := cs.CallTool(ctx, &mcp.CallToolParams{
			Name:      "browser_navigate",
			Arguments: map[string]any{"url": "https://example.com"},
		})
		Expect(err).NotTo(HaveOccurred())
		text, ok := res.Content[0].(*mcp.TextContent)
		Expect(ok).To(BeTrue())
		Expect(text.Text).To(Equal("called browser_navigate"))
	})

	// Round-tripping arguments through json.Unmarshal into `any` turns every
	// JSON number into a float64, which silently mangles integers past 2^53.
	It("forwards large JSON integers without precision loss", func() {
		var got json.RawMessage
		ups := []*upstream{
			fakeUpstreamCapturing(ctx, "browser", png, func(raw json.RawMessage) {
				got = append(json.RawMessage(nil), raw...)
			}, "browser_navigate"),
		}
		cs := connectToAggregator(ctx, ups, nil)

		// 2^53 + 1: the smallest positive integer a float64 cannot represent.
		_, err := cs.CallTool(ctx, &mcp.CallToolParams{
			Name:      "browser_navigate",
			Arguments: json.RawMessage(`{"id":9007199254740993}`),
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(string(got)).To(ContainSubstring("9007199254740993"),
			"upstream must observe the integer verbatim, not a float64 approximation")
	})

	It("drops tools that are not in a non-empty allowlist", func() {
		ups := []*upstream{fakeUpstream(ctx, "browser", png, "browser_navigate", "browser_click")}
		cs := connectToAggregator(ctx, ups, map[string]bool{"browser_navigate": true})

		res, err := cs.ListTools(ctx, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Tools).To(HaveLen(1))
		Expect(res.Tools[0].Name).To(Equal("browser_navigate"))
	})

	It("rejects a tool name exported by two upstreams", func() {
		ups := []*upstream{
			fakeUpstream(ctx, "computer", png, "shared_tool"),
			fakeUpstream(ctx, "browser", png, "shared_tool"),
		}
		agg := mcp.NewServer(&mcp.Implementation{Name: "cua", Version: "test"}, nil)
		err := aggregate(ctx, agg, ups, nil)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("shared_tool"))
	})
})
