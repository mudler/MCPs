package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

const (
	defaultTimeout    = 30 * time.Second
	defaultToolPrefix = "openhab_"
)

// validPrefix accepts what MCP clients accept in a tool name.
var validPrefix = regexp.MustCompile(`^[A-Za-z0-9_-]*$`)

// Config holds everything the server needs to reach an openHAB instance and
// decide which tools it is allowed to expose.
type Config struct {
	// BaseURL is the openHAB root, scheme included, without a trailing slash.
	BaseURL string

	// Token is an openHAB API token, preferred over basic auth when both are
	// supplied.
	Token    string
	Username string
	Password string

	// Timeout bounds every request.
	Timeout time.Duration

	// CACertPath points at a PEM bundle for an instance behind a private CA.
	CACertPath string
	// InsecureSkipVerify drops certificate verification. openHAB's stock
	// certificate carries no subjectAltName, so no CA bundle can validate it.
	InsecureSkipVerify bool

	// ReadOnly drops the command, state and rule-run tools.
	ReadOnly bool

	// ToolPrefix is prepended to every tool name, so this server's tools do
	// not collide with another server's.
	ToolPrefix string
}

// loadConfig reads the configuration from lookupEnv, which is os.LookupEnv in
// production and a map lookup in the specs. It reports presence as well as
// value because OPENHAB_TOOL_PREFIX set to empty means "no prefix", which is
// not the same as leaving it unset.
func loadConfig(lookupEnv func(string) (string, bool)) (Config, error) {
	getenv := func(key string) string {
		value, _ := lookupEnv(key)
		return value
	}

	cfg := Config{
		Token:    strings.TrimSpace(getenv("OPENHAB_API_TOKEN")),
		Username: strings.TrimSpace(getenv("OPENHAB_USERNAME")),
		Password: getenv("OPENHAB_PASSWORD"),
		Timeout:  defaultTimeout,

		CACertPath:         strings.TrimSpace(getenv("OPENHAB_CA_CERT")),
		InsecureSkipVerify: isTruthy(getenv("OPENHAB_INSECURE_SKIP_VERIFY")),
		ReadOnly:           isTruthy(getenv("OPENHAB_READ_ONLY")),
	}

	base := strings.TrimRight(strings.TrimSpace(getenv("OPENHAB_URL")), "/")
	if base == "" {
		return Config{}, fmt.Errorf("OPENHAB_URL is required (the openHAB base URL, for example http://openhab:8080)")
	}
	if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
		return Config{}, fmt.Errorf("OPENHAB_URL must start with http:// or https://, got %q", base)
	}
	cfg.BaseURL = base

	if cfg.Token == "" {
		if cfg.Username == "" {
			return Config{}, fmt.Errorf("credentials are required: set OPENHAB_API_TOKEN, or OPENHAB_USERNAME and OPENHAB_PASSWORD")
		}
		if cfg.Password == "" {
			return Config{}, fmt.Errorf("OPENHAB_PASSWORD is required when OPENHAB_USERNAME is set")
		}
	}

	if raw := strings.TrimSpace(getenv("OPENHAB_TIMEOUT")); raw != "" {
		timeout, err := time.ParseDuration(raw)
		if err != nil {
			return Config{}, fmt.Errorf("OPENHAB_TIMEOUT is not a duration: %w", err)
		}
		if timeout <= 0 {
			return Config{}, fmt.Errorf("OPENHAB_TIMEOUT must be positive, got %q", raw)
		}
		cfg.Timeout = timeout
	}

	cfg.ToolPrefix = defaultToolPrefix
	if raw, ok := lookupEnv("OPENHAB_TOOL_PREFIX"); ok {
		if !validPrefix.MatchString(raw) {
			return Config{}, fmt.Errorf(
				"OPENHAB_TOOL_PREFIX %q may only contain letters, digits, underscores and hyphens", raw)
		}
		cfg.ToolPrefix = raw
	}

	return cfg, nil
}

// httpClient builds the client every request goes through, applying the TLS
// choices the configuration made.
func (c Config) httpClient() (*http.Client, error) {
	tlsConfig := &tls.Config{InsecureSkipVerify: c.InsecureSkipVerify}

	if c.CACertPath != "" {
		pem, err := os.ReadFile(c.CACertPath)
		if err != nil {
			return nil, fmt.Errorf("OPENHAB_CA_CERT could not be read: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("OPENHAB_CA_CERT holds no certificates: %s", c.CACertPath)
		}
		tlsConfig.RootCAs = pool
	}

	return &http.Client{
		Timeout:   c.Timeout,
		Transport: &http.Transport{TLSClientConfig: tlsConfig},
	}, nil
}

// isTruthy reads the permissive set of yes-values these servers accept.
func isTruthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "t", "true", "y", "yes", "on":
		return true
	default:
		return false
	}
}

// osLookupEnv adapts os.LookupEnv to the signature loadConfig expects.
func osLookupEnv(key string) (string, bool) { return os.LookupEnv(key) }
