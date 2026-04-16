package main

// --- Input types ---

type GetIssueInput struct {
	URL    string `json:"url,omitempty" jsonschema:"full GitHub issue URL (alternative to owner/repo/number), e.g. https://github.com/owner/repo/issues/123"`
	Owner  string `json:"owner,omitempty" jsonschema:"repository owner (used together with repo and number)"`
	Repo   string `json:"repo,omitempty" jsonschema:"repository name (used together with owner and number)"`
	Number int    `json:"number,omitempty" jsonschema:"issue number (used together with owner and repo)"`
}

type GetPullRequestInput struct {
	URL         string `json:"url,omitempty" jsonschema:"full GitHub pull request URL (alternative to owner/repo/number), e.g. https://github.com/owner/repo/pull/123"`
	Owner       string `json:"owner,omitempty" jsonschema:"repository owner (used together with repo and number)"`
	Repo        string `json:"repo,omitempty" jsonschema:"repository name (used together with owner and number)"`
	Number      int    `json:"number,omitempty" jsonschema:"pull request number (used together with owner and repo)"`
	IncludeDiff bool   `json:"include_diff,omitempty" jsonschema:"when true, also fetch the unified diff of the pull request"`
}

// --- Output types ---

type CommentSummary struct {
	Author    string `json:"author" jsonschema:"the comment author's login"`
	CreatedAt string `json:"created_at,omitempty" jsonschema:"ISO-8601 timestamp when the comment was created"`
	Body      string `json:"body" jsonschema:"the comment body (may be truncated)"`
	Truncated bool   `json:"truncated,omitempty" jsonschema:"true if the body was truncated"`
}

type IssueDetail struct {
	Number        int              `json:"number" jsonschema:"the issue number"`
	Title         string           `json:"title" jsonschema:"the issue title"`
	State         string           `json:"state" jsonschema:"open or closed"`
	Author        string           `json:"author,omitempty" jsonschema:"the issue author's login"`
	CreatedAt     string           `json:"created_at,omitempty" jsonschema:"ISO-8601 timestamp when the issue was created"`
	UpdatedAt     string           `json:"updated_at,omitempty" jsonschema:"ISO-8601 timestamp when the issue was last updated"`
	Body          string           `json:"body,omitempty" jsonschema:"the issue description/body"`
	Labels        []string         `json:"labels,omitempty" jsonschema:"list of label names applied to the issue"`
	URL           string           `json:"url" jsonschema:"the HTML URL of the issue"`
	Comments      []CommentSummary `json:"comments,omitempty" jsonschema:"issue comments"`
	CommentsTotal int              `json:"comments_total" jsonschema:"total number of comments returned"`
}

type GetIssueOutput struct {
	Issue IssueDetail `json:"issue" jsonschema:"the issue details"`
}

type PullRequestDetail struct {
	Number        int              `json:"number" jsonschema:"the pull request number"`
	Title         string           `json:"title" jsonschema:"the pull request title"`
	State         string           `json:"state" jsonschema:"open or closed"`
	Merged        bool             `json:"merged" jsonschema:"true if the PR has been merged"`
	Draft         bool             `json:"draft" jsonschema:"true if the PR is a draft"`
	Author        string           `json:"author,omitempty" jsonschema:"the PR author's login"`
	CreatedAt     string           `json:"created_at,omitempty" jsonschema:"ISO-8601 timestamp when the PR was created"`
	UpdatedAt     string           `json:"updated_at,omitempty" jsonschema:"ISO-8601 timestamp when the PR was last updated"`
	Body          string           `json:"body,omitempty" jsonschema:"the PR description/body"`
	BaseRef       string           `json:"base_ref,omitempty" jsonschema:"the base branch name"`
	HeadRef       string           `json:"head_ref,omitempty" jsonschema:"the head branch name"`
	Labels        []string         `json:"labels,omitempty" jsonschema:"list of label names applied to the PR"`
	URL           string           `json:"url" jsonschema:"the HTML URL of the PR"`
	Comments      []CommentSummary `json:"comments,omitempty" jsonschema:"PR discussion comments"`
	CommentsTotal int              `json:"comments_total" jsonschema:"total number of comments returned"`
	Diff          string           `json:"diff,omitempty" jsonschema:"the unified diff (only present when include_diff was set)"`
}

type GetPullRequestOutput struct {
	PullRequest PullRequestDetail `json:"pull_request" jsonschema:"the pull request details"`
}

// --- Internal GitHub REST API response types ---

type ghUser struct {
	Login string `json:"login"`
}

type ghLabel struct {
	Name string `json:"name"`
}

type ghIssue struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	State     string    `json:"state"`
	Body      string    `json:"body"`
	HTMLURL   string    `json:"html_url"`
	User      ghUser    `json:"user"`
	Labels    []ghLabel `json:"labels"`
	CreatedAt string    `json:"created_at"`
	UpdatedAt string    `json:"updated_at"`
	Comments  int       `json:"comments"`
}

type ghRef struct {
	Ref string `json:"ref"`
}

type ghPullRequest struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	State     string    `json:"state"`
	Body      string    `json:"body"`
	HTMLURL   string    `json:"html_url"`
	User      ghUser    `json:"user"`
	Labels    []ghLabel `json:"labels"`
	CreatedAt string    `json:"created_at"`
	UpdatedAt string    `json:"updated_at"`
	Merged    bool      `json:"merged"`
	Draft     bool      `json:"draft"`
	Base      ghRef     `json:"base"`
	Head      ghRef     `json:"head"`
}

type ghComment struct {
	User      ghUser `json:"user"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
}
