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
	profileDir, err := discoverProfile()
	if err != nil {
		log.Fatalf("thunderbird: %v", err)
	}
	loaded, err := LoadApp(profileDir, os.Getenv("THUNDERBIRD_READ_ONLY") == "true", os.Getenv("THUNDERBIRD_ALLOW_SEND") == "true")
	if err != nil {
		log.Fatalf("thunderbird: %v", err)
	}
	app = loaded
	log.Printf("thunderbird: loaded profile %s (%d accounts)", profileDir, len(app.Config.Accounts))

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
