package main

import (
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("LoadConfig", func() {
	BeforeEach(func() {
		for _, k := range []string{
			"CUA_ENABLE_COMPUTER", "CUA_ENABLE_BROWSER", "CUA_DRIVER_CMD",
			"CUA_CHROME_PATH", "CUA_BROWSER_PROFILE_DIR", "CUA_ALLOW_PRIVATE_URLS",
			"CUA_TOOLS", "CUA_READY_TIMEOUT",
		} {
			os.Unsetenv(k)
		}
	})

	It("enables both capabilities by default", func() {
		c := LoadConfig()
		Expect(c.NibConfig.Computer.Enabled).To(BeTrue())
		Expect(c.NibConfig.Browser.Enabled).To(BeTrue())
	})

	It("defaults the driver command and args", func() {
		c := LoadConfig()
		Expect(c.DriverCmd).To(Equal("cua-driver"))
		Expect(c.DriverArgs).To(Equal([]string{"mcp"}))
		Expect(c.NibConfig.Computer.Command).To(Equal("cua-driver"))
		Expect(c.NibConfig.Computer.Args).To(Equal([]string{"mcp"}))
	})

	It("blocks private URLs by default", func() {
		c := LoadConfig()
		Expect(c.NibConfig.Browser.AllowPrivateURLs).To(BeFalse())
	})

	It("defaults the ready timeout to 60s", func() {
		c := LoadConfig()
		Expect(c.ReadyTimeout).To(Equal(60 * time.Second))
	})

	It("disables the browser when CUA_ENABLE_BROWSER=false", func() {
		os.Setenv("CUA_ENABLE_BROWSER", "false")
		c := LoadConfig()
		Expect(c.NibConfig.Browser.Enabled).To(BeFalse())
		Expect(c.NibConfig.Computer.Enabled).To(BeTrue())
	})

	It("honours overrides", func() {
		os.Setenv("CUA_DRIVER_CMD", "/opt/cua-driver")
		os.Setenv("CUA_CHROME_PATH", "/usr/bin/chromium")
		os.Setenv("CUA_BROWSER_PROFILE_DIR", "/home/cua/profile")
		os.Setenv("CUA_ALLOW_PRIVATE_URLS", "true")
		os.Setenv("CUA_READY_TIMEOUT", "5s")
		c := LoadConfig()
		Expect(c.DriverCmd).To(Equal("/opt/cua-driver"))
		Expect(c.NibConfig.Computer.Command).To(Equal("/opt/cua-driver"))
		Expect(c.NibConfig.Browser.ChromePath).To(Equal("/usr/bin/chromium"))
		Expect(c.NibConfig.Browser.ProfileDir).To(Equal("/home/cua/profile"))
		Expect(c.NibConfig.Browser.AllowPrivateURLs).To(BeTrue())
		Expect(c.ReadyTimeout).To(Equal(5 * time.Second))
	})

	It("returns an empty allowlist when CUA_TOOLS is unset or 'all'", func() {
		Expect(LoadConfig().ToolAllowlist).To(BeEmpty())
		os.Setenv("CUA_TOOLS", "all")
		Expect(LoadConfig().ToolAllowlist).To(BeEmpty())
	})

	It("parses CUA_TOOLS into an allowlist, trimming spaces", func() {
		os.Setenv("CUA_TOOLS", "computer_use, browser_navigate ")
		Expect(LoadConfig().ToolAllowlist).To(Equal(map[string]bool{
			"computer_use":     true,
			"browser_navigate": true,
		}))
	})

	It("falls back to the default ready timeout when unparseable", func() {
		os.Setenv("CUA_READY_TIMEOUT", "not-a-duration")
		Expect(LoadConfig().ReadyTimeout).To(Equal(60 * time.Second))
	})

	// "0 means unlimited" is a common operator convention, but time.ParseDuration
	// accepts these happily and a zero budget makes every readiness deadline
	// already-expired. Treat non-positive as "unset" rather than "no time".
	DescribeTable("falls back to the default ready timeout when non-positive",
		func(raw string) {
			os.Setenv("CUA_READY_TIMEOUT", raw)
			Expect(LoadConfig().ReadyTimeout).To(Equal(60 * time.Second))
		},
		Entry("bare zero", "0"),
		Entry("zero seconds", "0s"),
		Entry("negative", "-5s"),
	)
})
