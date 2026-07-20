package main

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type toolDef struct {
	tool     *mcp.Tool
	handler  interface{}
	category string // "read", "mutate", "compose"
}

func allTools() []toolDef { return nil } // filled in later tasks

func parseToolFilter(env string) map[string]bool {
	if env == "" || env == "all" {
		return nil
	}
	set := map[string]bool{}
	for _, n := range strings.Split(env, ",") {
		if n = strings.TrimSpace(n); n != "" {
			set[n] = true
		}
	}
	return set
}

func registerTool(server *mcp.Server, td toolDef) {
	// Typed switch added as handlers are implemented (mirrors jellyfin).
	log.Fatalf("no registration case for tool %s", td.tool.Name)
}

func main() {
	a := &App{
		ReadOnly:  os.Getenv("THUNDERBIRD_READ_ONLY") == "true",
		AllowSend: os.Getenv("THUNDERBIRD_ALLOW_SEND") == "true",
	}
	app = a

	// Profile loading wired in Task 2+.
	filter := parseToolFilter(os.Getenv("THUNDERBIRD_TOOLS"))
	server := mcp.NewServer(&mcp.Implementation{Name: "thunderbird", Version: "v1.0.0"}, nil)

	for _, td := range allTools() {
		if filter != nil && !filter[td.tool.Name] {
			continue
		}
		if app.ReadOnly && (td.category == "mutate" || td.category == "compose") {
			continue
		}
		if td.category == "compose" && !app.AllowSend {
			continue
		}
		registerTool(server, td)
	}

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
