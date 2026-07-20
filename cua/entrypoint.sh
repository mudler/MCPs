#!/bin/bash
set -euo pipefail

# Deliberately NO dbus-launch here.
#
# cua-driver reads the AT-SPI tree, and reaching it depends on who owns which
# bus. Measured against the real base image:
#
#   * The desktop's /usr/local/bin/xstartup.sh runs as the `cua` user and
#     launches the session bus itself -- but only when DBUS_SESSION_BUS_ADDRESS
#     is unset. XFCE then starts at-spi-bus-launcher and at-spi2-registryd as
#     `cua`, with no help from us.
#   * We run as root (supervisord requires it). A session bus started here
#     would be root-owned, and dbus-daemon's default policy admits only the
#     owning uid -- so `cua` could not connect, XFCE's accessibility startup
#     fails silently, and the tree ends up empty. That is the exact failure
#     this comment exists to prevent.
#   * Root does not need the session bus. at-spi2's client library resolves the
#     accessibility bus from the AT_SPI_BUS property on the X root window,
#     which at-spi-bus-launcher publishes, and /usr/share/defaults/at-spi2/
#     accessibility.conf carries an explicit `<allow user="root"/>` so root may
#     join the cua-owned a11y bus. Root reads the X display via
#     /home/cua/.Xauthority (HOME is /home/cua for root in this image).
#
# So the invariant is simply: leave DBUS_SESSION_BUS_ADDRESS unset and let the
# desktop own its bus. An inherited value would be handed to xstartup.sh and
# reintroduce the split, so clear it rather than trust the caller.
unset DBUS_SESSION_BUS_ADDRESS

# Start the desktop stack (TigerVNC :5901, noVNC :6901, XFCE on :1) in the
# background. Its logs go to stderr so they never corrupt the MCP stdio stream.
supervisord -c /etc/supervisor/supervisord.conf >&2 &

# Wait for the cua-driver daemon that xstartup.sh launches inside the desktop
# session. The server's own readiness gate waits for the X display, but that is
# not enough: Xtigervnc publishes its socket within about a second, while XFCE
# and then the daemon need appreciably longer. Without this wait the gate spawns
# `cua-driver mcp`, which exits immediately because no daemon is listening yet,
# and startup fails roughly two seconds in.
#
# Bounded, and deliberately not fatal: if the daemon never arrives, fall through
# and let the server's gate report the failure in its own terms.
# Derived from HOME rather than hardcoded: the daemon builds this path as
# $HOME/.cache/cua-driver/, so deriving it keeps the two in step. HOME is
# /home/cua for root in this image, which is why the literal path worked -- but
# if a future base image changes it, a hardcoded path would not fail, it would
# just wait the full 120s and then fall through, which reads as a slow start
# rather than a broken one.
driver_sock="${HOME:-/home/cua}/.cache/cua-driver/cua-driver.sock"
for _ in $(seq 120); do
  [ -S "$driver_sock" ] && break
  sleep 1
done

# Say so if it never arrived. Without this the loop is silent and the first
# symptom is a generic `initialize: EOF` from the server, which points nowhere
# near the real cause. stderr, never stdout -- stdout is the MCP stdio channel.
[ -S "$driver_sock" ] || echo "warning: cua-driver socket ${driver_sock} not present after 120s" >&2

# exec so the MCP server owns stdin/stdout and receives signals directly.
exec /usr/local/bin/cua "$@"
