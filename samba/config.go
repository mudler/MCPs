package main

import (
	"fmt"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	defaultPort         = "445"
	defaultTimeout      = 30 * time.Second
	defaultReadMaxBytes = int64(1048576)
	defaultToolPrefix   = "samba_"
)

// validPrefix accepts what MCP clients accept in a tool name.
var validPrefix = regexp.MustCompile(`^[A-Za-z0-9_-]*$`)

// Config holds everything the server needs to reach a share and decide which
// tools it is allowed to expose.
type Config struct {
	// Address is the host:port of the SMB server.
	Address string
	// Share is the share name to mount, without the leading slashes.
	Share string

	User     string
	Password string
	Domain   string

	// Timeout bounds the initial dial and every subsequent operation.
	Timeout time.Duration
	// ReadMaxBytes is the largest file the read tool will pull over the wire.
	ReadMaxBytes int64

	// ReadOnly drops the write, move and delete tools.
	ReadOnly bool
	// DisableDelete drops the delete tool and forbids the destructive
	// variants of the surviving tools.
	DisableDelete bool

	// ToolPrefix is prepended to every tool name, so this server's read and
	// write do not collide with another server's.
	ToolPrefix string
}

// loadConfig reads the configuration from lookupEnv, which is os.LookupEnv in
// production and a map lookup in the specs. It reports presence as well as
// value because SMB_TOOL_PREFIX set to empty means "no prefix", which is not
// the same as leaving it unset.
func loadConfig(lookupEnv func(string) (string, bool)) (Config, error) {
	getenv := func(key string) string {
		value, _ := lookupEnv(key)
		return value
	}

	cfg := Config{
		User:         getenv("SMB_USER"),
		Password:     getenv("SMB_PASSWORD"),
		Domain:       getenv("SMB_DOMAIN"),
		Timeout:      defaultTimeout,
		ReadMaxBytes: defaultReadMaxBytes,

		ReadOnly:      isTruthy(getenv("SMB_READ_ONLY")),
		DisableDelete: isTruthy(getenv("SMB_DISABLE_DELETE")),
	}

	host := strings.TrimSpace(getenv("SMB_HOST"))
	if host == "" {
		return Config{}, fmt.Errorf("SMB_HOST is required (the SMB server host, optionally with :port)")
	}
	if _, _, err := net.SplitHostPort(host); err != nil {
		host = net.JoinHostPort(host, defaultPort)
	}
	cfg.Address = host

	cfg.Share = strings.Trim(strings.TrimSpace(getenv("SMB_SHARE")), `/\`)
	if cfg.Share == "" {
		return Config{}, fmt.Errorf("SMB_SHARE is required (the share name, for example Data)")
	}

	if raw := strings.TrimSpace(getenv("SMB_TIMEOUT")); raw != "" {
		timeout, err := time.ParseDuration(raw)
		if err != nil {
			return Config{}, fmt.Errorf("SMB_TIMEOUT is not a valid duration: %w", err)
		}
		if timeout <= 0 {
			return Config{}, fmt.Errorf("SMB_TIMEOUT must be positive, got %s", raw)
		}
		cfg.Timeout = timeout
	}

	cfg.ToolPrefix = defaultToolPrefix
	if raw, ok := lookupEnv("SMB_TOOL_PREFIX"); ok {
		if !validPrefix.MatchString(raw) {
			return Config{}, fmt.Errorf(
				"SMB_TOOL_PREFIX %q may only contain letters, digits, underscores and hyphens", raw)
		}
		cfg.ToolPrefix = raw
	}

	if raw := strings.TrimSpace(getenv("SMB_READ_MAX_BYTES")); raw != "" {
		maxBytes, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return Config{}, fmt.Errorf("SMB_READ_MAX_BYTES is not a valid integer: %w", err)
		}
		if maxBytes <= 0 {
			return Config{}, fmt.Errorf("SMB_READ_MAX_BYTES must be positive, got %s", raw)
		}
		cfg.ReadMaxBytes = maxBytes
	}

	return cfg, nil
}

// isTruthy accepts the spellings people actually put in compose files.
func isTruthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "1", "yes", "on":
		return true
	default:
		return false
	}
}

// osLookupEnv adapts os.LookupEnv to the signature loadConfig expects.
func osLookupEnv(key string) (string, bool) { return os.LookupEnv(key) }
