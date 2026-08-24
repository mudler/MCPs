package main

import (
	"net/http"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// env returns a getenv function backed by a map, so the specs never touch the
// process environment.
func env(pairs map[string]string) func(string) string {
	return func(key string) string { return pairs[key] }
}

var _ = Describe("loadConfig", func() {
	It("should accept a URL and a token", func() {
		cfg, err := loadConfig(env(map[string]string{
			"OPENHAB_URL":       "https://10.9.0.26:8443",
			"OPENHAB_API_TOKEN": "oh.mcp.secret",
		}))
		Expect(err).ToNot(HaveOccurred())
		Expect(cfg.BaseURL).To(Equal("https://10.9.0.26:8443"))
		Expect(cfg.Token).To(Equal("oh.mcp.secret"))
		Expect(cfg.Timeout).To(Equal(30 * time.Second))
		Expect(cfg.ReadOnly).To(BeFalse())
	})

	It("should strip a trailing slash from the URL", func() {
		cfg, err := loadConfig(env(map[string]string{
			"OPENHAB_URL":       "http://openhab:8080/",
			"OPENHAB_API_TOKEN": "t",
		}))
		Expect(err).ToNot(HaveOccurred())
		Expect(cfg.BaseURL).To(Equal("http://openhab:8080"))
	})

	It("should accept basic auth instead of a token", func() {
		cfg, err := loadConfig(env(map[string]string{
			"OPENHAB_URL":      "http://openhab:8080",
			"OPENHAB_USERNAME": "admin",
			"OPENHAB_PASSWORD": "hunter2",
		}))
		Expect(err).ToNot(HaveOccurred())
		Expect(cfg.Username).To(Equal("admin"))
		Expect(cfg.Password).To(Equal("hunter2"))
	})

	It("should require OPENHAB_URL", func() {
		_, err := loadConfig(env(map[string]string{"OPENHAB_API_TOKEN": "t"}))
		Expect(err).To(MatchError(ContainSubstring("OPENHAB_URL is required")))
	})

	It("should reject a URL without a scheme", func() {
		_, err := loadConfig(env(map[string]string{
			"OPENHAB_URL":       "10.9.0.26:8443",
			"OPENHAB_API_TOKEN": "t",
		}))
		Expect(err).To(MatchError(ContainSubstring("http:// or https://")))
	})

	It("should require some form of credentials", func() {
		_, err := loadConfig(env(map[string]string{"OPENHAB_URL": "http://openhab:8080"}))
		Expect(err).To(MatchError(ContainSubstring("OPENHAB_API_TOKEN")))
	})

	It("should reject a username without a password", func() {
		_, err := loadConfig(env(map[string]string{
			"OPENHAB_URL":      "http://openhab:8080",
			"OPENHAB_USERNAME": "admin",
		}))
		Expect(err).To(MatchError(ContainSubstring("OPENHAB_PASSWORD")))
	})

	It("should parse the timeout", func() {
		cfg, err := loadConfig(env(map[string]string{
			"OPENHAB_URL":       "http://openhab:8080",
			"OPENHAB_API_TOKEN": "t",
			"OPENHAB_TIMEOUT":   "5s",
		}))
		Expect(err).ToNot(HaveOccurred())
		Expect(cfg.Timeout).To(Equal(5 * time.Second))
	})

	It("should reject an unparseable timeout", func() {
		_, err := loadConfig(env(map[string]string{
			"OPENHAB_URL":       "http://openhab:8080",
			"OPENHAB_API_TOKEN": "t",
			"OPENHAB_TIMEOUT":   "soon",
		}))
		Expect(err).To(MatchError(ContainSubstring("OPENHAB_TIMEOUT")))
	})

	It("should read the truthy flags", func() {
		cfg, err := loadConfig(env(map[string]string{
			"OPENHAB_URL":                  "http://openhab:8080",
			"OPENHAB_API_TOKEN":            "t",
			"OPENHAB_READ_ONLY":            "true",
			"OPENHAB_INSECURE_SKIP_VERIFY": "1",
		}))
		Expect(err).ToNot(HaveOccurred())
		Expect(cfg.ReadOnly).To(BeTrue())
		Expect(cfg.InsecureSkipVerify).To(BeTrue())
	})
})

var _ = Describe("Config.httpClient", func() {
	It("should apply the timeout", func() {
		client, err := Config{Timeout: 7 * time.Second}.httpClient()
		Expect(err).ToNot(HaveOccurred())
		Expect(client.Timeout).To(Equal(7 * time.Second))
	})

	It("should skip verification when asked", func() {
		client, err := Config{Timeout: time.Second, InsecureSkipVerify: true}.httpClient()
		Expect(err).ToNot(HaveOccurred())
		transport, ok := client.Transport.(*http.Transport)
		Expect(ok).To(BeTrue())
		Expect(transport.TLSClientConfig.InsecureSkipVerify).To(BeTrue())
	})

	It("should load a CA bundle from disk", func() {
		path := filepath.Join(GinkgoT().TempDir(), "ca.pem")
		Expect(os.WriteFile(path, []byte(testCAPEM), 0o644)).To(Succeed())

		client, err := Config{Timeout: time.Second, CACertPath: path}.httpClient()
		Expect(err).ToNot(HaveOccurred())
		transport, ok := client.Transport.(*http.Transport)
		Expect(ok).To(BeTrue())
		Expect(transport.TLSClientConfig.RootCAs).ToNot(BeNil())
	})

	It("should fail on a CA bundle that is not readable", func() {
		_, err := Config{CACertPath: "/nonexistent/ca.pem"}.httpClient()
		Expect(err).To(MatchError(ContainSubstring("OPENHAB_CA_CERT")))
	})

	It("should fail on a CA bundle that holds no certificate", func() {
		path := filepath.Join(GinkgoT().TempDir(), "junk.pem")
		Expect(os.WriteFile(path, []byte("not a certificate"), 0o644)).To(Succeed())

		_, err := Config{CACertPath: path}.httpClient()
		Expect(err).To(MatchError(ContainSubstring("no certificates")))
	})
})

// testCAPEM is a self-signed certificate used only to prove the PEM parses.
const testCAPEM = `-----BEGIN CERTIFICATE-----
MIIBmDCCAT+gAwIBAgIUb+KIIk6pqmT45MhWzc0GK37ncK8wCgYIKoZIzj0EAwIw
FDESMBAGA1UECgwJbWNwcyB0ZXN0MB4XDTI2MDgyNDE5NTEyN1oXDTQ2MDgxOTE5
NTEyN1owFDESMBAGA1UECgwJbWNwcyB0ZXN0MFkwEwYHKoZIzj0CAQYIKoZIzj0D
AQcDQgAE0wjfTM1faXZJaH5aoCrwCGVvGmBzQSjTOeseK7GOy2v9mKgpxAzIbgl5
1y7y+4VW2HD+Xf/sFKoCVA5KYM3ynaNvMG0wHQYDVR0OBBYEFH4a/F5y/ToN9NZE
CF8VCIup1jjoMB8GA1UdIwQYMBaAFH4a/F5y/ToN9NZECF8VCIup1jjoMA8GA1Ud
EwEB/wQFMAMBAf8wGgYDVR0RBBMwEYIJbG9jYWxob3N0hwR/AAABMAoGCCqGSM49
BAMCA0cAMEQCIEet0Iw0azx6jsA29h5lZ8T4Vr8pPVW35wTqRo0mwNCoAiAV7b48
5yy+BlK4G3/ioV32ZGLtaXEn3AJHd4NYHGC7sw==
-----END CERTIFICATE-----
`
