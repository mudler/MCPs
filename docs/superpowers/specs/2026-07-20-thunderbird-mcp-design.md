# Thunderbird MCP Server — Design

Date: 2026-07-20
Status: approved, ready for implementation planning

## Purpose

An MCP server exposing a user's Thunderbird mail, contacts, and calendar to an LLM:
search and read mail, change message state, send mail, and query contacts and
calendar events.

It ships as a Go binary and a Docker image, consistent with the other servers in
this repository.

## Background: why not an extension

The two existing projects in this space (`TKasperczyk/thunderbird-mcp` and the
abandoned `bb1/thunderbird-mcp`) both work the same way: a Thunderbird
*Experiment* WebExtension containing chrome-privileged JavaScript embeds an HTTP
server on localhost, and the MCP layer is a thin stdio-to-HTTP bridge.

We reject that approach for this repository because it requires shipping and
maintaining privileged JavaScript, requires the user to install an unsigned
add-on, requires Thunderbird to be running, and inherits Thunderbird's announced
(currently postponed) plan to disable Experiments on the Release channel.

Instead we read Thunderbird's own on-disk state directly and use IMAP/SMTP for
anything live. This yields one self-contained binary that works whether or not
Thunderbird is running.

The tradeoff, stated plainly: this is an email MCP server *configured from a
Thunderbird profile*, not a remote control for the Thunderbird application.
Sending happens over SMTP, not through Thunderbird's composer.

## Architecture

Package `thunderbird/`, following the existing `jellyfin/` layout:

| File | Responsibility |
|---|---|
| `main.go` | tool registry and server startup |
| `profile.go` | profile discovery, `prefs.js` parsing |
| `gloda.go` | read-only queries against the gloda search index |
| `secrets.go` | `logins.json` + `key4.db` decryption |
| `imap.go` | IMAP fetch and mutation |
| `smtp.go` | message construction and sending |
| `contacts.go` | `abook.sqlite` queries |
| `calendar.go` | `local.sqlite` queries |
| `handlers.go` | MCP tool handlers |
| `types.go` | input/output types with jsonschema tags |

### Data sources

| Source | File | Access | Used for |
|---|---|---|---|
| Account config | `prefs.js` | read | IMAP/SMTP hosts, ports, usernames, socket type, identities |
| Search index | `global-messages-db.sqlite` | read-only | full-text search across all mail |
| Credentials | `logins.json` + `key4.db` | read-only | IMAP/SMTP passwords |
| Contacts | `abook.sqlite` | read-only | contact lookup |
| Calendar | `calendar-data/local.sqlite` | read-only | calendar events |
| Live mail | IMAP / SMTP | read+write | bodies, flags, moves, sending |

### Profile discovery

`THUNDERBIRD_PROFILE` takes precedence. Otherwise parse `profiles.ini` and
select the default profile, searching the standard locations for the platform
including the Snap and Flatpak variants. Docker users bind-mount the profile and
set `THUNDERBIRD_PROFILE`.

### Gloda specifics

Gloda's schema has been frozen at version 30 since Thunderbird 11 and is stable.
It uses a rollback journal rather than WAL and does not take an exclusive lock,
so a read-only reader can coexist with a running Thunderbird. We set a generous
`busy_timeout` to absorb `SQLITE_BUSY` during indexing.

The FTS3 table `messagesText` uses Mozilla's custom `mozporter` tokenizer, which
no stock SQLite build can instantiate — any attempt to open that virtual table
errors. We therefore read the shadow content table `messagesText_content`
(`docid, c0body, c1subject, c2attachmentNames, c3author, c4recipients`) directly
and perform matching ourselves.

Joins: `messagesText_content.docid = messages.id`, and
`messages.folderID = folderLocations.id`. We exclude rows with `deleted != 0`
and "ghost" rows with NULL `folderID` or `messageKey`. The `date` column is
microseconds since epoch (PRTime). `headerMessageID` is the RFC Message-ID
without angle brackets.

Two limits shape the design: gloda's indexed body is truncated (roughly 20 KB),
and IMAP message bodies are indexed only when the message is available offline.
Search results are therefore treated as *pointers*; full bodies are always
fetched from IMAP (or read from mbox for local folders).

We never write to this database. Thunderbird nukes and fully reindexes if it
finds an unexpected `user_version`.

### Credentials

`logins.json` entries are decrypted using the key material in `key4.db`:
PBKDF2-HMAC-SHA256 to derive the key, then AES-256-CBC (3DES for older
profiles), with the parameters read from the ASN.1 blob. Implemented in pure Go
using `golang.org/x/crypto`, which is already a dependency.

A profile protected by a master password cannot be decrypted without that
password. In that case the server fails at startup with an explicit message
naming the master password as the cause, rather than surfacing a generic
authentication failure later.

### Message addressing

Messages are addressed by an opaque `message_ref` string of the form
`<folderURI>#<messageKey>`. Search returns these; every other message tool
accepts them. The model never constructs one itself. For IMAP folders
`messageKey` is the UID; for local folders it is the byte offset into the mbox
file.

### Write policy

Local folders — POP accounts, Local Folders, and archives stored as mbox or
maildir — are **read-only**. Writing to them behind a running Thunderbird risks
corrupting the user's mail store. Mutating tools return an explicit error
identifying the folder as local rather than failing silently.

All mutations go through IMAP, where the server is the source of truth and
Thunderbird picks the change up on its next sync.

Contacts and calendar are read-only for the same reason: they are
Thunderbird-owned SQLite files with a live process attached, and `local.sqlite`
is a CalDAV cache whose direct modification would desync the remote server.

### Failure posture

Each data source degrades independently. A missing or unreadable calendar
database disables the calendar tools rather than preventing startup. An
unreachable IMAP server still leaves search and metadata reads working from
gloda.

## Tool surface

### Discovery

- `list_accounts` — accounts with type (imap/pop/local) and their identities, so
  the model knows which addresses it may send as.
- `list_folders` — folder tree per account with unread and total counts.

### Search and read

- `search_messages` — gloda-backed. Filters: `query`, `account`, `folder`,
  `from`, `to`, `subject`, `since`, `until`, `unread_only`, `limit`, `offset`.
  Returns `message_ref`, subject, author, date, snippet.
- `get_message` — full headers, body (`text` or `html`), attachment list.
- `list_recent` — newest N messages, optionally scoped to a folder.

### Mutate (IMAP accounts only)

- `set_flags` — read/unread, flagged, and arbitrary keywords/tags.
- `move_message` — to another folder within the same account.
- `delete_message` — moves to Trash; permanent deletion requires an explicit
  `permanent: true`.

### Compose

- `send_mail` — to/cc/bcc/subject/body, with `from` selecting an identity.
- `reply_message` — quotes the original and sets `In-Reply-To` and `References`.
- `forward_message`
- `save_draft` — IMAP APPEND to the Drafts folder, so the message appears in
  Thunderbird's composer for the user to finish.

### Contacts and calendar (read-only)

- `search_contacts`
- `get_contact`
- `list_calendars`
- `list_events` — over a date range.

## Configuration

| Variable | Default | Effect |
|---|---|---|
| `THUNDERBIRD_PROFILE` | auto-discovered | path to the profile directory |
| `THUNDERBIRD_READ_ONLY` | `false` | when true, mutating and compose tools are not registered |
| `THUNDERBIRD_ALLOW_SEND` | `false` | the four compose tools are registered only when true |

Sending is gated by default because it is the one irreversible, outward-facing
capability in this server. `TKasperczyk/thunderbird-mcp` gates it for the same
reason.

## Dependencies

All new dependencies are CGO-free, preserving the repository's
`CGO_ENABLED=0` build:

- `modernc.org/sqlite` — pure-Go SQLite
- `github.com/emersion/go-imap/v2`
- `github.com/emersion/go-message`
- `github.com/emersion/go-sasl`

## Testing

Ginkgo, matching the existing `jellyfin` suite:

- A fixture profile directory containing a synthetic gloda SQLite database built
  during test setup, exercising the `messagesText_content` join path, ghost-row
  and deleted-row filtering, and PRTime conversion.
- A fixture `prefs.js` covering IMAP, POP, and Local Folders accounts.
- A real `logins.json` / `key4.db` pair with a known password, proving the
  decryption path end to end.
- IMAP and SMTP handlers tested against an in-process fake server, so the suite
  requires no network.
- Explicit coverage that mutating tools reject local folders, and that the
  `THUNDERBIRD_READ_ONLY` and `THUNDERBIRD_ALLOW_SEND` gates control tool
  registration.

## Deliverables

- `thunderbird/` Go package as described.
- A README section matching the format of the other servers, documenting the
  environment variables, the Docker invocation with the required profile
  bind-mount, and the LocalAI model configuration snippet.
- Makefile and CI wiring so `MCP_SERVER=thunderbird` builds and publishes
  `ghcr.io/mudler/mcps/thunderbird`.

## Explicit non-goals

- No Thunderbird extension or add-on of any kind.
- No writes to any Thunderbird-owned file: mbox, maildir, gloda, `abook.sqlite`,
  or `local.sqlite`.
- No OAuth2 support in the first version. Gmail and Outlook accounts using OAuth
  rather than a password are out of scope, and the server reports them clearly
  as unsupported rather than failing obscurely.
- No mail filter management, and no calendar or contact creation.
