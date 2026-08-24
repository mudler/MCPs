package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

const defaultTimeout = 30 * time.Second

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
}

// loadConfig reads the configuration from getenv, which is os.Getenv in
// production and a map lookup in the specs.
func loadConfig(getenv func(string) string) (Config, error) {
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
