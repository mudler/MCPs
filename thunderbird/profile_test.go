package main

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("parsePrefs", func() {
	It("parses strings, ints, and bools as literal strings", func() {
		in := `user_pref("a.b", "hello");
user_pref("a.c", 993);
user_pref("a.d", true);
// comment line ignored
user_pref("a.e", "with \"quote\"");`
		m, err := parsePrefs(strings.NewReader(in))
		Expect(err).NotTo(HaveOccurred())
		Expect(m["a.b"]).To(Equal("hello"))
		Expect(m["a.c"]).To(Equal("993"))
		Expect(m["a.d"]).To(Equal("true"))
		Expect(m["a.e"]).To(Equal(`with "quote"`))
	})
})

var _ = Describe("buildConfig", func() {
	var cfg *Config
	BeforeEach(func() {
		f, err := os.Open("testdata/prefs.js")
		Expect(err).NotTo(HaveOccurred())
		defer f.Close()
		prefs, err := parsePrefs(f)
		Expect(err).NotTo(HaveOccurred())
		cfg = buildConfig(prefs, "/profile")
	})
	It("builds imap and local accounts", func() {
		Expect(cfg.Accounts).To(HaveLen(2))
		imap := cfg.Accounts[0]
		Expect(imap.Type).To(Equal("imap"))
		Expect(imap.Hostname).To(Equal("imap.example.com"))
		Expect(imap.Port).To(Equal(993))
		Expect(imap.SocketType).To(Equal(3))
		Expect(imap.IsLocal()).To(BeFalse())
		Expect(imap.Identities).To(HaveLen(1))
		Expect(imap.Identities[0].Email).To(Equal("alice@example.com"))
		Expect(imap.Identities[0].SMTPKey).To(Equal("smtp1"))
	})
	It("marks the Local Folders account local", func() {
		Expect(cfg.Accounts[1].IsLocal()).To(BeTrue())
	})
	It("parses smtp servers", func() {
		Expect(cfg.SMTP["smtp1"].Hostname).To(Equal("smtp.example.com"))
		Expect(cfg.SMTP["smtp1"].Port).To(Equal(587))
		Expect(cfg.SMTP["smtp1"].SocketType).To(Equal(2))
	})
})

var _ = Describe("defaultProfileFromINI", func() {
	It("joins a relative (IsRelative=1) profile path to the root", func() {
		root := GinkgoT().TempDir()
		iniPath := filepath.Join(root, "profiles.ini")
		ini := "[General]\n" +
			"StartWithLastProfile=1\n" +
			"Version=2\n" +
			"\n" +
			"[Profile0]\n" +
			"Name=default\n" +
			"IsRelative=1\n" +
			"Path=abcd.default\n" +
			"Default=1\n"
		Expect(os.WriteFile(iniPath, []byte(ini), 0o600)).To(Succeed())

		got, err := defaultProfileFromINI(iniPath, root)
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(Equal(filepath.Join(root, "abcd.default")))
	})
})
