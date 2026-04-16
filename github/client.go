package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	defaultAccept = "application/vnd.github+json"
	diffAccept    = "application/vnd.github.v3.diff"
	apiVersion    = "2022-11-28"
)

type GitHubClient struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

func (c *GitHubClient) do(ctx context.Context, path, accept string, query url.Values) ([]byte, *http.Response, error) {
	u := strings.TrimRight(c.BaseURL, "/") + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("X-GitHub-Api-Version", apiVersion)
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet := string(body)
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		return nil, resp, fmt.Errorf("HTTP %d: %s", resp.StatusCode, snippet)
	}

	return body, resp, nil
}

func (c *GitHubClient) Get(ctx context.Context, path string, query url.Values) ([]byte, *http.Response, error) {
	return c.do(ctx, path, defaultAccept, query)
}

func (c *GitHubClient) GetRaw(ctx context.Context, path, accept string) ([]byte, error) {
	body, _, err := c.do(ctx, path, accept, nil)
	return body, err
}
