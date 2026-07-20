#!/bin/bash
set -e

# Replaces the base image's /usr/local/bin/xstartup.sh. Everything above the
# cua-driver block is the original, kept verbatim so the desktop still comes up
# exactly as trycua ships it.

# Start D-Bus
if [ -z "$DBUS_SESSION_BUS_ADDRESS" ]; then
    eval $(dbus-launch --sh-syntax --exit-with-session)
fi

# Start XFCE
startxfce4 &

# Wait for XFCE to start
sleep 2

# Disable screensaver and power management
xset s off
xset -dpms
xset s noblank

# `cua-driver mcp` -- which the MCP server spawns per session -- is only a
# stdio client; it refuses to start unless a `cua-driver serve` daemon is
# already listening on ~/.cache/cua-driver/cua-driver.sock. The daemon has to
# live *here*, in the desktop session, rather than as its own supervisord
# program: launching it from this script is what gives it the session bus
# created above, and without that bus it cannot advertise accessibility to the
# session -- which is how Chromium's AT-SPI tree ends up empty.
#
# It runs as `cua`, so the socket and the a11y bus are both cua-owned; the
# root-side `cua-driver mcp` still reaches them (root bypasses the socket's
# file mode, and at-spi2's accessibility.conf carries an explicit
# `<allow user="root"/>`).
cua-driver serve &

# Wait for the session
wait
