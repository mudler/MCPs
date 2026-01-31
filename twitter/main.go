package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/dghubble/oauth1"
	twitter "github.com/g8rswimmer/go-twitter/v2"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	defaultMaxTweets = 50
	unansweredWindow = 24 * time.Hour
)

var debugLogging bool

func init() {
	// Disable log output by default so it doesn't break MCP stdio (stdout is used for JSON-RPC).
	// Set TWITTER_DEBUG=1 or MCP_TWITTER_DEBUG=1 to enable logging for debugging.
	debugLogging = os.Getenv("TWITTER_DEBUG") != "" || os.Getenv("MCP_TWITTER_DEBUG") != ""
	if !debugLogging {
		log.SetOutput(io.Discard)
	}
}

func debugLog(v ...interface{}) {
	if debugLogging {
		log.Println(v...)
	}
}

func debugLogf(format string, v ...interface{}) {
	if debugLogging {
		log.Printf(format, v...)
	}
}

var (
	client     *twitter.Client
	authUserID string
	maxTweets  int
	hasUserCtx bool
	v1Client   *http.Client
)

// bearerAuthorizer adds Bearer token to requests
type bearerAuthorizer struct {
	token string
}

func (b bearerAuthorizer) Add(req *http.Request) {
	req.Header.Add("Authorization", "Bearer "+b.token)
}

// noopAuthorizer used when OAuth 1.0a Transport signs the request
type noopAuthorizer struct{}

func (noopAuthorizer) Add(req *http.Request) {}

// --- Input/Output structs ---

type GetTweetsInput struct {
	UserID     string `json:"user_id" jsonschema:"Twitter user ID (numeric string)"`
	Username   string `json:"username,omitempty" jsonschema:"Twitter username (handle) - used if user_id not set"`
	MaxResults int    `json:"max_results,omitempty" jsonschema:"max tweets to return (default 50, cap 50)"`
}

type GetProfileInput struct {
	UserID   string `json:"user_id,omitempty" jsonschema:"Twitter user ID (numeric string)"`
	Username string `json:"username,omitempty" jsonschema:"Twitter username (handle)"`
}

type SearchTweetsInput struct {
	Query      string `json:"query" jsonschema:"search query (keywords, hashtags)"`
	SortOrder  string `json:"sort_order,omitempty" jsonschema:"recency or relevancy"`
	MaxResults int    `json:"max_results,omitempty" jsonschema:"max tweets (default 50, cap 50)"`
}

type LikeTweetInput struct {
	TweetID string `json:"tweet_id" jsonschema:"tweet ID to like"`
	Like    bool   `json:"like" jsonschema:"true to like, false to unlike"`
}

type RetweetInput struct {
	TweetID string `json:"tweet_id" jsonschema:"tweet ID to retweet"`
	Retweet bool   `json:"retweet" jsonschema:"true to retweet, false to undo retweet"`
}

type PostTweetInput struct {
	Text             string   `json:"text" jsonschema:"tweet text"`
	MediaIDs         []string `json:"media_ids,omitempty" jsonschema:"media IDs from upload_media"`
	InReplyToTweetID string   `json:"in_reply_to_tweet_id,omitempty" jsonschema:"tweet ID to reply to"`
	QuoteTweetID     string   `json:"quote_tweet_id,omitempty" jsonschema:"tweet ID to quote"`
}

type CreateThreadInput struct {
	Tweets []string `json:"tweets" jsonschema:"array of tweet texts in order"`
}

type GetTimelineInput struct {
	TimelineType string `json:"timeline_type" jsonschema:"home, user, or mentions"`
	UserID       string `json:"user_id,omitempty" jsonschema:"user ID for user/mentions timeline"`
	MaxResults   int    `json:"max_results,omitempty" jsonschema:"max tweets (default 50, cap 50)"`
}

type GetListTweetsInput struct {
	ListID     string `json:"list_id" jsonschema:"Twitter list ID"`
	MaxResults int    `json:"max_results,omitempty" jsonschema:"max tweets (default 50, cap 50)"`
}

type GetTrendsInput struct {
	WOEID int `json:"woeid" jsonschema:"Where On Earth ID (e.g. 1=worldwide, 23424977=US)"`
}

type GetUserRelationshipsInput struct {
	UserID     string `json:"user_id" jsonschema:"user ID"`
	Type       string `json:"type" jsonschema:"followers or following"`
	MaxResults int    `json:"max_results,omitempty" jsonschema:"max users (default 50, cap 100)"`
}

type FollowUserInput struct {
	TargetUserID string `json:"target_user_id" jsonschema:"user ID to follow/unfollow"`
	Follow       bool   `json:"follow" jsonschema:"true to follow, false to unfollow"`
}

type UploadMediaInput struct {
	ImageBase64 string `json:"image_base64,omitempty" jsonschema:"base64-encoded image data"`
	ImageURL    string `json:"image_url,omitempty" jsonschema:"URL of image to upload"`
}

// Simplified output types (JSON-friendly)
type TweetOut struct {
	ID        string         `json:"id"`
	Text      string         `json:"text"`
	AuthorID  string         `json:"author_id,omitempty"`
	CreatedAt string         `json:"created_at,omitempty"`
	Metrics   map[string]int `json:"public_metrics,omitempty"`
	MediaURLs []string       `json:"media_urls,omitempty"`
}

type UserOut struct {
	ID              string         `json:"id"`
	Name            string         `json:"name"`
	Username        string         `json:"username"`
	Description     string         `json:"description,omitempty"`
	ProfileImageURL string         `json:"profile_image_url,omitempty"`
	PublicMetrics   map[string]int `json:"public_metrics,omitempty"`
}

type GetTweetsOutput struct {
	Tweets []TweetOut `json:"tweets"`
	Count  int        `json:"count"`
}

type GetProfileOutput struct {
	User UserOut `json:"user"`
}

type SearchTweetsOutput struct {
	Tweets []TweetOut `json:"tweets"`
	Count  int        `json:"count"`
}

type GetTimelineOutput struct {
	Tweets []TweetOut `json:"tweets"`
	Count  int        `json:"count"`
}

type GetListTweetsOutput struct {
	Tweets []TweetOut `json:"tweets"`
	Count  int        `json:"count"`
}

type TrendOut struct {
	Name        string `json:"name"`
	Query       string `json:"query"`
	TweetVolume int    `json:"tweet_volume,omitempty"`
}

type GetTrendsOutput struct {
	Trends []TrendOut `json:"trends"`
	Count  int        `json:"count"`
}

type GetUserRelationshipsOutput struct {
	Users []UserOut `json:"users"`
	Count int       `json:"count"`
}

type ActionOutput struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type PostTweetOutput struct {
	TweetID string `json:"tweet_id"`
	Text    string `json:"text"`
}

type CreateThreadOutput struct {
	TweetIDs []string `json:"tweet_ids"`
	Count    int      `json:"count"`
}

type UploadMediaOutput struct {
	MediaID string `json:"media_id"`
}

func capMax(n, cap int) int {
	if n <= 0 {
		return cap
	}
	if n > cap {
		return cap
	}
	return n
}

func tweetFromObj(t *twitter.TweetObj, includes *twitter.TweetRawIncludes) TweetOut {
	out := TweetOut{
		ID:        t.ID,
		Text:      t.Text,
		AuthorID:  t.AuthorID,
		CreatedAt: t.CreatedAt,
	}
	if t.PublicMetrics != nil {
		out.Metrics = map[string]int{
			"like_count":       t.PublicMetrics.Likes,
			"reply_count":      t.PublicMetrics.Replies,
			"retweet_count":    t.PublicMetrics.Retweets,
			"quote_count":      t.PublicMetrics.Quotes,
			"impression_count": t.PublicMetrics.Impressions,
		}
	}
	if includes != nil && t.Attachments != nil && len(t.Attachments.MediaKeys) > 0 {
		for _, k := range t.Attachments.MediaKeys {
			if m := includes.MediaByKeys()[k]; m != nil && m.URL != "" {
				out.MediaURLs = append(out.MediaURLs, m.URL)
			}
		}
	}
	return out
}

func userFromObj(u *twitter.UserObj) UserOut {
	if u == nil {
		return UserOut{}
	}
	out := UserOut{
		ID:              u.ID,
		Name:            u.Name,
		Username:        u.UserName,
		Description:     u.Description,
		ProfileImageURL: u.ProfileImageURL,
	}
	if u.PublicMetrics != nil {
		out.PublicMetrics = map[string]int{
			"followers_count": u.PublicMetrics.Followers,
			"following_count": u.PublicMetrics.Following,
			"tweet_count":     u.PublicMetrics.Tweets,
		}
	}
	return out
}

func errMsg(err error) string {
	if err == nil {
		return ""
	}
	if e, ok := err.(*twitter.ErrorResponse); ok {
		return e.Detail
	}
	if e, ok := err.(*twitter.HTTPError); ok {
		return e.Error()
	}
	return err.Error()
}

// --- Handlers ---

func GetTweets(ctx context.Context, req *mcp.CallToolRequest, input GetTweetsInput) (*mcp.CallToolResult, GetTweetsOutput, error) {
	userID := input.UserID
	if userID == "" && input.Username != "" {
		resp, err := client.UserNameLookup(ctx, []string{input.Username}, twitter.UserLookupOpts{})
		if err != nil {
			return nil, GetTweetsOutput{}, fmt.Errorf("user lookup: %w", err)
		}
		if resp.Raw == nil || len(resp.Raw.Users) == 0 || resp.Raw.Users[0] == nil {
			return nil, GetTweetsOutput{}, fmt.Errorf("user not found: %s", input.Username)
		}
		userID = resp.Raw.Users[0].ID
	}
	if userID == "" {
		return nil, GetTweetsOutput{}, fmt.Errorf("user_id or username required")
	}
	n := capMax(input.MaxResults, maxTweets)
	if n == 0 {
		n = maxTweets
	}
	opts := twitter.UserTweetTimelineOpts{
		MaxResults:  n,
		TweetFields: []twitter.TweetField{twitter.TweetFieldCreatedAt, twitter.TweetFieldAuthorID, twitter.TweetFieldAttachments, twitter.TweetFieldPublicMetrics},
		Expansions:  []twitter.Expansion{twitter.ExpansionAttachmentsMediaKeys},
		MediaFields: []twitter.MediaField{twitter.MediaFieldURL},
	}
	resp, err := client.UserTweetTimeline(ctx, userID, opts)
	if err != nil {
		return nil, GetTweetsOutput{}, fmt.Errorf("timeline: %w", err)
	}
	var tweets []TweetOut
	if resp.Raw != nil {
		for _, t := range resp.Raw.Tweets {
			tweets = append(tweets, tweetFromObj(t, resp.Raw.Includes))
		}
	}
	return nil, GetTweetsOutput{Tweets: tweets, Count: len(tweets)}, nil
}

func GetProfile(ctx context.Context, req *mcp.CallToolRequest, input GetProfileInput) (*mcp.CallToolResult, GetProfileOutput, error) {
	if input.UserID != "" {
		resp, err := client.UserLookup(ctx, []string{input.UserID}, twitter.UserLookupOpts{
			UserFields: []twitter.UserField{twitter.UserFieldPublicMetrics, twitter.UserFieldProfileImageURL, twitter.UserFieldDescription},
		})
		if err != nil {
			return nil, GetProfileOutput{}, fmt.Errorf("user lookup: %w", err)
		}
		if resp.Raw != nil && len(resp.Raw.Users) > 0 {
			if u := resp.Raw.Users[0]; u != nil {
				return nil, GetProfileOutput{User: userFromObj(u)}, nil
			}
		}
		return nil, GetProfileOutput{}, fmt.Errorf("user not found: %s", input.UserID)
	}
	if input.Username != "" {
		resp, err := client.UserNameLookup(ctx, []string{input.Username}, twitter.UserLookupOpts{
			UserFields: []twitter.UserField{twitter.UserFieldPublicMetrics, twitter.UserFieldProfileImageURL, twitter.UserFieldDescription},
		})
		if err != nil {
			return nil, GetProfileOutput{}, fmt.Errorf("user lookup: %w", err)
		}
		if resp.Raw != nil && len(resp.Raw.Users) > 0 {
			if u := resp.Raw.Users[0]; u != nil {
				return nil, GetProfileOutput{User: userFromObj(u)}, nil
			}
		}
		return nil, GetProfileOutput{}, fmt.Errorf("user not found: %s", input.Username)
	}
	return nil, GetProfileOutput{}, fmt.Errorf("user_id or username required")
}

func SearchTweets(ctx context.Context, req *mcp.CallToolRequest, input SearchTweetsInput) (*mcp.CallToolResult, SearchTweetsOutput, error) {
	n := capMax(input.MaxResults, maxTweets)
	if n == 0 {
		n = maxTweets
	}
	opts := twitter.TweetRecentSearchOpts{
		MaxResults:  n,
		TweetFields: []twitter.TweetField{twitter.TweetFieldCreatedAt, twitter.TweetFieldAuthorID, twitter.TweetFieldPublicMetrics},
		Expansions:  []twitter.Expansion{twitter.ExpansionAuthorID},
		UserFields:  []twitter.UserField{twitter.UserFieldUserName},
	}
	if input.SortOrder == "recency" {
		opts.SortOrder = twitter.TweetSearchSortOrderRecency
	} else {
		opts.SortOrder = twitter.TweetSearchSortOrderRelevancy
	}
	resp, err := client.TweetRecentSearch(ctx, input.Query, opts)
	if err != nil {
		return nil, SearchTweetsOutput{}, fmt.Errorf("search: %w", err)
	}
	var tweets []TweetOut
	if resp.Raw != nil {
		for _, t := range resp.Raw.Tweets {
			tweets = append(tweets, tweetFromObj(t, resp.Raw.Includes))
		}
	}
	return nil, SearchTweetsOutput{Tweets: tweets, Count: len(tweets)}, nil
}

func LikeTweet(ctx context.Context, req *mcp.CallToolRequest, input LikeTweetInput) (*mcp.CallToolResult, ActionOutput, error) {
	if !hasUserCtx {
		return nil, ActionOutput{}, fmt.Errorf("like_tweet requires user context (OAuth 1.0a)")
	}
	if input.Like {
		_, err := client.UserLikes(ctx, authUserID, input.TweetID)
		if err != nil {
			return nil, ActionOutput{Success: false, Message: errMsg(err)}, nil
		}
		return nil, ActionOutput{Success: true, Message: "liked"}, nil
	}
	_, err := client.DeleteUserLikes(ctx, authUserID, input.TweetID)
	if err != nil {
		return nil, ActionOutput{Success: false, Message: errMsg(err)}, nil
	}
	return nil, ActionOutput{Success: true, Message: "unliked"}, nil
}

func Retweet(ctx context.Context, req *mcp.CallToolRequest, input RetweetInput) (*mcp.CallToolResult, ActionOutput, error) {
	if !hasUserCtx {
		return nil, ActionOutput{}, fmt.Errorf("retweet requires user context (OAuth 1.0a)")
	}
	if input.Retweet {
		_, err := client.UserRetweet(ctx, authUserID, input.TweetID)
		if err != nil {
			return nil, ActionOutput{Success: false, Message: errMsg(err)}, nil
		}
		return nil, ActionOutput{Success: true, Message: "retweeted"}, nil
	}
	_, err := client.DeleteUserRetweet(ctx, authUserID, input.TweetID)
	if err != nil {
		return nil, ActionOutput{Success: false, Message: errMsg(err)}, nil
	}
	return nil, ActionOutput{Success: true, Message: "retweet undone"}, nil
}

func PostTweet(ctx context.Context, req *mcp.CallToolRequest, input PostTweetInput) (*mcp.CallToolResult, PostTweetOutput, error) {
	if !hasUserCtx {
		return nil, PostTweetOutput{}, fmt.Errorf("post_tweet requires user context (OAuth 1.0a)")
	}
	create := twitter.CreateTweetRequest{Text: input.Text}
	if input.InReplyToTweetID != "" {
		create.Reply = &twitter.CreateTweetReply{InReplyToTweetID: input.InReplyToTweetID}
	}
	if input.QuoteTweetID != "" {
		create.QuoteTweetID = input.QuoteTweetID
	}
	if len(input.MediaIDs) > 0 {
		create.Media = &twitter.CreateTweetMedia{IDs: input.MediaIDs}
	}
	if create.Text == "" && (create.Media == nil || len(create.Media.IDs) == 0) {
		return nil, PostTweetOutput{}, fmt.Errorf("text or media_ids required")
	}
	resp, err := client.CreateTweet(ctx, create)
	if err != nil {
		return nil, PostTweetOutput{}, fmt.Errorf("create tweet: %w", err)
	}
	if resp.Tweet == nil {
		return nil, PostTweetOutput{}, fmt.Errorf("no tweet in response")
	}
	return nil, PostTweetOutput{TweetID: resp.Tweet.ID, Text: resp.Tweet.Text}, nil
}

func CreateThread(ctx context.Context, req *mcp.CallToolRequest, input CreateThreadInput) (*mcp.CallToolResult, CreateThreadOutput, error) {
	if !hasUserCtx {
		return nil, CreateThreadOutput{}, fmt.Errorf("create_thread requires user context (OAuth 1.0a)")
	}
	if len(input.Tweets) == 0 {
		return nil, CreateThreadOutput{}, fmt.Errorf("tweets array required")
	}
	var ids []string
	var lastID string
	for i, text := range input.Tweets {
		create := twitter.CreateTweetRequest{Text: text}
		if i > 0 {
			create.Reply = &twitter.CreateTweetReply{InReplyToTweetID: lastID}
		}
		resp, err := client.CreateTweet(ctx, create)
		if err != nil {
			return nil, CreateThreadOutput{}, fmt.Errorf("tweet %d: %w", i+1, err)
		}
		if resp.Tweet == nil {
			return nil, CreateThreadOutput{}, fmt.Errorf("no tweet in response for tweet %d", i+1)
		}
		lastID = resp.Tweet.ID
		ids = append(ids, lastID)
	}
	return nil, CreateThreadOutput{TweetIDs: ids, Count: len(ids)}, nil
}

func GetTimeline(ctx context.Context, req *mcp.CallToolRequest, input GetTimelineInput) (*mcp.CallToolResult, GetTimelineOutput, error) {
	n := capMax(input.MaxResults, maxTweets)
	if n == 0 {
		n = maxTweets
	}
	opts := twitter.UserTweetTimelineOpts{
		MaxResults:  n,
		TweetFields: []twitter.TweetField{twitter.TweetFieldCreatedAt, twitter.TweetFieldAuthorID, twitter.TweetFieldPublicMetrics},
		Expansions:  []twitter.Expansion{twitter.ExpansionAuthorID},
		UserFields:  []twitter.UserField{twitter.UserFieldUserName},
	}
	var tweets []*twitter.TweetObj
	var includes *twitter.TweetRawIncludes
	switch strings.ToLower(input.TimelineType) {
	case "home":
		if !hasUserCtx {
			return nil, GetTimelineOutput{}, fmt.Errorf("home timeline requires user context (OAuth 1.0a)")
		}
		revOpts := twitter.UserTweetReverseChronologicalTimelineOpts{
			MaxResults:  n,
			TweetFields: []twitter.TweetField{twitter.TweetFieldCreatedAt, twitter.TweetFieldAuthorID, twitter.TweetFieldPublicMetrics},
			Expansions:  []twitter.Expansion{twitter.ExpansionAuthorID},
			UserFields:  []twitter.UserField{twitter.UserFieldUserName},
		}
		resp, err := client.UserTweetReverseChronologicalTimeline(ctx, authUserID, revOpts)
		if err != nil {
			return nil, GetTimelineOutput{}, fmt.Errorf("home timeline: %w", err)
		}
		if resp.Raw != nil {
			tweets = resp.Raw.Tweets
			includes = resp.Raw.Includes
		}
	case "user":
		uid := input.UserID
		if uid == "" && hasUserCtx {
			uid = authUserID
		}
		if uid == "" {
			return nil, GetTimelineOutput{}, fmt.Errorf("user_id required for user timeline")
		}
		resp, err := client.UserTweetTimeline(ctx, uid, opts)
		if err != nil {
			return nil, GetTimelineOutput{}, fmt.Errorf("user timeline: %w", err)
		}
		if resp.Raw != nil {
			tweets = resp.Raw.Tweets
			includes = resp.Raw.Includes
		}
	case "mentions":
		uid := input.UserID
		if uid == "" && hasUserCtx {
			uid = authUserID
		}
		if uid == "" {
			return nil, GetTimelineOutput{}, fmt.Errorf("user_id required for mentions timeline")
		}
		mentOpts := twitter.UserMentionTimelineOpts{
			MaxResults:  n,
			TweetFields: []twitter.TweetField{twitter.TweetFieldCreatedAt, twitter.TweetFieldAuthorID, twitter.TweetFieldPublicMetrics},
			Expansions:  []twitter.Expansion{twitter.ExpansionAuthorID},
			UserFields:  []twitter.UserField{twitter.UserFieldUserName},
		}
		resp, err := client.UserMentionTimeline(ctx, uid, mentOpts)
		if err != nil {
			return nil, GetTimelineOutput{}, fmt.Errorf("mentions timeline: %w", err)
		}
		if resp.Raw != nil {
			tweets = resp.Raw.Tweets
			includes = resp.Raw.Includes
		}
	default:
		return nil, GetTimelineOutput{}, fmt.Errorf("timeline_type must be home, user, or mentions")
	}
	out := make([]TweetOut, 0, len(tweets))
	for _, t := range tweets {
		out = append(out, tweetFromObj(t, includes))
	}
	return nil, GetTimelineOutput{Tweets: out, Count: len(out)}, nil
}

func GetUnansweredMentions(ctx context.Context, req *mcp.CallToolRequest, input struct{}) (*mcp.CallToolResult, GetTimelineOutput, error) {
	if !hasUserCtx {
		return nil, GetTimelineOutput{}, fmt.Errorf("get_unanswered_mentions requires user context (OAuth 1.0a)")
	}
	if authUserID == "" {
		return nil, GetTimelineOutput{}, fmt.Errorf("auth user ID not resolved (rate limited or lookup failed); try again later")
	}
	startTime := time.Now().Add(-unansweredWindow)
	mentOpts := twitter.UserMentionTimelineOpts{
		StartTime:   startTime,
		MaxResults:  100,
		TweetFields: []twitter.TweetField{twitter.TweetFieldCreatedAt, twitter.TweetFieldAuthorID, twitter.TweetFieldReferencedTweets},
	}
	mentResp, err := client.UserMentionTimeline(ctx, authUserID, mentOpts)
	if err != nil {
		return nil, GetTimelineOutput{}, fmt.Errorf("mentions: %w", err)
	}
	repliedToIDs := make(map[string]struct{})
	tlOpts := twitter.UserTweetTimelineOpts{
		MaxResults:  100,
		TweetFields: []twitter.TweetField{twitter.TweetFieldReferencedTweets},
	}
	tlResp, err := client.UserTweetTimeline(ctx, authUserID, tlOpts)
	if err == nil && tlResp.Raw != nil {
		for _, t := range tlResp.Raw.Tweets {
			for _, ref := range t.ReferencedTweets {
				if ref != nil && ref.Type == "replied_to" {
					repliedToIDs[ref.ID] = struct{}{}
				}
			}
		}
	}
	var out []TweetOut
	if mentResp.Raw != nil {
		for _, t := range mentResp.Raw.Tweets {
			if _, ok := repliedToIDs[t.ID]; ok {
				continue
			}
			out = append(out, tweetFromObj(t, mentResp.Raw.Includes))
			if len(out) >= maxTweets {
				break
			}
		}
	}
	return nil, GetTimelineOutput{Tweets: out, Count: len(out)}, nil
}

func GetListTweets(ctx context.Context, req *mcp.CallToolRequest, input GetListTweetsInput) (*mcp.CallToolResult, GetListTweetsOutput, error) {
	n := capMax(input.MaxResults, maxTweets)
	if n == 0 {
		n = maxTweets
	}
	opts := twitter.ListTweetLookupOpts{
		MaxResults:  n,
		TweetFields: []twitter.TweetField{twitter.TweetFieldCreatedAt, twitter.TweetFieldAuthorID, twitter.TweetFieldPublicMetrics},
		Expansions:  []twitter.Expansion{twitter.ExpansionAuthorID},
		UserFields:  []twitter.UserField{twitter.UserFieldUserName},
	}
	resp, err := client.ListTweetLookup(ctx, input.ListID, opts)
	if err != nil {
		return nil, GetListTweetsOutput{}, fmt.Errorf("list tweets: %w", err)
	}
	var tweets []TweetOut
	if resp.Raw != nil {
		for _, t := range resp.Raw.Tweets {
			tweets = append(tweets, tweetFromObj(t, resp.Raw.Includes))
		}
	}
	return nil, GetListTweetsOutput{Tweets: tweets, Count: len(tweets)}, nil
}

func GetTrends(ctx context.Context, _ *mcp.CallToolRequest, input GetTrendsInput) (*mcp.CallToolResult, GetTrendsOutput, error) {
	if v1Client == nil {
		return nil, GetTrendsOutput{}, fmt.Errorf("trends require OAuth 1.0a (v1.1 API)")
	}
	url := fmt.Sprintf("https://api.twitter.com/1.1/trends/place.json?id=%d", input.WOEID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, GetTrendsOutput{}, err
	}
	resp, err := v1Client.Do(httpReq)
	if err != nil {
		return nil, GetTrendsOutput{}, fmt.Errorf("trends request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, GetTrendsOutput{}, fmt.Errorf("trends API: %s %s", resp.Status, string(body))
	}
	var raw []struct {
		Trends []struct {
			Name        string `json:"name"`
			URL         string `json:"url"`
			Query       string `json:"query"`
			TweetVolume *int   `json:"tweet_volume,omitempty"`
		} `json:"trends"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, GetTrendsOutput{}, fmt.Errorf("decode trends: %w", err)
	}
	var trends []TrendOut
	if len(raw) > 0 {
		for _, tr := range raw[0].Trends {
			vol := 0
			if tr.TweetVolume != nil {
				vol = *tr.TweetVolume
			}
			trends = append(trends, TrendOut{Name: tr.Name, Query: tr.Query, TweetVolume: vol})
		}
	}
	return nil, GetTrendsOutput{Trends: trends, Count: len(trends)}, nil
}

func GetUserRelationships(ctx context.Context, req *mcp.CallToolRequest, input GetUserRelationshipsInput) (*mcp.CallToolResult, GetUserRelationshipsOutput, error) {
	n := capMax(input.MaxResults, 100)
	if n == 0 {
		n = 50
	}
	var users []UserOut
	if strings.ToLower(input.Type) == "followers" {
		opts := twitter.UserFollowersLookupOpts{MaxResults: n}
		resp, err := client.UserFollowersLookup(ctx, input.UserID, opts)
		if err != nil {
			return nil, GetUserRelationshipsOutput{}, fmt.Errorf("followers: %w", err)
		}
		if resp.Raw != nil {
			for _, u := range resp.Raw.Users {
				users = append(users, userFromObj(u))
			}
		}
	} else {
		opts := twitter.UserFollowingLookupOpts{MaxResults: n}
		resp, err := client.UserFollowingLookup(ctx, input.UserID, opts)
		if err != nil {
			return nil, GetUserRelationshipsOutput{}, fmt.Errorf("following: %w", err)
		}
		if resp.Raw != nil {
			for _, u := range resp.Raw.Users {
				users = append(users, userFromObj(u))
			}
		}
	}
	return nil, GetUserRelationshipsOutput{Users: users, Count: len(users)}, nil
}

func FollowUser(ctx context.Context, req *mcp.CallToolRequest, input FollowUserInput) (*mcp.CallToolResult, ActionOutput, error) {
	if !hasUserCtx {
		return nil, ActionOutput{}, fmt.Errorf("follow_user requires user context (OAuth 1.0a)")
	}
	if input.Follow {
		_, err := client.UserFollows(ctx, authUserID, input.TargetUserID)
		if err != nil {
			return nil, ActionOutput{Success: false, Message: errMsg(err)}, nil
		}
		return nil, ActionOutput{Success: true, Message: "following"}, nil
	}
	_, err := client.DeleteUserFollows(ctx, authUserID, input.TargetUserID)
	if err != nil {
		return nil, ActionOutput{Success: false, Message: errMsg(err)}, nil
	}
	return nil, ActionOutput{Success: true, Message: "unfollowed"}, nil
}

func UploadMedia(ctx context.Context, req *mcp.CallToolRequest, input UploadMediaInput) (*mcp.CallToolResult, UploadMediaOutput, error) {
	if !hasUserCtx || v1Client == nil {
		return nil, UploadMediaOutput{}, fmt.Errorf("upload_media requires OAuth 1.0a")
	}
	var body []byte
	if input.ImageBase64 != "" {
		var err error
		body, err = base64.StdEncoding.DecodeString(input.ImageBase64)
		if err != nil {
			return nil, UploadMediaOutput{}, fmt.Errorf("decode base64: %w", err)
		}
	} else if input.ImageURL != "" {
		resp, err := http.Get(input.ImageURL)
		if err != nil {
			return nil, UploadMediaOutput{}, fmt.Errorf("fetch image: %w", err)
		}
		defer resp.Body.Close()
		body, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, UploadMediaOutput{}, fmt.Errorf("read image: %w", err)
		}
	} else {
		return nil, UploadMediaOutput{}, fmt.Errorf("image_base64 or image_url required")
	}
	mediaID, err := uploadMediaV1(ctx, body)
	if err != nil {
		return nil, UploadMediaOutput{}, err
	}
	return nil, UploadMediaOutput{MediaID: mediaID}, nil
}

func uploadMediaV1(ctx context.Context, data []byte) (string, error) {
	initURL := "https://upload.twitter.com/1.1/media/upload.json"
	form := fmt.Sprintf("command=INIT&total_bytes=%d&media_type=image/jpeg", len(data))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, initURL, strings.NewReader(form))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := v1Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("media INIT: %w", err)
	}
	defer resp.Body.Close()
	var initResp struct {
		MediaIDString string `json:"media_id_string"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&initResp); err != nil {
		return "", fmt.Errorf("media INIT decode: %w", err)
	}
	if initResp.MediaIDString == "" {
		return "", fmt.Errorf("media INIT: no media_id_string")
	}
	appendURL := "https://upload.twitter.com/1.1/media/upload.json"
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	_ = w.WriteField("command", "APPEND")
	_ = w.WriteField("media_id", initResp.MediaIDString)
	_ = w.WriteField("segment_index", "0")
	part, _ := w.CreateFormFile("media", "image.jpg")
	_, _ = part.Write(data)
	contentType := w.FormDataContentType()
	_ = w.Close()
	req2, err := http.NewRequestWithContext(ctx, http.MethodPost, appendURL, &body)
	if err != nil {
		return "", err
	}
	req2.Header.Set("Content-Type", contentType)
	resp2, err := v1Client.Do(req2)
	if err != nil {
		return "", fmt.Errorf("media APPEND: %w", err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK && resp2.StatusCode != http.StatusNoContent {
		return "", fmt.Errorf("media APPEND: %s", resp2.Status)
	}
	finForm := fmt.Sprintf("command=FINALIZE&media_id=%s", initResp.MediaIDString)
	req3, err := http.NewRequestWithContext(ctx, http.MethodPost, appendURL, strings.NewReader(finForm))
	if err != nil {
		return "", err
	}
	req3.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp3, err := v1Client.Do(req3)
	if err != nil {
		return "", fmt.Errorf("media FINALIZE: %w", err)
	}
	defer resp3.Body.Close()
	var finResp struct {
		MediaIDString string `json:"media_id_string"`
	}
	if err := json.NewDecoder(resp3.Body).Decode(&finResp); err != nil {
		return "", fmt.Errorf("media FINALIZE decode: %w", err)
	}
	return initResp.MediaIDString, nil
}

// InitClientFromEnv initializes the Twitter client from environment variables.
// Used by main and by acceptance tests. Returns true if credentials were set.
func InitClientFromEnv() bool {
	maxTweets = defaultMaxTweets
	if n, err := strconv.Atoi(os.Getenv("TWITTER_MAX_TWEETS")); err == nil && n > 0 {
		maxTweets = n
		if maxTweets > 100 {
			maxTweets = 100
		}
	}
	apiKey := os.Getenv("TWITTER_API_KEY")
	apiSecret := os.Getenv("TWITTER_API_SECRET")
	accessToken := os.Getenv("TWITTER_ACCESS_TOKEN")
	accessSecret := os.Getenv("TWITTER_ACCESS_SECRET")
	bearer := os.Getenv("TWITTER_BEARER_TOKEN")
	if apiKey != "" && apiSecret != "" && accessToken != "" && accessSecret != "" {
		config := oauth1.NewConfig(apiKey, apiSecret)
		token := oauth1.NewToken(accessToken, accessSecret)
		v1Client = config.Client(context.Background(), token)
		client = &twitter.Client{
			Authorizer: noopAuthorizer{},
			Client:     v1Client,
			Host:       "https://api.twitter.com",
		}
		hasUserCtx = true
		ctx := context.Background()
		resp, err := client.AuthUserLookup(ctx, twitter.UserLookupOpts{})
		if err != nil {
			debugLogf("Warning: could not resolve auth user: %v", err)
		} else if resp.Raw != nil && len(resp.Raw.Users) > 0 {
			authUserID = resp.Raw.Users[0].ID
		}
		debugLog("Twitter MCP: using OAuth 1.0a user context")
		return true
	}
	if bearer != "" {
		client = &twitter.Client{
			Authorizer: bearerAuthorizer{token: bearer},
			Client:     http.DefaultClient,
			Host:       "https://api.twitter.com",
		}
		debugLog("Twitter MCP: using Bearer (app-only); write and home/mentions tools will fail without user context")
		return true
	}
	return false
}

func main() {
	if !InitClientFromEnv() {
		fmt.Fprintln(os.Stderr, "twitter MCP: Set TWITTER_BEARER_TOKEN or all of TWITTER_API_KEY, TWITTER_API_SECRET, TWITTER_ACCESS_TOKEN, TWITTER_ACCESS_SECRET")
		os.Exit(1)
	}

	server := mcp.NewServer(&mcp.Implementation{Name: "twitter", Version: "v1.0.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "get_tweets", Description: "Fetch recent tweets from a user (with media support)"}, GetTweets)
	mcp.AddTool(server, &mcp.Tool{Name: "get_profile", Description: "Get a user's profile information"}, GetProfile)
	mcp.AddTool(server, &mcp.Tool{Name: "search_tweets", Description: "Search for tweets by hashtag or keyword"}, SearchTweets)
	mcp.AddTool(server, &mcp.Tool{Name: "like_tweet", Description: "Like or unlike a tweet"}, LikeTweet)
	mcp.AddTool(server, &mcp.Tool{Name: "retweet", Description: "Retweet or undo retweet"}, Retweet)
	mcp.AddTool(server, &mcp.Tool{Name: "post_tweet", Description: "Post a new tweet with optional media, reply, or quote"}, PostTweet)
	mcp.AddTool(server, &mcp.Tool{Name: "create_thread", Description: "Create a Twitter thread"}, CreateThread)
	mcp.AddTool(server, &mcp.Tool{Name: "get_timeline", Description: "Get tweets from home, user, or mentions timeline"}, GetTimeline)
	mcp.AddTool(server, &mcp.Tool{Name: "get_unanswered_mentions", Description: "Get tweets that mention you and you have not replied to (last 24 hours)"}, GetUnansweredMentions)
	mcp.AddTool(server, &mcp.Tool{Name: "get_list_tweets", Description: "Get tweets from a Twitter list"}, GetListTweets)
	mcp.AddTool(server, &mcp.Tool{Name: "get_trends", Description: "Get current trending topics by place (WOEID)"}, GetTrends)
	mcp.AddTool(server, &mcp.Tool{Name: "get_user_relationships", Description: "Get followers or following list"}, GetUserRelationships)
	mcp.AddTool(server, &mcp.Tool{Name: "follow_user", Description: "Follow or unfollow a user"}, FollowUser)
	mcp.AddTool(server, &mcp.Tool{Name: "upload_media", Description: "Upload an image (JPEG/PNG/GIF) and get media_id for post_tweet"}, UploadMedia)
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		fmt.Fprintln(os.Stderr, "twitter MCP:", err)
		os.Exit(1)
	}
}
