package main

// --- Input types ---

type SearchInput struct {
	Query string `json:"query" jsonschema:"required,the search term to find media"`
	Limit int    `json:"limit,omitempty" jsonschema:"maximum number of results to return (default 20, max 100)"`
}

type BrowseLibraryInput struct {
	ParentID  string `json:"parent_id,omitempty" jsonschema:"library ID to browse (use list_libraries to get IDs)"`
	ItemTypes string `json:"item_types,omitempty" jsonschema:"comma-separated types to include (e.g. Movie,Series,Episode)"`
	SortBy    string `json:"sort_by,omitempty" jsonschema:"sort field (default SortName). Options: SortName,DateCreated,PremiereDate,CommunityRating,ProductionYear,Random"`
	SortOrder string `json:"sort_order,omitempty" jsonschema:"sort direction: Ascending (default) or Descending"`
	Genres    string `json:"genres,omitempty" jsonschema:"filter by genre (e.g. Action,Comedy)"`
	Years     string `json:"years,omitempty" jsonschema:"filter by year (e.g. 2023,2024)"`
	Limit     int    `json:"limit,omitempty" jsonschema:"maximum number of results (default 25, max 100)"`
	StartIndex int   `json:"start_index,omitempty" jsonschema:"offset for pagination (default 0)"`
}

type GetItemInput struct {
	ItemID string `json:"item_id" jsonschema:"required,the Jellyfin item ID"`
}

type ListLibrariesInput struct{}

type GetSimilarInput struct {
	ItemID string `json:"item_id" jsonschema:"required,the Jellyfin item ID to find similar items for"`
	Limit  int    `json:"limit,omitempty" jsonschema:"maximum number of results (default 10)"`
}

type GetLatestInput struct {
	ParentID string `json:"parent_id,omitempty" jsonschema:"library ID to filter by (use list_libraries to get IDs)"`
	Limit    int    `json:"limit,omitempty" jsonschema:"maximum number of results (default 20)"`
}

type GetSeasonsInput struct {
	SeriesID string `json:"series_id" jsonschema:"required,the Jellyfin series ID"`
}

type GetEpisodesInput struct {
	SeriesID string `json:"series_id" jsonschema:"required,the Jellyfin series ID"`
	SeasonID string `json:"season_id,omitempty" jsonschema:"season ID to filter episodes (omit for all episodes)"`
}

type GetNextUpInput struct {
	Limit int `json:"limit,omitempty" jsonschema:"maximum number of results (default 20)"`
}

type GetSessionsInput struct{}

type PlaybackControlInput struct {
	SessionID         string `json:"session_id" jsonschema:"required,the session ID (use get_sessions to find active sessions)"`
	Command           string `json:"command" jsonschema:"required,playback command: Pause, Unpause, Stop, NextTrack, PreviousTrack, or Seek"`
	SeekPositionTicks int64  `json:"seek_position_ticks,omitempty" jsonschema:"position to seek to in ticks (1 second = 10000000 ticks). Required for Seek command"`
}

type SetFavoriteInput struct {
	ItemID   string `json:"item_id" jsonschema:"required,the Jellyfin item ID"`
	Favorite bool   `json:"favorite" jsonschema:"required,true to mark as favorite, false to remove"`
}

type SetPlayedInput struct {
	ItemID string `json:"item_id" jsonschema:"required,the Jellyfin item ID"`
	Played bool   `json:"played" jsonschema:"required,true to mark as played, false to mark unplayed"`
}

// --- Output types ---

type MediaSummary struct {
	ID              string  `json:"id" jsonschema:"the Jellyfin item ID"`
	Name            string  `json:"name" jsonschema:"the item name"`
	Type            string  `json:"type" jsonschema:"the item type (Movie, Series, Episode, etc.)"`
	Year            int     `json:"year,omitempty" jsonschema:"the production year"`
	CommunityRating float64 `json:"community_rating,omitempty" jsonschema:"community rating (0-10)"`
	OfficialRating  string  `json:"official_rating,omitempty" jsonschema:"content rating (PG-13, R, TV-MA, etc.)"`
	Overview        string  `json:"overview,omitempty" jsonschema:"short description (truncated to 200 chars)"`
}

type SearchOutput struct {
	Items []MediaSummary `json:"items" jsonschema:"search results"`
	Count int            `json:"count" jsonschema:"number of results returned"`
}

type BrowseOutput struct {
	Items      []MediaSummary `json:"items" jsonschema:"browsed items"`
	TotalCount int            `json:"total_count" jsonschema:"total number of matching items"`
	StartIndex int            `json:"start_index" jsonschema:"current pagination offset"`
}

type ItemDetail struct {
	ID              string            `json:"id" jsonschema:"the Jellyfin item ID"`
	Name            string            `json:"name" jsonschema:"the item name"`
	Type            string            `json:"type" jsonschema:"the item type"`
	Year            int               `json:"year,omitempty" jsonschema:"the production year"`
	Overview        string            `json:"overview,omitempty" jsonschema:"full description"`
	CommunityRating float64           `json:"community_rating,omitempty" jsonschema:"community rating (0-10)"`
	OfficialRating  string            `json:"official_rating,omitempty" jsonschema:"content rating"`
	Genres          []string          `json:"genres,omitempty" jsonschema:"genre list"`
	Studios         []string          `json:"studios,omitempty" jsonschema:"studio names"`
	People          []PersonSummary   `json:"people,omitempty" jsonschema:"cast and crew (max 15)"`
	MediaSources    []SourceSummary   `json:"media_sources,omitempty" jsonschema:"available media files"`
	ProviderIDs     map[string]string `json:"provider_ids,omitempty" jsonschema:"external IDs (Imdb, Tmdb, Tvdb)"`
	SeriesName      string            `json:"series_name,omitempty" jsonschema:"parent series name (for episodes)"`
	SeasonName      string            `json:"season_name,omitempty" jsonschema:"parent season name (for episodes)"`
	IndexNumber     int               `json:"index_number,omitempty" jsonschema:"episode or track number"`
	RunTimeTicks    int64             `json:"runtime_ticks,omitempty" jsonschema:"duration in ticks (1 second = 10000000 ticks)"`
}

type GetItemOutput struct {
	Item ItemDetail `json:"item" jsonschema:"item details"`
}

type PersonSummary struct {
	Name string `json:"name" jsonschema:"person name"`
	Role string `json:"role,omitempty" jsonschema:"role or character name"`
	Type string `json:"type" jsonschema:"person type (Actor, Director, Writer, etc.)"`
}

type SourceSummary struct {
	Name       string `json:"name,omitempty" jsonschema:"source name"`
	Container  string `json:"container,omitempty" jsonschema:"file container (mkv, mp4, etc.)"`
	Size       int64  `json:"size,omitempty" jsonschema:"file size in bytes"`
	VideoCodec string `json:"video_codec,omitempty" jsonschema:"video codec (h264, hevc, etc.)"`
	AudioCodec string `json:"audio_codec,omitempty" jsonschema:"primary audio codec"`
	Resolution string `json:"resolution,omitempty" jsonschema:"video resolution (e.g. 1920x1080)"`
}

type LibrarySummary struct {
	ID             string `json:"id" jsonschema:"the library ID"`
	Name           string `json:"name" jsonschema:"the library name"`
	CollectionType string `json:"collection_type,omitempty" jsonschema:"library type (movies, tvshows, music, etc.)"`
}

type ListLibrariesOutput struct {
	Libraries []LibrarySummary `json:"libraries" jsonschema:"list of media libraries"`
	Count     int              `json:"count" jsonschema:"number of libraries"`
}

type SimilarOutput struct {
	Items []MediaSummary `json:"items" jsonschema:"similar items"`
	Count int            `json:"count" jsonschema:"number of results"`
}

type LatestOutput struct {
	Items []MediaSummary `json:"items" jsonschema:"recently added items"`
	Count int            `json:"count" jsonschema:"number of results"`
}

type SeasonSummary struct {
	ID          string `json:"id" jsonschema:"the season ID"`
	Name        string `json:"name" jsonschema:"the season name"`
	IndexNumber int    `json:"index_number" jsonschema:"the season number"`
	Overview    string `json:"overview,omitempty" jsonschema:"season description"`
}

type SeasonsOutput struct {
	Seasons []SeasonSummary `json:"seasons" jsonschema:"list of seasons"`
	Count   int             `json:"count" jsonschema:"number of seasons"`
}

type EpisodeSummary struct {
	ID              string  `json:"id" jsonschema:"the episode ID"`
	Name            string  `json:"name" jsonschema:"the episode name"`
	SeasonName      string  `json:"season_name,omitempty" jsonschema:"the season name"`
	IndexNumber     int     `json:"index_number,omitempty" jsonschema:"the episode number"`
	Overview        string  `json:"overview,omitempty" jsonschema:"episode description"`
	CommunityRating float64 `json:"community_rating,omitempty" jsonschema:"community rating (0-10)"`
}

type EpisodesOutput struct {
	Episodes []EpisodeSummary `json:"episodes" jsonschema:"list of episodes"`
	Count    int              `json:"count" jsonschema:"number of episodes"`
}

type NextUpOutput struct {
	Episodes []EpisodeSummary `json:"episodes" jsonschema:"next episodes to watch"`
	Count    int              `json:"count" jsonschema:"number of results"`
}

type SessionSummary struct {
	ID              string           `json:"id" jsonschema:"the session ID"`
	UserName        string           `json:"user_name,omitempty" jsonschema:"the user name"`
	Client          string           `json:"client,omitempty" jsonschema:"the client application name"`
	DeviceName      string           `json:"device_name,omitempty" jsonschema:"the device name"`
	NowPlayingItem  string           `json:"now_playing_item,omitempty" jsonschema:"currently playing item (formatted as Name (Year))"`
	PlayState       PlayStateSummary `json:"play_state" jsonschema:"current playback state"`
}

type PlayStateSummary struct {
	IsPaused      bool  `json:"is_paused" jsonschema:"whether playback is paused"`
	PositionTicks int64 `json:"position_ticks,omitempty" jsonschema:"current position in ticks"`
}

type SessionsOutput struct {
	Sessions []SessionSummary `json:"sessions" jsonschema:"active sessions"`
	Count    int              `json:"count" jsonschema:"number of active sessions"`
}

type PlaybackControlOutput struct {
	Success bool   `json:"success" jsonschema:"whether the command was successful"`
	Message string `json:"message" jsonschema:"status message"`
}

type SetFavoriteOutput struct {
	Success bool   `json:"success" jsonschema:"whether the operation was successful"`
	Message string `json:"message" jsonschema:"status message"`
}

type SetPlayedOutput struct {
	Success bool   `json:"success" jsonschema:"whether the operation was successful"`
	Message string `json:"message" jsonschema:"status message"`
}

// --- Internal Jellyfin API response types ---

type jellyfinItem struct {
	ID                string            `json:"Id"`
	Name              string            `json:"Name"`
	Type              string            `json:"Type"`
	ProductionYear    int               `json:"ProductionYear"`
	CommunityRating   float64           `json:"CommunityRating"`
	OfficialRating    string            `json:"OfficialRating"`
	Overview          string            `json:"Overview"`
	Genres            []string          `json:"Genres"`
	Studios           []jellyfinStudio  `json:"Studios"`
	People            []jellyfinPerson  `json:"People"`
	MediaSources      []jellyfinSource  `json:"MediaSources"`
	ProviderIds       map[string]string `json:"ProviderIds"`
	IndexNumber       int               `json:"IndexNumber"`
	ParentIndexNumber int               `json:"ParentIndexNumber"`
	SeriesName        string            `json:"SeriesName"`
	SeasonName        string            `json:"SeasonName"`
	RunTimeTicks      int64             `json:"RunTimeTicks"`
	CollectionType    string            `json:"CollectionType"`
}

type jellyfinStudio struct {
	Name string `json:"Name"`
	ID   string `json:"Id"`
}

type jellyfinPerson struct {
	Name string `json:"Name"`
	ID   string `json:"Id"`
	Role string `json:"Role"`
	Type string `json:"Type"`
}

type jellyfinSource struct {
	Name         string           `json:"Name"`
	Container    string           `json:"Container"`
	Size         int64            `json:"Size"`
	MediaStreams []jellyfinStream `json:"MediaStreams"`
}

type jellyfinStream struct {
	Type     string `json:"Type"`
	Codec    string `json:"Codec"`
	Width    int    `json:"Width"`
	Height   int    `json:"Height"`
}

type jellyfinItemsResponse struct {
	Items            []jellyfinItem `json:"Items"`
	TotalRecordCount int            `json:"TotalRecordCount"`
}

type jellyfinSearchHint struct {
	ItemId         string  `json:"ItemId"`
	Name           string  `json:"Name"`
	Type           string  `json:"Type"`
	ProductionYear int     `json:"ProductionYear"`
	RunTimeTicks   int64   `json:"RunTimeTicks"`
	Series         string  `json:"Series"`
}

type jellyfinSearchResponse struct {
	SearchHints      []jellyfinSearchHint `json:"SearchHints"`
	TotalRecordCount int                  `json:"TotalRecordCount"`
}

type jellyfinSession struct {
	Id             string             `json:"Id"`
	UserName       string             `json:"UserName"`
	Client         string             `json:"Client"`
	DeviceName     string             `json:"DeviceName"`
	NowPlayingItem *jellyfinItem      `json:"NowPlayingItem"`
	PlayState      jellyfinPlayState  `json:"PlayState"`
}

type jellyfinPlayState struct {
	IsPaused      bool  `json:"IsPaused"`
	PositionTicks int64 `json:"PositionTicks"`
}

type jellyfinUser struct {
	Name   string             `json:"Name"`
	ID     string             `json:"Id"`
	Policy jellyfinUserPolicy `json:"Policy"`
}

type jellyfinUserPolicy struct {
	IsAdministrator bool `json:"IsAdministrator"`
}
