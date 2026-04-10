package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var client *JellyfinClient

type toolDef struct {
	tool    *mcp.Tool
	handler interface{}
}

func allTools() []toolDef {
	return []toolDef{
		// Discovery & Browsing
		{&mcp.Tool{Name: "search", Description: "Search the Jellyfin library for movies, series, episodes, or other media by name. Returns up to 20 results by default."}, Search},
		{&mcp.Tool{Name: "browse_library", Description: "Browse items in a Jellyfin library with filtering and sorting. Supports pagination. Use list_libraries to get library IDs for the parent_id filter."}, BrowseLibrary},
		{&mcp.Tool{Name: "get_item", Description: "Get detailed information about a specific Jellyfin item (movie, series, episode, etc.) by its ID. Returns full metadata including cast, media quality, and external IDs."}, GetItem},
		{&mcp.Tool{Name: "list_libraries", Description: "List all media libraries (folders) on the Jellyfin server with their IDs and types."}, ListLibraries},
		{&mcp.Tool{Name: "get_similar", Description: "Find items similar to a given item. Useful for getting recommendations based on a movie or series."}, GetSimilar},
		{&mcp.Tool{Name: "get_latest", Description: "Get the most recently added items to the library, optionally filtered by a specific library. Requires JELLYFIN_USER_ID or JELLYFIN_USERNAME."}, GetLatest},
		// TV Shows
		{&mcp.Tool{Name: "get_seasons", Description: "List all seasons for a TV series by its series ID."}, GetSeasons},
		{&mcp.Tool{Name: "get_episodes", Description: "List episodes for a TV series, optionally filtered by season. Use get_seasons first to get season IDs."}, GetEpisodes},
		{&mcp.Tool{Name: "get_next_up", Description: "Get the next unwatched episodes across all in-progress TV series. Requires JELLYFIN_USER_ID or JELLYFIN_USERNAME."}, GetNextUp},
		// Playback & Sessions
		{&mcp.Tool{Name: "get_sessions", Description: "List all active sessions on the Jellyfin server. Shows who is watching what and on which device."}, GetSessions},
		{&mcp.Tool{Name: "playback_control", Description: "Control playback on an active session. Commands: Pause, Unpause, Stop, NextTrack, PreviousTrack, Seek. Use get_sessions to find session IDs."}, PlaybackControl},
		// User Data
		{&mcp.Tool{Name: "set_favorite", Description: "Mark or unmark a media item as a favorite. Requires JELLYFIN_USER_ID or JELLYFIN_USERNAME."}, SetFavorite},
		{&mcp.Tool{Name: "set_played", Description: "Mark or unmark a media item as played/watched. Requires JELLYFIN_USER_ID or JELLYFIN_USERNAME."}, SetPlayed},
	}
}

// registerTool registers a tool with the server using the correct generic type.
// This is needed because mcp.AddTool requires a typed handler function.
func registerTool(server *mcp.Server, td toolDef) {
	switch h := td.handler.(type) {
	case func(context.Context, *mcp.CallToolRequest, SearchInput) (*mcp.CallToolResult, SearchOutput, error):
		mcp.AddTool(server, td.tool, h)
	case func(context.Context, *mcp.CallToolRequest, BrowseLibraryInput) (*mcp.CallToolResult, BrowseOutput, error):
		mcp.AddTool(server, td.tool, h)
	case func(context.Context, *mcp.CallToolRequest, GetItemInput) (*mcp.CallToolResult, GetItemOutput, error):
		mcp.AddTool(server, td.tool, h)
	case func(context.Context, *mcp.CallToolRequest, ListLibrariesInput) (*mcp.CallToolResult, ListLibrariesOutput, error):
		mcp.AddTool(server, td.tool, h)
	case func(context.Context, *mcp.CallToolRequest, GetSimilarInput) (*mcp.CallToolResult, SimilarOutput, error):
		mcp.AddTool(server, td.tool, h)
	case func(context.Context, *mcp.CallToolRequest, GetLatestInput) (*mcp.CallToolResult, LatestOutput, error):
		mcp.AddTool(server, td.tool, h)
	case func(context.Context, *mcp.CallToolRequest, GetSeasonsInput) (*mcp.CallToolResult, SeasonsOutput, error):
		mcp.AddTool(server, td.tool, h)
	case func(context.Context, *mcp.CallToolRequest, GetEpisodesInput) (*mcp.CallToolResult, EpisodesOutput, error):
		mcp.AddTool(server, td.tool, h)
	case func(context.Context, *mcp.CallToolRequest, GetNextUpInput) (*mcp.CallToolResult, NextUpOutput, error):
		mcp.AddTool(server, td.tool, h)
	case func(context.Context, *mcp.CallToolRequest, GetSessionsInput) (*mcp.CallToolResult, SessionsOutput, error):
		mcp.AddTool(server, td.tool, h)
	case func(context.Context, *mcp.CallToolRequest, PlaybackControlInput) (*mcp.CallToolResult, PlaybackControlOutput, error):
		mcp.AddTool(server, td.tool, h)
	case func(context.Context, *mcp.CallToolRequest, SetFavoriteInput) (*mcp.CallToolResult, SetFavoriteOutput, error):
		mcp.AddTool(server, td.tool, h)
	case func(context.Context, *mcp.CallToolRequest, SetPlayedInput) (*mcp.CallToolResult, SetPlayedOutput, error):
		mcp.AddTool(server, td.tool, h)
	default:
		log.Fatalf("Unknown handler type for tool %s", td.tool.Name)
	}
}

func main() {
	jellyfinURL := os.Getenv("JELLYFIN_URL")
	if jellyfinURL == "" {
		log.Fatal("JELLYFIN_URL environment variable is required")
	}
	jellyfinURL = strings.TrimRight(jellyfinURL, "/")

	apiKey := os.Getenv("JELLYFIN_API_KEY")
	if apiKey == "" {
		log.Fatal("JELLYFIN_API_KEY environment variable is required")
	}

	userID := os.Getenv("JELLYFIN_USER_ID")

	client = &JellyfinClient{
		BaseURL: jellyfinURL,
		APIKey:  apiKey,
		UserID:  userID,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	// Resolve JELLYFIN_USERNAME to a user ID if JELLYFIN_USER_ID is not set
	if client.UserID == "" {
		if username := os.Getenv("JELLYFIN_USERNAME"); username != "" {
			resolvedID, err := ResolveUsername(context.Background(), client, username)
			if err != nil {
				log.Fatalf("Failed to resolve JELLYFIN_USERNAME %q: %v", username, err)
			}
			client.UserID = resolvedID
			log.Printf("Resolved username %q to user ID %s", username, resolvedID)
		}
	}

	// Test connectivity
	_, err := client.Get(context.Background(), "/System/Info/Public", nil)
	if err != nil {
		log.Printf("Warning: Could not connect to Jellyfin at %s: %v", jellyfinURL, err)
	} else {
		log.Printf("Connected to Jellyfin at %s", jellyfinURL)
	}

	server := mcp.NewServer(&mcp.Implementation{Name: "jellyfin", Version: "v1.0.0"}, nil)

	// JELLYFIN_TOOLS: comma-separated list of tool names to register.
	// Default (empty or "all"): register all tools.
	// Example: JELLYFIN_TOOLS=search,get_item,list_libraries
	enabledTools := os.Getenv("JELLYFIN_TOOLS")
	var enabledSet map[string]bool
	if enabledTools != "" && enabledTools != "all" {
		enabledSet = make(map[string]bool)
		for _, name := range strings.Split(enabledTools, ",") {
			name = strings.TrimSpace(name)
			if name != "" {
				enabledSet[name] = true
			}
		}
	}

	for _, td := range allTools() {
		if enabledSet != nil && !enabledSet[td.tool.Name] {
			continue
		}
		registerTool(server, td)
	}

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
