# Thunderbird MCP Server Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A Go MCP server exposing a Thunderbird user's mail, contacts, and calendar — searching and reading mail via the gloda index, mutating and sending via IMAP/SMTP — shipped as a binary and a Docker image consistent with the rest of this repository.

**Architecture:** Read Thunderbird's own on-disk profile state directly (prefs.js for account config, `global-messages-db.sqlite` for search, `logins.json`+`key4.db` for credentials, `abook.sqlite` and `calendar-data/local.sqlite` for contacts/calendar) and use IMAP/SMTP for anything live. No Thunderbird extension. One self-contained binary that works whether or not Thunderbird is running.

**Tech Stack:** Go 1.25, `github.com/modelcontextprotocol/go-sdk/mcp`, `modernc.org/sqlite` (pure-Go, CGO-free), `github.com/emersion/go-imap/v2`, `github.com/emersion/go-smtp`, `github.com/emersion/go-message`, `github.com/emersion/go-sasl`, `golang.org/x/crypto`. Tests use ginkgo/gomega.

## Global Constraints

- Build must stay `CGO_ENABLED=0` clean — every dependency is pure Go. Verify with `CGO_ENABLED=0 go build ./thunderbird/`.
- Package is `package main` in directory `thunderbird/`, following the `jellyfin/` layout exactly.
- Go module: `github.com/mudler/mcps`, Go 1.25.0. Do not bump the Go directive.
- Never write to any Thunderbird-owned file (mbox, maildir, gloda db, `key4.db`, `logins.json`, `abook.sqlite`, `local.sqlite`). All SQLite opens are read-only via `?mode=ro`. All mutations go through IMAP.
- Local folders (account `type` `pop3` or `none`) are read-only: mutating tools must reject them with an explicit error.
- Config env vars: `THUNDERBIRD_PROFILE` (path; auto-discovered if empty), `THUNDERBIRD_READ_ONLY` (`true` unregisters mutate+compose tools), `THUNDERBIRD_ALLOW_SEND` (compose tools registered only when `true`), `THUNDERBIRD_TOOLS` (comma-separated allowlist, empty/`all` = all enabled).
- Tool handler signature is the go-sdk generic form: `func(context.Context, *mcp.CallToolRequest, InInput) (*mcp.CallToolResult, OutOutput, error)`, registered through a typed switch in `registerTool` (mirror `jellyfin/main.go`).
- `imap.UID` is a distinct type (`type UID uint32`); build UID sets with `imap.UIDSetNum(uid)`. Fetch/Store/Move take a single `imap.NumSet` — pass a `UIDSet` for UID mode (there is no `UIDFetch`/`UIDStore`/`UIDMove` in v2). go-imap commands are async: call `.Wait()` / `.Collect()`.

---

## File Structure

| File | Responsibility |
|---|---|
| `thunderbird/main.go` | env config, tool registry, `registerTool` switch, server startup |
| `thunderbird/types.go` | all tool Input/Output structs with jsonschema tags, plus shared domain structs (`Account`, `Folder`, `MessageRef`, `MessageSummary`, `MessageDetail`, `Contact`, `CalendarEvent`) |
| `thunderbird/profile.go` | profile discovery, `profiles.ini` + `prefs.js` parsing into `Config` (accounts, servers, identities, smtp servers, calendars) |
| `thunderbird/secrets.go` | NSS `key4.db` + `logins.json` decryption; `Credentials.Lookup(host, user)` |
| `thunderbird/gloda.go` | read-only gloda queries (search, recent) against `messagesText_content` |
| `thunderbird/imap.go` | IMAP connection pool keyed by account, fetch bodies, set flags, move, delete, append draft |
| `thunderbird/smtp.go` | build RFC822 messages (send/reply/forward) and send via go-smtp |
| `thunderbird/mbox.go` | read a message body from a local mbox file at a byte offset (read-only) |
| `thunderbird/contacts.go` | `abook.sqlite` queries |
| `thunderbird/calendar.go` | `local.sqlite` + `calendar.registry.*` queries |
| `thunderbird/handlers.go` | MCP tool handlers wiring the above together |
| `thunderbird/app.go` | `App` struct holding `*Config`, `*Credentials`, gloda/contacts/calendar db handles, imap pool; single global `app *App` like jellyfin's global `client` |
| `thunderbird/*_test.go` | ginkgo specs + `thunderbird_suite_test.go` |
| `thunderbird/testdata/` | fixture `profiles.ini`, `prefs.js`, generated gloda/abook/calendar sqlite, NSS fixture profile |

Global state: a single `var app *App` (mirrors jellyfin's `var client *JellyfinClient`). Handlers read `app`.

---

## Task 1: Scaffolding — package, config, empty tool registry, green build

**Files:**
- Create: `thunderbird/main.go`, `thunderbird/app.go`, `thunderbird/types.go`, `thunderbird/thunderbird_suite_test.go`, `thunderbird/config_test.go`

**Interfaces:**
- Produces: `type App struct{ Config *Config; ReadOnly bool; AllowSend bool }` (fields grow in later tasks); `func parseToolFilter(env string) map[string]bool`; `var app *App`.

- [ ] **Step 1: Write the failing test** — `thunderbird/config_test.go`

```go
package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("parseToolFilter", func() {
	It("returns nil for empty or 'all'", func() {
		Expect(parseToolFilter("")).To(BeNil())
		Expect(parseToolFilter("all")).To(BeNil())
	})
	It("parses a comma list trimming spaces", func() {
		got := parseToolFilter("search_messages, get_message ,list_folders")
		Expect(got).To(Equal(map[string]bool{
			"search_messages": true, "get_message": true, "list_folders": true,
		}))
	})
})
```

- [ ] **Step 2: Create the suite runner** — `thunderbird/thunderbird_suite_test.go`

```go
package main

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestThunderbird(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Thunderbird Suite")
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `cd /home/mudler/_git/mcps && go test ./thunderbird/`
Expected: FAIL — `undefined: parseToolFilter`.

- [ ] **Step 4: Write `thunderbird/app.go`**

```go
package main

// App holds all runtime state for the Thunderbird MCP server.
// Fields are added as later tasks introduce data sources.
type App struct {
	Config    *Config
	ReadOnly  bool
	AllowSend bool
}

// app is the process-wide server state, populated in main().
var app *App
```

- [ ] **Step 5: Write `thunderbird/types.go` initial content**

```go
package main

// Config is the parsed Thunderbird profile configuration.
// Populated by parseProfile in profile.go (Task 2).
type Config struct {
	ProfileDir string
	Accounts   []Account
}

// Account is one Thunderbird mail account.
type Account struct {
	Key        string // e.g. "account1"
	Type       string // "imap", "pop3", "none" (Local Folders)
	Hostname   string
	Port       int
	Username   string
	SocketType int // 0 plain, 2 STARTTLS, 3 SSL/TLS
	AuthMethod int // 10 == OAuth2
	Directory  string
	Name       string
	Identities []Identity
}

// Identity is a sending identity attached to an account.
type Identity struct {
	Key         string
	Email       string
	FullName    string
	SMTPKey     string
	DraftFolder string // folder URI
	FccFolder   string // folder URI ("sent")
}

// IsLocal reports whether the account stores mail on disk (read-only for us).
func (a Account) IsLocal() bool { return a.Type == "pop3" || a.Type == "none" }
```

- [ ] **Step 6: Write `thunderbird/main.go`**

```go
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

func allTools() []toolDef { return nil } // filled in later tasks

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
	log.Fatalf("no registration case for tool %s", td.tool.Name)
}

func main() {
	a := &App{
		ReadOnly:  os.Getenv("THUNDERBIRD_READ_ONLY") == "true",
		AllowSend: os.Getenv("THUNDERBIRD_ALLOW_SEND") == "true",
	}
	app = a

	// Profile loading wired in Task 2+.
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
```

- [ ] **Step 7: Run tests to verify pass**

Run: `cd /home/mudler/_git/mcps && go test ./thunderbird/ && CGO_ENABLED=0 go build ./thunderbird/`
Expected: PASS, build OK. (`allTools` returns nil so `registerTool` is never hit.)

- [ ] **Step 8: Commit**

```bash
git add thunderbird/ && git commit -m "feat(thunderbird): scaffold package, config, tool registry"
```

---

## Task 2: Profile discovery and prefs.js parsing

**Files:**
- Create: `thunderbird/profile.go`, `thunderbird/profile_test.go`, `thunderbird/testdata/prefs.js`, `thunderbird/testdata/profiles.ini`

**Interfaces:**
- Consumes: `Config`, `Account`, `Identity` from types.go.
- Produces:
  - `func parsePrefs(r io.Reader) (map[string]string, error)` — every `user_pref("k", v)` as string values (numbers/bools kept as their literal text: `"993"`, `"true"`).
  - `func buildConfig(prefs map[string]string, profileDir string) *Config`
  - `func discoverProfile() (string, error)` — `THUNDERBIRD_PROFILE` else default from `profiles.ini`.
  - `func loadProfile(profileDir string) (*Config, error)` — read `prefs.js`, call parse+build.
  - `type SMTPServer struct{ Key, Hostname string; Port, SocketType, AuthMethod int; Username string }` and `Config.SMTP map[string]SMTPServer`; add `SMTP` field to `Config`.

- [ ] **Step 1: Write the fixture** — `thunderbird/testdata/prefs.js`

```javascript
user_pref("mail.accountmanager.accounts", "account1,account2");
user_pref("mail.accountmanager.defaultaccount", "account1");
user_pref("mail.account.account1.server", "server1");
user_pref("mail.account.account1.identities", "id1");
user_pref("mail.account.account2.server", "server2");
user_pref("mail.account.account2.identities", "id2");
user_pref("mail.server.server1.type", "imap");
user_pref("mail.server.server1.hostname", "imap.example.com");
user_pref("mail.server.server1.port", 993);
user_pref("mail.server.server1.userName", "alice@example.com");
user_pref("mail.server.server1.socketType", 3);
user_pref("mail.server.server1.authMethod", 3);
user_pref("mail.server.server1.name", "Example IMAP");
user_pref("mail.server.server1.directory-rel", "[ProfD]ImapMail/imap.example.com");
user_pref("mail.server.server2.type", "none");
user_pref("mail.server.server2.hostname", "Local Folders");
user_pref("mail.server.server2.userName", "nobody");
user_pref("mail.identity.id1.useremail", "alice@example.com");
user_pref("mail.identity.id1.fullName", "Alice Example");
user_pref("mail.identity.id1.smtpServer", "smtp1");
user_pref("mail.identity.id1.draft_folder", "imap://alice@example.com/Drafts");
user_pref("mail.identity.id1.fcc_folder", "imap://alice@example.com/Sent");
user_pref("mail.identity.id2.useremail", "local@localhost");
user_pref("mail.smtpserver.smtp1.hostname", "smtp.example.com");
user_pref("mail.smtpserver.smtp1.port", 587);
user_pref("mail.smtpserver.smtp1.username", "alice@example.com");
user_pref("mail.smtpserver.smtp1.try_ssl", 2);
user_pref("mail.smtpserver.smtp1.authMethod", 3);
user_pref("mail.smtp.defaultserver", "smtp1");
user_pref("calendar.registry.cal-uuid-1.type", "storage");
user_pref("calendar.registry.cal-uuid-1.name", "Home");
user_pref("calendar.registry.cal-uuid-1.uri", "moz-storage-calendar://");
```

- [ ] **Step 2: Write failing tests** — `thunderbird/profile_test.go`

```go
package main

import (
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("parsePrefs", func() {
	It("parses strings, ints, and bools as literal strings", func() {
		in := `user_pref("a.b", "hello");
user_pref("a.c", 993);
user_pref("a.d", true);
// comment line ignored
user_pref("a.e", "with \"quote\"");`
		m, err := parsePrefs(strings.NewReader(in))
		Expect(err).NotTo(HaveOccurred())
		Expect(m["a.b"]).To(Equal("hello"))
		Expect(m["a.c"]).To(Equal("993"))
		Expect(m["a.d"]).To(Equal("true"))
		Expect(m["a.e"]).To(Equal(`with "quote"`))
	})
})

var _ = Describe("buildConfig", func() {
	var cfg *Config
	BeforeEach(func() {
		f, err := os.Open("testdata/prefs.js")
		Expect(err).NotTo(HaveOccurred())
		defer f.Close()
		prefs, err := parsePrefs(f)
		Expect(err).NotTo(HaveOccurred())
		cfg = buildConfig(prefs, "/profile")
	})
	It("builds imap and local accounts", func() {
		Expect(cfg.Accounts).To(HaveLen(2))
		imap := cfg.Accounts[0]
		Expect(imap.Type).To(Equal("imap"))
		Expect(imap.Hostname).To(Equal("imap.example.com"))
		Expect(imap.Port).To(Equal(993))
		Expect(imap.SocketType).To(Equal(3))
		Expect(imap.IsLocal()).To(BeFalse())
		Expect(imap.Identities).To(HaveLen(1))
		Expect(imap.Identities[0].Email).To(Equal("alice@example.com"))
		Expect(imap.Identities[0].SMTPKey).To(Equal("smtp1"))
	})
	It("marks the Local Folders account local", func() {
		Expect(cfg.Accounts[1].IsLocal()).To(BeTrue())
	})
	It("parses smtp servers", func() {
		Expect(cfg.SMTP["smtp1"].Hostname).To(Equal("smtp.example.com"))
		Expect(cfg.SMTP["smtp1"].Port).To(Equal(587))
		Expect(cfg.SMTP["smtp1"].SocketType).To(Equal(2))
	})
})
```

- [ ] **Step 3: Run to verify fail**

Run: `go test ./thunderbird/ -run TestThunderbird`
Expected: FAIL — `undefined: parsePrefs`, `undefined: buildConfig`.

- [ ] **Step 4: Add `SMTP` and `Calendars` to `Config` in types.go**

```go
// add to Config struct
	SMTP      map[string]SMTPServer
	Calendars []CalendarRef

// add new types
type SMTPServer struct {
	Key, Hostname string
	Port          int
	SocketType    int
	AuthMethod    int
	Username      string
}

type CalendarRef struct {
	UUID string
	Name string
	Type string // "storage", "caldav", ...
	URI  string
}
```

- [ ] **Step 5: Implement `thunderbird/profile.go`**

```go
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var prefLine = regexp.MustCompile(`^\s*user_pref\("([^"]+)",\s*(.*)\);\s*$`)

// parsePrefs reads a prefs.js and returns each user_pref value as a string.
// String values are unquoted/unescaped; numbers and booleans are kept literal.
func parsePrefs(r io.Reader) (map[string]string, error) {
	out := map[string]string{}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		m := prefLine.FindStringSubmatch(sc.Text())
		if m == nil {
			continue
		}
		key, raw := m[1], strings.TrimSpace(m[2])
		if len(raw) >= 2 && raw[0] == '"' && raw[len(raw)-1] == '"' {
			s := raw[1 : len(raw)-1]
			s = strings.ReplaceAll(s, `\"`, `"`)
			s = strings.ReplaceAll(s, `\\`, `\`)
			out[key] = s
		} else {
			out[key] = raw // int or bool literal
		}
	}
	return out, sc.Err()
}

func atoi(s string) int { n, _ := strconv.Atoi(s); return n }

func splitList(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

// buildConfig assembles a Config by walking the account -> server/identity graph.
func buildConfig(p map[string]string, profileDir string) *Config {
	cfg := &Config{ProfileDir: profileDir, SMTP: map[string]SMTPServer{}}

	for _, acctKey := range splitList(p["mail.accountmanager.accounts"]) {
		srv := p["mail.account."+acctKey+".server"]
		if srv == "" {
			continue
		}
		sp := "mail.server." + srv + "."
		acct := Account{
			Key:        acctKey,
			Type:       p[sp+"type"],
			Hostname:   p[sp+"hostname"],
			Port:       atoi(p[sp+"port"]),
			Username:   p[sp+"userName"],
			SocketType: atoi(p[sp+"socketType"]),
			AuthMethod: atoi(p[sp+"authMethod"]),
			Directory:  resolveDir(p[sp+"directory"], p[sp+"directory-rel"], profileDir),
			Name:       p[sp+"name"],
		}
		if acct.Type == "" {
			continue
		}
		for _, idKey := range splitList(p["mail.account."+acctKey+".identities"]) {
			ip := "mail.identity." + idKey + "."
			acct.Identities = append(acct.Identities, Identity{
				Key:         idKey,
				Email:       p[ip+"useremail"],
				FullName:    p[ip+"fullName"],
				SMTPKey:     p[ip+"smtpServer"],
				DraftFolder: p[ip+"draft_folder"],
				FccFolder:   p[ip+"fcc_folder"],
			})
		}
		cfg.Accounts = append(cfg.Accounts, acct)
	}

	for key := range smtpKeys(p) {
		sp := "mail.smtpserver." + key + "."
		cfg.SMTP[key] = SMTPServer{
			Key: key, Hostname: p[sp+"hostname"], Port: atoi(p[sp+"port"]),
			Username: p[sp+"username"], SocketType: atoi(p[sp+"try_ssl"]),
			AuthMethod: atoi(p[sp+"authMethod"]),
		}
	}

	for uuid := range calendarKeys(p) {
		cp := "calendar.registry." + uuid + "."
		cfg.Calendars = append(cfg.Calendars, CalendarRef{
			UUID: uuid, Name: p[cp+"name"], Type: p[cp+"type"], URI: p[cp+"uri"],
		})
	}
	return cfg
}

// smtpKeys / calendarKeys discover instance keys from the flat pref map.
func smtpKeys(p map[string]string) map[string]bool {
	keys := map[string]bool{}
	for k := range p {
		if rest, ok := strings.CutPrefix(k, "mail.smtpserver."); ok {
			keys[strings.SplitN(rest, ".", 2)[0]] = true
		}
	}
	return keys
}

func calendarKeys(p map[string]string) map[string]bool {
	keys := map[string]bool{}
	for k := range p {
		if rest, ok := strings.CutPrefix(k, "calendar.registry."); ok {
			keys[strings.SplitN(rest, ".", 2)[0]] = true
		}
	}
	return keys
}

func resolveDir(abs, rel, profileDir string) string {
	if abs != "" {
		return abs
	}
	if r, ok := strings.CutPrefix(rel, "[ProfD]"); ok {
		return filepath.Join(profileDir, filepath.FromSlash(r))
	}
	return rel
}

// discoverProfile returns THUNDERBIRD_PROFILE or the default profile from profiles.ini.
func discoverProfile() (string, error) {
	if p := os.Getenv("THUNDERBIRD_PROFILE"); p != "" {
		return p, nil
	}
	root, err := thunderbirdRoot()
	if err != nil {
		return "", err
	}
	return defaultProfileFromINI(filepath.Join(root, "profiles.ini"), root)
}

func loadProfile(profileDir string) (*Config, error) {
	f, err := os.Open(filepath.Join(profileDir, "prefs.js"))
	if err != nil {
		return nil, fmt.Errorf("opening prefs.js: %w", err)
	}
	defer f.Close()
	prefs, err := parsePrefs(f)
	if err != nil {
		return nil, err
	}
	return buildConfig(prefs, profileDir), nil
}
```

- [ ] **Step 6: Implement `thunderbirdRoot` and `defaultProfileFromINI`** (append to profile.go)

```go
// thunderbirdRoot returns the platform Thunderbird data directory.
func thunderbirdRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	candidates := []string{
		filepath.Join(home, ".thunderbird"),
		filepath.Join(home, ".mozilla-thunderbird"),
		filepath.Join(home, "snap", "thunderbird", "common", ".thunderbird"),
		filepath.Join(home, ".var", "app", "org.mozilla.Thunderbird", ".thunderbird"),
		filepath.Join(home, "Library", "Thunderbird"),                                    // macOS
		filepath.Join(os.Getenv("APPDATA"), "Thunderbird"),                               // Windows
	}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(c, "profiles.ini")); err == nil {
			return c, nil
		}
	}
	return "", fmt.Errorf("no Thunderbird profiles.ini found; set THUNDERBIRD_PROFILE")
}

// defaultProfileFromINI picks the default profile path from profiles.ini.
// Honors [Install*] Default=, else the [Profile*] with Default=1, else the first.
func defaultProfileFromINI(iniPath, root string) (string, error) {
	f, err := os.Open(iniPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	type section struct{ kv map[string]string }
	var sections []section
	var cur *section
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "[") {
			sections = append(sections, section{kv: map[string]string{}})
			cur = &sections[len(sections)-1]
			continue
		}
		if cur == nil || !strings.Contains(line, "=") {
			continue
		}
		k, v, _ := strings.Cut(line, "=")
		cur.kv[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}

	resolve := func(path, rel string) string {
		if rel == "1" || filepath.IsAbs(path) {
			return path
		}
		return filepath.Join(root, path)
	}
	// Install section wins.
	for _, s := range sections {
		if d := s.kv["Default"]; d != "" && s.kv["Path"] == "" {
			return filepath.Join(root, d), nil
		}
	}
	var first string
	for _, s := range sections {
		if p := s.kv["Path"]; p != "" {
			full := resolve(p, s.kv["IsRelative"])
			if first == "" {
				first = full
			}
			if s.kv["Default"] == "1" {
				return full, nil
			}
		}
	}
	if first == "" {
		return "", fmt.Errorf("no profile found in %s", iniPath)
	}
	return first, nil
}
```

- [ ] **Step 7: Run tests to verify pass**

Run: `go test ./thunderbird/ && CGO_ENABLED=0 go build ./thunderbird/`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add thunderbird/ && git commit -m "feat(thunderbird): profile discovery and prefs.js parsing"
```

---

## Task 3: NSS credential decryption (key4.db + logins.json)

Decrypts the **local user's own** stored IMAP/SMTP passwords. Handles the modern PBES2 (PBKDF2-HMAC-SHA256 + AES-256-CBC) master-key path and the legacy 3DES path; logins.json entries are always 3DES-CBC with a per-entry IV. Detects a master-password profile via the `password-check` known plaintext and fails with a clear message.

**Files:**
- Create: `thunderbird/secrets.go`, `thunderbird/secrets_test.go`, `thunderbird/testdata/nss/key4.db`, `thunderbird/testdata/nss/logins.json`

**Interfaces:**
- Consumes: `modernc.org/sqlite`, `golang.org/x/crypto/pbkdf2`, `crypto/aes`, `crypto/des`, `crypto/cipher`, `crypto/sha1`, `encoding/asn1`.
- Produces:
  - `type Credentials struct{ byHostUser map[string]string }`
  - `func LoadCredentials(profileDir string) (*Credentials, error)` — returns `ErrMasterPassword` if a master password blocks decryption.
  - `func (c *Credentials) Lookup(host, user string) (string, bool)`
  - `var ErrMasterPassword = errors.New(...)`
  - internal, unit-tested: `func deriveKeyPBES2(globalSalt, entrySalt []byte, iterations int) []byte`, `func aesCBCDecrypt(key, iv, ct []byte) ([]byte, error)`, `func des3CBCDecrypt(key, iv, ct []byte) ([]byte, error)`, `func pkcs7Unpad(b []byte, blockSize int) ([]byte, error)`.

- [ ] **Step 1: Generate the NSS fixture** (one-time; commit the output)

Run this to create a password-less test profile with a known login. It needs Python 3 (available on the dev box). Save as `scratchpad/gen_nss.py` and run it:

```python
# Generates testdata/nss/{key4.db,logins.json} with NO master password,
# containing one login: imap://imap.example.com  alice@example.com / s3cret-imap
# Uses the system NSS via `certutil`/`pk12util`? No — use the `nss` python? Simplest:
# generate with a throwaway Thunderbird/Firefox is unreliable in CI. Instead we
# assert the crypto primitives against firepwd's published vectors (Step 2a) AND
# ship a fixture produced once on this machine. To produce the fixture, run a real
# Thunderbird against an empty profile dir, add an account with the password above,
# close it, and copy key4.db + logins.json into testdata/nss/. Document that here.
```

Because a deterministic pure-Python generator for `key4.db` is impractical, split verification:
- **Primitive vectors** (Step 2a) prove the math with hardcoded byte vectors — no fixture needed, runs in CI.
- **End-to-end** (Step 2b) runs only when `testdata/nss/key4.db` exists (generated once from a real Thunderbird with an empty master password and committed); it is `Skip`ped otherwise so CI stays green without shipping a profile if that is undesirable. Generate it by: create empty dir, `THUNDERBIRD_PROFILE` it, add the IMAP account `alice@example.com` with password `s3cret-imap`, quit Thunderbird, copy `key4.db`+`logins.json` into `testdata/nss/`.

- [ ] **Step 2a: Write primitive-vector tests** — `thunderbird/secrets_test.go`

```go
package main

import (
	"bytes"
	"encoding/hex"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("pkcs7Unpad", func() {
	It("strips valid padding", func() {
		out, err := pkcs7Unpad([]byte("password-check\x02\x02"), 16)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(out)).To(Equal("password-check"))
	})
	It("rejects bad padding", func() {
		_, err := pkcs7Unpad([]byte{1, 2, 9}, 8)
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("deriveKeyPBES2", func() {
	It("matches a known PBKDF2-HMAC-SHA256 vector", func() {
		// RFC-style vector: password=SHA1(globalSalt||""), salt, iters -> 32-byte key.
		globalSalt, _ := hex.DecodeString("2b8ea3b3...") // REPLACE with vector captured from a real key4.db during fixture generation (Step 1), plus expected key.
		entrySalt, _ := hex.DecodeString("aabbccdd")
		key := deriveKeyPBES2(globalSalt, entrySalt, 10000)
		Expect(key).To(HaveLen(32))
		_ = bytes.Equal // placeholder until the captured expected vector is pasted in
	})
})
```

Note to implementer: when you generate the fixture in Step 1, also dump `globalSalt`, `entrySalt`, `iterations`, and the resulting derived key (add a temporary `fmt.Printf` in `LoadCredentials`), and paste real hex into the vector above so the test is deterministic and CI-safe. Remove the `_ = bytes.Equal` placeholder and assert `hex.EncodeToString(key)` equals the captured value.

- [ ] **Step 2b: Write the end-to-end test (skips without fixture)**

```go
var _ = Describe("LoadCredentials (fixture)", func() {
	It("decrypts the stored IMAP password", func() {
		if _, err := os.Stat("testdata/nss/key4.db"); err != nil {
			Skip("no NSS fixture committed")
		}
		creds, err := LoadCredentials("testdata/nss")
		Expect(err).NotTo(HaveOccurred())
		pw, ok := creds.Lookup("imap.example.com", "alice@example.com")
		Expect(ok).To(BeTrue())
		Expect(pw).To(Equal("s3cret-imap"))
	})
})
```

- [ ] **Step 3: Run to verify fail**

Run: `go test ./thunderbird/ -run TestThunderbird`
Expected: FAIL — `undefined: pkcs7Unpad`, etc.

- [ ] **Step 4: Implement `thunderbird/secrets.go`**

```go
package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/des"
	"crypto/sha1"
	"database/sql"
	"encoding/asn1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"golang.org/x/crypto/pbkdf2"
	_ "modernc.org/sqlite"
)

var ErrMasterPassword = errors.New("thunderbird profile is protected by a master password; credential decryption is not supported")

type Credentials struct {
	byHostUser map[string]string // "host\x00user" -> password
}

func credKey(host, user string) string { return host + "\x00" + user }

func (c *Credentials) Lookup(host, user string) (string, bool) {
	if c == nil {
		return "", false
	}
	pw, ok := c.byHostUser[credKey(host, user)]
	return pw, ok
}

// ---- ASN.1 shapes ----

type algorithmIdentifier struct {
	Algorithm  asn1.ObjectIdentifier
	Parameters asn1.RawValue
}
type encryptedItem struct { // outer: { AlgorithmIdentifier, OCTET STRING }
	Algo   algorithmIdentifier
	Cipher []byte
}
type pbeParam3DES struct { // legacy params: { salt, iterations }
	EntrySalt  []byte
	Iterations int
}
type pbkdf2Params struct {
	EntrySalt  []byte
	Iterations int
	KeyLength  int
	Prf        algorithmIdentifier
}
type pbes2Params struct {
	KeyDerivation algorithmIdentifier // PBKDF2 + params
	Encryption    algorithmIdentifier // aes256-CBC + IV OCTET STRING
}
type loginASN1 struct { // logins.json entry: { keyID, {des-ede3-cbc, iv}, cipher }
	KeyID  []byte
	Algo   algorithmIdentifier
	Cipher []byte
}

var (
	oidPBES2      = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 5, 13}
	oid3DESLegacy = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 12, 5, 1, 3}
)

func pkcs7Unpad(b []byte, blockSize int) ([]byte, error) {
	if len(b) == 0 || len(b)%blockSize != 0 {
		return nil, fmt.Errorf("invalid padded length %d", len(b))
	}
	n := int(b[len(b)-1])
	if n == 0 || n > blockSize || n > len(b) {
		return nil, fmt.Errorf("invalid padding byte %d", n)
	}
	for _, c := range b[len(b)-n:] {
		if int(c) != n {
			return nil, errors.New("invalid padding")
		}
	}
	return b[:len(b)-n], nil
}

func deriveKeyPBES2(globalSalt, entrySalt []byte, iterations int) []byte {
	pw := sha1.Sum(append(append([]byte{}, globalSalt...))) // SHA1(globalSalt || "")  (empty master password)
	return pbkdf2.Key(pw[:], entrySalt, iterations, 32, sha1Sha256)
}
```

Correction for the SHA1 step (empty master password) — implement it explicitly and use SHA-256 for PBKDF2:

```go
import "crypto/sha256"

func sha256New() { /* referenced via pbkdf2.Key below */ }

func deriveKeyPBES2(globalSalt, entrySalt []byte, iterations int) []byte {
	h := sha1.New()
	h.Write(globalSalt) // master password is empty, so nothing appended
	pw := h.Sum(nil)    // 20 bytes
	return pbkdf2.Key(pw, entrySalt, iterations, 32, sha256.New)
}

func aesCBCDecrypt(key, iv, ct []byte) ([]byte, error) {
	blk, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(ct)%blk.BlockSize() != 0 {
		return nil, errors.New("aes: ciphertext not a multiple of block size")
	}
	out := make([]byte, len(ct))
	cipher.NewCBCDecrypter(blk, iv).CryptBlocks(out, ct)
	return out, nil
}

func des3CBCDecrypt(key, iv, ct []byte) ([]byte, error) {
	blk, err := des.NewTripleDESCipher(key)
	if err != nil {
		return nil, err
	}
	if len(ct)%blk.BlockSize() != 0 {
		return nil, errors.New("3des: ciphertext not a multiple of block size")
	}
	out := make([]byte, len(ct))
	cipher.NewCBCDecrypter(blk, iv).CryptBlocks(out, ct)
	return out, nil
}
```

(Delete the earlier broken `deriveKeyPBES2`/`sha1Sha256`/`sha256New` stubs — they are placeholders shown only to be replaced by the corrected versions above.)

- [ ] **Step 5: Implement master-key recovery and login decryption** (append to secrets.go)

```go
// decodeItem unwraps an outer {AlgorithmIdentifier, cipher} and returns the
// decrypted plaintext (still padded) using the recovered-or-derived key path.
func decryptPBEItem(globalSalt, raw []byte) ([]byte, error) {
	var item encryptedItem
	if _, err := asn1.Unmarshal(raw, &item); err != nil {
		return nil, err
	}
	switch {
	case item.Algo.Algorithm.Equal(oidPBES2):
		var p pbes2Params
		if _, err := asn1.Unmarshal(item.Algo.Parameters.FullBytes, &p); err != nil {
			return nil, err
		}
		var kdf pbkdf2Params
		if _, err := asn1.Unmarshal(p.KeyDerivation.Parameters.FullBytes, &kdf); err != nil {
			return nil, err
		}
		var iv14 []byte
		if _, err := asn1.Unmarshal(p.Encryption.Parameters.FullBytes, &iv14); err != nil {
			return nil, err
		}
		key := deriveKeyPBES2(globalSalt, kdf.EntrySalt, kdf.Iterations)
		iv := append([]byte{0x04, 0x0e}, iv14...) // NSS prefixes DER tag+len to make 16 bytes
		return aesCBCDecrypt(key, iv, item.Cipher)
	case item.Algo.Algorithm.Equal(oid3DESLegacy):
		var pp pbeParam3DES
		if _, err := asn1.Unmarshal(item.Algo.Parameters.FullBytes, &pp); err != nil {
			return nil, err
		}
		key, iv := legacy3DESKeyIV(globalSalt, pp.EntrySalt)
		return des3CBCDecrypt(key, iv, item.Cipher)
	default:
		return nil, fmt.Errorf("unsupported PBE algorithm %v", item.Algo.Algorithm)
	}
}

// legacy3DESKeyIV implements the PKCS#12 SHA1/HMAC derivation (firepwd decrypt3DES).
func legacy3DESKeyIV(globalSalt, entrySalt []byte) (key, iv []byte) {
	hp := sha1.Sum(append(append([]byte{}, globalSalt...)))
	pes := make([]byte, 20)
	copy(pes, entrySalt)
	chpArr := sha1.Sum(append(append([]byte{}, hp[:]...), entrySalt...))
	chp := chpArr[:]
	k1 := hmacSHA1(chp, append(append([]byte{}, pes...), entrySalt...))
	tk := hmacSHA1(chp, pes)
	k2 := hmacSHA1(chp, append(append([]byte{}, tk...), entrySalt...))
	k := append(append([]byte{}, k1...), k2...) // 40 bytes
	return k[:24], k[len(k)-8:]
}

// recoverMasterKey reads key4.db, verifies password-check, and returns the 24-byte
// 3DES master key from nssPrivate.a11.
func recoverMasterKey(key4Path string) ([]byte, error) {
	db, err := sql.Open("sqlite", "file:"+key4Path+"?mode=ro&immutable=1")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var globalSalt, item2 []byte
	err = db.QueryRow(`SELECT item1, item2 FROM metaData WHERE id = 'password'`).Scan(&globalSalt, &item2)
	if err != nil {
		return nil, fmt.Errorf("reading metaData: %w", err)
	}
	check, err := decryptPBEItem(globalSalt, item2)
	if err == nil {
		check, err = pkcs7Unpad(check, blockSizeFor(item2))
	}
	if err != nil || !bytes.Equal(check, []byte("password-check")) {
		return nil, ErrMasterPassword
	}

	var a11 []byte
	if err := db.QueryRow(`SELECT a11 FROM nssPrivate WHERE a11 IS NOT NULL LIMIT 1`).Scan(&a11); err != nil {
		return nil, fmt.Errorf("reading nssPrivate: %w", err)
	}
	dec, err := decryptPBEItem(globalSalt, a11)
	if err != nil {
		return nil, err
	}
	// The master key blob is an ASN.1 { OID, OCTET STRING key } after unpad; but
	// in practice NSS stores the 24-byte 3DES key as the last 24 bytes. Unpad then
	// take the trailing 24 bytes.
	dec, err = pkcs7Unpad(dec, blockSizeFor(a11))
	if err != nil {
		return nil, err
	}
	if len(dec) < 24 {
		return nil, fmt.Errorf("master key too short: %d", len(dec))
	}
	return dec[len(dec)-24:], nil
}

func blockSizeFor(raw []byte) int {
	var item encryptedItem
	if _, err := asn1.Unmarshal(raw, &item); err == nil && item.Algo.Algorithm.Equal(oidPBES2) {
		return 16
	}
	return 8
}

// LoadCredentials decrypts logins.json into a host/user -> password map.
func LoadCredentials(profileDir string) (*Credentials, error) {
	masterKey, err := recoverMasterKey(filepath.Join(profileDir, "key4.db"))
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(profileDir, "logins.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return &Credentials{byHostUser: map[string]string{}}, nil
		}
		return nil, err
	}
	var file struct {
		Logins []struct {
			Hostname          string `json:"hostname"`
			EncryptedUsername string `json:"encryptedUsername"`
			EncryptedPassword string `json:"encryptedPassword"`
		} `json:"logins"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, err
	}
	creds := &Credentials{byHostUser: map[string]string{}}
	for _, l := range file.Logins {
		user, err1 := decryptLogin(masterKey, l.EncryptedUsername)
		pass, err2 := decryptLogin(masterKey, l.EncryptedPassword)
		if err1 != nil || err2 != nil {
			continue
		}
		host := hostOnly(l.Hostname) // "imap://imap.example.com" -> "imap.example.com"
		creds.byHostUser[credKey(host, user)] = pass
	}
	return creds, nil
}

func decryptLogin(masterKey []byte, b64 string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", err
	}
	var l loginASN1
	if _, err := asn1.Unmarshal(raw, &l); err != nil {
		return "", err
	}
	var iv []byte
	if _, err := asn1.Unmarshal(l.Algo.Parameters.FullBytes, &iv); err != nil {
		return "", err
	}
	pt, err := des3CBCDecrypt(masterKey, iv, l.Cipher)
	if err != nil {
		return "", err
	}
	pt, err = pkcs7Unpad(pt, 8)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}

func hostOnly(raw string) string {
	if u, err := url.Parse(raw); err == nil && u.Host != "" {
		return u.Host
	}
	return raw
}
```

- [ ] **Step 6: Add `hmacSHA1` helper** (append to secrets.go)

```go
import "crypto/hmac"

func hmacSHA1(key, msg []byte) []byte {
	m := hmac.New(sha1.New, key)
	m.Write(msg)
	return m.Sum(nil)
}
```

- [ ] **Step 7: Add deps and run tests**

Run:
```bash
cd /home/mudler/_git/mcps
go get modernc.org/sqlite@v1.54.0 golang.org/x/crypto
go mod tidy
go test ./thunderbird/ && CGO_ENABLED=0 go build ./thunderbird/
```
Expected: PASS (end-to-end spec Skips unless the fixture exists). If the primitive vector in Step 2a still has the placeholder salt, generate the fixture per Step 1 and paste the real captured vector before this passes.

- [ ] **Step 8: Commit**

```bash
git add thunderbird/ go.mod go.sum && git commit -m "feat(thunderbird): NSS key4.db/logins.json credential decryption"
```

---

## Task 4: Gloda search index queries

**Files:**
- Create: `thunderbird/gloda.go`, `thunderbird/gloda_test.go`, `thunderbird/testdata/gloda_seed.sql`

**Interfaces:**
- Consumes: `modernc.org/sqlite`, `types.go`.
- Produces:
  - Add to types.go: `type MessageRef struct{ FolderURI string; MessageKey uint32 }` with `func (r MessageRef) String() string` (`<uri>#<key>`) and `func ParseMessageRef(s string) (MessageRef, error)`.
  - Add: `type MessageSummary struct{ Ref, Subject, Author, Snippet string; Date time.Time; Unread bool }`
  - `type Gloda struct{ db *sql.DB }`, `func OpenGloda(profileDir string) (*Gloda, error)` (nil-safe: returns `(nil, nil)` if the db is absent), `func (g *Gloda) Close() error`.
  - `type SearchQuery struct{ Text, From, To, Subject, FolderURI string; Since, Until time.Time; UnreadOnly bool; Limit, Offset int }`
  - `func (g *Gloda) Search(q SearchQuery) ([]MessageSummary, error)`
  - `func (g *Gloda) Recent(folderURI string, limit int) ([]MessageSummary, error)`

- [ ] **Step 1: Write the seed SQL** — `thunderbird/testdata/gloda_seed.sql`

Recreates the subset of the gloda schema the queries touch (real gloda uses an FTS3 `mozporter` table we cannot open; we read the shadow `messagesText_content` table, which we model directly here).

```sql
CREATE TABLE folderLocations (id INTEGER PRIMARY KEY, folderURI TEXT, name TEXT);
CREATE TABLE messages (
  id INTEGER PRIMARY KEY, folderID INTEGER, messageKey INTEGER,
  date INTEGER, headerMessageID TEXT, deleted INTEGER DEFAULT 0, jsonAttributes TEXT
);
CREATE TABLE messagesText_content (
  docid INTEGER PRIMARY KEY, c0body TEXT, c1subject TEXT,
  c2attachmentNames TEXT, c3author TEXT, c4recipients TEXT
);

INSERT INTO folderLocations VALUES (1, 'imap://alice@example.com/INBOX', 'INBOX');
INSERT INTO folderLocations VALUES (2, 'imap://alice@example.com/Archive', 'Archive');

-- date is microseconds since epoch (PRTime). 1700000000000000 = 2023-11-14.
INSERT INTO messages VALUES (10, 1, 101, 1700000000000000, 'msg-a@example.com', 0, NULL);
INSERT INTO messages VALUES (11, 1, 102, 1700100000000000, 'msg-b@example.com', 0, NULL);
INSERT INTO messages VALUES (12, 2, 103, 1699000000000000, 'msg-c@example.com', 0, NULL);
INSERT INTO messages VALUES (13, 1, 104, 1700200000000000, 'msg-d@example.com', 1, NULL); -- deleted
INSERT INTO messages VALUES (14, NULL, NULL, 1700300000000000, 'ghost@example.com', 0, NULL); -- ghost

INSERT INTO messagesText_content VALUES (10, 'quarterly report body', 'Quarterly report', '', 'bob@example.com', 'alice@example.com');
INSERT INTO messagesText_content VALUES (11, 'lunch tomorrow?', 'Lunch', '', 'carol@example.com', 'alice@example.com');
INSERT INTO messagesText_content VALUES (12, 'old archived note', 'Archive note', '', 'bob@example.com', 'alice@example.com');
INSERT INTO messagesText_content VALUES (13, 'deleted msg', 'Deleted', '', 'x@example.com', 'alice@example.com');
```

- [ ] **Step 2: Write failing tests** — `thunderbird/gloda_test.go`

```go
package main

import (
	"database/sql"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	_ "modernc.org/sqlite"
)

func buildGlodaFixture() *Gloda {
	tmp, err := os.CreateTemp("", "gloda-*.sqlite")
	Expect(err).NotTo(HaveOccurred())
	tmp.Close()
	seed, err := os.ReadFile("testdata/gloda_seed.sql")
	Expect(err).NotTo(HaveOccurred())
	db, err := sql.Open("sqlite", "file:"+tmp.Name())
	Expect(err).NotTo(HaveOccurred())
	_, err = db.Exec(string(seed))
	Expect(err).NotTo(HaveOccurred())
	db.Close()
	g, err := openGlodaAt(tmp.Name())
	Expect(err).NotTo(HaveOccurred())
	return g
}

var _ = Describe("Gloda.Search", func() {
	var g *Gloda
	BeforeEach(func() { g = buildGlodaFixture() })
	AfterEach(func() { g.Close() })

	It("finds by body text and excludes deleted and ghost rows", func() {
		res, err := g.Search(SearchQuery{Text: "report", Limit: 10})
		Expect(err).NotTo(HaveOccurred())
		Expect(res).To(HaveLen(1))
		Expect(res[0].Subject).To(Equal("Quarterly report"))
		Expect(res[0].Ref).To(Equal("imap://alice@example.com/INBOX#101"))
		Expect(res[0].Date.Year()).To(Equal(2023))
	})
	It("filters by folder", func() {
		res, err := g.Search(SearchQuery{FolderURI: "imap://alice@example.com/Archive", Limit: 10})
		Expect(err).NotTo(HaveOccurred())
		Expect(res).To(HaveLen(1))
		Expect(res[0].Subject).To(Equal("Archive note"))
	})
	It("filters by from", func() {
		res, err := g.Search(SearchQuery{From: "carol", Limit: 10})
		Expect(err).NotTo(HaveOccurred())
		Expect(res).To(HaveLen(1))
		Expect(res[0].Author).To(Equal("carol@example.com"))
	})
	It("orders recent newest-first and never returns deleted/ghost", func() {
		res, err := g.Recent("", 10)
		Expect(err).NotTo(HaveOccurred())
		Expect(res).To(HaveLen(3))
		Expect(res[0].Subject).To(Equal("Lunch")) // 1700100000000000 newest of the valid rows
	})
})
```

- [ ] **Step 3: Run to verify fail** — `go test ./thunderbird/ -run TestThunderbird` → FAIL undefined `Gloda`.

- [ ] **Step 4: Add ref types to types.go**

```go
import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type MessageRef struct {
	FolderURI  string
	MessageKey uint32
}

func (r MessageRef) String() string {
	return fmt.Sprintf("%s#%d", r.FolderURI, r.MessageKey)
}

func ParseMessageRef(s string) (MessageRef, error) {
	i := strings.LastIndex(s, "#")
	if i < 0 {
		return MessageRef{}, fmt.Errorf("invalid message_ref %q", s)
	}
	key, err := strconv.ParseUint(s[i+1:], 10, 32)
	if err != nil {
		return MessageRef{}, fmt.Errorf("invalid message_ref key in %q: %w", s, err)
	}
	return MessageRef{FolderURI: s[:i], MessageKey: uint32(key)}, nil
}

type MessageSummary struct {
	Ref     string    `json:"message_ref" jsonschema:"opaque reference: pass to get_message and mutation tools"`
	Subject string    `json:"subject"`
	Author  string    `json:"author"`
	Snippet string    `json:"snippet" jsonschema:"short body excerpt from the search index (may be truncated)"`
	Date    time.Time `json:"date"`
}
```

- [ ] **Step 5: Implement `thunderbird/gloda.go`**

```go
package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Gloda struct{ db *sql.DB }

// OpenGloda opens global-messages-db.sqlite read-only. Returns (nil, nil) when
// the index is absent so callers can degrade gracefully.
func OpenGloda(profileDir string) (*Gloda, error) {
	p := filepath.Join(profileDir, "global-messages-db.sqlite")
	if _, err := os.Stat(p); err != nil {
		return nil, nil
	}
	return openGlodaAt(p)
}

func openGlodaAt(path string) (*Gloda, error) {
	// mode=ro + a busy timeout so we coexist with a running Thunderbird's indexer.
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	return &Gloda{db: db}, nil
}

func (g *Gloda) Close() error {
	if g == nil {
		return nil
	}
	return g.db.Close()
}

type SearchQuery struct {
	Text, From, To, Subject, FolderURI string
	Since, Until                       time.Time
	Limit, Offset                      int
}

const glodaSelect = `
SELECT f.folderURI, m.messageKey, m.date, t.c1subject, t.c3author, t.c0body
FROM messagesText_content t
JOIN messages m ON m.id = t.docid
JOIN folderLocations f ON f.id = m.folderID
WHERE m.deleted = 0 AND m.folderID IS NOT NULL AND m.messageKey IS NOT NULL`

func (g *Gloda) Search(q SearchQuery) ([]MessageSummary, error) {
	if g == nil {
		return nil, fmt.Errorf("search index unavailable (gloda disabled or not built)")
	}
	sb := strings.Builder{}
	sb.WriteString(glodaSelect)
	var args []any
	like := func(col, val string) {
		if val != "" {
			sb.WriteString(" AND " + col + " LIKE ? ESCAPE '\\'")
			args = append(args, "%"+escapeLike(val)+"%")
		}
	}
	// Text matches body OR subject.
	if q.Text != "" {
		sb.WriteString(" AND (t.c0body LIKE ? ESCAPE '\\' OR t.c1subject LIKE ? ESCAPE '\\')")
		args = append(args, "%"+escapeLike(q.Text)+"%", "%"+escapeLike(q.Text)+"%")
	}
	like("t.c3author", q.From)
	like("t.c4recipients", q.To)
	like("t.c1subject", q.Subject)
	if q.FolderURI != "" {
		sb.WriteString(" AND f.folderURI = ?")
		args = append(args, q.FolderURI)
	}
	if !q.Since.IsZero() {
		sb.WriteString(" AND m.date >= ?")
		args = append(args, q.Since.UnixMicro())
	}
	if !q.Until.IsZero() {
		sb.WriteString(" AND m.date <= ?")
		args = append(args, q.Until.UnixMicro())
	}
	sb.WriteString(" ORDER BY m.date DESC LIMIT ? OFFSET ?")
	limit := q.Limit
	if limit <= 0 {
		limit = 20
	}
	args = append(args, limit, q.Offset)
	return g.queryRows(sb.String(), args...)
}

func (g *Gloda) Recent(folderURI string, limit int) ([]MessageSummary, error) {
	return g.Search(SearchQuery{FolderURI: folderURI, Limit: limit})
}

func (g *Gloda) queryRows(query string, args ...any) ([]MessageSummary, error) {
	rows, err := g.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MessageSummary
	for rows.Next() {
		var folderURI, subject, author, body string
		var key uint32
		var micros int64
		if err := rows.Scan(&folderURI, &key, &micros, &subject, &author, &body); err != nil {
			return nil, err
		}
		out = append(out, MessageSummary{
			Ref:     MessageRef{FolderURI: folderURI, MessageKey: key}.String(),
			Subject: subject, Author: author,
			Snippet: snippet(body, 200),
			Date:    time.UnixMicro(micros).UTC(),
		})
	}
	return out, rows.Err()
}

func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "%", `\%`)
	return strings.ReplaceAll(s, "_", `\_`)
}

func snippet(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}
```

- [ ] **Step 6: Run tests** — `go test ./thunderbird/ && CGO_ENABLED=0 go build ./thunderbird/` → PASS.

- [ ] **Step 7: Commit**

```bash
git add thunderbird/ && git commit -m "feat(thunderbird): gloda search index queries"
```

---

## Task 5: IMAP client — connect, fetch body, mutate, append

**Files:**
- Create: `thunderbird/imap.go`, `thunderbird/imap_test.go`

**Interfaces:**
- Consumes: `go-imap/v2`, `go-sasl`, `Config`, `Credentials`, `MessageRef`.
- Produces:
  - `type IMAP struct{ cfg *Config; creds *Credentials }`, `func NewIMAP(cfg *Config, creds *Credentials) *IMAP`.
  - `func (m *IMAP) accountForFolderURI(uri string) (*Account, error)` — matches an IMAP account by `imap://user@host` prefix.
  - `func (m *IMAP) connect(a *Account) (*imapclient.Client, error)` — DialTLS/DialStartTLS by SocketType, Login with looked-up creds. Returns a clear error if `a.AuthMethod == 10` (OAuth2 unsupported) or creds missing.
  - `func (m *IMAP) FetchBody(ref MessageRef) (raw []byte, err error)` — UID FETCH `BODY.PEEK[]`.
  - `func (m *IMAP) SetFlags(ref MessageRef, add, remove []imap.Flag) error`
  - `func (m *IMAP) Move(ref MessageRef, destFolder string) error`
  - `func (m *IMAP) Delete(ref MessageRef, permanent bool) error` — non-permanent = move to Trash (`\Deleted`+expunge only when permanent).
  - `func (m *IMAP) Append(folder string, raw []byte, flags []imap.Flag) error`
  - helper `folderNameFromURI(uri string) string` — path after host, URL-decoded (`imap://u@h/A/B` → `A/B`).

- [ ] **Step 1: Write the test with a fake server** — `thunderbird/imap_test.go`

The go-imap module ships an in-memory server for tests (`imapmemserver` + `imapserver`). Use it to exercise connect/fetch/flags/move without a network.

```go
package main

import (
	"net"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapserver"
	"github.com/emersion/go-imap/v2/imapserver/imapmemserver"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("IMAP", func() {
	var (
		ln   net.Listener
		srv  *imapserver.Server
		m    *IMAP
		acct Account
	)
	BeforeEach(func() {
		memServer := imapmemserver.New()
		user := imapmemserver.NewUser("alice@example.com", "s3cret-imap")
		user.Create("INBOX", nil)
		user.Create("Trash", nil)
		user.Create("Archive", nil)
		memServer.AddUser(user)
		srv = imapserver.New(&imapserver.Options{
			NewSession: func(*imapserver.Conn) (imapserver.Session, *imapserver.GreetingData, error) {
				return memServer.NewSession(), nil, nil
			},
			InsecureAuth: true,
		})
		var err error
		ln, err = net.Listen("tcp", "127.0.0.1:0")
		Expect(err).NotTo(HaveOccurred())
		go srv.Serve(ln)

		host, port, _ := net.SplitHostPort(ln.Addr().String())
		acct = Account{
			Type: "imap", Hostname: host, Port: atoi(port), SocketType: 0,
			Username: "alice@example.com",
			Identities: []Identity{{Email: "alice@example.com"}},
		}
		cfg := &Config{Accounts: []Account{acct}}
		creds := &Credentials{byHostUser: map[string]string{
			credKey(host, "alice@example.com"): "s3cret-imap",
		}}
		m = NewIMAP(cfg, creds)
	})
	AfterEach(func() { ln.Close() })

	It("appends and fetches a message body by UID", func() {
		raw := []byte("From: bob@example.com\r\nSubject: Hi\r\n\r\nbody text\r\n")
		Expect(m.Append("INBOX", raw, nil)).To(Succeed())
		// UID of the first appended message in a fresh mailbox is 1.
		ref := MessageRef{FolderURI: "imap://alice@example.com@" + acct.hostPort() + "/INBOX", MessageKey: 1}
		got, err := m.FetchBody(ref)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(got)).To(ContainSubstring("body text"))
	})
})
```

Note: the folder-URI-to-account matching in the fake uses the ephemeral host:port, so add a small helper `Account.hostPort()` returning `host:port` used only to build test URIs, and have `accountForFolderURI` match on hostname (ignore port and user, which is what real Thunderbird URIs encode). Keep `accountForFolderURI` matching by the account whose `Hostname` appears in the URI host component.

- [ ] **Step 2: Run to verify fail** — FAIL undefined `NewIMAP`.

- [ ] **Step 3: Implement `thunderbird/imap.go`**

```go
package main

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

type IMAP struct {
	cfg   *Config
	creds *Credentials
}

func NewIMAP(cfg *Config, creds *Credentials) *IMAP { return &IMAP{cfg: cfg, creds: creds} }

func (a Account) hostPort() string { return net.JoinHostPort(a.Hostname, strconv.Itoa(a.Port)) }

func (m *IMAP) accountForFolderURI(uri string) (*Account, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("bad folder uri %q: %w", uri, err)
	}
	host := u.Hostname()
	for i := range m.cfg.Accounts {
		a := &m.cfg.Accounts[i]
		if a.Type == "imap" && a.Hostname == host {
			return a, nil
		}
	}
	if strings.HasPrefix(uri, "mailbox://") {
		return nil, fmt.Errorf("folder %q is in a local (non-IMAP) account and is read-only", uri)
	}
	return nil, fmt.Errorf("no IMAP account matches folder %q", uri)
}

func (m *IMAP) connect(a *Account) (*imapclient.Client, error) {
	if a.AuthMethod == 10 {
		return nil, fmt.Errorf("account %s uses OAuth2, which is not supported in this version", a.Hostname)
	}
	pw, ok := m.creds.Lookup(a.Hostname, a.Username)
	if !ok {
		return nil, fmt.Errorf("no stored password for %s@%s", a.Username, a.Hostname)
	}
	addr := a.hostPort()
	opts := &imapclient.Options{TLSConfig: &tls.Config{ServerName: a.Hostname}}
	var (
		c   *imapclient.Client
		err error
	)
	switch a.SocketType {
	case 3: // SSL/TLS
		c, err = imapclient.DialTLS(addr, opts)
	case 2: // STARTTLS
		c, err = imapclient.DialStartTLS(addr, opts)
	default: // 0 plain (also used by the in-memory test server)
		c, err = imapclient.DialInsecure(addr, opts)
	}
	if err != nil {
		return nil, fmt.Errorf("connecting to %s: %w", addr, err)
	}
	if err := c.Login(a.Username, pw).Wait(); err != nil {
		c.Close()
		return nil, fmt.Errorf("IMAP login failed for %s: %w", a.Username, err)
	}
	return c, nil
}

func folderNameFromURI(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		return strings.TrimPrefix(uri, "/")
	}
	name, _ := url.PathUnescape(strings.TrimPrefix(u.Path, "/"))
	return name
}

// withFolder connects, selects the folder for ref, and runs fn with the client and UID set.
func (m *IMAP) withFolder(uri string, readOnly bool, fn func(c *imapclient.Client, mailbox string) error) error {
	a, err := m.accountForFolderURI(uri)
	if err != nil {
		return err
	}
	c, err := m.connect(a)
	if err != nil {
		return err
	}
	defer c.Close()
	mailbox := folderNameFromURI(uri)
	if _, err := c.Select(mailbox, &imap.SelectOptions{ReadOnly: readOnly}).Wait(); err != nil {
		return fmt.Errorf("selecting %s: %w", mailbox, err)
	}
	if err := fn(c, mailbox); err != nil {
		return err
	}
	return c.Logout().Wait()
}

func (m *IMAP) FetchBody(ref MessageRef) ([]byte, error) {
	var raw []byte
	err := m.withFolder(ref.FolderURI, true, func(c *imapclient.Client, _ string) error {
		uidSet := imap.UIDSetNum(imap.UID(ref.MessageKey))
		section := &imap.FetchItemBodySection{Peek: true} // whole body, no \Seen
		opts := &imap.FetchOptions{BodySection: []*imap.FetchItemBodySection{section}}
		msgs, err := c.Fetch(uidSet, opts).Collect()
		if err != nil {
			return err
		}
		if len(msgs) == 0 {
			return fmt.Errorf("message UID %d not found in %s", ref.MessageKey, ref.FolderURI)
		}
		raw = msgs[0].FindBodySection(section)
		return nil
	})
	return raw, err
}

func (m *IMAP) SetFlags(ref MessageRef, add, remove []imap.Flag) error {
	return m.withFolder(ref.FolderURI, false, func(c *imapclient.Client, _ string) error {
		uidSet := imap.UIDSetNum(imap.UID(ref.MessageKey))
		store := func(op imap.StoreFlagsOp, flags []imap.Flag) error {
			if len(flags) == 0 {
				return nil
			}
			cmd := c.Store(uidSet, &imap.StoreFlags{Op: op, Silent: true, Flags: flags}, nil)
			return cmd.Close()
		}
		if err := store(imap.StoreFlagsAdd, add); err != nil {
			return err
		}
		return store(imap.StoreFlagsDel, remove)
	})
}

func (m *IMAP) Move(ref MessageRef, destFolder string) error {
	return m.withFolder(ref.FolderURI, false, func(c *imapclient.Client, _ string) error {
		uidSet := imap.UIDSetNum(imap.UID(ref.MessageKey))
		_, err := c.Move(uidSet, destFolder).Wait()
		return err
	})
}

func (m *IMAP) Delete(ref MessageRef, permanent bool) error {
	if !permanent {
		return m.Move(ref, "Trash")
	}
	return m.withFolder(ref.FolderURI, false, func(c *imapclient.Client, _ string) error {
		uidSet := imap.UIDSetNum(imap.UID(ref.MessageKey))
		if err := c.Store(uidSet, &imap.StoreFlags{Op: imap.StoreFlagsAdd, Silent: true, Flags: []imap.Flag{imap.FlagDeleted}}, nil).Close(); err != nil {
			return err
		}
		return c.Expunge().Close()
	})
}

func (m *IMAP) Append(folder string, raw []byte, flags []imap.Flag) error {
	// Append needs an account; resolve from the first IMAP account if folder is a bare name (tests),
	// else from the folder URI.
	a, err := m.appendAccount(folder)
	if err != nil {
		return err
	}
	c, err := m.connect(a)
	if err != nil {
		return err
	}
	defer c.Close()
	mailbox := folder
	if strings.Contains(folder, "://") {
		mailbox = folderNameFromURI(folder)
	}
	cmd := c.Append(mailbox, int64(len(raw)), &imap.AppendOptions{Flags: flags})
	if _, err := cmd.Write(raw); err != nil {
		return err
	}
	if err := cmd.Close(); err != nil {
		return err
	}
	if _, err := cmd.Wait(); err != nil {
		return err
	}
	return c.Logout().Wait()
}

func (m *IMAP) appendAccount(folder string) (*Account, error) {
	if strings.Contains(folder, "://") {
		return m.accountForFolderURI(folder)
	}
	for i := range m.cfg.Accounts {
		if m.cfg.Accounts[i].Type == "imap" {
			return &m.cfg.Accounts[i], nil
		}
	}
	return nil, fmt.Errorf("no IMAP account available")
}
```

- [ ] **Step 4: Add deps**

Run:
```bash
go get github.com/emersion/go-imap/v2@v2.0.0-beta.8
go mod tidy
```

- [ ] **Step 5: Run tests** — `go test ./thunderbird/ && CGO_ENABLED=0 go build ./thunderbird/` → PASS.

If the memory server's `NewUser`/`Create`/session wiring differs from the sketch, adjust to the actual `imapmemserver` API (verified present in v2.0.0-beta.8); the assertions on `Append`+`FetchBody` are what matter.

- [ ] **Step 6: Commit**

```bash
git add thunderbird/ go.mod go.sum && git commit -m "feat(thunderbird): IMAP fetch, flags, move, delete, append"
```

---

## Task 6: SMTP send + message building (send/reply/forward)

**Files:**
- Create: `thunderbird/smtp.go`, `thunderbird/smtp_test.go`

**Interfaces:**
- Consumes: `go-message/mail`, `go-smtp`, `go-sasl`, `Config`, `Credentials`, `IMAP` (to fetch the original for reply/forward).
- Produces:
  - `type OutgoingMessage struct{ From, To, Cc, Bcc []string; Subject, BodyText string; InReplyTo, References string }`
  - `func BuildRFC822(msg OutgoingMessage) ([]byte, error)`
  - `type Sender struct{ cfg *Config; creds *Credentials }`, `func NewSender(cfg, creds) *Sender`
  - `func (s *Sender) smtpForIdentity(fromEmail string) (*SMTPServer, *Identity, *Account, error)`
  - `func (s *Sender) Send(msg OutgoingMessage) error`
  - `func quoteForReply(orig MessageDetail) string`, `func replyHeaders(orig MessageDetail) (inReplyTo, references string)`

- [ ] **Step 1: Write tests** — `thunderbird/smtp_test.go`

```go
package main

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("BuildRFC822", func() {
	It("produces a well-formed message with headers and body", func() {
		raw, err := BuildRFC822(OutgoingMessage{
			From: []string{"alice@example.com"}, To: []string{"bob@example.com"},
			Subject: "Hello", BodyText: "hi there", InReplyTo: "<msg-a@example.com>",
		})
		Expect(err).NotTo(HaveOccurred())
		s := string(raw)
		Expect(s).To(ContainSubstring("From: <alice@example.com>"))
		Expect(s).To(ContainSubstring("To: <bob@example.com>"))
		Expect(s).To(ContainSubstring("Subject: Hello"))
		Expect(s).To(ContainSubstring("In-Reply-To: <msg-a@example.com>"))
		Expect(s).To(ContainSubstring("hi there"))
	})
})

var _ = Describe("smtpForIdentity", func() {
	It("resolves the SMTP server for a From address", func() {
		cfg := &Config{
			Accounts: []Account{{Type: "imap", Identities: []Identity{{Email: "alice@example.com", SMTPKey: "smtp1"}}}},
			SMTP:     map[string]SMTPServer{"smtp1": {Hostname: "smtp.example.com", Port: 587, SocketType: 2, Username: "alice@example.com"}},
		}
		s := NewSender(cfg, &Credentials{})
		srv, id, _, err := s.smtpForIdentity("alice@example.com")
		Expect(err).NotTo(HaveOccurred())
		Expect(srv.Hostname).To(Equal("smtp.example.com"))
		Expect(id.Email).To(Equal("alice@example.com"))
	})
	It("errors on an unknown From", func() {
		s := NewSender(&Config{}, &Credentials{})
		_, _, _, err := s.smtpForIdentity("nobody@nowhere")
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("quoteForReply", func() {
	It("prefixes each line with '> '", func() {
		q := quoteForReply(MessageDetail{Author: "bob@example.com", BodyText: "line1\nline2"})
		Expect(q).To(ContainSubstring("> line1"))
		Expect(q).To(ContainSubstring("> line2"))
		Expect(q).To(ContainSubstring("bob@example.com"))
	})
})
```

- [ ] **Step 2: Add `MessageDetail` to types.go**

```go
type MessageDetail struct {
	Ref         string       `json:"message_ref"`
	Subject     string       `json:"subject"`
	Author      string       `json:"author"`
	To          []string     `json:"to"`
	Cc          []string     `json:"cc"`
	Date        time.Time    `json:"date"`
	MessageID   string       `json:"message_id"`
	References  string       `json:"references"`
	BodyText    string       `json:"body_text"`
	BodyHTML    string       `json:"body_html,omitempty"`
	Attachments []Attachment `json:"attachments,omitempty"`
}

type Attachment struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int    `json:"size"`
}
```

- [ ] **Step 3: Run to verify fail** — FAIL undefined `BuildRFC822`.

- [ ] **Step 4: Implement `thunderbird/smtp.go`**

```go
package main

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"net"
	"net/mail"
	"strconv"
	"strings"
	"time"

	gomail "github.com/emersion/go-message/mail"
	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
)

type OutgoingMessage struct {
	From, To, Cc, Bcc  []string
	Subject, BodyText  string
	InReplyTo          string
	References         string
}

func addrs(list []string) []*gomail.Address {
	out := make([]*gomail.Address, 0, len(list))
	for _, a := range list {
		out = append(out, &gomail.Address{Address: strings.TrimSpace(a)})
	}
	return out
}

func BuildRFC822(msg OutgoingMessage) ([]byte, error) {
	var h gomail.Header
	h.SetAddressList("From", addrs(msg.From))
	h.SetAddressList("To", addrs(msg.To))
	if len(msg.Cc) > 0 {
		h.SetAddressList("Cc", addrs(msg.Cc))
	}
	h.SetSubject(msg.Subject)
	h.SetDate(fixedNow())
	if msg.InReplyTo != "" {
		h.Set("In-Reply-To", msg.InReplyTo)
	}
	if msg.References != "" {
		h.Set("References", msg.References)
	}

	var buf bytes.Buffer
	mw, err := gomail.CreateWriter(&buf, h)
	if err != nil {
		return nil, err
	}
	var ih gomail.InlineHeader
	ih.Set("Content-Type", "text/plain; charset=utf-8")
	iw, err := mw.CreateSingleInline(ih)
	if err != nil {
		return nil, err
	}
	if _, err := iw.Write([]byte(msg.BodyText)); err != nil {
		return nil, err
	}
	iw.Close()
	mw.Close()
	return buf.Bytes(), nil
}

// fixedNow is overridable in tests; production uses time.Now.
var fixedNow = time.Now

type Sender struct {
	cfg   *Config
	creds *Credentials
}

func NewSender(cfg *Config, creds *Credentials) *Sender { return &Sender{cfg: cfg, creds: creds} }

func (s *Sender) smtpForIdentity(fromEmail string) (*SMTPServer, *Identity, *Account, error) {
	for i := range s.cfg.Accounts {
		a := &s.cfg.Accounts[i]
		for j := range a.Identities {
			id := &a.Identities[j]
			if strings.EqualFold(id.Email, fromEmail) {
				key := id.SMTPKey
				if key == "" {
					return nil, nil, nil, fmt.Errorf("identity %s has no SMTP server configured", fromEmail)
				}
				srv, ok := s.cfg.SMTP[key]
				if !ok {
					return nil, nil, nil, fmt.Errorf("SMTP server %q not found for %s", key, fromEmail)
				}
				return &srv, id, a, nil
			}
		}
	}
	return nil, nil, nil, fmt.Errorf("no identity matches From address %q", fromEmail)
}

func (s *Sender) Send(msg OutgoingMessage) error {
	if len(msg.From) == 0 {
		return fmt.Errorf("From is required")
	}
	srv, _, _, err := s.smtpForIdentity(msg.From[0])
	if err != nil {
		return err
	}
	if srv.AuthMethod == 10 {
		return fmt.Errorf("SMTP server %s uses OAuth2, which is not supported in this version", srv.Hostname)
	}
	pw, ok := s.creds.Lookup(srv.Hostname, srv.Username)
	if !ok {
		return fmt.Errorf("no stored password for SMTP %s@%s", srv.Username, srv.Hostname)
	}
	raw, err := BuildRFC822(msg)
	if err != nil {
		return err
	}
	rcpts := append(append(append([]string{}, msg.To...), msg.Cc...), msg.Bcc...)
	auth := sasl.NewPlainClient("", srv.Username, pw)
	addr := net.JoinHostPort(srv.Hostname, strconv.Itoa(srv.Port))
	tlsConf := &tls.Config{ServerName: srv.Hostname}

	if srv.SocketType == 3 { // implicit TLS
		return smtp.SendMailTLS(addr, auth, msg.From[0], rcpts, bytes.NewReader(raw))
	}
	// STARTTLS (SocketType 2) — go-smtp SendMail does STARTTLS then auth.
	c, err := smtp.DialStartTLS(addr, tlsConf)
	if err != nil {
		return err
	}
	defer c.Close()
	if err := c.Auth(auth); err != nil {
		return err
	}
	if err := c.SendMail(msg.From[0], rcpts, bytes.NewReader(raw)); err != nil {
		return err
	}
	return c.Quit()
}

func quoteForReply(orig MessageDetail) string {
	var b strings.Builder
	fmt.Fprintf(&b, "On %s, %s wrote:\n", orig.Date.Format("2006-01-02 15:04"), orig.Author)
	for _, line := range strings.Split(orig.BodyText, "\n") {
		b.WriteString("> " + line + "\n")
	}
	return b.String()
}

func replyHeaders(orig MessageDetail) (inReplyTo, references string) {
	inReplyTo = orig.MessageID
	references = strings.TrimSpace(orig.References + " " + orig.MessageID)
	return
}

var _ = mail.ParseAddress // keep net/mail import if unused elsewhere
```

- [ ] **Step 5: Add deps**

```bash
go get github.com/emersion/go-smtp@v0.24.0 github.com/emersion/go-message@v0.18.2 github.com/emersion/go-sasl
go mod tidy
```

- [ ] **Step 6: Run tests** — `go test ./thunderbird/ && CGO_ENABLED=0 go build ./thunderbird/` → PASS. Remove the `var _ = mail.ParseAddress` line if `net/mail` ends up used or drop the import.

- [ ] **Step 7: Commit**

```bash
git add thunderbird/ go.mod go.sum && git commit -m "feat(thunderbird): SMTP send and message building"
```

---

## Task 7: Message parsing + local mbox reader

**Files:**
- Create: `thunderbird/message.go`, `thunderbird/mbox.go`, `thunderbird/message_test.go`

**Interfaces:**
- Consumes: `go-message/mail`.
- Produces:
  - `func ParseMessage(ref string, raw []byte) (MessageDetail, error)` — headers, text + html parts, attachment list (metadata only).
  - `func readMboxAt(mboxPath string, offset uint32) ([]byte, error)` — reads one message starting at a byte offset until the next `\nFrom ` separator or EOF.
  - `func localMboxPath(a *Account, folderURI string) string` — maps a `mailbox://` folder to its on-disk mbox file under `a.Directory`.

- [ ] **Step 1: Write tests** — `thunderbird/message_test.go`

```go
package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ParseMessage", func() {
	It("extracts headers, text body and attachment metadata", func() {
		raw := "From: bob@example.com\r\n" +
			"To: alice@example.com\r\n" +
			"Subject: Report\r\n" +
			"Message-ID: <m1@example.com>\r\n" +
			"Content-Type: text/plain; charset=utf-8\r\n\r\n" +
			"the body\r\n"
		d, err := ParseMessage("imap://h/INBOX#1", []byte(raw))
		Expect(err).NotTo(HaveOccurred())
		Expect(d.Subject).To(Equal("Report"))
		Expect(d.Author).To(ContainSubstring("bob@example.com"))
		Expect(d.MessageID).To(Equal("<m1@example.com>"))
		Expect(d.BodyText).To(ContainSubstring("the body"))
	})
})

var _ = Describe("readMboxAt", func() {
	It("reads a single message at an offset", func() {
		dir := GinkgoT().TempDir()
		p := filepath.Join(dir, "INBOX")
		content := "From - Mon Jan 1\r\nSubject: one\r\n\r\nbody one\r\n" +
			"From - Mon Jan 2\r\nSubject: two\r\n\r\nbody two\r\n"
		Expect(os.WriteFile(p, []byte(content), 0o600)).To(Succeed())
		off := uint32(len("From - Mon Jan 1\r\nSubject: one\r\n\r\nbody one\r\n"))
		raw, err := readMboxAt(p, off)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(raw)).To(ContainSubstring("body two"))
		Expect(string(raw)).NotTo(ContainSubstring("body one"))
	})
})
```

- [ ] **Step 2: Run to verify fail** — FAIL undefined `ParseMessage`.

- [ ] **Step 3: Implement `thunderbird/message.go`**

```go
package main

import (
	"bytes"
	"io"
	"strings"

	gomail "github.com/emersion/go-message/mail"
)

func ParseMessage(ref string, raw []byte) (MessageDetail, error) {
	d := MessageDetail{Ref: ref}
	mr, err := gomail.CreateReader(bytes.NewReader(raw))
	if err != nil {
		return d, err
	}
	h := mr.Header
	d.Subject, _ = h.Subject()
	d.Date, _ = h.Date()
	d.MessageID = h.Get("Message-ID")
	d.References = h.Get("References")
	if from, _ := h.AddressList("From"); len(from) > 0 {
		d.Author = from[0].String()
	}
	for _, a := range mustAddrs(h, "To") {
		d.To = append(d.To, a.String())
	}
	for _, a := range mustAddrs(h, "Cc") {
		d.Cc = append(d.Cc, a.String())
	}
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return d, err
		}
		switch ph := p.Header.(type) {
		case *gomail.InlineHeader:
			ct, _, _ := ph.ContentType()
			body, _ := io.ReadAll(p.Body)
			if strings.EqualFold(ct, "text/html") {
				d.BodyHTML = string(body)
			} else {
				d.BodyText += string(body)
			}
		case *gomail.AttachmentHeader:
			fn, _ := ph.Filename()
			ct, _, _ := ph.ContentType()
			body, _ := io.ReadAll(p.Body)
			d.Attachments = append(d.Attachments, Attachment{
				Filename: fn, ContentType: ct, Size: len(body),
			})
		}
	}
	return d, nil
}

func mustAddrs(h gomail.Header, key string) []*gomail.Address {
	a, _ := h.AddressList(key)
	return a
}
```

- [ ] **Step 4: Implement `thunderbird/mbox.go`**

```go
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// readMboxAt returns the raw bytes of one mbox message beginning at offset,
// stopping before the next "From " separator line or at EOF.
func readMboxAt(mboxPath string, offset uint32) ([]byte, error) {
	f, err := os.Open(mboxPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if _, err := f.Seek(int64(offset), io.SeekStart); err != nil {
		return nil, err
	}
	br := bufio.NewReader(f)
	var out bytes.Buffer
	first := true
	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			// A "From " at the start of a line (after the first) marks the next message.
			if !first && (bytes.HasPrefix(line, []byte("From ")) || bytes.HasPrefix(line, []byte("From - "))) {
				break
			}
			// Skip the leading separator line of this message.
			if !(first && (bytes.HasPrefix(line, []byte("From ")) || bytes.HasPrefix(line, []byte("From - ")))) {
				out.Write(line)
			}
			first = false
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}
	return out.Bytes(), nil
}

// localMboxPath maps a mailbox:// folder URI to its on-disk mbox file.
// mailbox://nobody@Local%20Folders/Sub/Leaf -> <dir>/Sub.sbd/Leaf
func localMboxPath(a *Account, folderURI string) string {
	u, err := url.Parse(folderURI)
	if err != nil {
		return ""
	}
	name, _ := url.PathUnescape(strings.TrimPrefix(u.Path, "/"))
	parts := strings.Split(name, "/")
	segs := make([]string, 0, len(parts)*2)
	for i, p := range parts {
		if i < len(parts)-1 {
			segs = append(segs, p+".sbd")
		} else {
			segs = append(segs, p)
		}
	}
	return filepath.Join(append([]string{a.Directory}, segs...)...)
}

var _ = fmt.Sprintf
```

- [ ] **Step 5: Run tests** — `go test ./thunderbird/ && CGO_ENABLED=0 go build ./thunderbird/` → PASS. Drop the `var _ = fmt.Sprintf` if unused.

- [ ] **Step 6: Commit**

```bash
git add thunderbird/ && git commit -m "feat(thunderbird): MIME message parsing and local mbox reader"
```

---

## Task 8: Contacts (abook.sqlite) and calendar (local.sqlite)

**Files:**
- Create: `thunderbird/contacts.go`, `thunderbird/calendar.go`, `thunderbird/contacts_test.go`, `thunderbird/calendar_test.go`, `thunderbird/testdata/abook_seed.sql`, `thunderbird/testdata/calendar_seed.sql`

**Interfaces:**
- Produces:
  - types.go: `type Contact struct{ Name, Email, Nickname string }`, `type CalendarEvent struct{ ID, Calendar, Title, Location string; Start, End time.Time }`
  - `type Contacts struct{ db *sql.DB }`, `func OpenContacts(profileDir string) (*Contacts, error)` (nil-safe), `func (c *Contacts) Close() error`, `func (c *Contacts) Search(term string, limit int) ([]Contact, error)`, `func (c *Contacts) Get(email string) (Contact, bool, error)`
  - `type Calendar struct{ db *sql.DB; names map[string]string }`, `func OpenCalendar(profileDir string, cals []CalendarRef) (*Calendar, error)` (nil-safe), `func (c *Calendar) Close() error`, `func (c *Calendar) Events(since, until time.Time, limit int) ([]CalendarEvent, error)`

- [ ] **Step 1: Seeds**

`thunderbird/testdata/abook_seed.sql`:
```sql
CREATE TABLE properties (card TEXT, name TEXT, value TEXT);
CREATE INDEX properties_card ON properties(card);
CREATE INDEX properties_name ON properties(name);
INSERT INTO properties VALUES ('c1','DisplayName','Bob Jones');
INSERT INTO properties VALUES ('c1','PrimaryEmail','bob@example.com');
INSERT INTO properties VALUES ('c1','NickName','Bobby');
INSERT INTO properties VALUES ('c2','DisplayName','Carol Smith');
INSERT INTO properties VALUES ('c2','PrimaryEmail','carol@example.com');
```

`thunderbird/testdata/calendar_seed.sql`:
```sql
CREATE TABLE cal_events (
  cal_id TEXT, id TEXT, title TEXT, event_start INTEGER, event_end INTEGER,
  event_start_tz TEXT, event_end_tz TEXT, flags INTEGER
);
CREATE TABLE cal_properties (cal_id TEXT, item_id TEXT, key TEXT, value TEXT);
-- 1700000000000000 microseconds = 2023-11-14T22:13:20Z
INSERT INTO cal_events VALUES ('cal-uuid-1','e1','Standup',1700000000000000,1700003600000000,'floating','floating',0);
INSERT INTO cal_properties VALUES ('cal-uuid-1','e1','LOCATION','Room A');
INSERT INTO cal_events VALUES ('cal-uuid-1','e2','Review',1700100000000000,1700103600000000,'floating','floating',0);
```

- [ ] **Step 2: Write tests** — `thunderbird/contacts_test.go` and `thunderbird/calendar_test.go`

```go
// contacts_test.go
package main

import (
	"database/sql"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	_ "modernc.org/sqlite"
)

func seedDB(seedFile string) string {
	tmp, _ := os.CreateTemp("", "seed-*.sqlite")
	tmp.Close()
	seed, err := os.ReadFile(seedFile)
	Expect(err).NotTo(HaveOccurred())
	db, err := sql.Open("sqlite", "file:"+tmp.Name())
	Expect(err).NotTo(HaveOccurred())
	_, err = db.Exec(string(seed))
	Expect(err).NotTo(HaveOccurred())
	db.Close()
	return tmp.Name()
}

var _ = Describe("Contacts.Search", func() {
	It("pivots properties into contacts and matches by name or email", func() {
		c, err := openContactsAt(seedDB("testdata/abook_seed.sql"))
		Expect(err).NotTo(HaveOccurred())
		defer c.Close()
		res, err := c.Search("bob", 10)
		Expect(err).NotTo(HaveOccurred())
		Expect(res).To(HaveLen(1))
		Expect(res[0].Name).To(Equal("Bob Jones"))
		Expect(res[0].Email).To(Equal("bob@example.com"))
	})
})
```

```go
// calendar_test.go
package main

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Calendar.Events", func() {
	It("returns events in a range with location and converts microseconds", func() {
		c, err := openCalendarAt(seedDB("testdata/calendar_seed.sql"), map[string]string{"cal-uuid-1": "Home"})
		Expect(err).NotTo(HaveOccurred())
		defer c.Close()
		evs, err := c.Events(time.Unix(1699000000, 0), time.Unix(1701000000, 0), 10)
		Expect(err).NotTo(HaveOccurred())
		Expect(evs).To(HaveLen(2))
		Expect(evs[0].Title).To(Equal("Standup"))
		Expect(evs[0].Location).To(Equal("Room A"))
		Expect(evs[0].Calendar).To(Equal("Home"))
		Expect(evs[0].Start.Year()).To(Equal(2023))
	})
})
```

- [ ] **Step 3: Run to verify fail** — FAIL undefined `openContactsAt`.

- [ ] **Step 4: Add types to types.go**

```go
type Contact struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Nickname string `json:"nickname,omitempty"`
}

type CalendarEvent struct {
	ID       string    `json:"id"`
	Calendar string    `json:"calendar"`
	Title    string    `json:"title"`
	Location string    `json:"location,omitempty"`
	Start    time.Time `json:"start"`
	End      time.Time `json:"end"`
}
```

- [ ] **Step 5: Implement `thunderbird/contacts.go`**

```go
package main

import (
	"database/sql"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type Contacts struct{ db *sql.DB }

func OpenContacts(profileDir string) (*Contacts, error) {
	p := filepath.Join(profileDir, "abook.sqlite")
	if _, err := os.Stat(p); err != nil {
		return nil, nil
	}
	return openContactsAt(p)
}

func openContactsAt(path string) (*Contacts, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		return nil, err
	}
	return &Contacts{db: db}, nil
}

func (c *Contacts) Close() error {
	if c == nil {
		return nil
	}
	return c.db.Close()
}

const contactPivot = `
SELECT
  MAX(CASE WHEN name='DisplayName'  THEN value END) AS name,
  MAX(CASE WHEN name='PrimaryEmail' THEN value END) AS email,
  MAX(CASE WHEN name='NickName'     THEN value END) AS nick
FROM properties GROUP BY card`

func (c *Contacts) Search(term string, limit int) ([]Contact, error) {
	if c == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 25
	}
	q := "SELECT name, email, nick FROM (" + contactPivot + ") WHERE " +
		"(name LIKE ? OR email LIKE ? OR nick LIKE ?) LIMIT ?"
	like := "%" + term + "%"
	rows, err := c.db.Query(q, like, like, like, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanContacts(rows)
}

func (c *Contacts) Get(email string) (Contact, bool, error) {
	if c == nil {
		return Contact{}, false, nil
	}
	q := "SELECT name, email, nick FROM (" + contactPivot + ") WHERE email = ? LIMIT 1"
	rows, err := c.db.Query(q, email)
	if err != nil {
		return Contact{}, false, err
	}
	defer rows.Close()
	list, err := scanContacts(rows)
	if err != nil || len(list) == 0 {
		return Contact{}, false, err
	}
	return list[0], true, nil
}

func scanContacts(rows *sql.Rows) ([]Contact, error) {
	var out []Contact
	for rows.Next() {
		var name, email, nick sql.NullString
		if err := rows.Scan(&name, &email, &nick); err != nil {
			return nil, err
		}
		out = append(out, Contact{Name: name.String, Email: email.String, Nickname: nick.String})
	}
	return out, rows.Err()
}
```

- [ ] **Step 6: Implement `thunderbird/calendar.go`**

```go
package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type Calendar struct {
	db    *sql.DB
	names map[string]string // cal_id -> display name
}

func OpenCalendar(profileDir string, cals []CalendarRef) (*Calendar, error) {
	p := filepath.Join(profileDir, "calendar-data", "local.sqlite")
	if _, err := os.Stat(p); err != nil {
		return nil, nil
	}
	names := map[string]string{}
	for _, c := range cals {
		names[c.UUID] = c.Name
	}
	return openCalendarAt(p, names)
}

func openCalendarAt(path string, names map[string]string) (*Calendar, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		return nil, err
	}
	return &Calendar{db: db, names: names}, nil
}

func (c *Calendar) Close() error {
	if c == nil {
		return nil
	}
	return c.db.Close()
}

const eventsQuery = `
SELECT e.cal_id, e.id, e.title, e.event_start, e.event_end, p.value
FROM cal_events e
LEFT JOIN cal_properties p ON p.item_id = e.id AND p.cal_id = e.cal_id AND p.key = 'LOCATION'
WHERE e.event_start >= ? AND e.event_start <= ?
ORDER BY e.event_start ASC LIMIT ?`

func (c *Calendar) Events(since, until time.Time, limit int) ([]CalendarEvent, error) {
	if c == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 50
	}
	rows, err := c.db.Query(eventsQuery, since.UnixMicro(), until.UnixMicro(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CalendarEvent
	for rows.Next() {
		var calID, id, title string
		var startUS, endUS int64
		var loc sql.NullString
		if err := rows.Scan(&calID, &id, &title, &startUS, &endUS, &loc); err != nil {
			return nil, err
		}
		name := c.names[calID]
		if name == "" {
			name = calID
		}
		out = append(out, CalendarEvent{
			ID: id, Calendar: name, Title: title, Location: loc.String,
			Start: time.UnixMicro(startUS).UTC(), End: time.UnixMicro(endUS).UTC(),
		})
	}
	return out, rows.Err()
}
```

- [ ] **Step 7: Run tests** — `go test ./thunderbird/ && CGO_ENABLED=0 go build ./thunderbird/` → PASS.

- [ ] **Step 8: Commit**

```bash
git add thunderbird/ && git commit -m "feat(thunderbird): contacts and calendar read-only queries"
```

---

## Task 9: App wiring + main() profile loading

**Files:**
- Modify: `thunderbird/app.go`, `thunderbird/main.go`
- Create: `thunderbird/app_test.go`

**Interfaces:**
- Consumes: all `Open*`/`New*` constructors.
- Produces:
  - Extend `App`: `Gloda *Gloda; Contacts *Contacts; Calendar *Calendar; IMAP *IMAP; Sender *Sender`.
  - `func LoadApp(profileDir string, readOnly, allowSend bool) (*App, error)` — loads config + creds + opens all data sources, degrading (log + nil) on individual source failure, but failing hard on `ErrMasterPassword` and missing prefs.js.
  - `func (a *App) accountByKey / folderIsLocal(uri string) (bool, error)` helper used by mutating handlers.

- [ ] **Step 1: Write test** — `thunderbird/app_test.go`

```go
package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("LoadApp (profile only)", func() {
	It("loads config and degrades gracefully when data sources are absent", func() {
		// testdata dir has prefs.js but no gloda/abook/calendar/key4 dbs.
		a, err := loadAppFromConfig(&Config{ProfileDir: "testdata", Accounts: nil, SMTP: map[string]SMTPServer{}}, &Credentials{}, false, false)
		Expect(err).NotTo(HaveOccurred())
		Expect(a.Gloda).To(BeNil())     // no global-messages-db.sqlite in testdata
		Expect(a.Contacts).To(BeNil())  // no abook.sqlite
		Expect(a.IMAP).NotTo(BeNil())   // always constructed
	})
})

var _ = Describe("folderIsLocal", func() {
	It("treats mailbox:// as local and imap:// as remote", func() {
		a := &App{Config: &Config{Accounts: []Account{
			{Type: "imap", Hostname: "imap.example.com"},
			{Type: "none", Hostname: "Local Folders"},
		}}}
		Expect(a.folderIsLocal("mailbox://nobody@Local%20Folders/INBOX")).To(BeTrue())
		Expect(a.folderIsLocal("imap://alice@imap.example.com/INBOX")).To(BeFalse())
	})
})
```

- [ ] **Step 2: Run to verify fail** — FAIL undefined `loadAppFromConfig`.

- [ ] **Step 3: Extend `thunderbird/app.go`**

```go
package main

import (
	"log"
	"strings"
)

type App struct {
	Config    *Config
	ReadOnly  bool
	AllowSend bool

	Gloda    *Gloda
	Contacts *Contacts
	Calendar *Calendar
	IMAP     *IMAP
	Sender   *Sender
}

var app *App

// LoadApp discovers/loads the profile and opens every data source.
func LoadApp(profileDir string, readOnly, allowSend bool) (*App, error) {
	cfg, err := loadProfile(profileDir)
	if err != nil {
		return nil, err
	}
	creds, err := LoadCredentials(profileDir)
	if err != nil {
		return nil, err // includes ErrMasterPassword — fail hard with a clear message
	}
	return loadAppFromConfig(cfg, creds, readOnly, allowSend)
}

func loadAppFromConfig(cfg *Config, creds *Credentials, readOnly, allowSend bool) (*App, error) {
	a := &App{Config: cfg, ReadOnly: readOnly, AllowSend: allowSend}
	a.IMAP = NewIMAP(cfg, creds)
	a.Sender = NewSender(cfg, creds)

	if g, err := OpenGloda(cfg.ProfileDir); err != nil {
		log.Printf("thunderbird: gloda unavailable: %v", err)
	} else {
		a.Gloda = g
	}
	if c, err := OpenContacts(cfg.ProfileDir); err != nil {
		log.Printf("thunderbird: contacts unavailable: %v", err)
	} else {
		a.Contacts = c
	}
	if c, err := OpenCalendar(cfg.ProfileDir, cfg.Calendars); err != nil {
		log.Printf("thunderbird: calendar unavailable: %v", err)
	} else {
		a.Calendar = c
	}
	return a, nil
}

func (a *App) folderIsLocal(folderURI string) bool {
	return strings.HasPrefix(folderURI, "mailbox://")
}
```

- [ ] **Step 4: Wire `main()` to load the app**

Replace the profile TODO in `main.go` with:

```go
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
```

(Remove the earlier standalone `app := &App{...}` block; `app` is now set from `loaded`.)

- [ ] **Step 5: Run tests** — `go test ./thunderbird/ && CGO_ENABLED=0 go build ./thunderbird/` → PASS.

- [ ] **Step 6: Commit**

```bash
git add thunderbird/ && git commit -m "feat(thunderbird): app wiring and profile loading in main"
```

---

## Task 10: Read tools — list_accounts, list_folders, search_messages, get_message, list_recent

**Files:**
- Modify: `thunderbird/types.go`, `thunderbird/main.go`
- Create: `thunderbird/handlers.go`, `thunderbird/handlers_test.go`

**Interfaces:**
- Produces (all handlers `func(ctx, *mcp.CallToolRequest, XInput) (*mcp.CallToolResult, XOutput, error)`):
  - `ListAccounts`, `ListFolders`, `SearchMessages`, `GetMessage`, `ListRecent`
  - Input/Output structs listed in Step 1.
  - `registerTool` gains typed cases for each; `allTools()` returns these five with `category:"read"`.

- [ ] **Step 1: Add tool I/O types to types.go**

```go
type ListAccountsInput struct{}
type ListAccountsOutput struct {
	Accounts []AccountInfo `json:"accounts"`
}
type AccountInfo struct {
	Key        string   `json:"key"`
	Type       string   `json:"type"`
	Name       string   `json:"name"`
	Hostname   string   `json:"hostname"`
	Local      bool     `json:"local" jsonschema:"true for POP/Local Folders accounts which are read-only"`
	Identities []string `json:"identities" jsonschema:"email addresses you may send as"`
}

type ListFoldersInput struct {
	Account string `json:"account,omitempty" jsonschema:"optional account key to filter (see list_accounts)"`
}
type ListFoldersOutput struct {
	Folders []FolderInfo `json:"folders"`
}
type FolderInfo struct {
	URI     string `json:"uri" jsonschema:"folder URI; used as folder filter and move destination"`
	Name    string `json:"name"`
	Account string `json:"account"`
	Unread  int    `json:"unread"`
	Total   int    `json:"total"`
	Local   bool   `json:"local"`
}

type SearchMessagesInput struct {
	Query      string `json:"query,omitempty" jsonschema:"free text matched against subject and indexed body"`
	Account    string `json:"account,omitempty"`
	Folder     string `json:"folder,omitempty" jsonschema:"folder URI to restrict the search"`
	From       string `json:"from,omitempty"`
	To         string `json:"to,omitempty"`
	Subject    string `json:"subject,omitempty"`
	Since      string `json:"since,omitempty" jsonschema:"RFC3339 or YYYY-MM-DD lower bound on date"`
	Until      string `json:"until,omitempty" jsonschema:"RFC3339 or YYYY-MM-DD upper bound on date"`
	Limit      int    `json:"limit,omitempty"`
	Offset     int    `json:"offset,omitempty"`
}
type SearchMessagesOutput struct {
	Messages []MessageSummary `json:"messages"`
	Count    int              `json:"count"`
}

type GetMessageInput struct {
	MessageRef string `json:"message_ref" jsonschema:"reference returned by search_messages or list_recent"`
	BodyFormat string `json:"body_format,omitempty" jsonschema:"text (default) or html"`
}
type GetMessageOutput struct {
	Message MessageDetail `json:"message"`
}

type ListRecentInput struct {
	Folder string `json:"folder,omitempty" jsonschema:"folder URI; omit for across all folders"`
	Limit  int    `json:"limit,omitempty"`
}
type ListRecentOutput struct {
	Messages []MessageSummary `json:"messages"`
	Count    int              `json:"count"`
}
```

- [ ] **Step 2: Write handler tests** — `thunderbird/handlers_test.go`

```go
package main

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Read handlers", func() {
	BeforeEach(func() {
		f, err := os.Open("testdata/prefs.js") // reuse Task 2 fixture
		Expect(err).NotTo(HaveOccurred())
		defer f.Close()
		prefs, _ := parsePrefs(f)
		cfg := buildConfig(prefs, "testdata")
		a, err := loadAppFromConfig(cfg, &Credentials{}, false, false)
		Expect(err).NotTo(HaveOccurred())
		a.Gloda = buildGlodaFixture() // from gloda_test.go
		app = a
	})

	It("list_accounts returns imap and local accounts with the local flag", func() {
		_, out, err := ListAccounts(context.Background(), nil, ListAccountsInput{})
		Expect(err).NotTo(HaveOccurred())
		Expect(out.Accounts).To(HaveLen(2))
		Expect(out.Accounts[0].Local).To(BeFalse())
		Expect(out.Accounts[1].Local).To(BeTrue())
		Expect(out.Accounts[0].Identities).To(ContainElement("alice@example.com"))
	})

	It("search_messages returns index hits", func() {
		_, out, err := SearchMessages(context.Background(), nil, SearchMessagesInput{Query: "report"})
		Expect(err).NotTo(HaveOccurred())
		Expect(out.Count).To(Equal(1))
		Expect(out.Messages[0].Subject).To(Equal("Quarterly report"))
	})
})
```

(Add `import "os"` to the test file.)

- [ ] **Step 3: Run to verify fail** — FAIL undefined `ListAccounts`.

- [ ] **Step 4: Implement `thunderbird/handlers.go`**

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func parseDate(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	return time.Parse("2006-01-02", s)
}

func ListAccounts(_ context.Context, _ *mcp.CallToolRequest, _ ListAccountsInput) (*mcp.CallToolResult, ListAccountsOutput, error) {
	var out ListAccountsOutput
	for _, a := range app.Config.Accounts {
		info := AccountInfo{
			Key: a.Key, Type: a.Type, Name: a.Name, Hostname: a.Hostname, Local: a.IsLocal(),
		}
		for _, id := range a.Identities {
			if id.Email != "" {
				info.Identities = append(info.Identities, id.Email)
			}
		}
		out.Accounts = append(out.Accounts, info)
	}
	return nil, out, nil
}

func SearchMessages(_ context.Context, _ *mcp.CallToolRequest, in SearchMessagesInput) (*mcp.CallToolResult, SearchMessagesOutput, error) {
	since, err := parseDate(in.Since)
	if err != nil {
		return nil, SearchMessagesOutput{}, fmt.Errorf("invalid since: %w", err)
	}
	until, err := parseDate(in.Until)
	if err != nil {
		return nil, SearchMessagesOutput{}, fmt.Errorf("invalid until: %w", err)
	}
	res, err := app.Gloda.Search(SearchQuery{
		Text: in.Query, From: in.From, To: in.To, Subject: in.Subject,
		FolderURI: in.Folder, Since: since, Until: until,
		Limit: in.Limit, Offset: in.Offset,
	})
	if err != nil {
		return nil, SearchMessagesOutput{}, err
	}
	return nil, SearchMessagesOutput{Messages: res, Count: len(res)}, nil
}

func GetMessage(_ context.Context, _ *mcp.CallToolRequest, in GetMessageInput) (*mcp.CallToolResult, GetMessageOutput, error) {
	ref, err := ParseMessageRef(in.MessageRef)
	if err != nil {
		return nil, GetMessageOutput{}, err
	}
	var raw []byte
	if app.folderIsLocal(ref.FolderURI) {
		a, err := app.localAccountFor(ref.FolderURI)
		if err != nil {
			return nil, GetMessageOutput{}, err
		}
		raw, err = readMboxAt(localMboxPath(a, ref.FolderURI), ref.MessageKey)
		if err != nil {
			return nil, GetMessageOutput{}, fmt.Errorf("reading local message: %w", err)
		}
	} else {
		raw, err = app.IMAP.FetchBody(ref)
		if err != nil {
			return nil, GetMessageOutput{}, err
		}
	}
	detail, err := ParseMessage(in.MessageRef, raw)
	if err != nil {
		return nil, GetMessageOutput{}, err
	}
	if in.BodyFormat != "html" {
		detail.BodyHTML = "" // keep response small unless html requested
	}
	return nil, GetMessageOutput{Message: detail}, nil
}

func ListRecent(_ context.Context, _ *mcp.CallToolRequest, in ListRecentInput) (*mcp.CallToolResult, ListRecentOutput, error) {
	limit := in.Limit
	if limit <= 0 {
		limit = 20
	}
	res, err := app.Gloda.Recent(in.Folder, limit)
	if err != nil {
		return nil, ListRecentOutput{}, err
	}
	return nil, ListRecentOutput{Messages: res, Count: len(res)}, nil
}
```

- [ ] **Step 5: Implement `ListFolders`** (append to handlers.go)

`list_folders` reads live per-folder counts over IMAP for IMAP accounts, and lists local folders by walking the on-disk directory (counts omitted/zero for local, which is acceptable read-only behavior).

```go
import (
	"os"
	"path/filepath"
	"strings"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

func ListFolders(_ context.Context, _ *mcp.CallToolRequest, in ListFoldersInput) (*mcp.CallToolResult, ListFoldersOutput, error) {
	var out ListFoldersOutput
	for i := range app.Config.Accounts {
		a := &app.Config.Accounts[i]
		if in.Account != "" && a.Key != in.Account {
			continue
		}
		if a.Type == "imap" {
			fs, err := app.IMAP.listIMAPFolders(a)
			if err != nil {
				// Degrade: report the account with an error-free empty list rather than failing all.
				continue
			}
			out.Folders = append(out.Folders, fs...)
		} else {
			out.Folders = append(out.Folders, listLocalFolders(a)...)
		}
	}
	return nil, out, nil
}

func listLocalFolders(a *Account) []FolderInfo {
	var out []FolderInfo
	_ = filepath.Walk(a.Directory, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		name := info.Name()
		if strings.HasSuffix(name, ".msf") || strings.HasSuffix(name, ".dat") {
			return nil
		}
		out = append(out, FolderInfo{
			Name:    name,
			URI:     "mailbox://" + a.Username + "@" + a.Hostname + "/" + name,
			Account: a.Key, Local: true,
		})
		return nil
	})
	return out
}
```

Add `listIMAPFolders` to imap.go:

```go
func (m *IMAP) listIMAPFolders(a *Account) ([]FolderInfo, error) {
	c, err := m.connect(a)
	if err != nil {
		return nil, err
	}
	defer c.Close()
	listCmd := c.List("", "*", &imap.ListOptions{
		ReturnStatus: &imap.StatusOptions{NumMessages: true, NumUnseen: true},
	})
	data, err := listCmd.Collect()
	if err != nil {
		return nil, err
	}
	var out []FolderInfo
	for _, d := range data {
		fi := FolderInfo{
			Name:    d.Mailbox,
			URI:     "imap://" + a.Username + "@" + a.Hostname + "/" + d.Mailbox,
			Account: a.Key,
		}
		if d.Status != nil {
			if d.Status.NumMessages != nil {
				fi.Total = int(*d.Status.NumMessages)
			}
			if d.Status.NumUnseen != nil {
				fi.Unread = int(*d.Status.NumUnseen)
			}
		}
		out = append(out, fi)
	}
	_ = c.Logout().Wait()
	return out, nil
}

var _ = imapclient.Client{} // ensure import used
```

Add `localAccountFor` to app.go:

```go
import "fmt"

func (a *App) localAccountFor(folderURI string) (*Account, error) {
	for i := range a.Config.Accounts {
		acct := &a.Config.Accounts[i]
		if acct.IsLocal() {
			return acct, nil
		}
	}
	return nil, fmt.Errorf("no local account for %q", folderURI)
}
```

- [ ] **Step 6: Register the read tools in main.go**

Add to `allTools()`:

```go
func allTools() []toolDef {
	return []toolDef{
		{&mcp.Tool{Name: "list_accounts", Description: "List Thunderbird mail accounts, their type, and the identities (email addresses) you may send as. Local (POP/Local Folders) accounts are read-only."}, ListAccounts, "read"},
		{&mcp.Tool{Name: "list_folders", Description: "List mail folders with unread/total counts. Returns folder URIs used as the folder filter in search and as move destinations."}, ListFolders, "read"},
		{&mcp.Tool{Name: "search_messages", Description: "Search mail via Thunderbird's index. Filter by query text, from, to, subject, folder URI, and date range. Returns message references."}, SearchMessages, "read"},
		{&mcp.Tool{Name: "get_message", Description: "Fetch a full message (headers, body, attachment list) by the message_ref from search_messages or list_recent."}, GetMessage, "read"},
		{&mcp.Tool{Name: "list_recent", Description: "List the most recent messages, optionally within a folder URI."}, ListRecent, "read"},
	}
}
```

Add the typed cases to `registerTool`:

```go
func registerTool(server *mcp.Server, td toolDef) {
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
	default:
		log.Fatalf("no registration case for tool %s", td.tool.Name)
	}
}
```

- [ ] **Step 7: Run tests** — `go test ./thunderbird/ && CGO_ENABLED=0 go build ./thunderbird/` → PASS.

- [ ] **Step 8: Commit**

```bash
git add thunderbird/ && git commit -m "feat(thunderbird): read tools (accounts, folders, search, get, recent)"
```

---

## Task 11: Mutate tools — set_flags, move_message, delete_message

**Files:**
- Modify: `thunderbird/types.go`, `thunderbird/handlers.go`, `thunderbird/main.go`, `thunderbird/handlers_test.go`

**Interfaces:**
- Produces: `SetFlags`, `MoveMessage`, `DeleteMessage` handlers (`category:"mutate"`), each rejecting local folders via `app.folderIsLocal`.

- [ ] **Step 1: Add types to types.go**

```go
type SetFlagsInput struct {
	MessageRef string   `json:"message_ref"`
	Read       *bool    `json:"read,omitempty" jsonschema:"set read (true) or unread (false)"`
	Flagged    *bool    `json:"flagged,omitempty" jsonschema:"set or clear the star/flag"`
	AddTags    []string `json:"add_tags,omitempty"`
	RemoveTags []string `json:"remove_tags,omitempty"`
}
type SetFlagsOutput struct {
	Success bool `json:"success"`
}

type MoveMessageInput struct {
	MessageRef  string `json:"message_ref"`
	Destination string `json:"destination" jsonschema:"destination folder name within the same account (e.g. Archive)"`
}
type MoveMessageOutput struct {
	Success bool `json:"success"`
}

type DeleteMessageInput struct {
	MessageRef string `json:"message_ref"`
	Permanent  bool   `json:"permanent,omitempty" jsonschema:"if true, expunge permanently; default moves to Trash"`
}
type DeleteMessageOutput struct {
	Success bool `json:"success"`
}
```

- [ ] **Step 2: Add a failing test asserting local folders are rejected** — append to handlers_test.go

```go
var _ = Describe("Mutate handlers reject local folders", func() {
	BeforeEach(func() {
		app = &App{Config: &Config{Accounts: []Account{{Type: "none", Hostname: "Local Folders"}}}}
		app.IMAP = NewIMAP(app.Config, &Credentials{})
	})
	It("set_flags on a mailbox:// ref errors", func() {
		_, _, err := SetFlags(context.Background(), nil, SetFlagsInput{
			MessageRef: "mailbox://nobody@Local%20Folders/INBOX#42", Read: ptr(true),
		})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("read-only"))
	})
})

func ptr[T any](v T) *T { return &v }
```

- [ ] **Step 3: Run to verify fail** — FAIL undefined `SetFlags`.

- [ ] **Step 4: Implement the mutate handlers** — append to handlers.go

```go
func requireRemote(ref MessageRef) error {
	if app.folderIsLocal(ref.FolderURI) {
		return fmt.Errorf("folder %q is a local (POP/Local Folders) account and is read-only", ref.FolderURI)
	}
	return nil
}

func SetFlags(_ context.Context, _ *mcp.CallToolRequest, in SetFlagsInput) (*mcp.CallToolResult, SetFlagsOutput, error) {
	ref, err := ParseMessageRef(in.MessageRef)
	if err != nil {
		return nil, SetFlagsOutput{}, err
	}
	if err := requireRemote(ref); err != nil {
		return nil, SetFlagsOutput{}, err
	}
	var add, remove []imap.Flag
	if in.Read != nil {
		if *in.Read {
			add = append(add, imap.FlagSeen)
		} else {
			remove = append(remove, imap.FlagSeen)
		}
	}
	if in.Flagged != nil {
		if *in.Flagged {
			add = append(add, imap.FlagFlagged)
		} else {
			remove = append(remove, imap.FlagFlagged)
		}
	}
	for _, t := range in.AddTags {
		add = append(add, imap.Flag(t))
	}
	for _, t := range in.RemoveTags {
		remove = append(remove, imap.Flag(t))
	}
	if err := app.IMAP.SetFlags(ref, add, remove); err != nil {
		return nil, SetFlagsOutput{}, err
	}
	return nil, SetFlagsOutput{Success: true}, nil
}

func MoveMessage(_ context.Context, _ *mcp.CallToolRequest, in MoveMessageInput) (*mcp.CallToolResult, MoveMessageOutput, error) {
	ref, err := ParseMessageRef(in.MessageRef)
	if err != nil {
		return nil, MoveMessageOutput{}, err
	}
	if err := requireRemote(ref); err != nil {
		return nil, MoveMessageOutput{}, err
	}
	if in.Destination == "" {
		return nil, MoveMessageOutput{}, fmt.Errorf("destination is required")
	}
	if err := app.IMAP.Move(ref, in.Destination); err != nil {
		return nil, MoveMessageOutput{}, err
	}
	return nil, MoveMessageOutput{Success: true}, nil
}

func DeleteMessage(_ context.Context, _ *mcp.CallToolRequest, in DeleteMessageInput) (*mcp.CallToolResult, DeleteMessageOutput, error) {
	ref, err := ParseMessageRef(in.MessageRef)
	if err != nil {
		return nil, DeleteMessageOutput{}, err
	}
	if err := requireRemote(ref); err != nil {
		return nil, DeleteMessageOutput{}, err
	}
	if err := app.IMAP.Delete(ref, in.Permanent); err != nil {
		return nil, DeleteMessageOutput{}, err
	}
	return nil, DeleteMessageOutput{Success: true}, nil
}
```

- [ ] **Step 5: Register mutate tools** — add to `allTools()` and `registerTool`

```go
// allTools() entries:
{&mcp.Tool{Name: "set_flags", Description: "Mark a message read/unread, set or clear its flag/star, and add or remove tags. IMAP accounts only."}, SetFlags, "mutate"},
{&mcp.Tool{Name: "move_message", Description: "Move a message to another folder in the same IMAP account."}, MoveMessage, "mutate"},
{&mcp.Tool{Name: "delete_message", Description: "Delete a message: moves to Trash by default, or expunges permanently when permanent=true. IMAP accounts only."}, DeleteMessage, "mutate"},

// registerTool() cases:
case func(context.Context, *mcp.CallToolRequest, SetFlagsInput) (*mcp.CallToolResult, SetFlagsOutput, error):
	mcp.AddTool(server, td.tool, h)
case func(context.Context, *mcp.CallToolRequest, MoveMessageInput) (*mcp.CallToolResult, MoveMessageOutput, error):
	mcp.AddTool(server, td.tool, h)
case func(context.Context, *mcp.CallToolRequest, DeleteMessageInput) (*mcp.CallToolResult, DeleteMessageOutput, error):
	mcp.AddTool(server, td.tool, h)
```

- [ ] **Step 6: Run tests** — `go test ./thunderbird/ && CGO_ENABLED=0 go build ./thunderbird/` → PASS.

- [ ] **Step 7: Commit**

```bash
git add thunderbird/ && git commit -m "feat(thunderbird): mutate tools (set_flags, move, delete)"
```

---

## Task 12: Compose tools — send_mail, reply_message, forward_message, save_draft

**Files:**
- Modify: `thunderbird/types.go`, `thunderbird/handlers.go`, `thunderbird/main.go`, `thunderbird/handlers_test.go`

**Interfaces:**
- Produces: `SendMail`, `ReplyMessage`, `ForwardMessage`, `SaveDraft` handlers (`category:"compose"`). Reply/forward fetch the original via `get_message`'s path, then build and send.

- [ ] **Step 1: Add types to types.go**

```go
type SendMailInput struct {
	From    string   `json:"from" jsonschema:"identity email to send as (see list_accounts)"`
	To      []string `json:"to"`
	Cc      []string `json:"cc,omitempty"`
	Bcc     []string `json:"bcc,omitempty"`
	Subject string   `json:"subject"`
	Body    string   `json:"body"`
}
type SendMailOutput struct {
	Success bool `json:"success"`
}

type ReplyMessageInput struct {
	MessageRef string `json:"message_ref" jsonschema:"message being replied to"`
	From       string `json:"from,omitempty" jsonschema:"identity to send as; defaults to the account's identity"`
	Body       string `json:"body"`
	ReplyAll   bool   `json:"reply_all,omitempty"`
}
type ReplyMessageOutput struct {
	Success bool `json:"success"`
}

type ForwardMessageInput struct {
	MessageRef string   `json:"message_ref"`
	From       string   `json:"from,omitempty"`
	To         []string `json:"to"`
	Body       string   `json:"body,omitempty"`
}
type ForwardMessageOutput struct {
	Success bool `json:"success"`
}

type SaveDraftInput struct {
	From    string   `json:"from"`
	To      []string `json:"to,omitempty"`
	Subject string   `json:"subject,omitempty"`
	Body    string   `json:"body,omitempty"`
}
type SaveDraftOutput struct {
	Success bool `json:"success"`
}
```

- [ ] **Step 2: Write a test for send via the fake SMTP path**

go-smtp ships an in-memory server harness. Add a spec that stands up a `smtp.Server` with a capturing `Backend`, points a one-identity `Config` at it (SocketType 0 → adjust `Send` to allow a plaintext path in tests) and asserts the message is received.

Because `Sender.Send` currently only does implicit-TLS or STARTTLS, add a plaintext branch used when `SocketType == 0` (valid for the loopback test server and for pl\_ain local relays):

```go
// in smtp.go Send(), before the SocketType==3 check:
if srv.SocketType == 0 {
	c, err := smtp.Dial(addr)
	if err != nil {
		return err
	}
	defer c.Close()
	if err := c.SendMail(msg.From[0], rcpts, bytes.NewReader(raw)); err != nil {
		return err
	}
	return c.Quit()
}
```

```go
// smtp_test.go addition
var _ = Describe("Sender.Send over a loopback server", func() {
	It("delivers a message", func() {
		be := &captureBackend{}
		s := smtp.NewServer(be)
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		Expect(err).NotTo(HaveOccurred())
		defer ln.Close()
		go s.Serve(ln)
		host, port, _ := net.SplitHostPort(ln.Addr().String())
		cfg := &Config{
			Accounts: []Account{{Type: "imap", Identities: []Identity{{Email: "alice@example.com", SMTPKey: "s1"}}}},
			SMTP:     map[string]SMTPServer{"s1": {Hostname: host, Port: atoi(port), SocketType: 0, Username: "alice@example.com"}},
		}
		sender := NewSender(cfg, &Credentials{byHostUser: map[string]string{credKey(host, "alice@example.com"): "pw"}})
		err = sender.Send(OutgoingMessage{From: []string{"alice@example.com"}, To: []string{"bob@example.com"}, Subject: "Hi", BodyText: "yo"})
		Expect(err).NotTo(HaveOccurred())
		Expect(be.received).To(ContainSubstring("yo"))
	})
})
```

Provide a minimal `captureBackend` implementing go-smtp's `Backend`/`Session` interfaces (NewSession → session that records `Data`). Consult `go doc github.com/emersion/go-smtp Backend` for the exact method set at v0.24.0 and implement the required methods (`AuthMechanisms`, `Auth`, `Mail`, `Rcpt`, `Data`, `Reset`, `Logout`).

- [ ] **Step 3: Run to verify fail** — FAIL undefined `SendMail`.

- [ ] **Step 4: Implement compose handlers** — append to handlers.go

```go
func SendMail(_ context.Context, _ *mcp.CallToolRequest, in SendMailInput) (*mcp.CallToolResult, SendMailOutput, error) {
	if in.From == "" || len(in.To) == 0 {
		return nil, SendMailOutput{}, fmt.Errorf("from and to are required")
	}
	err := app.Sender.Send(OutgoingMessage{
		From: []string{in.From}, To: in.To, Cc: in.Cc, Bcc: in.Bcc,
		Subject: in.Subject, BodyText: in.Body,
	})
	if err != nil {
		return nil, SendMailOutput{}, err
	}
	return nil, SendMailOutput{Success: true}, nil
}

func loadDetail(messageRef string) (MessageDetail, error) {
	ref, err := ParseMessageRef(messageRef)
	if err != nil {
		return MessageDetail{}, err
	}
	var raw []byte
	if app.folderIsLocal(ref.FolderURI) {
		a, err := app.localAccountFor(ref.FolderURI)
		if err != nil {
			return MessageDetail{}, err
		}
		raw, err = readMboxAt(localMboxPath(a, ref.FolderURI), ref.MessageKey)
		if err != nil {
			return MessageDetail{}, err
		}
	} else {
		raw, err = app.IMAP.FetchBody(ref)
		if err != nil {
			return MessageDetail{}, err
		}
	}
	return ParseMessage(messageRef, raw)
}

func defaultFrom(orig MessageDetail, given string) string {
	if given != "" {
		return given
	}
	// Fall back to the first identity of the first account.
	for _, a := range app.Config.Accounts {
		if len(a.Identities) > 0 {
			return a.Identities[0].Email
		}
	}
	return ""
}

func ReplyMessage(_ context.Context, _ *mcp.CallToolRequest, in ReplyMessageInput) (*mcp.CallToolResult, ReplyMessageOutput, error) {
	orig, err := loadDetail(in.MessageRef)
	if err != nil {
		return nil, ReplyMessageOutput{}, err
	}
	inReplyTo, references := replyHeaders(orig)
	to := []string{firstAddress(orig.Author)}
	var cc []string
	if in.ReplyAll {
		cc = append(cc, orig.To...)
		cc = append(cc, orig.Cc...)
	}
	body := in.Body + "\n\n" + quoteForReply(orig)
	err = app.Sender.Send(OutgoingMessage{
		From: []string{defaultFrom(orig, in.From)}, To: to, Cc: cc,
		Subject: ensurePrefix(orig.Subject, "Re: "), BodyText: body,
		InReplyTo: inReplyTo, References: references,
	})
	if err != nil {
		return nil, ReplyMessageOutput{}, err
	}
	return nil, ReplyMessageOutput{Success: true}, nil
}

func ForwardMessage(_ context.Context, _ *mcp.CallToolRequest, in ForwardMessageInput) (*mcp.CallToolResult, ForwardMessageOutput, error) {
	if len(in.To) == 0 {
		return nil, ForwardMessageOutput{}, fmt.Errorf("to is required")
	}
	orig, err := loadDetail(in.MessageRef)
	if err != nil {
		return nil, ForwardMessageOutput{}, err
	}
	body := in.Body + "\n\n---------- Forwarded message ----------\n" +
		"From: " + orig.Author + "\nSubject: " + orig.Subject + "\n\n" + orig.BodyText
	err = app.Sender.Send(OutgoingMessage{
		From: []string{defaultFrom(orig, in.From)}, To: in.To,
		Subject: ensurePrefix(orig.Subject, "Fwd: "), BodyText: body,
	})
	if err != nil {
		return nil, ForwardMessageOutput{}, err
	}
	return nil, ForwardMessageOutput{Success: true}, nil
}

func SaveDraft(_ context.Context, _ *mcp.CallToolRequest, in SaveDraftInput) (*mcp.CallToolResult, SaveDraftOutput, error) {
	raw, err := BuildRFC822(OutgoingMessage{
		From: []string{in.From}, To: in.To, Subject: in.Subject, BodyText: in.Body,
	})
	if err != nil {
		return nil, SaveDraftOutput{}, err
	}
	folder := draftFolderFor(in.From)
	if err := app.IMAP.Append(folder, raw, []imap.Flag{imap.FlagDraft}); err != nil {
		return nil, SaveDraftOutput{}, err
	}
	return nil, SaveDraftOutput{Success: true}, nil
}
```

- [ ] **Step 5: Add the small string helpers** — append to handlers.go

```go
import "strings"

func firstAddress(s string) string {
	if i := strings.LastIndex(s, "<"); i >= 0 {
		if j := strings.Index(s[i:], ">"); j >= 0 {
			return s[i+1 : i+j]
		}
	}
	return strings.TrimSpace(s)
}

func ensurePrefix(subject, prefix string) string {
	if strings.HasPrefix(strings.ToLower(subject), strings.ToLower(prefix)) {
		return subject
	}
	return prefix + subject
}

// draftFolderFor returns the identity's draft folder URI, or a bare "Drafts".
func draftFolderFor(fromEmail string) string {
	for _, a := range app.Config.Accounts {
		for _, id := range a.Identities {
			if strings.EqualFold(id.Email, fromEmail) && id.DraftFolder != "" {
				return id.DraftFolder
			}
		}
	}
	return "Drafts"
}
```

- [ ] **Step 6: Register compose tools** — add to `allTools()` and `registerTool`

```go
// allTools():
{&mcp.Tool{Name: "send_mail", Description: "Send a new email as one of your identities. Requires THUNDERBIRD_ALLOW_SEND=true."}, SendMail, "compose"},
{&mcp.Tool{Name: "reply_message", Description: "Reply to a message, quoting the original and threading correctly. Requires THUNDERBIRD_ALLOW_SEND=true."}, ReplyMessage, "compose"},
{&mcp.Tool{Name: "forward_message", Description: "Forward a message to new recipients. Requires THUNDERBIRD_ALLOW_SEND=true."}, ForwardMessage, "compose"},
{&mcp.Tool{Name: "save_draft", Description: "Save a draft to the Drafts folder (visible in Thunderbird). Requires THUNDERBIRD_ALLOW_SEND=true."}, SaveDraft, "compose"},

// registerTool():
case func(context.Context, *mcp.CallToolRequest, SendMailInput) (*mcp.CallToolResult, SendMailOutput, error):
	mcp.AddTool(server, td.tool, h)
case func(context.Context, *mcp.CallToolRequest, ReplyMessageInput) (*mcp.CallToolResult, ReplyMessageOutput, error):
	mcp.AddTool(server, td.tool, h)
case func(context.Context, *mcp.CallToolRequest, ForwardMessageInput) (*mcp.CallToolResult, ForwardMessageOutput, error):
	mcp.AddTool(server, td.tool, h)
case func(context.Context, *mcp.CallToolRequest, SaveDraftInput) (*mcp.CallToolResult, SaveDraftOutput, error):
	mcp.AddTool(server, td.tool, h)
```

- [ ] **Step 7: Run tests** — `go test ./thunderbird/ && CGO_ENABLED=0 go build ./thunderbird/` → PASS.

- [ ] **Step 8: Commit**

```bash
git add thunderbird/ && git commit -m "feat(thunderbird): compose tools (send, reply, forward, save_draft)"
```

---

## Task 13: Contacts + calendar tools

**Files:**
- Modify: `thunderbird/types.go`, `thunderbird/handlers.go`, `thunderbird/main.go`, `thunderbird/handlers_test.go`

**Interfaces:**
- Produces: `SearchContacts`, `GetContact`, `ListCalendars`, `ListEvents` (`category:"read"`).

- [ ] **Step 1: Add types to types.go**

```go
type SearchContactsInput struct {
	Query string `json:"query" jsonschema:"name or email substring"`
	Limit int    `json:"limit,omitempty"`
}
type SearchContactsOutput struct {
	Contacts []Contact `json:"contacts"`
	Count    int       `json:"count"`
}

type GetContactInput struct {
	Email string `json:"email"`
}
type GetContactOutput struct {
	Contact Contact `json:"contact"`
	Found   bool    `json:"found"`
}

type ListCalendarsInput struct{}
type ListCalendarsOutput struct {
	Calendars []CalendarRef `json:"calendars"`
}

type ListEventsInput struct {
	Since string `json:"since,omitempty" jsonschema:"RFC3339 or YYYY-MM-DD; defaults to now"`
	Until string `json:"until,omitempty" jsonschema:"RFC3339 or YYYY-MM-DD; defaults to 30 days out"`
	Limit int    `json:"limit,omitempty"`
}
type ListEventsOutput struct {
	Events []CalendarEvent `json:"events"`
	Count  int             `json:"count"`
}
```

- [ ] **Step 2: Write tests** — append to handlers_test.go

```go
var _ = Describe("Contacts and calendar handlers", func() {
	BeforeEach(func() {
		app = &App{Config: &Config{Calendars: []CalendarRef{{UUID: "cal-uuid-1", Name: "Home", Type: "storage"}}}}
		c, err := openContactsAt(seedDB("testdata/abook_seed.sql"))
		Expect(err).NotTo(HaveOccurred())
		app.Contacts = c
		cal, err := openCalendarAt(seedDB("testdata/calendar_seed.sql"), map[string]string{"cal-uuid-1": "Home"})
		Expect(err).NotTo(HaveOccurred())
		app.Calendar = cal
	})
	It("search_contacts finds by substring", func() {
		_, out, err := SearchContacts(context.Background(), nil, SearchContactsInput{Query: "carol"})
		Expect(err).NotTo(HaveOccurred())
		Expect(out.Count).To(Equal(1))
		Expect(out.Contacts[0].Email).To(Equal("carol@example.com"))
	})
	It("list_events returns events in the default window", func() {
		_, out, err := ListEvents(context.Background(), nil, ListEventsInput{
			Since: "2023-11-01", Until: "2023-12-01",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(out.Count).To(Equal(2))
	})
	It("list_calendars lists registered calendars", func() {
		_, out, err := ListCalendars(context.Background(), nil, ListCalendarsInput{})
		Expect(err).NotTo(HaveOccurred())
		Expect(out.Calendars).To(HaveLen(1))
		Expect(out.Calendars[0].Name).To(Equal("Home"))
	})
})
```

- [ ] **Step 3: Run to verify fail** — FAIL undefined `SearchContacts`.

- [ ] **Step 4: Implement handlers** — append to handlers.go

```go
func SearchContacts(_ context.Context, _ *mcp.CallToolRequest, in SearchContactsInput) (*mcp.CallToolResult, SearchContactsOutput, error) {
	if app.Contacts == nil {
		return nil, SearchContactsOutput{}, fmt.Errorf("no address book available")
	}
	res, err := app.Contacts.Search(in.Query, in.Limit)
	if err != nil {
		return nil, SearchContactsOutput{}, err
	}
	return nil, SearchContactsOutput{Contacts: res, Count: len(res)}, nil
}

func GetContact(_ context.Context, _ *mcp.CallToolRequest, in GetContactInput) (*mcp.CallToolResult, GetContactOutput, error) {
	if app.Contacts == nil {
		return nil, GetContactOutput{}, fmt.Errorf("no address book available")
	}
	c, found, err := app.Contacts.Get(in.Email)
	if err != nil {
		return nil, GetContactOutput{}, err
	}
	return nil, GetContactOutput{Contact: c, Found: found}, nil
}

func ListCalendars(_ context.Context, _ *mcp.CallToolRequest, _ ListCalendarsInput) (*mcp.CallToolResult, ListCalendarsOutput, error) {
	return nil, ListCalendarsOutput{Calendars: app.Config.Calendars}, nil
}

func ListEvents(_ context.Context, _ *mcp.CallToolRequest, in ListEventsInput) (*mcp.CallToolResult, ListEventsOutput, error) {
	if app.Calendar == nil {
		return nil, ListEventsOutput{}, fmt.Errorf("no calendar available")
	}
	since, err := parseDate(in.Since)
	if err != nil {
		return nil, ListEventsOutput{}, fmt.Errorf("invalid since: %w", err)
	}
	until, err := parseDate(in.Until)
	if err != nil {
		return nil, ListEventsOutput{}, fmt.Errorf("invalid until: %w", err)
	}
	if since.IsZero() {
		since = fixedNow()
	}
	if until.IsZero() {
		until = since.AddDate(0, 0, 30)
	}
	res, err := app.Calendar.Events(since, until, in.Limit)
	if err != nil {
		return nil, ListEventsOutput{}, err
	}
	return nil, ListEventsOutput{Events: res, Count: len(res)}, nil
}
```

- [ ] **Step 5: Register the four tools** — add to `allTools()` and `registerTool`

```go
// allTools():
{&mcp.Tool{Name: "search_contacts", Description: "Search the Thunderbird address book by name or email substring."}, SearchContacts, "read"},
{&mcp.Tool{Name: "get_contact", Description: "Get a single contact by exact email address."}, GetContact, "read"},
{&mcp.Tool{Name: "list_calendars", Description: "List the calendars registered in Thunderbird."}, ListCalendars, "read"},
{&mcp.Tool{Name: "list_events", Description: "List calendar events in a date range (defaults to the next 30 days)."}, ListEvents, "read"},

// registerTool():
case func(context.Context, *mcp.CallToolRequest, SearchContactsInput) (*mcp.CallToolResult, SearchContactsOutput, error):
	mcp.AddTool(server, td.tool, h)
case func(context.Context, *mcp.CallToolRequest, GetContactInput) (*mcp.CallToolResult, GetContactOutput, error):
	mcp.AddTool(server, td.tool, h)
case func(context.Context, *mcp.CallToolRequest, ListCalendarsInput) (*mcp.CallToolResult, ListCalendarsOutput, error):
	mcp.AddTool(server, td.tool, h)
case func(context.Context, *mcp.CallToolRequest, ListEventsInput) (*mcp.CallToolResult, ListEventsOutput, error):
	mcp.AddTool(server, td.tool, h)
```

- [ ] **Step 6: Run tests** — `go test ./thunderbird/ && CGO_ENABLED=0 go build ./thunderbird/` → PASS.

- [ ] **Step 7: Commit**

```bash
git add thunderbird/ && git commit -m "feat(thunderbird): contacts and calendar tools"
```

---

## Task 14: CI, Docker, README

**Files:**
- Modify: `.github/workflows/image.yml`, `README.md`

**Interfaces:** none (packaging).

- [ ] **Step 1: Add the CI matrix entry** — `.github/workflows/image.yml`

After the `github` entry (line ~81-83), add:

```yaml
          - mcp: thunderbird
            dockerfile: ./Dockerfile
            context: ./
```

- [ ] **Step 2: Verify the shared Dockerfile builds thunderbird**

Run:
```bash
cd /home/mudler/_git/mcps
docker build --build-arg MCP_SERVER=thunderbird -t mcps/thunderbird:test -f ./Dockerfile ./
```
Expected: build succeeds (the shared Dockerfile does `go build -o main ./${MCP_SERVER}/` with `CGO_ENABLED=0`).

If docker is unavailable in the environment, instead run `CGO_ENABLED=0 go build -o /tmp/tb ./thunderbird/` and confirm it produces a binary.

- [ ] **Step 3: Add the README section** — insert after the GitHub server section in `README.md`

````markdown
### 🐦 Thunderbird Server

Exposes a local Thunderbird profile's mail, contacts, and calendar. Search and read
use Thunderbird's own index; mutations and sending use IMAP/SMTP with credentials
decrypted from the profile.

**Tools:**
- `list_accounts`, `list_folders`
- `search_messages`, `get_message`, `list_recent`
- `set_flags`, `move_message`, `delete_message` (IMAP accounts only)
- `send_mail`, `reply_message`, `forward_message`, `save_draft` (require `THUNDERBIRD_ALLOW_SEND=true`)
- `search_contacts`, `get_contact`, `list_calendars`, `list_events`

**Configuration:**
- `THUNDERBIRD_PROFILE` — path to the profile directory (auto-discovered if unset)
- `THUNDERBIRD_READ_ONLY` — `true` disables all mutating and sending tools
- `THUNDERBIRD_ALLOW_SEND` — `true` enables the compose/send tools (off by default)
- `THUNDERBIRD_TOOLS` — comma-separated allowlist of tool names (default: all)

**Notes:**
- Local (POP/Local Folders) accounts are read-only.
- Accounts using OAuth2 (many Gmail/Outlook setups) are not supported for live IMAP/SMTP;
  they still appear in search results from the index.
- A profile protected by a master password cannot have its credentials decrypted.

**Docker image:**
```bash
docker run -i --rm \
  -e THUNDERBIRD_PROFILE=/profile \
  -v "$HOME/.thunderbird/xxxx.default-release:/profile:ro" \
  ghcr.io/mudler/mcps/thunderbird:master
```

**LocalAI configuration (add to the model config):**
```yaml
mcp:
  stdio: |
    {
      "mcpServers": {
        "thunderbird": {
          "command": "docker",
          "args": [
            "run", "-i", "--rm", "--network", "host",
            "-e", "THUNDERBIRD_PROFILE=/profile",
            "-e", "THUNDERBIRD_ALLOW_SEND=false",
            "-v", "/home/user/.thunderbird/xxxx.default-release:/profile:ro",
            "ghcr.io/mudler/mcps/thunderbird:master"
          ]
        }
      }
    }
```
> `--network host` lets the container reach your IMAP/SMTP servers directly. Mounting
> the profile read-only is recommended; the server never writes to profile files.
````

- [ ] **Step 4: Run the full suite one more time**

Run: `cd /home/mudler/_git/mcps && go test ./thunderbird/ && CGO_ENABLED=0 go build ./thunderbird/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add .github/workflows/image.yml README.md && git commit -m "ci(thunderbird): add to build matrix and document the server"
```

---

## Self-Review Notes

- **Spec coverage:** every tool in the spec's tool surface maps to a task (Discovery→T10, Search/read→T10, Mutate→T11, Compose→T12, Contacts/calendar→T13). Data sources: prefs.js→T2, credentials→T3, gloda→T4, IMAP→T5, SMTP→T6, mbox+parse→T7, abook/calendar→T8. Config gates (`READ_ONLY`, `ALLOW_SEND`, `TOOLS`) implemented in T1 registry and enforced there. Packaging/README/CI→T14. Non-goals (no extension, no writes to owned files, no OAuth, read-only local/contacts/calendar) are enforced: OAuth rejected in `connect`/`Send`, local rejected in mutate handlers, all SQLite opened `mode=ro`.
- **Known softening for the executor:** the NSS end-to-end test (T3) depends on a fixture generated once from a real Thunderbird; the crypto primitives are unit-tested with captured vectors so CI stays green without shipping a profile. The exact `imapmemserver`/go-smtp `Backend` method sets should be confirmed with `go doc` at the pinned versions when writing T5/T12 tests — the handler and library-call code is verified against v2.0.0-beta.8 / v0.24.0, but the in-memory test harness API is the one place to double-check.
- **Type consistency:** `MessageRef.String()`/`ParseMessageRef` (T4) are the sole ref format, consumed identically by T10–T12. `SMTPServer`, `Identity`, `Account`, `CalendarRef` defined in T1–T2 and reused unchanged. `fixedNow` (T6) is the single time source used by `ListEvents` (T13) and `BuildRFC822`.
