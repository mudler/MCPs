# CUA MCP Server — Design

Date: 2026-07-20
Status: Approved for planning

## Summary

A new MCP server in this repository, `cua`, exposing computer-use (desktop) and
browser-control tools backed by a containerised XFCE desktop. It ships as a
single Docker image built on `trycua/cua-xfce`, with our Go binary as the
container's stdio entrypoint and the desktop viewable live over noVNC.

The tool implementations are not written here. They are imported from
`github.com/mudler/nib` (the module in `~/_git/wiz`), which already implements
both toolsets and carries a large amount of hard-won behavioural tuning.

## Motivation

LocalAI and other MCP clients have no turnkey way to give a model a real
desktop. The pieces exist but do not fit together:

- `trycua/cua-xfce` provides the desktop, but its in-container Python
  `computer-server` has `get_accessibility_tree` and `find_element` **stubbed
  out on Linux**, so it can only offer pixel-level control.
- `cua-driver` (trycua's Rust daemon, MCP over stdio) *does* have a first-class
  Linux AT-SPI backend and returns indexed elements, but it is distributed as a
  host binary and is not in the container image.
- `nib` wraps `cua-driver` and Chromium with the ergonomics a model actually
  needs, but is packaged as a desktop application, not as a standalone MCP
  server.

This server composes the three into one image that a user can run with a single
`docker run`, consistent with every other server in this repository.

### Why not just use the container's own `/mcp` endpoint

`trycua/cua-xfce` mounts an MCP server at `/mcp` exposing `computer_screenshot`,
`computer_click`, and friends. We do not use it because it offers raw pixel
clicking with no element indices (its AX tree is stubbed on Linux), no browser
tools, no destructive-action safety gate, and no packaging consistent with this
repository.

## Non-goals

- **Container lifecycle management.** The server does not start, stop, or
  supervise Docker containers. It runs *inside* the container.
- **Reimplementing the tool surface.** We expose nib's tools unchanged. We add
  no tool schemas of our own.
- **Supporting the stock `trycua/cua-xfce` image unmodified.** Our image adds
  packages that are required for the design to work.
- **macOS or Windows targets.** This is a Linux container image.

## Architecture

```
docker run -i --rm -p 6901:6901 ghcr.io/mudler/mcps/cua
   |
   +- supervisord (background)
   |    +- TigerVNC        :5901
   |    +- noVNC           :6901   <- live view of the desktop
   |    +- XFCE on DISPLAY=:1
   |    +- session D-Bus
   |
   +- cua (our Go binary, stdio MCP, foreground)
        +- nib StartComputerMCPServer -> spawns `cua-driver mcp` (stdio child, AT-SPI on :1)
        +- nib StartBrowserMCPServer  -> launches headed Chromium on :1 (chromedp)
```

Chromium runs *on the XFCE desktop*, not headless. This is deliberate: the
browser window is visible over noVNC and is also reachable by `computer_use`, so
the two toolsets compose rather than living in separate worlds.

### Components

Our binary has three responsibilities and nothing else.

**1. Configuration.** Read environment variables into nib's `types.Config`,
setting `Computer.Enabled` and `Browser.Enabled`. No CLI flags, matching the
house style of this repository.

**2. Tool aggregation.** nib's `StartComputerMCPServer` and
`StartBrowserMCPServer` each construct their own `mcp.Server` bound to a
transport passed in:

```go
func StartComputerMCPServer(ctx context.Context, transport mcp.Transport, cfg types.Config) error
func StartBrowserMCPServer(ctx context.Context, transport mcp.Transport, cfg types.Config) error
```

Two servers cannot share one stdio transport, so we follow the same pattern
nib's own `StartTransports` uses: start each on an `mcp.NewInMemoryTransports()`
pair, connect a client session to each, and expose a **single** stdio server
that merges their tool lists and forwards `CallTool` by name.

The aggregator must forward tool *results verbatim*, including
`mcp.ImageContent`. Screenshots are the primary output of both toolsets; a
forwarder that stringifies content would silently destroy the server's value.

We deliberately do not start nib's `bash`, `filesystem`, or `web` servers. This
repository already ships `shell` and `duckduckgo` for that.

**3. Readiness gating.** Block startup until the desktop is usable, so the first
tool call is not a race against XFCE booting. The check is layered:

- wait for `DISPLAY=:1` to accept connections;
- wait for `cua-driver`'s `health_report` to report an AT-SPI capability.

If AT-SPI never becomes available within the timeout, start anyway but log a
prominent warning to stderr: element-index addressing will be unavailable and
`computer_use` will degrade to pixel-only. This is a degradation, not a failure.

### Dependency on nib

We import `github.com/mudler/nib` at a **pseudo-version**, not a tag. The
`computer.go` and `browser.go` files landed after `v0.4.1`; the latest tag does
not contain them. `nib-desktop` pins a pseudo-version for the same reason.

Verified: nib's `mcp` package compiles against this repository's
`go-sdk v1.4.0` even though nib itself pins `v1.0.0`, so Go's minimal version
selection resolving upward to v1.4.0 is safe.

**Accepted cost:** this pulls roughly 300 modules into the repository's shared
`go.mod` — bubbletea, wazero/pdfium, langchaingo, go-openai — none of which the
`cua` server uses. This is a known and accepted trade for not forking nib's
tuning. If it becomes painful, the exit is to split `cua` into its own module.

## Image

Built `FROM trycua/cua-xfce:latest`, published as `ghcr.io/mudler/mcps/cua`.
Because it needs a base image and extra packages, it uses a dedicated
`cua/Dockerfile` rather than the repository's shared root `Dockerfile`, with the
`MCP_SERVER=cua` override wired into the `Makefile` alongside the existing
`opencode` override.

Additions to the base image:

**`at-spi2-core` and a session D-Bus.** Without both, the AT-SPI tree is empty,
`get_window_state` returns no elements, and the whole element-index design
collapses to pixel-only. This is the highest-risk item in the build and must be
verified explicitly, not assumed. trycua's own CI establishes the recipe: it
runs its Linux E2E suite under `xvfb-run` with `dbus-run-session` and
`at-spi2-core` installed.

**`cua-driver`**, the Linux release binary from trycua's GitHub releases,
version-pinned in the Dockerfile. Its glibc floor is 2.31 and the base image is
Debian Bookworm, so it runs. Distribution is via GitHub releases or PyPI; it is
not on crates.io.

**`chromium`, plus a wrapper script at `/usr/bin/chromium`** that execs the real
binary with `--no-sandbox` appended. Chromium will not start as a non-root user
in a container without it, and nib does not pass the flag. A wrapper is the
clean fix because nib's `discoverChrome` probes `/usr/bin/chromium` and chromedp
honours the resulting `ExecPath`. The real binary is moved aside (for example to
`/usr/lib/chromium/chromium-real`) and the wrapper takes its path.

**Entrypoint script.** Starts `supervisord` in the background, waits for the
display, then `exec`s our binary so it owns stdin/stdout as PID 1's foreground
child and signals propagate correctly.

### Ports

`6901` (noVNC) is the port users publish, giving a live browser-based view of
what the model is doing. `5901` (raw VNC) is exposed by the base image and may
be published for a native VNC client. Neither is required for the MCP server to
function — they are observability.

The base image's `VNC_PW` and `VNCOPTIONS` environment variables are passed
through unchanged.

## Tool surface

Exactly what nib exposes, unchanged:

- **`computer_use`** — a single action-dispatched tool. Actions: `capture`,
  `click`, `double_click`, `right_click`, `middle_click`, `drag`, `scroll`,
  `type`, `key`, `set_value`, `wait`, `list_apps`, `open_app`, `close_app`,
  `focus_app`. Targets are addressed by `element` (1-based index from the last
  capture) or by `coordinate` pixels.
- **`browser_navigate`, `browser_snapshot`, `browser_click`, `browser_type`,
  `browser_press`, `browser_scroll`, `browser_vision`** — ref-based web control
  over a Playwright-style aria snapshot, where interactive nodes get `@eN` refs.

Behaviour we inherit and specifically want: the destructive-action safety gate
(`IsDestructiveComputerAction`, `BlockedComputerReason`), AT-SPI tree settling,
field-aware typing checks, the app-switch guard, action and key-name aliases,
the model-facing numbered element list, reliable Enter/submit handling, and the
SSRF guard on navigation.

## Configuration

Environment variables only, no CLI flags.

| Variable | Default | Effect |
| --- | --- | --- |
| `CUA_ENABLE_COMPUTER` | `true` | Register `computer_use` |
| `CUA_ENABLE_BROWSER` | `true` | Register `browser_*` |
| `CUA_DRIVER_CMD` | `cua-driver` | Path to the driver binary |
| `CUA_CHROME_PATH` | auto-discover | Overrides `discoverChrome` |
| `CUA_BROWSER_PROFILE_DIR` | nib default | Persistent Chromium profile |
| `CUA_ALLOW_PRIVATE_URLS` | `false` | Keeps nib's SSRF guard on |
| `CUA_TOOLS` | all | Comma-separated tool allowlist |
| `CUA_READY_TIMEOUT` | `60s` | Readiness gate budget |

`CUA_TOOLS` follows the `THUNDERBIRD_TOOLS` convention: empty or `all` means no
filter. Filtering happens in the aggregator, which drops non-allowlisted tools
from the merged list.

## Error handling

- **Driver spawn failure** — fail startup with an actionable message naming the
  resolved binary path. The server is useless without it.
- **AT-SPI unavailable** — degrade to pixel-only with a loud stderr warning, do
  not fail. nib already emits a `noAXHint` to the model in this case.
- **Chromium launch failure** — fail the first `browser_*` call with the
  underlying error, not at startup. Desktop control should still work.
- **Tool call errors** — forwarded verbatim from nib. We add no error wrapping;
  nib's messages are already written to steer the model toward recovery.

## Testing

Ginkgo and Gomega, matching this repository's existing servers.

**Unit, no container required:**

- Environment parsing into `types.Config`, including defaults and the
  `CUA_TOOLS` allowlist.
- Aggregator: merged tool listing, name-based dispatch, unknown-tool error, and
  a test asserting `ImageContent` survives forwarding intact. This last one is
  the regression guard for the failure mode that would quietly gut the server.
- Readiness gate: reaches ready, times out, and degrades on missing AT-SPI —
  driven against a fake driver session rather than a real one.

**Integration, container required, opt-in via a build tag or an env guard so
default `go test ./...` stays fast and hermetic:**

- Build the image, run it, and assert `tools/list` returns both toolsets.
- `computer_use` with `action: capture` returns an image *and* a non-empty
  element list — this is the test that proves AT-SPI is actually wired up, and
  is the one most worth having.
- `browser_navigate` to a local fixture returns a snapshot containing `@e` refs.

## Known limitations

These are inherited, understood, and documented rather than worked around.

**Screenshot sizing.** nib hardcodes `max_image_dimension = 0` on Linux, working
around a Wayland grayscale-resize bug in the driver that does not apply to our
X11 container. We therefore get native-resolution PNGs. At the base image's
default `VNC_RESOLUTION=1024x768` this is roughly 1.1k vision tokens, which is
fine. Raising the resolution raises capture cost proportionally and we cannot
configure it away without patching nib. Documented; the mitigation is to keep
the default resolution.

**Input fidelity.** `/dev/uinput` is absent in a container, so cua-driver falls
back to its `XSendEvent` path. Per its own source comments, right, middle, and
double clicks may not register on GTK and Qt toolkits that drop synthetic
pointer events. Clicks by `element_index` go through AT-SPI actions and are
unaffected — which is the practical reason element-based addressing, and
therefore `at-spi2-core`, matters so much here.

**Firefox ESR is present but unused.** The base image ships it. Our browser
tools drive Chromium. Firefox remains launchable via `computer_use` if a user
wants it, but gets no ref-based control.

## Open risks

1. **AT-SPI inside the container is the load-bearing assumption.** trycua's CI
   proves the pattern works headlessly, but not in this specific image. The
   integration test asserting a non-empty element list is what converts this
   from assumption to verified fact, and should be built early.
2. **Pseudo-version pinning to nib** means upstream changes to unexported
   internals can break us without a version signal. Mitigated by pinning
   exactly and upgrading deliberately.
3. **Dependency weight** on the shared `go.mod`, as described above.
