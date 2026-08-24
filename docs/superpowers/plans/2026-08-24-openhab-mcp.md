# openHAB MCP Server Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship `ghcr.io/mudler/mcps/openhab`, a Go MCP server exposing eight read-and-control tools over an openHAB instance's REST API.

**Architecture:** A `server` struct holds the parsed `Config` and an `openHABClient` interface; each tool is a method on it, registered through the same `toolSpec`/`spec[In,Out]` helper the samba server uses so that read-only mode drops tools from registration entirely. The REST client is a thin `net/http` wrapper — openHAB returns whole collections, so filtering, pagination and payload compaction happen in the handlers.

**Tech Stack:** Go 1.26, `github.com/modelcontextprotocol/go-sdk/mcp` v1.4.0, `net/http` (no openHAB client library), ginkgo v2 + gomega, `net/http/httptest` for a hermetic fake openHAB.

**Spec:** `docs/superpowers/specs/2026-08-24-openhab-mcp-design.md`

## Global Constraints

- Module is `github.com/mudler/mcps`, Go 1.26. Every server directory is `package main`.
- No new dependencies. Everything needed is already in `go.mod`.
- All work happens in `openhab/`. Do not modify other servers.
- Transport is stdio only. Logs go to **stderr** — stdout carries the JSON-RPC stream.
- **Tool errors go in the output payload, not as a Go error.** Handlers return `(nil, output{Success: false, Error: "..."}, nil)`. A non-nil Go error is for transport-level failures only. This matches `samba/tools_read.go`.
- Every field on every input and output struct carries a `jsonschema:"..."` description tag. The model reads these.
- Restricted tools are never registered, not registered-and-refusing.
- Run `gofmt -w` on every file before committing. Tests run with `go test ./openhab/...`.

---

### Task 1: Configuration

**Files:**
- Create: `openhab/config.go`
- Create: `openhab/config_test.go`
- Create: `openhab/openhab_suite_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `type Config struct` with fields `BaseURL string`, `Token string`, `Username string`, `Password string`, `Timeout time.Duration`, `CACertPath string`, `InsecureSkipVerify bool`, `ReadOnly bool`; `func loadConfig(getenv func(string) string) (Config, error)`; `func (c Config) httpClient() (*http.Client, error)`; `func isTruthy(value string) bool`.

- [ ] **Step 1: Create the ginkgo suite entrypoint**

Create `openhab/openhab_suite_test.go`:

```go
package main

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestOpenHAB(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "OpenHAB Suite")
}
```

- [ ] **Step 2: Write the failing config tests**

Create `openhab/config_test.go`:

```go
package main

import (
	"net/http"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// env returns a getenv function backed by a map, so the specs never touch the
// process environment.
func env(pairs map[string]string) func(string) string {
	return func(key string) string { return pairs[key] }
}

var _ = Describe("loadConfig", func() {
	It("should accept a URL and a token", func() {
		cfg, err := loadConfig(env(map[string]string{
			"OPENHAB_URL":       "https://10.9.0.26:8443",
			"OPENHAB_API_TOKEN": "oh.mcp.secret",
		}))
		Expect(err).ToNot(HaveOccurred())
		Expect(cfg.BaseURL).To(Equal("https://10.9.0.26:8443"))
		Expect(cfg.Token).To(Equal("oh.mcp.secret"))
		Expect(cfg.Timeout).To(Equal(30 * time.Second))
		Expect(cfg.ReadOnly).To(BeFalse())
	})

	It("should strip a trailing slash from the URL", func() {
		cfg, err := loadConfig(env(map[string]string{
			"OPENHAB_URL":       "http://openhab:8080/",
			"OPENHAB_API_TOKEN": "t",
		}))
		Expect(err).ToNot(HaveOccurred())
		Expect(cfg.BaseURL).To(Equal("http://openhab:8080"))
	})

	It("should accept basic auth instead of a token", func() {
		cfg, err := loadConfig(env(map[string]string{
			"OPENHAB_URL":      "http://openhab:8080",
			"OPENHAB_USERNAME": "admin",
			"OPENHAB_PASSWORD": "hunter2",
		}))
		Expect(err).ToNot(HaveOccurred())
		Expect(cfg.Username).To(Equal("admin"))
		Expect(cfg.Password).To(Equal("hunter2"))
	})

	It("should require OPENHAB_URL", func() {
		_, err := loadConfig(env(map[string]string{"OPENHAB_API_TOKEN": "t"}))
		Expect(err).To(MatchError(ContainSubstring("OPENHAB_URL is required")))
	})

	It("should reject a URL without a scheme", func() {
		_, err := loadConfig(env(map[string]string{
			"OPENHAB_URL":       "10.9.0.26:8443",
			"OPENHAB_API_TOKEN": "t",
		}))
		Expect(err).To(MatchError(ContainSubstring("http:// or https://")))
	})

	It("should require some form of credentials", func() {
		_, err := loadConfig(env(map[string]string{"OPENHAB_URL": "http://openhab:8080"}))
		Expect(err).To(MatchError(ContainSubstring("OPENHAB_API_TOKEN")))
	})

	It("should reject a username without a password", func() {
		_, err := loadConfig(env(map[string]string{
			"OPENHAB_URL":      "http://openhab:8080",
			"OPENHAB_USERNAME": "admin",
		}))
		Expect(err).To(MatchError(ContainSubstring("OPENHAB_PASSWORD")))
	})

	It("should parse the timeout", func() {
		cfg, err := loadConfig(env(map[string]string{
			"OPENHAB_URL":       "http://openhab:8080",
			"OPENHAB_API_TOKEN": "t",
			"OPENHAB_TIMEOUT":   "5s",
		}))
		Expect(err).ToNot(HaveOccurred())
		Expect(cfg.Timeout).To(Equal(5 * time.Second))
	})

	It("should reject an unparseable timeout", func() {
		_, err := loadConfig(env(map[string]string{
			"OPENHAB_URL":       "http://openhab:8080",
			"OPENHAB_API_TOKEN": "t",
			"OPENHAB_TIMEOUT":   "soon",
		}))
		Expect(err).To(MatchError(ContainSubstring("OPENHAB_TIMEOUT")))
	})

	It("should read the truthy flags", func() {
		cfg, err := loadConfig(env(map[string]string{
			"OPENHAB_URL":                  "http://openhab:8080",
			"OPENHAB_API_TOKEN":            "t",
			"OPENHAB_READ_ONLY":            "true",
			"OPENHAB_INSECURE_SKIP_VERIFY": "1",
		}))
		Expect(err).ToNot(HaveOccurred())
		Expect(cfg.ReadOnly).To(BeTrue())
		Expect(cfg.InsecureSkipVerify).To(BeTrue())
	})
})

var _ = Describe("Config.httpClient", func() {
	It("should apply the timeout", func() {
		client, err := Config{Timeout: 7 * time.Second}.httpClient()
		Expect(err).ToNot(HaveOccurred())
		Expect(client.Timeout).To(Equal(7 * time.Second))
	})

	It("should skip verification when asked", func() {
		client, err := Config{Timeout: time.Second, InsecureSkipVerify: true}.httpClient()
		Expect(err).ToNot(HaveOccurred())
		transport, ok := client.Transport.(*http.Transport)
		Expect(ok).To(BeTrue())
		Expect(transport.TLSClientConfig.InsecureSkipVerify).To(BeTrue())
	})

	It("should load a CA bundle from disk", func() {
		path := filepath.Join(GinkgoT().TempDir(), "ca.pem")
		Expect(os.WriteFile(path, []byte(testCAPEM), 0o644)).To(Succeed())

		client, err := Config{Timeout: time.Second, CACertPath: path}.httpClient()
		Expect(err).ToNot(HaveOccurred())
		transport, ok := client.Transport.(*http.Transport)
		Expect(ok).To(BeTrue())
		Expect(transport.TLSClientConfig.RootCAs).ToNot(BeNil())
	})

	It("should fail on a CA bundle that is not readable", func() {
		_, err := Config{CACertPath: "/nonexistent/ca.pem"}.httpClient()
		Expect(err).To(MatchError(ContainSubstring("OPENHAB_CA_CERT")))
	})

	It("should fail on a CA bundle that holds no certificate", func() {
		path := filepath.Join(GinkgoT().TempDir(), "junk.pem")
		Expect(os.WriteFile(path, []byte("not a certificate"), 0o644)).To(Succeed())

		_, err := Config{CACertPath: path}.httpClient()
		Expect(err).To(MatchError(ContainSubstring("no certificates")))
	})
})
```

Append this constant to the same file — a throwaway self-signed CA, valid only as parseable PEM:

```go
// testCAPEM is a self-signed certificate used only to prove the PEM parses.
const testCAPEM = `-----BEGIN CERTIFICATE-----
MIIBmDCCAT+gAwIBAgIUb+KIIk6pqmT45MhWzc0GK37ncK8wCgYIKoZIzj0EAwIw
FDESMBAGA1UECgwJbWNwcyB0ZXN0MB4XDTI2MDgyNDE5NTEyN1oXDTQ2MDgxOTE5
NTEyN1owFDESMBAGA1UECgwJbWNwcyB0ZXN0MFkwEwYHKoZIzj0CAQYIKoZIzj0D
AQcDQgAE0wjfTM1faXZJaH5aoCrwCGVvGmBzQSjTOeseK7GOy2v9mKgpxAzIbgl5
1y7y+4VW2HD+Xf/sFKoCVA5KYM3ynaNvMG0wHQYDVR0OBBYEFH4a/F5y/ToN9NZE
CF8VCIup1jjoMB8GA1UdIwQYMBaAFH4a/F5y/ToN9NZECF8VCIup1jjoMA8GA1Ud
EwEB/wQFMAMBAf8wGgYDVR0RBBMwEYIJbG9jYWxob3N0hwR/AAABMAoGCCqGSM49
BAMCA0cAMEQCIEet0Iw0azx6jsA29h5lZ8T4Vr8pPVW35wTqRo0mwNCoAiAV7b48
5yy+BlK4G3/ioV32ZGLtaXEn3AJHd4NYHGC7sw==
-----END CERTIFICATE-----
`
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./openhab/...`
Expected: FAIL — `undefined: loadConfig`, `undefined: Config`.

- [ ] **Step 4: Write the implementation**

Create `openhab/config.go`:

```go
package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

const defaultTimeout = 30 * time.Second

// Config holds everything the server needs to reach an openHAB instance and
// decide which tools it is allowed to expose.
type Config struct {
	// BaseURL is the openHAB root, scheme included, without a trailing slash.
	BaseURL string

	// Token is an openHAB API token, preferred over basic auth when both are
	// supplied.
	Token    string
	Username string
	Password string

	// Timeout bounds every request.
	Timeout time.Duration

	// CACertPath points at a PEM bundle for an instance behind a private CA.
	CACertPath string
	// InsecureSkipVerify drops certificate verification. openHAB's stock
	// certificate carries no subjectAltName, so no CA bundle can validate it.
	InsecureSkipVerify bool

	// ReadOnly drops the command, state and rule-run tools.
	ReadOnly bool
}

// loadConfig reads the configuration from getenv, which is os.Getenv in
// production and a map lookup in the specs.
func loadConfig(getenv func(string) string) (Config, error) {
	cfg := Config{
		Token:    strings.TrimSpace(getenv("OPENHAB_API_TOKEN")),
		Username: strings.TrimSpace(getenv("OPENHAB_USERNAME")),
		Password: getenv("OPENHAB_PASSWORD"),
		Timeout:  defaultTimeout,

		CACertPath:         strings.TrimSpace(getenv("OPENHAB_CA_CERT")),
		InsecureSkipVerify: isTruthy(getenv("OPENHAB_INSECURE_SKIP_VERIFY")),
		ReadOnly:           isTruthy(getenv("OPENHAB_READ_ONLY")),
	}

	base := strings.TrimRight(strings.TrimSpace(getenv("OPENHAB_URL")), "/")
	if base == "" {
		return Config{}, fmt.Errorf("OPENHAB_URL is required (the openHAB base URL, for example http://openhab:8080)")
	}
	if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
		return Config{}, fmt.Errorf("OPENHAB_URL must start with http:// or https://, got %q", base)
	}
	cfg.BaseURL = base

	if cfg.Token == "" {
		if cfg.Username == "" {
			return Config{}, fmt.Errorf("credentials are required: set OPENHAB_API_TOKEN, or OPENHAB_USERNAME and OPENHAB_PASSWORD")
		}
		if cfg.Password == "" {
			return Config{}, fmt.Errorf("OPENHAB_PASSWORD is required when OPENHAB_USERNAME is set")
		}
	}

	if raw := strings.TrimSpace(getenv("OPENHAB_TIMEOUT")); raw != "" {
		timeout, err := time.ParseDuration(raw)
		if err != nil {
			return Config{}, fmt.Errorf("OPENHAB_TIMEOUT is not a duration: %w", err)
		}
		if timeout <= 0 {
			return Config{}, fmt.Errorf("OPENHAB_TIMEOUT must be positive, got %q", raw)
		}
		cfg.Timeout = timeout
	}

	return cfg, nil
}

// httpClient builds the client every request goes through, applying the TLS
// choices the configuration made.
func (c Config) httpClient() (*http.Client, error) {
	tlsConfig := &tls.Config{InsecureSkipVerify: c.InsecureSkipVerify}

	if c.CACertPath != "" {
		pem, err := os.ReadFile(c.CACertPath)
		if err != nil {
			return nil, fmt.Errorf("OPENHAB_CA_CERT could not be read: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("OPENHAB_CA_CERT holds no certificates: %s", c.CACertPath)
		}
		tlsConfig.RootCAs = pool
	}

	return &http.Client{
		Timeout:   c.Timeout,
		Transport: &http.Transport{TLSClientConfig: tlsConfig},
	}, nil
}

// isTruthy reads the permissive set of yes-values these servers accept.
func isTruthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "t", "true", "y", "yes", "on":
		return true
	default:
		return false
	}
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./openhab/...`
Expected: PASS, all config specs green.

- [ ] **Step 6: Commit**

```bash
gofmt -w openhab/
git add openhab/config.go openhab/config_test.go openhab/openhab_suite_test.go
git commit -m "feat(openhab): configuration and TLS options"
```

---

### Task 2: REST client

**Files:**
- Create: `openhab/types.go`
- Create: `openhab/client.go`
- Create: `openhab/client_test.go`
- Create: `openhab/fake_test.go`

**Interfaces:**
- Consumes: `Config`, `Config.httpClient()` from Task 1.
- Produces:
  - Wire types `Item`, `Thing`, `ThingStatus`, `Rule`, `RuleStatus` (in `types.go`).
  - `type openHABClient interface` with methods `Items(ctx) ([]Item, error)`, `Item(ctx, name string, withMetadata bool) (Item, error)`, `SendCommand(ctx, name, command string) error`, `UpdateState(ctx, name, state string) error`, `Things(ctx) ([]Thing, error)`, `ThingStatus(ctx, uid string) (ThingStatus, error)`, `Rules(ctx, tag string) ([]Rule, error)`, `RunRule(ctx, uid string) error`.
  - `func newRESTClient(cfg Config) (*restClient, error)`.
  - `var errNotFound = errors.New("not found")` — wrapped by `Item` and `ThingStatus` on a 404 so handlers can tell a missing name from a dead connection.
  - Test helper `func newFakeOpenHAB(handler http.HandlerFunc) (*httptest.Server, *restClient)` (in `fake_test.go`).

- [ ] **Step 1: Write the wire types**

Create `openhab/types.go`. These mirror openHAB's REST payloads; only the fields the tools use are declared, so the rest is dropped at decode time.

```go
package main

// Item is one openHAB item as the REST API returns it.
type Item struct {
	Name       string            `json:"name"`
	Type       string            `json:"type"`
	State      string            `json:"state"`
	Label      string            `json:"label"`
	Category   string            `json:"category"`
	Tags       []string          `json:"tags"`
	GroupNames []string          `json:"groupNames"`
	Metadata   map[string]any    `json:"metadata,omitempty"`
}

// Thing is one openHAB thing. The channels array is deliberately absent: it
// dominates the payload and no tool reports it.
type Thing struct {
	UID          string         `json:"UID"`
	ThingTypeUID string         `json:"thingTypeUID"`
	Label        string         `json:"label"`
	BridgeUID    string         `json:"bridgeUID"`
	StatusInfo   ThingStatus    `json:"statusInfo"`
	Properties   map[string]any `json:"properties"`
}

// ThingStatus is the status block openHAB reports for a thing.
type ThingStatus struct {
	Status       string `json:"status"`
	StatusDetail string `json:"statusDetail"`
	Description  string `json:"description"`
}

// Rule is one openHAB rule. Triggers, conditions and actions are omitted: the
// read-and-control surface reports rules and runs them, it does not edit them.
type Rule struct {
	UID         string     `json:"uid"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Tags        []string   `json:"tags"`
	Status      RuleStatus `json:"status"`
	Editable    bool       `json:"editable"`
}

// RuleStatus is the status block openHAB reports for a rule.
type RuleStatus struct {
	Status       string `json:"status"`
	StatusDetail string `json:"statusDetail"`
}
```

- [ ] **Step 2: Write the fake openHAB helper**

Create `openhab/fake_test.go`:

```go
package main

import (
	"net/http"
	"net/http/httptest"
	"time"

	. "github.com/onsi/gomega"
)

// newFakeOpenHAB starts an HTTP server running handler and returns a client
// pointed at it. The caller closes the server.
func newFakeOpenHAB(handler http.HandlerFunc) (*httptest.Server, *restClient) {
	server := httptest.NewServer(handler)
	client, err := newRESTClient(Config{
		BaseURL: server.URL,
		Token:   "test-token",
		Timeout: 5 * time.Second,
	})
	Expect(err).ToNot(HaveOccurred())
	return server, client
}

// itemsJSON is a two-item response in openHAB's wire format.
const itemsJSON = `[
  {"name":"AlarmTrigger","type":"Switch","state":"OFF","label":"Sirena","tags":["Switchable"],"groupNames":[]},
  {"name":"Ozone","type":"Number","state":"12.4","label":"Ozono","tags":[],"groupNames":["Sensors"]}
]`

// thingsJSON carries a channels array, which the client must drop.
const thingsJSON = `[
  {"UID":"astro:sun:home","thingTypeUID":"astro:sun","label":"Dati Astro Sole",
   "statusInfo":{"status":"ONLINE","statusDetail":"NONE","description":""},
   "channels":[{"uid":"astro:sun:home:rise#start","id":"rise#start"}],
   "properties":{}}
]`

// rulesJSON is a one-rule response.
const rulesJSON = `[
  {"uid":"nightmode","name":"Night mode","description":"Shutters down","tags":["night"],
   "status":{"status":"IDLE","statusDetail":"NONE"},"editable":true}
]`
```

- [ ] **Step 3: Write the failing client tests**

Create `openhab/client_test.go`:

```go
package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("restClient", func() {
	var ctx context.Context

	BeforeEach(func() { ctx = context.Background() })

	It("should send the bearer token", func() {
		var seen string
		server, client := newFakeOpenHAB(func(w http.ResponseWriter, r *http.Request) {
			seen = r.Header.Get("Authorization")
			w.Write([]byte(itemsJSON))
		})
		defer server.Close()

		_, err := client.Items(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(seen).To(Equal("Bearer test-token"))
	})

	It("should fall back to basic auth when no token is set", func() {
		var user, pass string
		var ok bool
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, pass, ok = r.BasicAuth()
			w.Write([]byte(itemsJSON))
		}))
		defer server.Close()

		client, err := newRESTClient(Config{
			BaseURL: server.URL, Username: "admin", Password: "hunter2", Timeout: time.Second,
		})
		Expect(err).ToNot(HaveOccurred())

		_, err = client.Items(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(ok).To(BeTrue())
		Expect(user).To(Equal("admin"))
		Expect(pass).To(Equal("hunter2"))
	})

	It("should decode items", func() {
		server, client := newFakeOpenHAB(func(w http.ResponseWriter, r *http.Request) {
			Expect(r.URL.Path).To(Equal("/rest/items"))
			w.Write([]byte(itemsJSON))
		})
		defer server.Close()

		items, err := client.Items(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(items).To(HaveLen(2))
		Expect(items[0].Name).To(Equal("AlarmTrigger"))
		Expect(items[0].State).To(Equal("OFF"))
		Expect(items[1].GroupNames).To(ConsistOf("Sensors"))
	})

	It("should request metadata only when asked", func() {
		var query string
		server, client := newFakeOpenHAB(func(w http.ResponseWriter, r *http.Request) {
			query = r.URL.RawQuery
			w.Write([]byte(`{"name":"AlarmTrigger","type":"Switch","state":"OFF"}`))
		})
		defer server.Close()

		_, err := client.Item(ctx, "AlarmTrigger", false)
		Expect(err).ToNot(HaveOccurred())
		Expect(query).To(BeEmpty())

		_, err = client.Item(ctx, "AlarmTrigger", true)
		Expect(err).ToNot(HaveOccurred())
		Expect(query).To(Equal("metadata=.%2A"))
	})

	It("should escape item names in the path", func() {
		var path string
		server, client := newFakeOpenHAB(func(w http.ResponseWriter, r *http.Request) {
			path = r.URL.EscapedPath()
			w.Write([]byte(`{"name":"a b","type":"Switch","state":"OFF"}`))
		})
		defer server.Close()

		_, err := client.Item(ctx, "a b", false)
		Expect(err).ToNot(HaveOccurred())
		Expect(path).To(Equal("/rest/items/a%20b"))
	})

	It("should report a 404 as errNotFound", func() {
		server, client := newFakeOpenHAB(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})
		defer server.Close()

		_, err := client.Item(ctx, "Nope", false)
		Expect(errors.Is(err, errNotFound)).To(BeTrue())
	})

	It("should carry the status and body on other failures", func() {
		server, client := newFakeOpenHAB(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("boom"))
		})
		defer server.Close()

		_, err := client.Items(ctx)
		Expect(err).To(MatchError(ContainSubstring("500")))
		Expect(err).To(MatchError(ContainSubstring("boom")))
	})

	It("should post a command as text/plain", func() {
		var method, contentType, body string
		server, client := newFakeOpenHAB(func(w http.ResponseWriter, r *http.Request) {
			method = r.Method
			contentType = r.Header.Get("Content-Type")
			raw, _ := io.ReadAll(r.Body)
			body = string(raw)
			w.WriteHeader(http.StatusOK)
		})
		defer server.Close()

		Expect(client.SendCommand(ctx, "AlarmTrigger", "ON")).To(Succeed())
		Expect(method).To(Equal(http.MethodPost))
		Expect(contentType).To(Equal("text/plain"))
		Expect(body).To(Equal("ON"))
	})

	It("should put a state update on the state sub-resource", func() {
		var method, path, body string
		server, client := newFakeOpenHAB(func(w http.ResponseWriter, r *http.Request) {
			method = r.Method
			path = r.URL.Path
			raw, _ := io.ReadAll(r.Body)
			body = string(raw)
			w.WriteHeader(http.StatusAccepted)
		})
		defer server.Close()

		Expect(client.UpdateState(ctx, "Ozone", "13.1")).To(Succeed())
		Expect(method).To(Equal(http.MethodPut))
		Expect(path).To(Equal("/rest/items/Ozone/state"))
		Expect(body).To(Equal("13.1"))
	})

	It("should decode things and ignore their channels", func() {
		server, client := newFakeOpenHAB(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(thingsJSON))
		})
		defer server.Close()

		things, err := client.Things(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(things).To(HaveLen(1))
		Expect(things[0].UID).To(Equal("astro:sun:home"))
		Expect(things[0].StatusInfo.Status).To(Equal("ONLINE"))
	})

	It("should fetch a thing status", func() {
		server, client := newFakeOpenHAB(func(w http.ResponseWriter, r *http.Request) {
			Expect(r.URL.Path).To(Equal("/rest/things/astro:sun:home/status"))
			w.Write([]byte(`{"status":"ONLINE","statusDetail":"NONE","description":""}`))
		})
		defer server.Close()

		status, err := client.ThingStatus(ctx, "astro:sun:home")
		Expect(err).ToNot(HaveOccurred())
		Expect(status.Status).To(Equal("ONLINE"))
	})

	It("should pass a rule tag filter to openHAB", func() {
		var query string
		server, client := newFakeOpenHAB(func(w http.ResponseWriter, r *http.Request) {
			query = r.URL.RawQuery
			w.Write([]byte(rulesJSON))
		})
		defer server.Close()

		rules, err := client.Rules(ctx, "night")
		Expect(err).ToNot(HaveOccurred())
		Expect(query).To(Equal("tags=night"))
		Expect(rules).To(HaveLen(1))
		Expect(rules[0].UID).To(Equal("nightmode"))
	})

	It("should run a rule", func() {
		var method, path string
		server, client := newFakeOpenHAB(func(w http.ResponseWriter, r *http.Request) {
			method = r.Method
			path = r.URL.Path
			w.WriteHeader(http.StatusOK)
		})
		defer server.Close()

		Expect(client.RunRule(ctx, "nightmode")).To(Succeed())
		Expect(method).To(Equal(http.MethodPost))
		Expect(path).To(Equal("/rest/rules/nightmode/runnow"))
	})

	It("should refuse an untrusted certificate by default", func() {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(itemsJSON))
		}))
		defer server.Close()

		client, err := newRESTClient(Config{BaseURL: server.URL, Token: "t", Timeout: time.Second})
		Expect(err).ToNot(HaveOccurred())

		_, err = client.Items(ctx)
		Expect(err).To(HaveOccurred())
	})

	It("should accept an untrusted certificate when verification is skipped", func() {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(itemsJSON))
		}))
		defer server.Close()

		client, err := newRESTClient(Config{
			BaseURL: server.URL, Token: "t", Timeout: time.Second, InsecureSkipVerify: true,
		})
		Expect(err).ToNot(HaveOccurred())

		items, err := client.Items(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(items).To(HaveLen(2))
	})

	It("should accept a certificate signed by the configured CA", func() {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(itemsJSON))
		}))
		defer server.Close()

		path := filepath.Join(GinkgoT().TempDir(), "ca.pem")
		Expect(os.WriteFile(path, encodeCertPEM(server.Certificate().Raw), 0o644)).To(Succeed())

		client, err := newRESTClient(Config{
			BaseURL: server.URL, Token: "t", Timeout: time.Second, CACertPath: path,
		})
		Expect(err).ToNot(HaveOccurred())

		items, err := client.Items(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(items).To(HaveLen(2))
	})
})
```

Add this helper to `openhab/fake_test.go` — `httptest`'s certificate is valid for `127.0.0.1`, so trusting it is enough to prove the CA path works:

```go
// encodeCertPEM wraps a DER certificate in the PEM armour a CA bundle needs.
func encodeCertPEM(der []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}
```

with `"encoding/pem"` added to that file's imports.

- [ ] **Step 4: Run the tests to verify they fail**

Run: `go test ./openhab/...`
Expected: FAIL — `undefined: newRESTClient`, `undefined: errNotFound`.

- [ ] **Step 5: Write the implementation**

Create `openhab/client.go`:

```go
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
```

Note: thing and rule UIDs contain colons, which `url.PathEscape` would leave alone but which are legal in a path segment — they are interpolated directly, matching how openHAB's own documentation writes these URLs.

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test ./openhab/...`
Expected: PASS, including the three TLS specs.

- [ ] **Step 7: Commit**

```bash
gofmt -w openhab/
git add openhab/client.go openhab/types.go openhab/client_test.go openhab/fake_test.go
git commit -m "feat(openhab): REST client with token, basic auth and TLS options"
```

---

### Task 3: Item read tools

**Files:**
- Create: `openhab/server.go`
- Create: `openhab/tools_items.go`
- Create: `openhab/tools_items_test.go`
- Create: `openhab/stub_test.go`

**Interfaces:**
- Consumes: `Config`, `openHABClient`, `Item` from Tasks 1-2.
- Produces:
  - `type server struct { cfg Config; client openHABClient }` and `func newServer(cfg Config, client openHABClient) *server` (in `server.go`).
  - `type ItemSummary struct` — the compact row type.
  - `func (s *server) listItems(ctx, *mcp.CallToolRequest, listItemsInput) (*mcp.CallToolResult, listItemsOutput, error)`.
  - `func (s *server) getItem(ctx, *mcp.CallToolRequest, getItemInput) (*mcp.CallToolResult, getItemOutput, error)`.
  - `type stubClient struct` implementing `openHABClient` (in `stub_test.go`).

- [ ] **Step 1: Write the server struct**

Create `openhab/server.go`:

```go
package main

// server carries the configuration and the openHAB client shared by every tool
// handler.
type server struct {
	cfg    Config
	client openHABClient
}

func newServer(cfg Config, client openHABClient) *server {
	return &server{cfg: cfg, client: client}
}
```

- [ ] **Step 2: Write the client stub**

Create `openhab/stub_test.go`. Every method returns what its field holds, so a spec sets only what it needs:

```go
package main

import "context"

// stubClient is an openHABClient whose answers the specs set directly.
type stubClient struct {
	items       []Item
	item        Item
	things      []Thing
	thingStatus ThingStatus
	rules       []Rule

	err error

	// Recorded calls.
	commanded  [][2]string
	stated     [][2]string
	ranRules   []string
	ruleTag    string
	wantedMeta bool
}

func (s *stubClient) Items(context.Context) ([]Item, error) {
	return s.items, s.err
}

func (s *stubClient) Item(_ context.Context, _ string, withMetadata bool) (Item, error) {
	s.wantedMeta = withMetadata
	return s.item, s.err
}

func (s *stubClient) SendCommand(_ context.Context, name, command string) error {
	s.commanded = append(s.commanded, [2]string{name, command})
	return s.err
}

func (s *stubClient) UpdateState(_ context.Context, name, state string) error {
	s.stated = append(s.stated, [2]string{name, state})
	return s.err
}

func (s *stubClient) Things(context.Context) ([]Thing, error) {
	return s.things, s.err
}

func (s *stubClient) ThingStatus(context.Context, string) (ThingStatus, error) {
	return s.thingStatus, s.err
}

func (s *stubClient) Rules(_ context.Context, tag string) ([]Rule, error) {
	s.ruleTag = tag
	return s.rules, s.err
}

func (s *stubClient) RunRule(_ context.Context, uid string) error {
	s.ranRules = append(s.ranRules, uid)
	return s.err
}
```

- [ ] **Step 3: Write the failing tests**

Create `openhab/tools_items_test.go`:

```go
package main

import (
	"context"
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// sampleItems is a small house: two switches, a number, a group.
func sampleItems() []Item {
	return []Item{
		{Name: "AlarmTrigger", Type: "Switch", State: "OFF", Label: "Sirena", Tags: []string{"Switchable"}},
		{Name: "AutomaticGate", Type: "Switch", State: "OFF", Label: "Apertura cancello"},
		{Name: "Ozone", Type: "Number", State: "12.4", Label: "Ozono", GroupNames: []string{"Sensors"}},
		{Name: "gMobiles", Type: "Group", State: "NULL", Label: "Telefoni"},
	}
}

var _ = Describe("item read tools", func() {
	var (
		stub *stubClient
		srv  *server
		ctx  context.Context
	)

	BeforeEach(func() {
		stub = &stubClient{items: sampleItems()}
		srv = newServer(Config{}, stub)
		ctx = context.Background()
	})

	Describe("list_items", func() {
		It("should return every item as a compact row", func() {
			_, out, err := srv.listItems(ctx, nil, listItemsInput{})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeTrue())
			Expect(out.Items).To(HaveLen(4))
			Expect(out.Total).To(Equal(4))
			Expect(out.Items[0].Name).To(Equal("AlarmTrigger"))
			Expect(out.Items[0].State).To(Equal("OFF"))
			Expect(out.Items[0].Label).To(Equal("Sirena"))
		})

		It("should filter by type", func() {
			_, out, err := srv.listItems(ctx, nil, listItemsInput{FilterType: "switch"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Items).To(HaveLen(2))
			Expect(out.Total).To(Equal(2))
		})

		It("should filter by name as a case-insensitive substring", func() {
			_, out, err := srv.listItems(ctx, nil, listItemsInput{FilterName: "auto"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Items).To(HaveLen(1))
			Expect(out.Items[0].Name).To(Equal("AutomaticGate"))
		})

		It("should filter by label", func() {
			_, out, err := srv.listItems(ctx, nil, listItemsInput{FilterLabel: "sirena"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Items).To(HaveLen(1))
			Expect(out.Items[0].Name).To(Equal("AlarmTrigger"))
		})

		It("should filter by tag", func() {
			_, out, err := srv.listItems(ctx, nil, listItemsInput{FilterTag: "Switchable"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Items).To(HaveLen(1))
			Expect(out.Items[0].Name).To(Equal("AlarmTrigger"))
		})

		It("should combine filters", func() {
			_, out, err := srv.listItems(ctx, nil, listItemsInput{FilterType: "Switch", FilterName: "gate"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Items).To(HaveLen(1))
			Expect(out.Items[0].Name).To(Equal("AutomaticGate"))
		})

		It("should paginate and report the unpaginated total", func() {
			_, out, err := srv.listItems(ctx, nil, listItemsInput{Page: 2, PageSize: 3})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Items).To(HaveLen(1))
			Expect(out.Total).To(Equal(4))
			Expect(out.Page).To(Equal(2))
		})

		It("should return an empty page past the end rather than an error", func() {
			_, out, err := srv.listItems(ctx, nil, listItemsInput{Page: 99, PageSize: 50})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeTrue())
			Expect(out.Items).To(BeEmpty())
			Expect(out.Total).To(Equal(4))
		})

		It("should cap the page size", func() {
			stub.items = make([]Item, 500)
			for i := range stub.items {
				stub.items[i] = Item{Name: fmt.Sprintf("Item%d", i), Type: "Switch", State: "OFF"}
			}
			_, out, err := srv.listItems(ctx, nil, listItemsInput{PageSize: 10000})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Items).To(HaveLen(maxPageSize))
		})

		It("should reject a negative page", func() {
			_, out, err := srv.listItems(ctx, nil, listItemsInput{Page: -1})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeFalse())
			Expect(out.Error).To(ContainSubstring("page"))
		})

		It("should report a client failure in the payload", func() {
			stub.err = errors.New("connection refused")
			_, out, err := srv.listItems(ctx, nil, listItemsInput{})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeFalse())
			Expect(out.Error).To(ContainSubstring("connection refused"))
		})
	})

	Describe("get_item", func() {
		BeforeEach(func() {
			stub.item = Item{
				Name: "AlarmTrigger", Type: "Switch", State: "OFF", Label: "Sirena",
				Tags: []string{"Switchable"},
			}
		})

		It("should return the item", func() {
			_, out, err := srv.getItem(ctx, nil, getItemInput{Name: "AlarmTrigger"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeTrue())
			Expect(out.Item.Name).To(Equal("AlarmTrigger"))
			Expect(out.Item.Tags).To(ConsistOf("Switchable"))
		})

		It("should require a name", func() {
			_, out, err := srv.getItem(ctx, nil, getItemInput{})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeFalse())
			Expect(out.Error).To(ContainSubstring("name is required"))
		})

		It("should ask for metadata only when requested", func() {
			_, _, err := srv.getItem(ctx, nil, getItemInput{Name: "AlarmTrigger"})
			Expect(err).ToNot(HaveOccurred())
			Expect(stub.wantedMeta).To(BeFalse())

			_, _, err = srv.getItem(ctx, nil, getItemInput{Name: "AlarmTrigger", WithMetadata: true})
			Expect(err).ToNot(HaveOccurred())
			Expect(stub.wantedMeta).To(BeTrue())
		})

		It("should say plainly when the item does not exist", func() {
			stub.err = fmt.Errorf("/rest/items/Nope: %w", errNotFound)
			_, out, err := srv.getItem(ctx, nil, getItemInput{Name: "Nope"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeFalse())
			Expect(out.Error).To(ContainSubstring(`no item named "Nope"`))
		})
	})
})
```

- [ ] **Step 4: Run the tests to verify they fail**

Run: `go test ./openhab/...`
Expected: FAIL — `undefined: listItemsInput`, `undefined: maxPageSize`.

- [ ] **Step 5: Write the implementation**

Create `openhab/tools_items.go`:

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	defaultPageSize = 50
	maxPageSize     = 200
)

// ItemSummary is the compact form returned by listings. get_item returns the
// full Item when more is needed.
type ItemSummary struct {
	Name  string `json:"name" jsonschema:"the item name, used by every other tool"`
	Type  string `json:"type" jsonschema:"item type, for example Switch, Number, String or Group"`
	State string `json:"state" jsonschema:"current state as openHAB reports it"`
	Label string `json:"label" jsonschema:"human-readable label, often in the user's own language"`
}

type listItemsInput struct {
	FilterName  string `json:"filter_name,omitempty" jsonschema:"only items whose name contains this text, case-insensitive"`
	FilterLabel string `json:"filter_label,omitempty" jsonschema:"only items whose label contains this text, case-insensitive"`
	FilterType  string `json:"filter_type,omitempty" jsonschema:"only items of this type, for example Switch or Number"`
	FilterTag   string `json:"filter_tag,omitempty" jsonschema:"only items carrying this tag, for example Switchable"`
	Page        int    `json:"page,omitempty" jsonschema:"1-based page number, defaults to 1"`
	PageSize    int    `json:"page_size,omitempty" jsonschema:"items per page, defaults to 50 and is capped at 200"`
}

type listItemsOutput struct {
	Items    []ItemSummary `json:"items" jsonschema:"the items on this page"`
	Total    int           `json:"total" jsonschema:"how many items matched the filters, before pagination"`
	Page     int           `json:"page" jsonschema:"the page that was returned"`
	PageSize int           `json:"page_size" jsonschema:"the page size that was applied"`
	Success  bool          `json:"success" jsonschema:"whether the operation succeeded"`
	Error    string        `json:"error,omitempty" jsonschema:"error message if it failed"`
}

// listItems returns a compact view of the items, filtered and paginated here
// because openHAB's REST API returns the whole collection.
func (s *server) listItems(ctx context.Context, _ *mcp.CallToolRequest, input listItemsInput) (
	*mcp.CallToolResult,
	listItemsOutput,
	error,
) {
	page, pageSize, err := paginationOf(input.Page, input.PageSize)
	if err != nil {
		return nil, listItemsOutput{Error: err.Error()}, nil
	}

	items, err := s.client.Items(ctx)
	if err != nil {
		return nil, listItemsOutput{Error: err.Error()}, nil
	}

	matched := make([]ItemSummary, 0, len(items))
	for _, item := range items {
		if !contains(item.Name, input.FilterName) ||
			!contains(item.Label, input.FilterLabel) ||
			!equalFold(item.Type, input.FilterType) ||
			!hasTag(item.Tags, input.FilterTag) {
			continue
		}
		matched = append(matched, ItemSummary{
			Name:  item.Name,
			Type:  item.Type,
			State: item.State,
			Label: item.Label,
		})
	}

	return nil, listItemsOutput{
		Items:    pageOf(matched, page, pageSize),
		Total:    len(matched),
		Page:     page,
		PageSize: pageSize,
		Success:  true,
	}, nil
}

type getItemInput struct {
	Name         string `json:"name" jsonschema:"the item name, exactly as list_items reports it"`
	WithMetadata bool   `json:"with_metadata,omitempty" jsonschema:"include the item's metadata namespaces"`
}

type getItemOutput struct {
	Item    Item   `json:"item" jsonschema:"the full item, including tags and group membership"`
	Success bool   `json:"success" jsonschema:"whether the operation succeeded"`
	Error   string `json:"error,omitempty" jsonschema:"error message if it failed"`
}

// getItem returns one item in full.
func (s *server) getItem(ctx context.Context, _ *mcp.CallToolRequest, input getItemInput) (
	*mcp.CallToolResult,
	getItemOutput,
	error,
) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, getItemOutput{Error: "name is required (the item name, as reported by list_items)"}, nil
	}

	item, err := s.client.Item(ctx, name, input.WithMetadata)
	if err != nil {
		return nil, getItemOutput{Error: describeItemError(name, err)}, nil
	}

	return nil, getItemOutput{Item: item, Success: true}, nil
}

// describeItemError turns a 404 into language a model can act on.
func describeItemError(name string, err error) string {
	if errors.Is(err, errNotFound) {
		return fmt.Sprintf("no item named %q; use list_items to find the exact name", name)
	}
	return err.Error()
}

// paginationOf validates and defaults the paging inputs.
func paginationOf(page, pageSize int) (int, int, error) {
	if page < 0 {
		return 0, 0, fmt.Errorf("page must be 1 or greater, got %d", page)
	}
	if pageSize < 0 {
		return 0, 0, fmt.Errorf("page_size must be 1 or greater, got %d", pageSize)
	}
	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	return page, pageSize, nil
}

// pageOf slices one page out of rows, returning empty past the end.
func pageOf[T any](rows []T, page, pageSize int) []T {
	start := (page - 1) * pageSize
	if start >= len(rows) {
		return []T{}
	}
	end := start + pageSize
	if end > len(rows) {
		end = len(rows)
	}
	return rows[start:end]
}

// contains reports whether value holds needle, case-insensitively. An empty
// needle matches everything.
func contains(value, needle string) bool {
	if needle == "" {
		return true
	}
	return strings.Contains(strings.ToLower(value), strings.ToLower(needle))
}

// equalFold compares case-insensitively. An empty want matches everything.
func equalFold(value, want string) bool {
	if want == "" {
		return true
	}
	return strings.EqualFold(value, want)
}

// hasTag reports whether tags holds want. An empty want matches everything.
func hasTag(tags []string, want string) bool {
	if want == "" {
		return true
	}
	for _, tag := range tags {
		if strings.EqualFold(tag, want) {
			return true
		}
	}
	return false
}
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test ./openhab/...`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
gofmt -w openhab/
git add openhab/server.go openhab/tools_items.go openhab/tools_items_test.go openhab/stub_test.go
git commit -m "feat(openhab): list_items and get_item"
```

---

### Task 4: Item control tools

**Files:**
- Create: `openhab/tools_control.go`
- Create: `openhab/tools_control_test.go`

**Interfaces:**
- Consumes: `server`, `openHABClient`, `describeItemError`, `stubClient` from Task 3.
- Produces:
  - `func (s *server) sendCommand(ctx, *mcp.CallToolRequest, sendCommandInput) (*mcp.CallToolResult, commandOutput, error)`.
  - `func (s *server) updateState(ctx, *mcp.CallToolRequest, updateStateInput) (*mcp.CallToolResult, commandOutput, error)`.

- [ ] **Step 1: Write the failing tests**

Create `openhab/tools_control_test.go`:

```go
package main

import (
	"context"
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("item control tools", func() {
	var (
		stub *stubClient
		srv  *server
		ctx  context.Context
	)

	BeforeEach(func() {
		stub = &stubClient{}
		srv = newServer(Config{}, stub)
		ctx = context.Background()
	})

	Describe("send_command", func() {
		It("should send the command to the named item", func() {
			_, out, err := srv.sendCommand(ctx, nil, sendCommandInput{Name: "AlarmTrigger", Command: "ON"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeTrue())
			Expect(stub.commanded).To(ConsistOf([2]string{"AlarmTrigger", "ON"}))
		})

		It("should require a name", func() {
			_, out, err := srv.sendCommand(ctx, nil, sendCommandInput{Command: "ON"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeFalse())
			Expect(out.Error).To(ContainSubstring("name is required"))
			Expect(stub.commanded).To(BeEmpty())
		})

		It("should require a command", func() {
			_, out, err := srv.sendCommand(ctx, nil, sendCommandInput{Name: "AlarmTrigger"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeFalse())
			Expect(out.Error).To(ContainSubstring("command is required"))
			Expect(stub.commanded).To(BeEmpty())
		})

		It("should say plainly when the item does not exist", func() {
			stub.err = fmt.Errorf("/rest/items/Nope: %w", errNotFound)
			_, out, err := srv.sendCommand(ctx, nil, sendCommandInput{Name: "Nope", Command: "ON"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeFalse())
			Expect(out.Error).To(ContainSubstring(`no item named "Nope"`))
		})

		It("should report a client failure in the payload", func() {
			stub.err = errors.New("connection refused")
			_, out, err := srv.sendCommand(ctx, nil, sendCommandInput{Name: "AlarmTrigger", Command: "ON"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeFalse())
			Expect(out.Error).To(ContainSubstring("connection refused"))
		})
	})

	Describe("update_item_state", func() {
		It("should update the state of the named item", func() {
			_, out, err := srv.updateState(ctx, nil, updateStateInput{Name: "Ozone", State: "13.1"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeTrue())
			Expect(stub.stated).To(ConsistOf([2]string{"Ozone", "13.1"}))
			Expect(stub.commanded).To(BeEmpty())
		})

		It("should require a state", func() {
			_, out, err := srv.updateState(ctx, nil, updateStateInput{Name: "Ozone"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeFalse())
			Expect(out.Error).To(ContainSubstring("state is required"))
			Expect(stub.stated).To(BeEmpty())
		})
	})
})
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./openhab/...`
Expected: FAIL — `undefined: sendCommandInput`.

- [ ] **Step 3: Write the implementation**

Create `openhab/tools_control.go`:

```go
package main

import (
	"context"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// commandOutput is the shared result of the two acting tools.
type commandOutput struct {
	Name    string `json:"name" jsonschema:"the item that was addressed"`
	Sent    string `json:"sent" jsonschema:"the command or state that was sent"`
	Success bool   `json:"success" jsonschema:"whether the operation succeeded"`
	Error   string `json:"error,omitempty" jsonschema:"error message if it failed"`
}

type sendCommandInput struct {
	Name    string `json:"name" jsonschema:"the item name, exactly as list_items reports it"`
	Command string `json:"command" jsonschema:"the command to send, for example ON, OFF, UP, DOWN, TOGGLE or a number"`
}

// sendCommand posts a command to an item. openHAB propagates a command through
// rules and bindings, which is what turning a device on means.
func (s *server) sendCommand(ctx context.Context, _ *mcp.CallToolRequest, input sendCommandInput) (
	*mcp.CallToolResult,
	commandOutput,
	error,
) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, commandOutput{Error: "name is required (the item name, as reported by list_items)"}, nil
	}
	command := strings.TrimSpace(input.Command)
	if command == "" {
		return nil, commandOutput{Name: name, Error: "command is required (for example ON, OFF or a number)"}, nil
	}

	if err := s.client.SendCommand(ctx, name, command); err != nil {
		return nil, commandOutput{Name: name, Sent: command, Error: describeItemError(name, err)}, nil
	}

	return nil, commandOutput{Name: name, Sent: command, Success: true}, nil
}

type updateStateInput struct {
	Name  string `json:"name" jsonschema:"the item name, exactly as list_items reports it"`
	State string `json:"state" jsonschema:"the state to set, for example ON, OFF or a number"`
}

// updateState sets an item's state without triggering rules. Use send_command
// to act on a device; use this to record a value.
func (s *server) updateState(ctx context.Context, _ *mcp.CallToolRequest, input updateStateInput) (
	*mcp.CallToolResult,
	commandOutput,
	error,
) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, commandOutput{Error: "name is required (the item name, as reported by list_items)"}, nil
	}
	state := strings.TrimSpace(input.State)
	if state == "" {
		return nil, commandOutput{Name: name, Error: "state is required (for example ON, OFF or a number)"}, nil
	}

	if err := s.client.UpdateState(ctx, name, state); err != nil {
		return nil, commandOutput{Name: name, Sent: state, Error: describeItemError(name, err)}, nil
	}

	return nil, commandOutput{Name: name, Sent: state, Success: true}, nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./openhab/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w openhab/
git add openhab/tools_control.go openhab/tools_control_test.go
git commit -m "feat(openhab): send_command and update_item_state"
```

---

### Task 5: Thing and rule tools

**Files:**
- Create: `openhab/tools_things.go`
- Create: `openhab/tools_things_test.go`

**Interfaces:**
- Consumes: `server`, `Thing`, `ThingStatus`, `Rule`, `paginationOf`, `pageOf`, `contains`, `stubClient` from Tasks 2-3.
- Produces:
  - `func (s *server) listThings(ctx, *mcp.CallToolRequest, listThingsInput) (*mcp.CallToolResult, listThingsOutput, error)`.
  - `func (s *server) getThingStatus(ctx, *mcp.CallToolRequest, getThingStatusInput) (*mcp.CallToolResult, getThingStatusOutput, error)`.
  - `func (s *server) listRules(ctx, *mcp.CallToolRequest, listRulesInput) (*mcp.CallToolResult, listRulesOutput, error)`.
  - `func (s *server) runRule(ctx, *mcp.CallToolRequest, runRuleInput) (*mcp.CallToolResult, runRuleOutput, error)`.
  - `type ThingSummary struct`, `type RuleSummary struct`.

- [ ] **Step 1: Write the failing tests**

Create `openhab/tools_things_test.go`:

```go
package main

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("thing and rule tools", func() {
	var (
		stub *stubClient
		srv  *server
		ctx  context.Context
	)

	BeforeEach(func() {
		stub = &stubClient{
			things: []Thing{
				{UID: "astro:sun:home", ThingTypeUID: "astro:sun", Label: "Dati Astro Sole",
					StatusInfo: ThingStatus{Status: "ONLINE", StatusDetail: "NONE"}},
				{UID: "zwave:device:controller:node12", ThingTypeUID: "zwave:device", Label: "Serranda Sala",
					StatusInfo: ThingStatus{Status: "OFFLINE", StatusDetail: "COMMUNICATION_ERROR",
						Description: "Node is not responding"}},
			},
			rules: []Rule{
				{UID: "nightmode", Name: "Night mode", Tags: []string{"night"},
					Status: RuleStatus{Status: "IDLE"}},
			},
			thingStatus: ThingStatus{Status: "ONLINE", StatusDetail: "NONE"},
		}
		srv = newServer(Config{}, stub)
		ctx = context.Background()
	})

	Describe("list_things", func() {
		It("should return a compact row per thing", func() {
			_, out, err := srv.listThings(ctx, nil, listThingsInput{})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeTrue())
			Expect(out.Things).To(HaveLen(2))
			Expect(out.Things[0].UID).To(Equal("astro:sun:home"))
			Expect(out.Things[0].Status).To(Equal("ONLINE"))
			Expect(out.Things[1].StatusDetail).To(Equal("COMMUNICATION_ERROR"))
		})

		It("should filter by uid", func() {
			_, out, err := srv.listThings(ctx, nil, listThingsInput{FilterUID: "zwave"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Things).To(HaveLen(1))
			Expect(out.Things[0].UID).To(ContainSubstring("zwave"))
		})

		It("should filter by label", func() {
			_, out, err := srv.listThings(ctx, nil, listThingsInput{FilterLabel: "serranda"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Things).To(HaveLen(1))
		})

		It("should paginate", func() {
			_, out, err := srv.listThings(ctx, nil, listThingsInput{Page: 2, PageSize: 1})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Things).To(HaveLen(1))
			Expect(out.Total).To(Equal(2))
		})

		It("should report a client failure in the payload", func() {
			stub.err = errors.New("connection refused")
			_, out, err := srv.listThings(ctx, nil, listThingsInput{})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeFalse())
			Expect(out.Error).To(ContainSubstring("connection refused"))
		})
	})

	Describe("get_thing_status", func() {
		It("should return the status", func() {
			_, out, err := srv.getThingStatus(ctx, nil, getThingStatusInput{UID: "astro:sun:home"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeTrue())
			Expect(out.Status.Status).To(Equal("ONLINE"))
		})

		It("should require a uid", func() {
			_, out, err := srv.getThingStatus(ctx, nil, getThingStatusInput{})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeFalse())
			Expect(out.Error).To(ContainSubstring("uid is required"))
		})
	})

	Describe("list_rules", func() {
		It("should return a compact row per rule", func() {
			_, out, err := srv.listRules(ctx, nil, listRulesInput{})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeTrue())
			Expect(out.Rules).To(HaveLen(1))
			Expect(out.Rules[0].UID).To(Equal("nightmode"))
			Expect(out.Rules[0].Status).To(Equal("IDLE"))
		})

		It("should pass the tag filter to openHAB", func() {
			_, _, err := srv.listRules(ctx, nil, listRulesInput{FilterTag: "night"})
			Expect(err).ToNot(HaveOccurred())
			Expect(stub.ruleTag).To(Equal("night"))
		})
	})

	Describe("run_rule_now", func() {
		It("should run the rule", func() {
			_, out, err := srv.runRule(ctx, nil, runRuleInput{UID: "nightmode"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeTrue())
			Expect(stub.ranRules).To(ConsistOf("nightmode"))
		})

		It("should require a uid", func() {
			_, out, err := srv.runRule(ctx, nil, runRuleInput{})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeFalse())
			Expect(out.Error).To(ContainSubstring("uid is required"))
			Expect(stub.ranRules).To(BeEmpty())
		})
	})
})
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./openhab/...`
Expected: FAIL — `undefined: listThingsInput`.

- [ ] **Step 3: Write the implementation**

Create `openhab/tools_things.go`:

```go
package main

import (
	"context"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ThingSummary is the compact form returned by the thing listing. openHAB's
// own payload carries every channel, which dwarfs the part that matters.
type ThingSummary struct {
	UID          string `json:"uid" jsonschema:"the thing UID, used by get_thing_status"`
	ThingTypeUID string `json:"thing_type_uid" jsonschema:"the binding and thing type, for example zwave:device"`
	Label        string `json:"label" jsonschema:"human-readable label"`
	Status       string `json:"status" jsonschema:"ONLINE, OFFLINE, UNINITIALIZED or similar"`
	StatusDetail string `json:"status_detail,omitempty" jsonschema:"why it is in that status, for example COMMUNICATION_ERROR"`
}

type listThingsInput struct {
	FilterUID   string `json:"filter_uid,omitempty" jsonschema:"only things whose UID contains this text, case-insensitive"`
	FilterLabel string `json:"filter_label,omitempty" jsonschema:"only things whose label contains this text, case-insensitive"`
	Page        int    `json:"page,omitempty" jsonschema:"1-based page number, defaults to 1"`
	PageSize    int    `json:"page_size,omitempty" jsonschema:"things per page, defaults to 50 and is capped at 200"`
}

type listThingsOutput struct {
	Things   []ThingSummary `json:"things" jsonschema:"the things on this page"`
	Total    int            `json:"total" jsonschema:"how many things matched the filters, before pagination"`
	Page     int            `json:"page" jsonschema:"the page that was returned"`
	PageSize int            `json:"page_size" jsonschema:"the page size that was applied"`
	Success  bool           `json:"success" jsonschema:"whether the operation succeeded"`
	Error    string         `json:"error,omitempty" jsonschema:"error message if it failed"`
}

// listThings reports the bindings' things and whether they are healthy.
func (s *server) listThings(ctx context.Context, _ *mcp.CallToolRequest, input listThingsInput) (
	*mcp.CallToolResult,
	listThingsOutput,
	error,
) {
	page, pageSize, err := paginationOf(input.Page, input.PageSize)
	if err != nil {
		return nil, listThingsOutput{Error: err.Error()}, nil
	}

	things, err := s.client.Things(ctx)
	if err != nil {
		return nil, listThingsOutput{Error: err.Error()}, nil
	}

	matched := make([]ThingSummary, 0, len(things))
	for _, thing := range things {
		if !contains(thing.UID, input.FilterUID) || !contains(thing.Label, input.FilterLabel) {
			continue
		}
		matched = append(matched, ThingSummary{
			UID:          thing.UID,
			ThingTypeUID: thing.ThingTypeUID,
			Label:        thing.Label,
			Status:       thing.StatusInfo.Status,
			StatusDetail: thing.StatusInfo.StatusDetail,
		})
	}

	return nil, listThingsOutput{
		Things:   pageOf(matched, page, pageSize),
		Total:    len(matched),
		Page:     page,
		PageSize: pageSize,
		Success:  true,
	}, nil
}

type getThingStatusInput struct {
	UID string `json:"uid" jsonschema:"the thing UID, exactly as list_things reports it"`
}

type getThingStatusOutput struct {
	UID     string      `json:"uid" jsonschema:"the thing that was queried"`
	Status  ThingStatus `json:"status" jsonschema:"status, detail and description"`
	Success bool        `json:"success" jsonschema:"whether the operation succeeded"`
	Error   string      `json:"error,omitempty" jsonschema:"error message if it failed"`
}

// getThingStatus reports one thing's health, including the description a
// binding attaches when something is wrong.
func (s *server) getThingStatus(ctx context.Context, _ *mcp.CallToolRequest, input getThingStatusInput) (
	*mcp.CallToolResult,
	getThingStatusOutput,
	error,
) {
	uid := strings.TrimSpace(input.UID)
	if uid == "" {
		return nil, getThingStatusOutput{Error: "uid is required (the thing UID, as reported by list_things)"}, nil
	}

	status, err := s.client.ThingStatus(ctx, uid)
	if err != nil {
		return nil, getThingStatusOutput{UID: uid, Error: err.Error()}, nil
	}

	return nil, getThingStatusOutput{UID: uid, Status: status, Success: true}, nil
}

// RuleSummary is the compact form returned by the rule listing.
type RuleSummary struct {
	UID         string   `json:"uid" jsonschema:"the rule UID, used by run_rule_now"`
	Name        string   `json:"name" jsonschema:"human-readable rule name"`
	Description string   `json:"description,omitempty" jsonschema:"what the rule does, when the author wrote it down"`
	Tags        []string `json:"tags,omitempty" jsonschema:"tags on the rule"`
	Status      string   `json:"status" jsonschema:"IDLE, RUNNING or UNINITIALIZED"`
}

type listRulesInput struct {
	FilterTag string `json:"filter_tag,omitempty" jsonschema:"only rules carrying this tag; openHAB applies this filter itself"`
	Page      int    `json:"page,omitempty" jsonschema:"1-based page number, defaults to 1"`
	PageSize  int    `json:"page_size,omitempty" jsonschema:"rules per page, defaults to 50 and is capped at 200"`
}

type listRulesOutput struct {
	Rules    []RuleSummary `json:"rules" jsonschema:"the rules on this page"`
	Total    int           `json:"total" jsonschema:"how many rules matched, before pagination"`
	Page     int           `json:"page" jsonschema:"the page that was returned"`
	PageSize int           `json:"page_size" jsonschema:"the page size that was applied"`
	Success  bool          `json:"success" jsonschema:"whether the operation succeeded"`
	Error    string        `json:"error,omitempty" jsonschema:"error message if it failed"`
}

// listRules reports the rules openHAB knows about. The tag filter is openHAB's
// own, so it happens server-side.
func (s *server) listRules(ctx context.Context, _ *mcp.CallToolRequest, input listRulesInput) (
	*mcp.CallToolResult,
	listRulesOutput,
	error,
) {
	page, pageSize, err := paginationOf(input.Page, input.PageSize)
	if err != nil {
		return nil, listRulesOutput{Error: err.Error()}, nil
	}

	rules, err := s.client.Rules(ctx, strings.TrimSpace(input.FilterTag))
	if err != nil {
		return nil, listRulesOutput{Error: err.Error()}, nil
	}

	summaries := make([]RuleSummary, 0, len(rules))
	for _, rule := range rules {
		summaries = append(summaries, RuleSummary{
			UID:         rule.UID,
			Name:        rule.Name,
			Description: rule.Description,
			Tags:        rule.Tags,
			Status:      rule.Status.Status,
		})
	}

	return nil, listRulesOutput{
		Rules:    pageOf(summaries, page, pageSize),
		Total:    len(summaries),
		Page:     page,
		PageSize: pageSize,
		Success:  true,
	}, nil
}

type runRuleInput struct {
	UID string `json:"uid" jsonschema:"the rule UID, exactly as list_rules reports it"`
}

type runRuleOutput struct {
	UID     string `json:"uid" jsonschema:"the rule that was run"`
	Success bool   `json:"success" jsonschema:"whether the operation succeeded"`
	Error   string `json:"error,omitempty" jsonschema:"error message if it failed"`
}

// runRule triggers a rule immediately, as the openHAB UI's "run now" does.
func (s *server) runRule(ctx context.Context, _ *mcp.CallToolRequest, input runRuleInput) (
	*mcp.CallToolResult,
	runRuleOutput,
	error,
) {
	uid := strings.TrimSpace(input.UID)
	if uid == "" {
		return nil, runRuleOutput{Error: "uid is required (the rule UID, as reported by list_rules)"}, nil
	}

	if err := s.client.RunRule(ctx, uid); err != nil {
		return nil, runRuleOutput{UID: uid, Error: err.Error()}, nil
	}

	return nil, runRuleOutput{UID: uid, Success: true}, nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./openhab/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w openhab/
git add openhab/tools_things.go openhab/tools_things_test.go
git commit -m "feat(openhab): thing and rule tools"
```

---

### Task 6: Tool registration and entrypoint

**Files:**
- Create: `openhab/tools.go`
- Create: `openhab/tools_test.go`
- Create: `openhab/main.go`

**Interfaces:**
- Consumes: every handler from Tasks 3-5, `loadConfig`, `newRESTClient`, `newServer`.
- Produces: `type toolSpec struct`, `func (s *server) toolSpecs() []toolSpec`, `func spec[In, Out any](...) toolSpec`, and the `main` entrypoint.

- [ ] **Step 1: Write the failing registration tests**

Create `openhab/tools_test.go`:

```go
package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// toolNames lists the tools a configuration exposes.
func toolNames(cfg Config) []string {
	srv := newServer(cfg, &stubClient{})
	names := []string{}
	for _, spec := range srv.toolSpecs() {
		names = append(names, spec.name)
	}
	return names
}

var _ = Describe("toolSpecs", func() {
	It("should expose all eight tools by default", func() {
		Expect(toolNames(Config{})).To(ConsistOf(
			"list_items", "get_item", "send_command", "update_item_state",
			"list_things", "get_thing_status", "list_rules", "run_rule_now",
		))
	})

	It("should drop the acting tools in read-only mode", func() {
		names := toolNames(Config{ReadOnly: true})
		Expect(names).To(ConsistOf("list_items", "get_item", "list_things", "get_thing_status", "list_rules"))
		Expect(names).ToNot(ContainElement("send_command"))
		Expect(names).ToNot(ContainElement("update_item_state"))
		Expect(names).ToNot(ContainElement("run_rule_now"))
	})

	It("should describe every tool it exposes", func() {
		srv := newServer(Config{}, &stubClient{})
		for _, spec := range srv.toolSpecs() {
			Expect(spec.description).ToNot(BeEmpty(), spec.name)
		}
	})
})
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./openhab/...`
Expected: FAIL — `srv.toolSpecs undefined`.

- [ ] **Step 3: Write the registration**

Create `openhab/tools.go`:

```go
package main

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// toolSpec pairs a tool's advertised identity with the call that registers it,
// so the set of tools a configuration exposes is one list rather than a
// scattering of conditionals in main.
type toolSpec struct {
	name        string
	description string
	add         func(*mcp.Server)
}

// toolSpecs returns the tools this configuration exposes. Restricted tools are
// never registered rather than registered-and-refusing, so a model does not
// waste a call discovering it is not allowed.
func (s *server) toolSpecs() []toolSpec {
	specs := []toolSpec{
		spec("list_items", listItemsDescription, s.listItems),
		spec("get_item", getItemDescription, s.getItem),
		spec("list_things", listThingsDescription, s.listThings),
		spec("get_thing_status", getThingStatusDescription, s.getThingStatus),
		spec("list_rules", listRulesDescription, s.listRules),
	}

	if s.cfg.ReadOnly {
		return specs
	}

	return append(specs,
		spec("send_command", sendCommandDescription, s.sendCommand),
		spec("update_item_state", updateStateDescription, s.updateState),
		spec("run_rule_now", runRuleDescription, s.runRule),
	)
}

// spec captures one tool's registration, deferring the AddTool call so the
// list can be inspected without an MCP server in hand.
func spec[In, Out any](
	name, description string,
	handler func(context.Context, *mcp.CallToolRequest, In) (*mcp.CallToolResult, Out, error),
) toolSpec {
	return toolSpec{
		name:        name,
		description: description,
		add: func(server *mcp.Server) {
			mcp.AddTool(server, &mcp.Tool{Name: name, Description: description}, handler)
		},
	}
}

const (
	listItemsDescription = "List openHAB items with their current state. Items are the things you read and control: switches, sensors, setpoints. Filter by name, label, type or tag, and page through the results. Always find the exact item name here before acting on it."
	getItemDescription   = "Get one openHAB item in full, including its tags and the groups it belongs to. Use list_items first to find the exact name."
	sendCommandDescription = "Send a command to an openHAB item, for example ON, OFF, UP, DOWN, TOGGLE or a number. This is how you act on the house: the command travels through rules and bindings to the device."
	updateStateDescription = "Set an openHAB item's state directly, without triggering rules. Use this to record a value; use send_command to act on a device."
	listThingsDescription  = "List openHAB things with their status. Things are the physical devices and bindings behind the items. Use this to find out whether something is ONLINE before blaming a rule."
	getThingStatusDescription = "Get one openHAB thing's status, including the detail and description a binding reports when it is offline."
	listRulesDescription      = "List openHAB rules with their status, optionally filtered by tag. Rules are the automations that run on triggers."
	runRuleDescription        = "Run an openHAB rule immediately, as the 'run now' button in the openHAB UI does. Use list_rules to find the exact UID."
)
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./openhab/...`
Expected: PASS.

- [ ] **Step 5: Write the entrypoint**

Create `openhab/main.go`:

```go
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	// Logs go to stderr: stdout carries the JSON-RPC stream and must stay
	// clean.
	log.SetOutput(os.Stderr)
	log.SetFlags(0)

	if err := run(); err != nil {
		log.Fatalf("openhab mcp: %v", err)
	}
}

func run() error {
	cfg, err := loadConfig(os.Getenv)
	if err != nil {
		return err
	}

	client, err := newRESTClient(cfg)
	if err != nil {
		return err
	}

	// Stop on the signals a container runtime sends.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	srv := newServer(cfg, client)

	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:    "openhab",
		Version: "v1.0.0",
	}, nil)

	tools := srv.toolSpecs()
	for _, tool := range tools {
		tool.add(mcpServer)
	}

	mode := "read-write"
	if cfg.ReadOnly {
		mode = "read-only"
	}
	if cfg.InsecureSkipVerify {
		log.Printf("warning: TLS certificate verification is disabled")
	}
	log.Printf("serving %s with %d tools (%s)", cfg.BaseURL, len(tools), mode)

	if err := mcpServer.Run(ctx, &mcp.StdioTransport{}); err != nil {
		// A signal cancels the context; that is a clean shutdown, not a
		// failure to report.
		if ctx.Err() != nil {
			return nil
		}
		return err
	}
	return nil
}
```

- [ ] **Step 6: Verify it builds and the whole suite passes**

Run: `go build ./openhab/ && go vet ./openhab/ && go test ./openhab/...`
Expected: builds clean, vet silent, all specs PASS.

- [ ] **Step 7: Commit**

```bash
gofmt -w openhab/
git add openhab/tools.go openhab/tools_test.go openhab/main.go
git commit -m "feat(openhab): tool registration and stdio entrypoint"
```

---

### Task 7: Packaging and documentation

**Files:**
- Modify: `.github/workflows/image.yml` (the `matrix.include` list, alongside the `jellyfin` entry)
- Modify: `README.md`
- Create: `openhab/SYSTEM_PROMPT.md`

**Interfaces:**
- Consumes: the finished server from Task 6.
- Produces: no Go symbols. A CI matrix entry, a README section, a system prompt.

- [ ] **Step 1: Add the CI matrix entry**

In `.github/workflows/image.yml`, inside `matrix.include`, next to the `jellyfin` entry:

```yaml
          - mcp: openhab
            dockerfile: ./Dockerfile
            context: ./
```

No new Dockerfile is needed: the shared one builds `./${MCP_SERVER}/`.

- [ ] **Step 2: Verify the image builds**

Run: `make MCP_SERVER=openhab build`
Expected: image `ghcr.io/mudler/mcps/openhab:latest` builds.

- [ ] **Step 3: Write the system prompt**

Create `openhab/SYSTEM_PROMPT.md`:

```markdown
# openHAB MCP — System Prompt

You control a home through openHAB.

Never guess an item name. Call `list_items` with a filter and read the exact
name off the result before acting. Item labels are often in the household's own
language while names are not, so filter on `filter_label` when the user names
something in words and on `filter_name` when they use an identifier.

`send_command` acts on the house — it is what turns a light on, opens a gate or
sets a thermostat. `update_item_state` only records a value and does not reach
the device; reach for it when a rule or a sensor reading needs writing down, not
when the user asks for something to happen.

When a command appears to do nothing, check the device rather than repeating the
command: `list_things` shows which bindings are ONLINE, and `get_thing_status`
explains why one is not.

Confirm before acting on anything that affects safety or security — alarms,
sirens, gates, locks, heating left running. State plainly what you are about to
do and which item you will send it to.
```

- [ ] **Step 4: Write the README section**

Add a section to `README.md` in the same shape as the samba and jellyfin sections — heading, a sentence on what it does, the environment table, a `docker run` line and an `mcpServers` snippet:

````markdown
### openHAB

Read and control a home through [openHAB](https://www.openhab.org/)'s REST API:
list items and their state, send commands, check whether the devices behind them
are online, and run rules.

| Variable | Default | Description |
| --- | --- | --- |
| `OPENHAB_URL` | required | Base URL, for example `http://openhab:8080` or `https://10.0.0.5:8443` |
| `OPENHAB_API_TOKEN` | empty | openHAB API token; preferred over basic auth |
| `OPENHAB_USERNAME` | empty | Basic-auth user, used when no token is set |
| `OPENHAB_PASSWORD` | empty | Basic-auth password |
| `OPENHAB_TIMEOUT` | `30s` | Bounds every request |
| `OPENHAB_CA_CERT` | empty | Path to a PEM bundle, for an instance behind a private CA |
| `OPENHAB_INSECURE_SKIP_VERIFY` | `false` | Skip TLS verification entirely |
| `OPENHAB_READ_ONLY` | `false` | When true, only the read tools are exposed |

Tools: `list_items`, `get_item`, `send_command`, `update_item_state`,
`list_things`, `get_thing_status`, `list_rules`, `run_rule_now`.

`send_command` sends a command, which travels through rules and bindings to the
device. `update_item_state` sets an item's state without triggering rules. They
are different acts and the server keeps them apart.

openHAB's default HTTPS certificate is self-signed and carries no
`subjectAltName`, so no CA bundle can validate it — `OPENHAB_INSECURE_SKIP_VERIFY=true`
is the way to reach such an instance until its certificate is reissued.

```bash
docker run -i --rm \
  -e OPENHAB_URL=https://10.0.0.5:8443 \
  -e OPENHAB_API_TOKEN=oh.mytoken.xxxxx \
  -e OPENHAB_INSECURE_SKIP_VERIFY=true \
  ghcr.io/mudler/mcps/openhab:latest
```

```json
{
    "mcpServers": {
        "openhab": {
            "command": "docker",
            "args": [
                "run", "-i", "--rm",
                "-e", "OPENHAB_URL",
                "-e", "OPENHAB_API_TOKEN",
                "ghcr.io/mudler/mcps/openhab:master"
            ],
            "env": {
                "OPENHAB_URL": "http://your-openhab:8080",
                "OPENHAB_API_TOKEN": "oh.mytoken.xxxxx"
            }
        }
    }
}
```
````

- [ ] **Step 5: Commit**

```bash
git add .github/workflows/image.yml README.md openhab/SYSTEM_PROMPT.md
git commit -m "docs(openhab): README, system prompt and image build"
```

---

### Task 8: Acceptance against a real instance

**Files:**
- Create: `openhab/README-test.md`

**Interfaces:**
- Consumes: the built image from Task 7.
- Produces: a recorded acceptance run. No Go symbols.

This task is manual. The specs in Tasks 1-6 are hermetic; this proves the server
works against openHAB itself, including the TLS path that motivated the project.

- [ ] **Step 1: Run the image against the real instance**

With `OPENHAB_URL` and `OPENHAB_API_TOKEN` set for the target instance:

```bash
printf '%s\n' \
 '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"cli","version":"0"}}}' \
 '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
 '{"jsonrpc":"2.0","id":2,"method":"tools/list"}' \
 '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"list_items","arguments":{"page_size":5}}}' \
 '{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"list_things","arguments":{}}}' \
| docker run -i --rm \
    -e OPENHAB_URL \
    -e OPENHAB_API_TOKEN \
    -e OPENHAB_INSECURE_SKIP_VERIFY=true \
    ghcr.io/mudler/mcps/openhab:latest
```

Expected: `tools/list` reports eight tools; `list_items` returns real items with
states; `list_things` reports real things with their status.

- [ ] **Step 2: Verify read-only mode hides the acting tools**

Re-run the same pipe with `-e OPENHAB_READ_ONLY=true`.
Expected: `tools/list` reports five tools, and `send_command`,
`update_item_state` and `run_rule_now` are absent.

- [ ] **Step 3: Verify TLS verification is on by default**

Re-run the first pipe without `OPENHAB_INSECURE_SKIP_VERIFY`.
Expected: `list_items` fails with a certificate error in the output payload —
proof the skip flag is doing something and is not the default.

- [ ] **Step 4: Send one real command**

Pick a harmless item (a lamp, not the siren) and send it a command, then read it
back with `get_item` and confirm the state changed.

- [ ] **Step 5: Record the run**

Create `openhab/README-test.md` documenting how to run this acceptance check:
the environment variables needed, the pipe above, and what each step proves.
Follow the shape of `samba/README-test.md`.

- [ ] **Step 6: Commit**

```bash
git add openhab/README-test.md
git commit -m "docs(openhab): acceptance test procedure"
```

---

## Self-Review Notes

Checked against `docs/superpowers/specs/2026-08-24-openhab-mcp-design.md`:

- All eight tools in the spec's table have a task: items in Task 3, control in
  Task 4, things and rules in Task 5.
- All eight configuration variables are parsed and tested in Task 1;
  `OPENHAB_READ_ONLY`'s registration effect is tested in Task 6.
- The three "details the implementation must get right" are each covered: the
  command/state split (Tasks 2 and 4), compact listings (Tasks 3 and 5), and
  client-side filtering and pagination (Task 3, reused in Task 5).
- The spec's `handlers.go` is split into `tools_items.go`, `tools_control.go`
  and `tools_things.go`. This follows samba's `tools_read.go`/`tools_write.go`
  split and keeps each file to one tool family; the spec's intent — handlers
  separate from the client — is preserved.
- The spec's testing section is covered by Tasks 1-6, with the TLS specs in
  Task 2 using `httptest.NewTLSServer` as the spec requires.
- Packaging (Task 7) and the acceptance criterion (Task 8) match the spec's
  final two sections.
