package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func toMediaSummary(item jellyfinItem) MediaSummary {
	overview := item.Overview
	if len(overview) > 200 {
		overview = overview[:200] + "..."
	}
	return MediaSummary{
		ID:              item.ID,
		Name:            item.Name,
		Type:            item.Type,
		Year:            item.ProductionYear,
		CommunityRating: item.CommunityRating,
		OfficialRating:  item.OfficialRating,
		Overview:        overview,
	}
}

func searchHintToMediaSummary(hint jellyfinSearchHint) MediaSummary {
	name := hint.Name
	if hint.Series != "" {
		name = hint.Series + " - " + hint.Name
	}
	return MediaSummary{
		ID:   hint.ItemId,
		Name: name,
		Type: hint.Type,
		Year: hint.ProductionYear,
	}
}

func requireUserID() error {
	if client.UserID == "" {
		return fmt.Errorf("JELLYFIN_USER_ID (or JELLYFIN_USERNAME) environment variable is required for this operation")
	}
	return nil
}

// ResolveUsername looks up a username and returns the corresponding user ID.
func ResolveUsername(ctx context.Context, c *JellyfinClient, username string) (string, error) {
	data, err := c.Get(ctx, "/Users", nil)
	if err != nil {
		return "", fmt.Errorf("failed to list users: %w", err)
	}

	var users []jellyfinUser
	if err := json.Unmarshal(data, &users); err != nil {
		return "", fmt.Errorf("parsing users response: %w", err)
	}

	for _, u := range users {
		if u.Name == username {
			return u.ID, nil
		}
	}
	return "", fmt.Errorf("user %q not found", username)
}

// --- Discovery & Browsing ---

func Search(ctx context.Context, req *mcp.CallToolRequest, input SearchInput) (*mcp.CallToolResult, SearchOutput, error) {
	limit := input.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	q := url.Values{}
	q.Set("searchTerm", input.Query)
	q.Set("Limit", strconv.Itoa(limit))

	data, err := client.Get(ctx, "/Search/Hints", q)
	if err != nil {
		return nil, SearchOutput{}, fmt.Errorf("search failed: %w", err)
	}

	var resp jellyfinSearchResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, SearchOutput{}, fmt.Errorf("parsing search response: %w", err)
	}

	items := make([]MediaSummary, 0, len(resp.SearchHints))
	for _, hint := range resp.SearchHints {
		items = append(items, searchHintToMediaSummary(hint))
	}

	return nil, SearchOutput{Items: items, Count: len(items)}, nil
}

func BrowseLibrary(ctx context.Context, req *mcp.CallToolRequest, input BrowseLibraryInput) (*mcp.CallToolResult, BrowseOutput, error) {
	limit := input.Limit
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}

	q := url.Values{}
	q.Set("Recursive", "true")
	q.Set("Fields", "Overview,Genres,CommunityRating,OfficialRating,ProductionYear")
	q.Set("Limit", strconv.Itoa(limit))
	q.Set("StartIndex", strconv.Itoa(input.StartIndex))

	if input.ParentID != "" {
		q.Set("parentId", input.ParentID)
	}
	if input.ItemTypes != "" {
		q.Set("IncludeItemTypes", input.ItemTypes)
	}
	if input.SortBy != "" {
		q.Set("SortBy", input.SortBy)
	} else {
		q.Set("SortBy", "SortName")
	}
	if input.SortOrder != "" {
		q.Set("SortOrder", input.SortOrder)
	} else {
		q.Set("SortOrder", "Ascending")
	}
	if input.Genres != "" {
		q.Set("Genres", input.Genres)
	}
	if input.Years != "" {
		q.Set("Years", input.Years)
	}

	data, err := client.Get(ctx, "/Items", q)
	if err != nil {
		return nil, BrowseOutput{}, fmt.Errorf("browse failed: %w", err)
	}

	var resp jellyfinItemsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, BrowseOutput{}, fmt.Errorf("parsing browse response: %w", err)
	}

	items := make([]MediaSummary, 0, len(resp.Items))
	for _, item := range resp.Items {
		items = append(items, toMediaSummary(item))
	}

	return nil, BrowseOutput{
		Items:      items,
		TotalCount: resp.TotalRecordCount,
		StartIndex: input.StartIndex,
	}, nil
}

func GetItem(ctx context.Context, req *mcp.CallToolRequest, input GetItemInput) (*mcp.CallToolResult, GetItemOutput, error) {
	q := url.Values{}
	q.Set("Fields", "Overview,People,MediaSources,Genres,Studios,ProviderIds,CommunityRating,OfficialRating,ProductionYear")
	if client.UserID != "" {
		q.Set("UserId", client.UserID)
	}

	data, err := client.Get(ctx, "/Items/"+input.ItemID, q)
	if err != nil {
		return nil, GetItemOutput{}, fmt.Errorf("get item failed: %w", err)
	}

	var item jellyfinItem
	if err := json.Unmarshal(data, &item); err != nil {
		return nil, GetItemOutput{}, fmt.Errorf("parsing item response: %w", err)
	}

	// Convert studios
	studios := make([]string, 0, len(item.Studios))
	for _, s := range item.Studios {
		studios = append(studios, s.Name)
	}

	// Convert people (max 15)
	people := make([]PersonSummary, 0)
	for i, p := range item.People {
		if i >= 15 {
			break
		}
		people = append(people, PersonSummary{
			Name: p.Name,
			Role: p.Role,
			Type: p.Type,
		})
	}

	// Convert media sources
	sources := make([]SourceSummary, 0, len(item.MediaSources))
	for _, s := range item.MediaSources {
		source := SourceSummary{
			Name:      s.Name,
			Container: s.Container,
			Size:      s.Size,
		}
		for _, stream := range s.MediaStreams {
			switch stream.Type {
			case "Video":
				source.VideoCodec = stream.Codec
				if stream.Width > 0 && stream.Height > 0 {
					source.Resolution = fmt.Sprintf("%dx%d", stream.Width, stream.Height)
				}
			case "Audio":
				if source.AudioCodec == "" {
					source.AudioCodec = stream.Codec
				}
			}
		}
		sources = append(sources, source)
	}

	detail := ItemDetail{
		ID:              item.ID,
		Name:            item.Name,
		Type:            item.Type,
		Year:            item.ProductionYear,
		Overview:        item.Overview,
		CommunityRating: item.CommunityRating,
		OfficialRating:  item.OfficialRating,
		Genres:          item.Genres,
		Studios:         studios,
		People:          people,
		MediaSources:    sources,
		ProviderIDs:     item.ProviderIds,
		SeriesName:      item.SeriesName,
		SeasonName:      item.SeasonName,
		IndexNumber:     item.IndexNumber,
		RunTimeTicks:    item.RunTimeTicks,
	}

	return nil, GetItemOutput{Item: detail}, nil
}

func ListLibraries(ctx context.Context, req *mcp.CallToolRequest, input ListLibrariesInput) (*mcp.CallToolResult, ListLibrariesOutput, error) {
	data, err := client.Get(ctx, "/Library/MediaFolders", nil)
	if err != nil {
		return nil, ListLibrariesOutput{}, fmt.Errorf("list libraries failed: %w", err)
	}

	var resp jellyfinItemsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, ListLibrariesOutput{}, fmt.Errorf("parsing libraries response: %w", err)
	}

	libraries := make([]LibrarySummary, 0, len(resp.Items))
	for _, item := range resp.Items {
		libraries = append(libraries, LibrarySummary{
			ID:             item.ID,
			Name:           item.Name,
			CollectionType: item.CollectionType,
		})
	}

	return nil, ListLibrariesOutput{Libraries: libraries, Count: len(libraries)}, nil
}

func GetSimilar(ctx context.Context, req *mcp.CallToolRequest, input GetSimilarInput) (*mcp.CallToolResult, SimilarOutput, error) {
	limit := input.Limit
	if limit <= 0 {
		limit = 10
	}

	q := url.Values{}
	q.Set("Limit", strconv.Itoa(limit))
	q.Set("Fields", "Overview,Genres,CommunityRating,OfficialRating,ProductionYear")

	data, err := client.Get(ctx, "/Items/"+input.ItemID+"/Similar", q)
	if err != nil {
		return nil, SimilarOutput{}, fmt.Errorf("get similar failed: %w", err)
	}

	var resp jellyfinItemsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, SimilarOutput{}, fmt.Errorf("parsing similar response: %w", err)
	}

	items := make([]MediaSummary, 0, len(resp.Items))
	for _, item := range resp.Items {
		items = append(items, toMediaSummary(item))
	}

	return nil, SimilarOutput{Items: items, Count: len(items)}, nil
}

func GetLatest(ctx context.Context, req *mcp.CallToolRequest, input GetLatestInput) (*mcp.CallToolResult, LatestOutput, error) {
	if err := requireUserID(); err != nil {
		return nil, LatestOutput{}, err
	}

	limit := input.Limit
	if limit <= 0 {
		limit = 20
	}

	q := url.Values{}
	q.Set("UserId", client.UserID)
	q.Set("Limit", strconv.Itoa(limit))
	q.Set("Fields", "Overview,Genres,CommunityRating,OfficialRating,ProductionYear")
	if input.ParentID != "" {
		q.Set("parentId", input.ParentID)
	}

	data, err := client.Get(ctx, "/Items/Latest", q)
	if err != nil {
		return nil, LatestOutput{}, fmt.Errorf("get latest failed: %w", err)
	}

	// /Items/Latest returns a plain array, not wrapped in {Items: [...]}
	var items []jellyfinItem
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, LatestOutput{}, fmt.Errorf("parsing latest response: %w", err)
	}

	summaries := make([]MediaSummary, 0, len(items))
	for _, item := range items {
		summaries = append(summaries, toMediaSummary(item))
	}

	return nil, LatestOutput{Items: summaries, Count: len(summaries)}, nil
}

// --- TV Shows ---

func GetSeasons(ctx context.Context, req *mcp.CallToolRequest, input GetSeasonsInput) (*mcp.CallToolResult, SeasonsOutput, error) {
	q := url.Values{}
	q.Set("Fields", "Overview")

	data, err := client.Get(ctx, "/Shows/"+input.SeriesID+"/Seasons", q)
	if err != nil {
		return nil, SeasonsOutput{}, fmt.Errorf("get seasons failed: %w", err)
	}

	var resp jellyfinItemsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, SeasonsOutput{}, fmt.Errorf("parsing seasons response: %w", err)
	}

	seasons := make([]SeasonSummary, 0, len(resp.Items))
	for _, item := range resp.Items {
		seasons = append(seasons, SeasonSummary{
			ID:          item.ID,
			Name:        item.Name,
			IndexNumber: item.IndexNumber,
			Overview:    item.Overview,
		})
	}

	return nil, SeasonsOutput{Seasons: seasons, Count: len(seasons)}, nil
}

func GetEpisodes(ctx context.Context, req *mcp.CallToolRequest, input GetEpisodesInput) (*mcp.CallToolResult, EpisodesOutput, error) {
	q := url.Values{}
	q.Set("Fields", "Overview,CommunityRating")
	if input.SeasonID != "" {
		q.Set("SeasonId", input.SeasonID)
	}

	data, err := client.Get(ctx, "/Shows/"+input.SeriesID+"/Episodes", q)
	if err != nil {
		return nil, EpisodesOutput{}, fmt.Errorf("get episodes failed: %w", err)
	}

	var resp jellyfinItemsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, EpisodesOutput{}, fmt.Errorf("parsing episodes response: %w", err)
	}

	episodes := make([]EpisodeSummary, 0, len(resp.Items))
	for _, item := range resp.Items {
		episodes = append(episodes, EpisodeSummary{
			ID:              item.ID,
			Name:            item.Name,
			SeasonName:      item.SeasonName,
			IndexNumber:     item.IndexNumber,
			Overview:        item.Overview,
			CommunityRating: item.CommunityRating,
		})
	}

	return nil, EpisodesOutput{Episodes: episodes, Count: len(episodes)}, nil
}

func GetNextUp(ctx context.Context, req *mcp.CallToolRequest, input GetNextUpInput) (*mcp.CallToolResult, NextUpOutput, error) {
	if err := requireUserID(); err != nil {
		return nil, NextUpOutput{}, err
	}

	limit := input.Limit
	if limit <= 0 {
		limit = 20
	}

	q := url.Values{}
	q.Set("UserId", client.UserID)
	q.Set("Limit", strconv.Itoa(limit))
	q.Set("Fields", "Overview")

	data, err := client.Get(ctx, "/Shows/NextUp", q)
	if err != nil {
		return nil, NextUpOutput{}, fmt.Errorf("get next up failed: %w", err)
	}

	var resp jellyfinItemsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, NextUpOutput{}, fmt.Errorf("parsing next up response: %w", err)
	}

	episodes := make([]EpisodeSummary, 0, len(resp.Items))
	for _, item := range resp.Items {
		episodes = append(episodes, EpisodeSummary{
			ID:              item.ID,
			Name:            item.Name,
			SeasonName:      item.SeasonName,
			IndexNumber:     item.IndexNumber,
			Overview:        item.Overview,
			CommunityRating: item.CommunityRating,
		})
	}

	return nil, NextUpOutput{Episodes: episodes, Count: len(episodes)}, nil
}

// --- Playback & Sessions ---

func GetSessions(ctx context.Context, req *mcp.CallToolRequest, input GetSessionsInput) (*mcp.CallToolResult, SessionsOutput, error) {
	data, err := client.Get(ctx, "/Sessions", nil)
	if err != nil {
		return nil, SessionsOutput{}, fmt.Errorf("get sessions failed: %w", err)
	}

	var sessions []jellyfinSession
	if err := json.Unmarshal(data, &sessions); err != nil {
		return nil, SessionsOutput{}, fmt.Errorf("parsing sessions response: %w", err)
	}

	summaries := make([]SessionSummary, 0, len(sessions))
	for _, s := range sessions {
		summary := SessionSummary{
			ID:         s.Id,
			UserName:   s.UserName,
			Client:     s.Client,
			DeviceName: s.DeviceName,
			PlayState: PlayStateSummary{
				IsPaused:      s.PlayState.IsPaused,
				PositionTicks: s.PlayState.PositionTicks,
			},
		}
		if s.NowPlayingItem != nil {
			np := s.NowPlayingItem.Name
			if s.NowPlayingItem.ProductionYear > 0 {
				np = fmt.Sprintf("%s (%d)", np, s.NowPlayingItem.ProductionYear)
			}
			summary.NowPlayingItem = np
		}
		summaries = append(summaries, summary)
	}

	return nil, SessionsOutput{Sessions: summaries, Count: len(summaries)}, nil
}

func PlaybackControl(ctx context.Context, req *mcp.CallToolRequest, input PlaybackControlInput) (*mcp.CallToolResult, PlaybackControlOutput, error) {
	validCommands := map[string]bool{
		"Pause": true, "Unpause": true, "Stop": true,
		"NextTrack": true, "PreviousTrack": true, "Seek": true,
	}
	if !validCommands[input.Command] {
		return nil, PlaybackControlOutput{
			Success: false,
			Message: fmt.Sprintf("Invalid command: %s. Valid commands: Pause, Unpause, Stop, NextTrack, PreviousTrack, Seek", input.Command),
		}, nil
	}

	path := fmt.Sprintf("/Sessions/%s/Playing/%s", input.SessionID, input.Command)

	if input.Command == "Seek" {
		path += fmt.Sprintf("?SeekPositionTicks=%d", input.SeekPositionTicks)
	}

	_, err := client.Post(ctx, path, nil)
	if err != nil {
		return nil, PlaybackControlOutput{
			Success: false,
			Message: fmt.Sprintf("Playback control failed: %v", err),
		}, nil
	}

	return nil, PlaybackControlOutput{
		Success: true,
		Message: fmt.Sprintf("Successfully sent %s command to session %s", input.Command, input.SessionID),
	}, nil
}

// --- User Data ---

func SetFavorite(ctx context.Context, req *mcp.CallToolRequest, input SetFavoriteInput) (*mcp.CallToolResult, SetFavoriteOutput, error) {
	if err := requireUserID(); err != nil {
		return nil, SetFavoriteOutput{}, err
	}

	path := fmt.Sprintf("/UserFavoriteItems/%s", input.ItemID)
	q := url.Values{}
	q.Set("UserId", client.UserID)

	if input.Favorite {
		_, err := client.Post(ctx, path+"?"+q.Encode(), nil)
		if err != nil {
			return nil, SetFavoriteOutput{Success: false, Message: fmt.Sprintf("Failed: %v", err)}, nil
		}
	} else {
		err := client.Delete(ctx, path, q)
		if err != nil {
			return nil, SetFavoriteOutput{Success: false, Message: fmt.Sprintf("Failed: %v", err)}, nil
		}
	}

	action := "added to"
	if !input.Favorite {
		action = "removed from"
	}
	return nil, SetFavoriteOutput{
		Success: true,
		Message: fmt.Sprintf("Item %s %s favorites", input.ItemID, action),
	}, nil
}

func SetPlayed(ctx context.Context, req *mcp.CallToolRequest, input SetPlayedInput) (*mcp.CallToolResult, SetPlayedOutput, error) {
	if err := requireUserID(); err != nil {
		return nil, SetPlayedOutput{}, err
	}

	path := fmt.Sprintf("/UserPlayedItems/%s", input.ItemID)
	q := url.Values{}
	q.Set("UserId", client.UserID)

	if input.Played {
		_, err := client.Post(ctx, path+"?"+q.Encode(), nil)
		if err != nil {
			return nil, SetPlayedOutput{Success: false, Message: fmt.Sprintf("Failed: %v", err)}, nil
		}
	} else {
		err := client.Delete(ctx, path, q)
		if err != nil {
			return nil, SetPlayedOutput{Success: false, Message: fmt.Sprintf("Failed: %v", err)}, nil
		}
	}

	action := "marked as played"
	if !input.Played {
		action = "marked as unplayed"
	}
	return nil, SetPlayedOutput{
		Success: true,
		Message: fmt.Sprintf("Item %s %s", input.ItemID, action),
	}, nil
}

