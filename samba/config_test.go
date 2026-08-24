package main

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// envMap turns a map into the getenv function loadConfig expects, so specs
// never have to mutate the real process environment.
func envMap(vars map[string]string) func(string) string {
	return func(key string) string { return vars[key] }
}

var _ = Describe("loadConfig", func() {
	Context("required variables", func() {
		It("should fail when SMB_HOST is missing", func() {
			_, err := loadConfig(envMap(map[string]string{"SMB_SHARE": "Data"}))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("SMB_HOST"))
		})

		It("should fail when SMB_SHARE is missing", func() {
			_, err := loadConfig(envMap(map[string]string{"SMB_HOST": "nas.local"}))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("SMB_SHARE"))
		})
	})

	Context("host normalisation", func() {
		It("should append the default SMB port when none is given", func() {
			cfg, err := loadConfig(envMap(map[string]string{
				"SMB_HOST":  "192.168.68.102",
				"SMB_SHARE": "Data",
			}))
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg.Address).To(Equal("192.168.68.102:445"))
		})

		It("should keep an explicit port", func() {
			cfg, err := loadConfig(envMap(map[string]string{
				"SMB_HOST":  "192.168.68.102:1445",
				"SMB_SHARE": "Data",
			}))
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg.Address).To(Equal("192.168.68.102:1445"))
		})
	})

	Context("defaults", func() {
		var cfg Config

		BeforeEach(func() {
			var err error
			cfg, err = loadConfig(envMap(map[string]string{
				"SMB_HOST":  "nas.local",
				"SMB_SHARE": "Data",
			}))
			Expect(err).ToNot(HaveOccurred())
		})

		It("should default the timeout to 30 seconds", func() {
			Expect(cfg.Timeout).To(Equal(30 * time.Second))
		})

		It("should default the read cap to one megabyte", func() {
			Expect(cfg.ReadMaxBytes).To(Equal(int64(1048576)))
		})

		It("should allow writes and deletes by default", func() {
			Expect(cfg.ReadOnly).To(BeFalse())
			Expect(cfg.DisableDelete).To(BeFalse())
		})
	})

	Context("optional variables", func() {
		It("should parse the timeout", func() {
			cfg, err := loadConfig(envMap(map[string]string{
				"SMB_HOST":    "nas.local",
				"SMB_SHARE":   "Data",
				"SMB_TIMEOUT": "5s",
			}))
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg.Timeout).To(Equal(5 * time.Second))
		})

		It("should reject an unparseable timeout", func() {
			_, err := loadConfig(envMap(map[string]string{
				"SMB_HOST":    "nas.local",
				"SMB_SHARE":   "Data",
				"SMB_TIMEOUT": "soon",
			}))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("SMB_TIMEOUT"))
		})

		It("should reject a non-positive timeout", func() {
			_, err := loadConfig(envMap(map[string]string{
				"SMB_HOST":    "nas.local",
				"SMB_SHARE":   "Data",
				"SMB_TIMEOUT": "0s",
			}))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("SMB_TIMEOUT"))
		})

		It("should parse the read cap", func() {
			cfg, err := loadConfig(envMap(map[string]string{
				"SMB_HOST":           "nas.local",
				"SMB_SHARE":          "Data",
				"SMB_READ_MAX_BYTES": "2048",
			}))
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg.ReadMaxBytes).To(Equal(int64(2048)))
		})

		It("should reject a non-positive read cap", func() {
			_, err := loadConfig(envMap(map[string]string{
				"SMB_HOST":           "nas.local",
				"SMB_SHARE":          "Data",
				"SMB_READ_MAX_BYTES": "0",
			}))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("SMB_READ_MAX_BYTES"))
		})

		It("should carry credentials through", func() {
			cfg, err := loadConfig(envMap(map[string]string{
				"SMB_HOST":     "nas.local",
				"SMB_SHARE":    "Data",
				"SMB_USER":     "anonymous",
				"SMB_PASSWORD": "secret",
				"SMB_DOMAIN":   "WORKGROUP",
			}))
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg.User).To(Equal("anonymous"))
			Expect(cfg.Password).To(Equal("secret"))
			Expect(cfg.Domain).To(Equal("WORKGROUP"))
		})
	})

	Context("restriction switches", func() {
		DescribeTable("truthy values enable read-only mode",
			func(value string, expected bool) {
				cfg, err := loadConfig(envMap(map[string]string{
					"SMB_HOST":      "nas.local",
					"SMB_SHARE":     "Data",
					"SMB_READ_ONLY": value,
				}))
				Expect(err).ToNot(HaveOccurred())
				Expect(cfg.ReadOnly).To(Equal(expected))
			},
			Entry("true", "true", true),
			Entry("TRUE", "TRUE", true),
			Entry("1", "1", true),
			Entry("false", "false", false),
			Entry("empty", "", false),
			Entry("nonsense", "maybe", false),
		)

		It("should disable deletion when SMB_DISABLE_DELETE is truthy", func() {
			cfg, err := loadConfig(envMap(map[string]string{
				"SMB_HOST":           "nas.local",
				"SMB_SHARE":          "Data",
				"SMB_DISABLE_DELETE": "true",
			}))
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg.DisableDelete).To(BeTrue())
		})
	})
})
