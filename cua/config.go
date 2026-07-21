package main

import (
	"os"
	"strings"
	"time"

	"github.com/mudler/nib/types"
)

const defaultReadyTimeout = 60 * time.Second

// Config is the fully resolved runtime configuration for the cua server.
type Config struct {
	// NibConfig is handed to nib's computer and browser MCP servers.
	NibConfig types.Config
	// ToolAllowlist filters the aggregated tool list. Empty means no filter.
	ToolAllowlist map[string]bool
	// ReadyTimeout bounds the startup readiness gate.
	ReadyTimeout time.Duration
	// DriverCmd and DriverArgs locate cua-driver, both for nib and for our
	// own readiness probe.
	DriverCmd  string
	DriverArgs []string
}

// envBool reports the boolean value of key, or def when unset or unparseable.
func envBool(key string, def bool) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "":
		return def
	case "true", "1", "yes":
		return true
	case "false", "0", "no":
		return false
	default:
		return def
	}
}

// parseToolAllowlist splits a comma-separated allowlist. An empty value or
// "all" means no filtering, matching the THUNDERBIRD_TOOLS convention.
func parseToolAllowlist(raw string) map[string]bool {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "all" {
		return nil
	}
	set := map[string]bool{}
	for _, part := range strings.Split(raw, ",") {
		if name := strings.TrimSpace(part); name != "" {
			set[name] = true
		}
	}
	return set
}

// LoadConfig reads the CUA_* environment variables into a Config.
func LoadConfig() Config {
	driverCmd := os.Getenv("CUA_DRIVER_CMD")
	if driverCmd == "" {
		driverCmd = "cua-driver"
	}
	driverArgs := []string{"mcp"}

	readyTimeout := defaultReadyTimeout
	if raw := os.Getenv("CUA_READY_TIMEOUT"); raw != "" {
		// A zero or negative budget would make every readiness deadline
		// already-expired, so treat it as "unset" rather than "no time".
		if d, err := time.ParseDuration(raw); err == nil && d > 0 {
			readyTimeout = d
		}
	}

	return Config{
		NibConfig: types.Config{
			Computer: types.ComputerConfig{
				Enabled: envBool("CUA_ENABLE_COMPUTER", true),
				Command: driverCmd,
				Args:    driverArgs,
			},
			Browser: types.BrowserConfig{
				Enabled:          envBool("CUA_ENABLE_BROWSER", true),
				ChromePath:       os.Getenv("CUA_CHROME_PATH"),
				ProfileDir:       os.Getenv("CUA_BROWSER_PROFILE_DIR"),
				AllowPrivateURLs: envBool("CUA_ALLOW_PRIVATE_URLS", false),
			},
		},
		ToolAllowlist: parseToolAllowlist(os.Getenv("CUA_TOOLS")),
		ReadyTimeout:  readyTimeout,
		DriverCmd:     driverCmd,
		DriverArgs:    driverArgs,
	}
}
