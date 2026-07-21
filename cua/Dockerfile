# The go directive in go.mod is 1.26 (nib requires it), so this cannot be
# parameterised by the Makefile's GO_VERSION build arg. Pinned directly,
# matching the root Dockerfile; the unused build arg is ignored by Docker.
FROM golang:1.26-alpine AS builder

WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 go build -o /out/cua ./cua/

# Ubuntu 22.04 (jammy) under the hood, running TigerVNC, noVNC and XFCE from
# supervisord. glibc 2.35 clears cua-driver's 2.31 floor.
FROM trycua/cua-xfce:latest

USER root

ARG CUA_DRIVER_VERSION=0.9.1
# amd64 in practice: trycua publishes cua-xfce for linux/amd64 only, and Google
# ships no linux/arm64 Chrome deb. The arch mapping below is kept honest rather
# than load-bearing -- an arm64 build fails at the Chrome apt step, not silently.
ARG TARGETARCH=amd64

# at-spi2-core and dbus-x11 are what make element-index addressing work: without
# them cua-driver reports an empty accessibility tree and computer_use falls
# back to pixel coordinates. Both already ship in the base image, so these are
# assertions rather than installs -- they keep the dependency explicit if the
# base ever drops them.
#
# Google Chrome comes from Google's own signed apt repository because jammy has
# no usable chromium: the `chromium` package does not exist there and
# `chromium-browser` is a transitional stub that redirects to snap, which cannot
# run in a container. nib probes /usr/bin/google-chrome before /usr/bin/chromium
# so this satisfies its discovery either way.
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        at-spi2-core \
        dbus-x11 \
        ca-certificates \
        curl \
        gnupg \
    && curl -fsSL https://dl.google.com/linux/linux_signing_key.pub \
        | gpg --dearmor -o /usr/share/keyrings/google-chrome.gpg \
    && echo "deb [arch=${TARGETARCH} signed-by=/usr/share/keyrings/google-chrome.gpg] https://dl.google.com/linux/chrome/deb/ stable main" \
        > /etc/apt/sources.list.d/google-chrome.list \
    && apt-get update \
    && apt-get install -y --no-install-recommends google-chrome-stable \
    && rm -rf /var/lib/apt/lists/*

# Chrome will not start sandboxed as root in a container and nib does not pass
# --no-sandbox, so the wrapper takes over both paths nib's discoverChrome
# probes. /usr/bin/google-chrome arrives from the deb as an update-alternatives
# symlink; pointing it at the wrapper instead is deliberate.
#
# google-chrome-stable is covered too. nib does not probe that name, so nothing
# depends on it today -- but it is the name the deb advertises and the one a
# human or a future caller reaches for, and leaving it pointed at the unwrapped
# binary means that caller gets no --no-sandbox and an opaque launch failure.
COPY cua/chromium-wrapper.sh /usr/local/lib/chromium-wrapper.sh
RUN chmod +x /usr/local/lib/chromium-wrapper.sh \
    && ln -sf /usr/local/lib/chromium-wrapper.sh /usr/bin/google-chrome \
    && ln -sf /usr/local/lib/chromium-wrapper.sh /usr/bin/chromium \
    && ln -sf /usr/local/lib/chromium-wrapper.sh /usr/bin/google-chrome-stable

# The apt repo above is unpinned, so record what this build actually shipped;
# otherwise "which Chrome was in that image" stops being answerable. Running it
# through the wrapper symlink doubles as a build-time check that the wrapper
# really does launch Chrome.
RUN google-chrome-stable --version > /etc/cua-chrome-version \
    && cat /etc/cua-chrome-version

# cua-driver supplies the AT-SPI element tree and input injection. The
# `-binary` release asset unpacks a bare `cua-driver` executable.
RUN case "${TARGETARCH}" in \
        amd64) CUA_ARCH=x86_64 ;; \
        arm64) CUA_ARCH=arm64 ;; \
        *) echo "unsupported TARGETARCH: ${TARGETARCH}" >&2; exit 1 ;; \
    esac \
    && curl -fsSL -o /tmp/cua-driver.tar.gz \
        "https://github.com/trycua/cua/releases/download/cua-driver-rs-v${CUA_DRIVER_VERSION}/cua-driver-rs-${CUA_DRIVER_VERSION}-linux-${CUA_ARCH}-binary.tar.gz" \
    && tar -xzf /tmp/cua-driver.tar.gz -C /usr/local/bin \
    && rm /tmp/cua-driver.tar.gz \
    && chmod +x /usr/local/bin/cua-driver

# Opt out of the driver's default-on telemetry and its start-up update check,
# which would otherwise call GitHub on every container start. The file is the
# same shape `cua-driver telemetry disable` writes; it is written directly
# because `cua-driver config set` needs a running daemon.
RUN install -d -o cua -g cua /home/cua/.cua-driver \
    && printf '{\n  "telemetry_enabled": false,\n  "update_check_enabled": false\n}\n' \
        > /home/cua/.cua-driver/config.json \
    && chown cua:cua /home/cua/.cua-driver/config.json

# The cua-driver daemon has to start inside the desktop session -- see
# cua-driver-supervise.sh for why -- which means hooking the base image's
# xstartup.sh.
#
# This *injects* a line rather than shipping our own copy of that file. A
# verbatim copy would silently win over any future change trycua makes to their
# xstartup (a new program, different bus handling, a different XFCE
# invocation): no build error, no runtime error, just a desktop quietly drifted
# from what the base image expects. Injection inverts that failure mode. The
# base's xstartup ends with exactly one bare `wait`, and if a future base image
# no longer does -- none, or more than one, since sed substitutes globally and
# two anchors would race two daemons over one socket -- the count check takes
# the build down with a message: loudly, at build time, instead of silently at
# runtime. This matters more than usual
# because the FROM above is unpinned `:latest`.
COPY cua/cua-driver-supervise.sh /usr/local/bin/cua-driver-supervise
RUN chmod +x /usr/local/bin/cua-driver-supervise \
    && { [ "$(grep -cx 'wait' /usr/local/bin/xstartup.sh)" = 1 ] \
        || { echo "ERROR: expected exactly one bare 'wait' line in /usr/local/bin/xstartup.sh" >&2; \
             echo "to anchor to; found $(grep -cx 'wait' /usr/local/bin/xstartup.sh)." >&2; \
             echo "The base image's xstartup.sh has changed shape; re-check the injection below." >&2; \
             echo "Note sed substitutes globally: two anchors would start two daemons." >&2; \
             exit 1; }; } \
    && sed -i 's|^wait$|/usr/local/bin/cua-driver-supervise \&\nwait|' /usr/local/bin/xstartup.sh \
    && [ "$(grep -cx '/usr/local/bin/cua-driver-supervise &' /usr/local/bin/xstartup.sh)" = 1 ]

COPY --from=builder /out/cua /usr/local/bin/cua
COPY cua/entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

ENV DISPLAY=:1

# Stays root, unlike most images here: supervisord owns /var/log/supervisor and
# /var/run/supervisor.sock and drops to the `cua` user per program, which it
# can only do as root. The base image runs as root for the same reason.

EXPOSE 5901 6901

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
