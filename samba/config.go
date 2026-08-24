package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultPort         = "445"
	defaultTimeout      = 30 * time.Second
	defaultReadMaxBytes = int64(1048576)
)

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
}

// loadConfig reads the configuration from getenv, which is os.Getenv in
// production and a map lookup in the specs.
func loadConfig(getenv func(string) string) (Config, error) {
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

// osGetenv adapts os.Getenv to the signature loadConfig expects.
func osGetenv(key string) string { return os.Getenv(key) }
