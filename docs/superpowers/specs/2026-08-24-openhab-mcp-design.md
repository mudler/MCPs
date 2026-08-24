# openHAB MCP Server — Design

Date: 2026-08-24
Status: Approved for planning

## Summary

A new MCP server in this repository, `openhab`, exposing read and control tools
for an openHAB home-automation instance over its REST API. It ships as
`ghcr.io/mudler/mcps/openhab`, built by the shared `Dockerfile`, and speaks MCP
over stdio like every other Go server here.

The tool surface is deliberately small: eight tools that let a model answer
questions about the house and act on it. Configuration CRUD — creating items,
things, rules and scripts — is out of scope.

## Motivation

The existing option is `ghcr.io/tdeckers/openhab-mcp`, a Python server exposing
39 tools. It works, but three things make a Go server in this repository the
better fit.

**It cannot talk to a default openHAB installation over HTTPS.** openHAB ships a
self-signed certificate with `CN=openhab.org` and no `subjectAltName`. urllib3 v2
(which the image bundles) does its own hostname matching and, unlike OpenSSL,
will not fall back to the CN when no SAN is present, so verification fails with
`Hostname mismatch` no matter which CA bundle is supplied. The image reads only
six environment variables (`MCP_MODE`, `MCP_TRANSPORT`, `OPENHAB_URL`,
`OPENHAB_API_TOKEN`, `OPENHAB_USERNAME`, `OPENHAB_PASSWORD`) and its client
builds a bare `requests.Session()` that never passes `verify=`, so there is no
knob to turn. The only workarounds are patching `requests` at runtime or
reissuing the certificate. Owning the client makes this a three-line option.

**39 tools is a standing cost.** Every tool schema is sent on every request the
agent makes. Most of that surface is configuration CRUD a model should not be
reaching for in the first place.

**Consistency.** `homeassistant` and `jellyfin` already live here. An openHAB
server belongs beside them, with the same build, the same registry, the same
`docker run` shape, and an Alpine image instead of a Python runtime.

## Non-goals

- **Configuration CRUD.** No create/update/delete for items, things, links,
  rules or scripts. A model with a control surface can turn the lights on; a
  model with a configuration surface can dismantle the house.
- **Item metadata and channel-link management.** Diagnostic surface, not
  control surface. Can be added later if a real need appears.
- **Feature parity with the Python server.** This is not a drop-in replacement
  and should not be described as one.
- **HTTP/SSE transport.** stdio only, matching every other Go server here.

## Tools

Eight tools. Names and openHAB endpoints:

| Tool | Endpoint | Purpose |
|---|---|---|
| `list_items` | `GET /rest/items` | Compact listing with filters and pagination |
| `get_item` | `GET /rest/items/{name}` | Full detail for one item, optional metadata |
| `send_command` | `POST /rest/items/{name}` | Send a command (runs through rules) |
| `update_item_state` | `PUT /rest/items/{name}/state` | Set state directly, bypassing rules |
| `list_things` | `GET /rest/things` | Thing listing with `channels` stripped |
| `get_thing_status` | `GET /rest/things/{uid}/status` | Status, detail and description |
| `list_rules` | `GET /rest/rules` | Rule listing, optional tag filter |
| `run_rule_now` | `POST /rest/rules/{uid}/runnow` | Trigger a rule |

Three details the implementation must get right:

**Command versus state.** `POST /rest/items/{name}` with a `text/plain` body
sends a *command*, which propagates through rules and bindings — this is what
"turn the light on" means. `PUT /rest/items/{name}/state` sets the state without
triggering rules, which is what a sensor update means. The Python server exposes
only the POST and names it `update_item_state`, conflating the two. We expose
both, named for what they do.

**Compact listings.** `list_items` returns `name`, `type`, `state` and `label`
per row, not the full item representation; `get_item` is there when more is
needed. `list_things` drops the `channels` array from every thing, which
otherwise dominates the payload. This follows `EntitySummary` in
`homeassistant/main.go`.

**Client-side filtering and pagination.** openHAB's REST API returns whole
collections. Filters (`filter_tag`, `filter_type`, `filter_name`,
`filter_label`, `filter_uid`) and pagination are applied after the fetch, as the
Python server does. Filters are case-insensitive substring matches; page numbers
are 1-based, and a page past the end returns an empty list rather than an error.

## Configuration

| Variable | Default | Description |
|---|---|---|
| `OPENHAB_URL` | required | Base URL, e.g. `https://10.9.0.26:8443` |
| `OPENHAB_API_TOKEN` | empty | Bearer token; preferred over basic auth |
| `OPENHAB_USERNAME` | empty | Basic-auth user, used when no token is set |
| `OPENHAB_PASSWORD` | empty | Basic-auth password |
| `OPENHAB_TIMEOUT` | `30s` | Bounds every request |
| `OPENHAB_CA_CERT` | empty | Path to a PEM bundle for a private CA |
| `OPENHAB_INSECURE_SKIP_VERIFY` | `false` | Skip TLS verification entirely |
| `OPENHAB_READ_ONLY` | `false` | When true, expose only the read tools |

`OPENHAB_READ_ONLY=true` means `send_command`, `update_item_state` and
`run_rule_now` are never registered on the server — they are absent from
`tools/list`, not merely refused when called. This mirrors `SMB_READ_ONLY` in
the samba server.

Startup fails with a clear message when `OPENHAB_URL` is unset or unparseable,
when neither a token nor a username/password pair is present, and when
`OPENHAB_CA_CERT` names a file that cannot be read or parsed.

## Structure

Modelled on `jellyfin/`, the closest sibling — a REST-backed server with token
auth and a read-plus-control tool mix:

- `main.go` — config load, HTTP client construction, tool registration, stdio
  serve, signal handling.
- `config.go` — environment parsing and validation, including TLS setup.
- `client.go` — the openHAB REST client: one method per endpoint, returning
  typed values. Holds the `*http.Client` and the auth header.
- `types.go` — wire types for items, things and rules, plus the compact forms
  returned by listings, with `jsonschema` tags on every field.
- `handlers.go` — one handler per tool: input validation, a client call, then
  filtering, pagination and compaction.
- `openhab_suite_test.go`, `client_test.go`, `handlers_test.go` — ginkgo suite.

Tools are registered with direct `mcp.AddTool` calls in `main.go`, as
`homeassistant` does. `jellyfin`'s `toolDef` slice plus `registerTool` type
switch exists only to funnel heterogeneous handlers through a single generic
call; direct registration is shorter and keeps the type checking at the call
site.

The client is reached through an interface so handlers can be tested against a
fake, following the `backend` interface in `samba/server.go`.

## Testing

Ginkgo, against an `httptest.Server` returning canned openHAB payloads.

- **Client:** bearer header set when a token is configured, basic auth when it
  is not; `send_command` posts a `text/plain` body; `update_item_state` uses
  `PUT` on the `/state` sub-resource; non-2xx responses become errors carrying
  the status and body; a 404 on `get_item` is distinguishable from a transport
  failure.
- **Config:** each variable parses, defaults apply, invalid values are rejected;
  `OPENHAB_INSECURE_SKIP_VERIFY` and `OPENHAB_CA_CERT` produce the expected
  `tls.Config`; `OPENHAB_READ_ONLY` gates the write tools out of registration.
- **Handlers:** filters match case-insensitively on substrings; pagination
  boundaries; `list_things` strips `channels`; `list_items` returns compact rows.

TLS behaviour is tested against `httptest.NewTLSServer`, whose certificate is
untrusted by default — verification must fail without the knobs and succeed with
either of them.

## Packaging

The shared `Dockerfile` builds `./${MCP_SERVER}/`, so no new Dockerfile is
needed. Registration is one matrix entry in `.github/workflows/image.yml`:

```yaml
          - mcp: openhab
            dockerfile: ./Dockerfile
            context: ./
```

Plus a README section in the house format — environment table, `docker run`
line, and an `mcpServers` JSON snippet — and a `SYSTEM_PROMPT.md` as `jellyfin`
carries, describing when a model should reach for these tools.

## Acceptance

The openHAB instance at `10.9.0.26:8443`, which serves the stock self-signed
certificate, is the acceptance test. With `OPENHAB_INSECURE_SKIP_VERIFY=true`
and a valid API token, `list_items` returns real items (`AlarmTrigger` /
"Sirena", `AutomaticGate`, `AutoShutterers`) and `list_things` reports
`astro:sun:home` as `ONLINE`. Both currently require a runtime monkeypatch
against the Python image; neither should require anything here beyond the
environment variable.
