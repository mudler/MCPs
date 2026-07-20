package main

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

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

// mustSchema infers the JSON schema for T and overrides the "action" property's
// enum with the (dynamically gated) set of allowed values.
func mustSchema[T any](enumValues []any) *jsonschema.Schema {
	s, err := jsonschema.For[T](nil)
	if err != nil {
		log.Fatalf("schema: %v", err)
	}
	if p, ok := s.Properties["action"]; ok {
		p.Enum = enumValues
	}
	return s
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
	allowed := func(name string) bool { return filter == nil || filter[name] }

	server := mcp.NewServer(&mcp.Implementation{Name: "thunderbird", Version: "v1.0.0"}, nil)

	mailActions := []any{"list_accounts", "list_folders", "search_messages", "get_message", "list_recent"}
	if !app.ReadOnly {
		mailActions = append(mailActions, "set_flags", "move_message", "delete_message")
	}
	if app.AllowSend && !app.ReadOnly {
		mailActions = append(mailActions, "send_mail", "reply_message", "forward_message", "save_draft")
	}

	if allowed("thunderbird_mail") {
		mcp.AddTool(server, &mcp.Tool{
			Name:        "thunderbird_mail",
			Description: "Thunderbird mail operations. Set `action` to one of the enum values. Read actions (list_accounts, list_folders, search_messages, get_message, list_recent) are always available; set_flags/move_message/delete_message require write access; send_mail/reply_message/forward_message/save_draft require THUNDERBIRD_ALLOW_SEND=true.",
			InputSchema: mustSchema[MailInput](mailActions),
		}, ThunderbirdMail)
	}

	if allowed("thunderbird_contacts") {
		mcp.AddTool(server, &mcp.Tool{
			Name:        "thunderbird_contacts",
			Description: "Thunderbird address book (read-only). action: search_contacts (by name/email substring) or get_contact (by exact email).",
			InputSchema: mustSchema[ContactsInput]([]any{"search_contacts", "get_contact"}),
		}, ThunderbirdContacts)
	}

	if allowed("thunderbird_calendar") {
		mcp.AddTool(server, &mcp.Tool{
			Name:        "thunderbird_calendar",
			Description: "Thunderbird calendar (read-only). action: list_calendars or list_events (date range).",
			InputSchema: mustSchema[CalendarInput]([]any{"list_calendars", "list_events"}),
		}, ThunderbirdCalendar)
	}

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
