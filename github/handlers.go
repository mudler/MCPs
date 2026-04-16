package main

import (
	"context"
	"encoding/json"
	"fmt"
	neturl "net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	kindIssue       = "issue"
	kindPullRequest = "pull"
	commentsPerPage = 100
	commentsMaxPage = 5
)

var ghURLRegexp = regexp.MustCompile(`(?i)^https?://[^/]+/([^/]+)/([^/]+)/(issues|pull)/(\d+)(?:[/?#].*)?$`)

// parseGitHubURL parses a GitHub issue or pull request URL into its parts.
// kind is either kindIssue or kindPullRequest.
func parseGitHubURL(rawURL string) (owner, repo string, number int, kind string, err error) {
	rawURL = strings.TrimSpace(rawURL)
	m := ghURLRegexp.FindStringSubmatch(rawURL)
	if m == nil {
		return "", "", 0, "", fmt.Errorf("not a recognized GitHub issue or pull request URL: %q", rawURL)
	}
	owner = m[1]
	repo = m[2]
	switch strings.ToLower(m[3]) {
	case "issues":
		kind = kindIssue
	case "pull":
		kind = kindPullRequest
	}
	number, err = strconv.Atoi(m[4])
	if err != nil {
		return "", "", 0, "", fmt.Errorf("invalid number in URL %q: %w", rawURL, err)
	}
	return owner, repo, number, kind, nil
}

// resolveTarget normalizes (owner, repo, number) or url inputs into canonical parts.
// expectKind is kindIssue or kindPullRequest — if the URL's kind doesn't match, returns an error.
func resolveTarget(url, owner, repo string, number int, expectKind string) (string, string, int, error) {
	if url != "" {
		o, r, n, k, err := parseGitHubURL(url)
		if err != nil {
			return "", "", 0, err
		}
		if k != expectKind {
			return "", "", 0, fmt.Errorf("URL %q is a %s URL but this tool expects a %s URL", url, k, expectKind)
		}
		return o, r, n, nil
	}
	if owner == "" || repo == "" || number <= 0 {
		return "", "", 0, fmt.Errorf("either url or (owner, repo, number) must be provided")
	}
	return owner, repo, number, nil
}

func truncateBody(s string) (string, bool) {
	if maxCommentLen <= 0 || len(s) <= maxCommentLen {
		return s, false
	}
	return s[:maxCommentLen] + "\n...[truncated]", true
}

func toCommentSummaries(in []ghComment) []CommentSummary {
	out := make([]CommentSummary, 0, len(in))
	for _, c := range in {
		body, truncated := truncateBody(c.Body)
		out = append(out, CommentSummary{
			Author:    c.User.Login,
			CreatedAt: c.CreatedAt,
			Body:      body,
			Truncated: truncated,
		})
	}
	return out
}

func labelNames(in []ghLabel) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	for _, l := range in {
		out = append(out, l.Name)
	}
	return out
}

// fetchIssueComments follows the Link header up to commentsMaxPage pages.
func fetchIssueComments(ctx context.Context, owner, repo string, number int) ([]ghComment, error) {
	var all []ghComment
	path := fmt.Sprintf("/repos/%s/%s/issues/%d/comments", owner, repo, number)
	q := neturl.Values{}
	q.Set("per_page", strconv.Itoa(commentsPerPage))

	for page := 1; page <= commentsMaxPage; page++ {
		q.Set("page", strconv.Itoa(page))
		body, resp, err := client.Get(ctx, path, q)
		if err != nil {
			return nil, fmt.Errorf("fetching comments (page %d): %w", page, err)
		}
		var batch []ghComment
		if err := json.Unmarshal(body, &batch); err != nil {
			return nil, fmt.Errorf("parsing comments response: %w", err)
		}
		all = append(all, batch...)
		if len(batch) < commentsPerPage {
			break
		}
		if resp != nil && !linkHeaderHasNext(resp.Header.Get("Link")) {
			break
		}
	}
	return all, nil
}

func linkHeaderHasNext(h string) bool {
	if h == "" {
		return false
	}
	return strings.Contains(h, `rel="next"`)
}

// GetIssue fetches an issue and its comments.
func GetIssue(ctx context.Context, _ *mcp.CallToolRequest, input GetIssueInput) (*mcp.CallToolResult, GetIssueOutput, error) {
	owner, repo, number, err := resolveTarget(input.URL, input.Owner, input.Repo, input.Number, kindIssue)
	if err != nil {
		return nil, GetIssueOutput{}, err
	}

	body, _, err := client.Get(ctx, fmt.Sprintf("/repos/%s/%s/issues/%d", owner, repo, number), nil)
	if err != nil {
		return nil, GetIssueOutput{}, fmt.Errorf("fetching issue: %w", err)
	}
	var issue ghIssue
	if err := json.Unmarshal(body, &issue); err != nil {
		return nil, GetIssueOutput{}, fmt.Errorf("parsing issue response: %w", err)
	}

	var comments []CommentSummary
	if issue.Comments > 0 {
		raw, err := fetchIssueComments(ctx, owner, repo, number)
		if err != nil {
			return nil, GetIssueOutput{}, err
		}
		comments = toCommentSummaries(raw)
	}

	issueBody, _ := truncateBody(issue.Body)
	return nil, GetIssueOutput{
		Issue: IssueDetail{
			Number:        issue.Number,
			Title:         issue.Title,
			State:         issue.State,
			Author:        issue.User.Login,
			CreatedAt:     issue.CreatedAt,
			UpdatedAt:     issue.UpdatedAt,
			Body:          issueBody,
			Labels:        labelNames(issue.Labels),
			URL:           issue.HTMLURL,
			Comments:      comments,
			CommentsTotal: len(comments),
		},
	}, nil
}

// GetPullRequest fetches a pull request, its comments, and (optionally) its diff.
func GetPullRequest(ctx context.Context, _ *mcp.CallToolRequest, input GetPullRequestInput) (*mcp.CallToolResult, GetPullRequestOutput, error) {
	owner, repo, number, err := resolveTarget(input.URL, input.Owner, input.Repo, input.Number, kindPullRequest)
	if err != nil {
		return nil, GetPullRequestOutput{}, err
	}

	path := fmt.Sprintf("/repos/%s/%s/pulls/%d", owner, repo, number)
	body, _, err := client.Get(ctx, path, nil)
	if err != nil {
		return nil, GetPullRequestOutput{}, fmt.Errorf("fetching pull request: %w", err)
	}
	var pr ghPullRequest
	if err := json.Unmarshal(body, &pr); err != nil {
		return nil, GetPullRequestOutput{}, fmt.Errorf("parsing pull request response: %w", err)
	}

	raw, err := fetchIssueComments(ctx, owner, repo, number)
	if err != nil {
		return nil, GetPullRequestOutput{}, err
	}
	comments := toCommentSummaries(raw)

	var diff string
	if input.IncludeDiff {
		diffBytes, err := client.GetRaw(ctx, path, diffAccept)
		if err != nil {
			return nil, GetPullRequestOutput{}, fmt.Errorf("fetching diff: %w", err)
		}
		diff = string(diffBytes)
	}

	prBody, _ := truncateBody(pr.Body)
	return nil, GetPullRequestOutput{
		PullRequest: PullRequestDetail{
			Number:        pr.Number,
			Title:         pr.Title,
			State:         pr.State,
			Merged:        pr.Merged,
			Draft:         pr.Draft,
			Author:        pr.User.Login,
			CreatedAt:     pr.CreatedAt,
			UpdatedAt:     pr.UpdatedAt,
			Body:          prBody,
			BaseRef:       pr.Base.Ref,
			HeadRef:       pr.Head.Ref,
			Labels:        labelNames(pr.Labels),
			URL:           pr.HTMLURL,
			Comments:      comments,
			CommentsTotal: len(comments),
			Diff:          diff,
		},
	}, nil
}
