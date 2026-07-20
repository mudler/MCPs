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

func allTools() []toolDef {
	return []toolDef{
		{&mcp.Tool{Name: "list_accounts", Description: "List Thunderbird mail accounts, their type, and the identities (email addresses) you may send as. Local (POP/Local Folders) accounts are read-only."}, ListAccounts, "read"},
		{&mcp.Tool{Name: "list_folders", Description: "List mail folders with unread/total counts. Returns folder URIs used as the folder filter in search and as move destinations."}, ListFolders, "read"},
		{&mcp.Tool{Name: "search_messages", Description: "Search mail via Thunderbird's index. Filter by query text, from, to, subject, folder URI, and date range. Returns message references."}, SearchMessages, "read"},
		{&mcp.Tool{Name: "get_message", Description: "Fetch a full message (headers, body, attachment list) by the message_ref from search_messages or list_recent."}, GetMessage, "read"},
		{&mcp.Tool{Name: "list_recent", Description: "List the most recent messages, optionally within a folder URI."}, ListRecent, "read"},
		{&mcp.Tool{Name: "set_flags", Description: "Mark a message read/unread, set or clear its flag/star, and add or remove tags. IMAP accounts only."}, SetFlags, "mutate"},
		{&mcp.Tool{Name: "move_message", Description: "Move a message to another folder in the same IMAP account."}, MoveMessage, "mutate"},
		{&mcp.Tool{Name: "delete_message", Description: "Delete a message: moves to Trash by default, or expunges permanently when permanent=true. IMAP accounts only."}, DeleteMessage, "mutate"},
		{&mcp.Tool{Name: "send_mail", Description: "Send a new email as one of your identities. Requires THUNDERBIRD_ALLOW_SEND=true."}, SendMail, "compose"},
		{&mcp.Tool{Name: "reply_message", Description: "Reply to a message, quoting the original and threading correctly. Requires THUNDERBIRD_ALLOW_SEND=true."}, ReplyMessage, "compose"},
		{&mcp.Tool{Name: "forward_message", Description: "Forward a message to new recipients. Requires THUNDERBIRD_ALLOW_SEND=true."}, ForwardMessage, "compose"},
		{&mcp.Tool{Name: "save_draft", Description: "Save a draft to the Drafts folder (visible in Thunderbird). Requires THUNDERBIRD_ALLOW_SEND=true."}, SaveDraft, "compose"},
		{&mcp.Tool{Name: "search_contacts", Description: "Search the Thunderbird address book by name or email substring."}, SearchContacts, "read"},
		{&mcp.Tool{Name: "get_contact", Description: "Get a single contact by exact email address."}, GetContact, "read"},
		{&mcp.Tool{Name: "list_calendars", Description: "List the calendars registered in Thunderbird."}, ListCalendars, "read"},
		{&mcp.Tool{Name: "list_events", Description: "List calendar events in a date range (defaults to the next 30 days)."}, ListEvents, "read"},
	}
}

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
	switch h := td.handler.(type) {
	case func(context.Context, *mcp.CallToolRequest, ListAccountsInput) (*mcp.CallToolResult, ListAccountsOutput, error):
		mcp.AddTool(server, td.tool, h)
	case func(context.Context, *mcp.CallToolRequest, ListFoldersInput) (*mcp.CallToolResult, ListFoldersOutput, error):
		mcp.AddTool(server, td.tool, h)
	case func(context.Context, *mcp.CallToolRequest, SearchMessagesInput) (*mcp.CallToolResult, SearchMessagesOutput, error):
		mcp.AddTool(server, td.tool, h)
	case func(context.Context, *mcp.CallToolRequest, GetMessageInput) (*mcp.CallToolResult, GetMessageOutput, error):
		mcp.AddTool(server, td.tool, h)
	case func(context.Context, *mcp.CallToolRequest, ListRecentInput) (*mcp.CallToolResult, ListRecentOutput, error):
		mcp.AddTool(server, td.tool, h)
	case func(context.Context, *mcp.CallToolRequest, SetFlagsInput) (*mcp.CallToolResult, SetFlagsOutput, error):
		mcp.AddTool(server, td.tool, h)
	case func(context.Context, *mcp.CallToolRequest, MoveMessageInput) (*mcp.CallToolResult, MoveMessageOutput, error):
		mcp.AddTool(server, td.tool, h)
	case func(context.Context, *mcp.CallToolRequest, DeleteMessageInput) (*mcp.CallToolResult, DeleteMessageOutput, error):
		mcp.AddTool(server, td.tool, h)
	case func(context.Context, *mcp.CallToolRequest, SendMailInput) (*mcp.CallToolResult, SendMailOutput, error):
		mcp.AddTool(server, td.tool, h)
	case func(context.Context, *mcp.CallToolRequest, ReplyMessageInput) (*mcp.CallToolResult, ReplyMessageOutput, error):
		mcp.AddTool(server, td.tool, h)
	case func(context.Context, *mcp.CallToolRequest, ForwardMessageInput) (*mcp.CallToolResult, ForwardMessageOutput, error):
		mcp.AddTool(server, td.tool, h)
	case func(context.Context, *mcp.CallToolRequest, SaveDraftInput) (*mcp.CallToolResult, SaveDraftOutput, error):
		mcp.AddTool(server, td.tool, h)
	case func(context.Context, *mcp.CallToolRequest, SearchContactsInput) (*mcp.CallToolResult, SearchContactsOutput, error):
		mcp.AddTool(server, td.tool, h)
	case func(context.Context, *mcp.CallToolRequest, GetContactInput) (*mcp.CallToolResult, GetContactOutput, error):
		mcp.AddTool(server, td.tool, h)
	case func(context.Context, *mcp.CallToolRequest, ListCalendarsInput) (*mcp.CallToolResult, ListCalendarsOutput, error):
		mcp.AddTool(server, td.tool, h)
	case func(context.Context, *mcp.CallToolRequest, ListEventsInput) (*mcp.CallToolResult, ListEventsOutput, error):
		mcp.AddTool(server, td.tool, h)
	default:
		log.Fatalf("no registration case for tool %s", td.tool.Name)
	}
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
