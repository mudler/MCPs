#!/bin/sh
# nib launches the browser itself, via chromedp's ExecAllocator, and never
# passes --no-sandbox. Chrome's setuid sandbox cannot start inside this
# container (we run as root, which Chrome refuses to sandbox), so without the
# flag every browser_* tool fails at launch. nib's discoverChrome probes
# /usr/bin/google-chrome first and then /usr/bin/chromium, and chromedp honours
# whichever it finds as the exec path -- so a wrapper installed at those paths
# is the clean fix.
#
# Running Chrome as root with --no-sandbox is an accepted posture here, not an
# oversight. This image is a single-purpose, throwaway desktop sandbox: the
# container *is* the security boundary, and everything in it (supervisord, the
# MCP server, the desktop) already runs as root because supervisord requires it
# to drop privileges per program. Dropping Chrome to an unprivileged user would
# let it keep its own sandbox, but it would not add a boundary that the
# container does not already provide, and it would cost the shared X session,
# the a11y bus ownership and the AT-SPI tree that cua-driver reads. The
# trade is deliberate: the browser is only ever pointed at whatever the
# operator's agent drives it to.
#
# This execs /opt/google/chrome/chrome directly rather than Google's own
# /opt/google/chrome/google-chrome wrapper, which reroutes stdout and stderr
# through `cat` subprocesses. chromedp reads the DevTools websocket URL off
# Chrome's stderr, so that rerouting is worth staying clear of.
exec /opt/google/chrome/chrome --no-sandbox --disable-dev-shm-usage "$@"
