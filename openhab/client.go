package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// errNotFound marks a 404 from openHAB, so a handler can say "no such item"
// rather than reporting a connection problem.
var errNotFound = errors.New("not found")

// openHABClient is the slice of openHAB's REST API the tools need. The
// handlers hold this rather than *restClient so the specs can drive them
// against a stub.
type openHABClient interface {
	Items(ctx context.Context) ([]Item, error)
	Item(ctx context.Context, name string, withMetadata bool) (Item, error)
	SendCommand(ctx context.Context, name, command string) error
	UpdateState(ctx context.Context, name, state string) error
	Things(ctx context.Context) ([]Thing, error)
	ThingStatus(ctx context.Context, uid string) (ThingStatus, error)
	Rules(ctx context.Context, tag string) ([]Rule, error)
	RunRule(ctx context.Context, uid string) error
}

// restClient talks to openHAB's REST API over HTTP.
type restClient struct {
	baseURL string
	cfg     Config
	http    *http.Client
}

func newRESTClient(cfg Config) (*restClient, error) {
	httpClient, err := cfg.httpClient()
	if err != nil {
		return nil, err
	}
	return &restClient{baseURL: strings.TrimRight(cfg.BaseURL, "/"), cfg: cfg, http: httpClient}, nil
}

// do performs one request, applying authentication and turning a non-2xx
// response into an error carrying the status and a snippet of the body.
func (c *restClient) do(ctx context.Context, method, path string, query url.Values, body string) ([]byte, error) {
	endpoint := c.baseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	if c.cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.Token)
	} else if c.cfg.Username != "" {
		req.SetBasicAuth(c.cfg.Username, c.cfg.Password)
	}
	if body != "" {
		// openHAB expects raw command and state values, not JSON.
		req.Header.Set("Content-Type", "text/plain")
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling openHAB: %w", err)
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("%s: %w", path, errNotFound)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet := string(payload)
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		return nil, fmt.Errorf("openHAB returned HTTP %d for %s: %s", resp.StatusCode, path, snippet)
	}

	return payload, nil
}

// getJSON performs a GET and decodes the response into target.
func (c *restClient) getJSON(ctx context.Context, path string, query url.Values, target any) error {
	payload, err := c.do(ctx, http.MethodGet, path, query, "")
	if err != nil {
		return err
	}
	if err := json.Unmarshal(payload, target); err != nil {
		return fmt.Errorf("decoding response from %s: %w", path, err)
	}
	return nil
}

// itemPath builds the path for one item, escaping the name.
func itemPath(name string) string {
	return "/rest/items/" + url.PathEscape(name)
}

func (c *restClient) Items(ctx context.Context) ([]Item, error) {
	var items []Item
	if err := c.getJSON(ctx, "/rest/items", nil, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (c *restClient) Item(ctx context.Context, name string, withMetadata bool) (Item, error) {
	var query url.Values
	if withMetadata {
		query = url.Values{"metadata": []string{".*"}}
	}
	var item Item
	if err := c.getJSON(ctx, itemPath(name), query, &item); err != nil {
		return Item{}, err
	}
	return item, nil
}

// SendCommand posts a command, which propagates through rules and bindings.
func (c *restClient) SendCommand(ctx context.Context, name, command string) error {
	_, err := c.do(ctx, http.MethodPost, itemPath(name), nil, command)
	return err
}

// UpdateState sets the state directly, without triggering rules.
func (c *restClient) UpdateState(ctx context.Context, name, state string) error {
	_, err := c.do(ctx, http.MethodPut, itemPath(name)+"/state", nil, state)
	return err
}

func (c *restClient) Things(ctx context.Context) ([]Thing, error) {
	var things []Thing
	if err := c.getJSON(ctx, "/rest/things", nil, &things); err != nil {
		return nil, err
	}
	return things, nil
}

func (c *restClient) ThingStatus(ctx context.Context, uid string) (ThingStatus, error) {
	var status ThingStatus
	path := "/rest/things/" + uid + "/status"
	if err := c.getJSON(ctx, path, nil, &status); err != nil {
		return ThingStatus{}, err
	}
	return status, nil
}

func (c *restClient) Rules(ctx context.Context, tag string) ([]Rule, error) {
	var query url.Values
	if tag != "" {
		query = url.Values{"tags": []string{tag}}
	}
	var rules []Rule
	if err := c.getJSON(ctx, "/rest/rules", query, &rules); err != nil {
		return nil, err
	}
	return rules, nil
}

func (c *restClient) RunRule(ctx context.Context, uid string) error {
	_, err := c.do(ctx, http.MethodPost, "/rest/rules/"+uid+"/runnow", nil, "")
	return err
}
