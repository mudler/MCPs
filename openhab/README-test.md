# Running the openHAB acceptance check

The specs in this package are hermetic: `go test ./openhab/` drives the tool
handlers against a fake HTTP server and a `httptest.NewTLSServer` for the TLS
paths. They prove the code is correct in isolation. This document is the
manual acceptance check that proves the server works against a real openHAB
instance, including the TLS path that motivated the project.

**This talks to a real house.** Point it at a throwaway or a real instance you
control, and never send a command to an item you have not personally checked
first — the read-only steps below are safe to run against any instance, but
sending a command is not included here on purpose (see "What this does not
cover").

## Environment variables

| Variable | Purpose |
| --- | --- |
| `OPENHAB_URL` | The openHAB base URL, for example `https://your-instance:8443`. |
| `OPENHAB_API_TOKEN` | An openHAB API token. Alternatively set `OPENHAB_USERNAME` and `OPENHAB_PASSWORD`. |
| `OPENHAB_INSECURE_SKIP_VERIFY` | Set to `true` to skip TLS certificate verification. openHAB's stock self-signed certificate carries no `subjectAltName`, so no CA bundle can validate it against an IP address; this is the escape hatch for that. Unset by default, so verification is on. |
| `OPENHAB_READ_ONLY` | Set to `true` to drop `send_command`, `update_item_state` and `run_rule_now` from the tool list entirely. |
| `OPENHAB_TOOL_PREFIX` | Prepended to every tool name, default `openhab_`. Set it to an explicit empty string to get bare tool names (`list_items` instead of `openhab_list_items`). |
| `OPENHAB_CA_CERT` | Path to a PEM bundle, for an instance behind a private CA instead of openHAB's stock cert. |
| `OPENHAB_TIMEOUT` | Per-request timeout, a Go duration string. Defaults to 30s. |

Never put `OPENHAB_API_TOKEN` (or a password) in a file that gets committed.
Export it in the shell for the duration of the check instead.

## Build the image

Build from the current tree before testing — an older image predates any
recent change to the tool names or config parsing:

```bash
make MCP_SERVER=openhab build
```

## The pipe

MCP over stdio speaks newline-delimited JSON-RPC on stdin/stdout. The
straightforward way to drive it is:

```bash
printf '%s\n' \
 '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"cli","version":"0"}}}' \
 '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
 '{"jsonrpc":"2.0","id":2,"method":"tools/list"}' \
 '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"openhab_list_items","arguments":{"page_size":5}}}' \
 '{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"openhab_list_things","arguments":{}}}' \
| docker run -i --rm \
    -e OPENHAB_URL \
    -e OPENHAB_API_TOKEN \
    -e OPENHAB_INSECURE_SKIP_VERIFY=true \
    ghcr.io/mudler/mcps/openhab:latest
```

**Caveat found while running this check:** `printf | docker run -i` closes
stdin (EOF) the instant `printf` finishes writing, which can race the
container's startup and the stdio transport's first read. When that race is
lost the server reports `server is closing: EOF` and exits having produced no
JSON-RPC output at all, even though every message was written correctly. This
is not specific to the prefix or TLS changes; it reproduces with a bare
`tools/list` request and with the binary run directly, outside Docker. If a
run comes back empty, that is this race, not a server bug — rerun, or use a
FIFO with short delays between messages so the container has time to start
reading before stdin closes:

```bash
mkfifo /tmp/openhab-mcp-in
docker run -i --rm \
    -e OPENHAB_URL \
    -e OPENHAB_API_TOKEN \
    -e OPENHAB_INSECURE_SKIP_VERIFY=true \
    ghcr.io/mudler/mcps/openhab:latest < /tmp/openhab-mcp-in &
sleep 1
{
  echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"cli","version":"0"}}}'
  sleep 0.5
  echo '{"jsonrpc":"2.0","method":"notifications/initialized"}'
  sleep 0.5
  echo '{"jsonrpc":"2.0","id":2,"method":"tools/list"}'
  sleep 1
} > /tmp/openhab-mcp-in
wait
rm /tmp/openhab-mcp-in
```

## What each step proves

- **Step 1 — real instance, `OPENHAB_INSECURE_SKIP_VERIFY=true`.** Run the
  pipe above. Expect `tools/list` to report eight tools, all prefixed
  (`openhab_list_items`, `openhab_get_item`, `openhab_send_command`,
  `openhab_update_item_state`, `openhab_list_things`,
  `openhab_get_thing_status`, `openhab_list_rules`, `openhab_run_rule_now`),
  `openhab_list_items` to return real items with real states, and
  `openhab_list_things` to return real things with their status. This proves
  the server reaches a real openHAB instance over its self-signed certificate
  and that the tool prefix is applied.

- **Step 2 — read-only mode.** Re-run with `-e OPENHAB_READ_ONLY=true` added.
  Expect `tools/list` to report five tools, with `openhab_send_command`,
  `openhab_update_item_state` and `openhab_run_rule_now` absent from the list
  — not present-and-refusing, genuinely not registered.

- **Step 3 — TLS verification is on by default.** Re-run the first pipe
  without `OPENHAB_INSECURE_SKIP_VERIFY`. Expect the `tools/call` for
  `openhab_list_items` to come back as a normal JSON-RPC result whose payload
  carries `"success":false` and a certificate error (an `x509` complaint about
  missing IP SANs) — proof the skip flag is doing real work and is not the
  default.

- **Escape hatch — `OPENHAB_TOOL_PREFIX=` (explicitly empty).** Re-run the
  first pipe with `-e OPENHAB_TOOL_PREFIX=` added. Expect `tools/list` to
  report the same eight tools under their bare names (`list_items`,
  `get_item`, `send_command`, `update_item_state`, `list_things`,
  `get_thing_status`, `list_rules`, `run_rule_now`).

## What this does not cover

Sending an actual command to an item and reading the state change back
(`send_command` followed by `get_item`) is a real acceptance step this
procedure does not automate, because it actuates real hardware. Picking a
safe item to toggle is a judgment call for whoever owns the instance, not
something to script. When you do it by hand: pick an item you can see and
verify (a lamp, not something like a siren or a gate), send it a command with
`openhab_send_command`, then confirm the new state with `openhab_get_item`.
