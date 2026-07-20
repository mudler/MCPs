package main

import (
	"context"
	"os"

	"github.com/google/jsonschema-go/jsonschema"
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

var _ = Describe("Contacts and calendar handlers", func() {
	BeforeEach(func() {
		app = &App{Config: &Config{Calendars: []CalendarRef{{UUID: "cal-uuid-1", Name: "Home", Type: "storage"}}}}
		c, err := openContactsAt(seedDB("testdata/abook_seed.sql"))
		Expect(err).NotTo(HaveOccurred())
		app.Contacts = c
		cal, err := openCalendarAt(seedDB("testdata/calendar_seed.sql"), map[string]string{"cal-uuid-1": "Home"})
		Expect(err).NotTo(HaveOccurred())
		app.Calendar = cal
	})
	It("search_contacts finds by substring", func() {
		_, out, err := SearchContacts(context.Background(), nil, SearchContactsInput{Query: "carol"})
		Expect(err).NotTo(HaveOccurred())
		Expect(out.Count).To(Equal(1))
		Expect(out.Contacts[0].Email).To(Equal("carol@example.com"))
	})
	It("list_events returns events in the default window", func() {
		_, out, err := ListEvents(context.Background(), nil, ListEventsInput{
			Since: "2023-11-01", Until: "2023-12-01",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(out.Count).To(Equal(2))
	})
	It("list_calendars lists registered calendars", func() {
		_, out, err := ListCalendars(context.Background(), nil, ListCalendarsInput{})
		Expect(err).NotTo(HaveOccurred())
		Expect(out.Calendars).To(HaveLen(1))
		Expect(out.Calendars[0].Name).To(Equal("Home"))
	})
})

var _ = Describe("ThunderbirdMail dispatcher", func() {
	BeforeEach(func() {
		f, err := os.Open("testdata/prefs.js")
		Expect(err).NotTo(HaveOccurred())
		defer f.Close()
		prefs, _ := parsePrefs(f)
		cfg := buildConfig(prefs, "testdata")
		a, err := loadAppFromConfig(cfg, &Credentials{}, false, false)
		Expect(err).NotTo(HaveOccurred())
		a.Gloda = buildGlodaFixture()
		app = a
	})

	It("list_accounts returns accounts", func() {
		_, out, err := ThunderbirdMail(context.Background(), nil, MailInput{Action: "list_accounts"})
		Expect(err).NotTo(HaveOccurred())
		Expect(out.Accounts).To(HaveLen(2))
	})

	It("search_messages returns messages", func() {
		_, out, err := ThunderbirdMail(context.Background(), nil, MailInput{Action: "search_messages", Query: "report"})
		Expect(err).NotTo(HaveOccurred())
		Expect(out.Count).To(Equal(1))
		Expect(out.Messages[0].Subject).To(Equal("Quarterly report"))
	})

	It("unknown action errors", func() {
		_, _, err := ThunderbirdMail(context.Background(), nil, MailInput{Action: "bogus"})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("unknown mail action"))
	})
})

var _ = Describe("ThunderbirdMail gating backstops", func() {
	It("mutate action is rejected when read-only", func() {
		app = &App{Config: &Config{}, ReadOnly: true}
		_, _, err := ThunderbirdMail(context.Background(), nil, MailInput{Action: "set_flags", MessageRef: "imap://x/INBOX#1"})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("READ_ONLY"))
	})
	It("compose action is rejected when send not allowed", func() {
		app = &App{Config: &Config{}, ReadOnly: false, AllowSend: false}
		_, _, err := ThunderbirdMail(context.Background(), nil, MailInput{Action: "send_mail"})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("ALLOW_SEND"))
	})
})

var _ = Describe("ThunderbirdContacts and ThunderbirdCalendar dispatchers", func() {
	BeforeEach(func() {
		app = &App{Config: &Config{Calendars: []CalendarRef{{UUID: "cal-uuid-1", Name: "Home", Type: "storage"}}}}
		c, err := openContactsAt(seedDB("testdata/abook_seed.sql"))
		Expect(err).NotTo(HaveOccurred())
		app.Contacts = c
		cal, err := openCalendarAt(seedDB("testdata/calendar_seed.sql"), map[string]string{"cal-uuid-1": "Home"})
		Expect(err).NotTo(HaveOccurred())
		app.Calendar = cal
	})

	It("search_contacts returns contacts", func() {
		_, out, err := ThunderbirdContacts(context.Background(), nil, ContactsInput{Action: "search_contacts", Query: "carol"})
		Expect(err).NotTo(HaveOccurred())
		Expect(out.Count).To(Equal(1))
		Expect(out.Contacts[0].Email).To(Equal("carol@example.com"))
	})

	It("contacts unknown action errors", func() {
		_, _, err := ThunderbirdContacts(context.Background(), nil, ContactsInput{Action: "bogus"})
		Expect(err).To(HaveOccurred())
	})

	It("list_events returns events", func() {
		_, out, err := ThunderbirdCalendar(context.Background(), nil, CalendarInput{Action: "list_events", Since: "2023-11-01", Until: "2023-12-01"})
		Expect(err).NotTo(HaveOccurred())
		Expect(out.Count).To(Equal(2))
	})

	It("calendar unknown action errors", func() {
		_, _, err := ThunderbirdCalendar(context.Background(), nil, CalendarInput{Action: "bogus"})
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("mustSchema", func() {
	It("sets the action enum and keeps action required", func() {
		s := mustSchema[MailInput]([]any{"list_accounts", "search_messages"})
		p, ok := s.Properties["action"]
		Expect(ok).To(BeTrue())
		Expect(p.Enum).To(Equal([]any{"list_accounts", "search_messages"}))
		Expect(s.Required).To(ContainElement("action"))
	})
	It("infers a jsonschema for the input type", func() {
		var s *jsonschema.Schema = mustSchema[ContactsInput]([]any{"search_contacts", "get_contact"})
		Expect(s.Properties).To(HaveKey("action"))
	})
})
