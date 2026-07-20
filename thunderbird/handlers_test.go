package main

import (
	"context"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Read handlers", func() {
	BeforeEach(func() {
		f, err := os.Open("testdata/prefs.js") // reuse Task 2 fixture
		Expect(err).NotTo(HaveOccurred())
		defer f.Close()
		prefs, _ := parsePrefs(f)
		cfg := buildConfig(prefs, "testdata")
		a, err := loadAppFromConfig(cfg, &Credentials{}, false, false)
		Expect(err).NotTo(HaveOccurred())
		a.Gloda = buildGlodaFixture() // from gloda_test.go
		app = a
	})

	It("list_accounts returns imap and local accounts with the local flag", func() {
		_, out, err := ListAccounts(context.Background(), nil, ListAccountsInput{})
		Expect(err).NotTo(HaveOccurred())
		Expect(out.Accounts).To(HaveLen(2))
		Expect(out.Accounts[0].Local).To(BeFalse())
		Expect(out.Accounts[1].Local).To(BeTrue())
		Expect(out.Accounts[0].Identities).To(ContainElement("alice@example.com"))
	})

	It("search_messages returns index hits", func() {
		_, out, err := SearchMessages(context.Background(), nil, SearchMessagesInput{Query: "report"})
		Expect(err).NotTo(HaveOccurred())
		Expect(out.Count).To(Equal(1))
		Expect(out.Messages[0].Subject).To(Equal("Quarterly report"))
	})
})

var _ = Describe("Mutate handlers reject local folders", func() {
	BeforeEach(func() {
		app = &App{Config: &Config{Accounts: []Account{{Type: "none", Hostname: "Local Folders"}}}}
		app.IMAP = NewIMAP(app.Config, &Credentials{})
	})
	It("set_flags on a mailbox:// ref errors", func() {
		_, _, err := SetFlags(context.Background(), nil, SetFlagsInput{
			MessageRef: "mailbox://nobody@Local%20Folders/INBOX#42", Read: ptr(true),
		})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("read-only"))
	})
})

func ptr[T any](v T) *T { return &v }
