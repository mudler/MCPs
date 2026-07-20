#!/bin/sh
# nib launches the browser itself, via chromedp's ExecAllocator, and never
# passes --no-sandbox. Chrome's setuid sandbox cannot start inside this
# container (we run as root, which Chrome refuses to sandbox), so without the
# flag every browser_* tool fails at launch. nib's discoverChrome probes
# /usr/bin/google-chrome first and then /usr/bin/chromium, and chromedp honours
# whichever it finds as the exec path -- so a wrapper installed at those paths
# is the clean fix.
#
# This execs /opt/google/chrome/chrome directly rather than Google's own
# /opt/google/chrome/google-chrome wrapper, which reroutes stdout and stderr
# through `cat` subprocesses. chromedp reads the DevTools websocket URL off
# Chrome's stderr, so that rerouting is worth staying clear of.
exec /opt/google/chrome/chrome --no-sandbox --disable-dev-shm-usage "$@"
