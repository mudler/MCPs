#!/bin/bash
# Respawn loop for the `cua-driver serve` daemon.
#
# Every computer_use and browser_* call routes through `cua-driver mcp`, which
# is only a stdio *client* of this daemon. If the daemon dies, nothing else
# notices: supervisord still has XFCE, TigerVNC and noVNC up, the container
# keeps looking healthy, and every tool call fails for the rest of its life.
# So the daemon gets supervised.
#
# It is supervised from here, invoked by xstartup.sh, rather than as its own
# supervisord program, because it needs the session bus that xstartup.sh
# creates -- without that bus it cannot advertise accessibility to the session,
# and Chromium's AT-SPI tree ends up empty. A supervisord program would run
# outside that session and lose the property.
#
# stdout is the MCP JSON-RPC channel, so every byte the daemon emits goes to
# this log file instead. Nothing here may write to stdout.
set -u

LOG=${CUA_DRIVER_LOG:-/tmp/cua-driver-serve.log}

log() {
    echo "[$(date -Is)] supervise: $*" >>"$LOG"
}

log "starting supervision of cua-driver serve"

while true; do
    cua-driver serve >>"$LOG" 2>&1
    status=$?
    log "cua-driver serve exited with status ${status}; restarting in 1s"
    sleep 1
done
