package main

import (
	"net"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapserver"
	"github.com/emersion/go-imap/v2/imapserver/imapmemserver"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("IMAP", func() {
	var (
		ln   net.Listener
		srv  *imapserver.Server
		m    *IMAP
		acct Account
	)
	BeforeEach(func() {
		memServer := imapmemserver.New()
		user := imapmemserver.NewUser("alice@example.com", "s3cret-imap")
		user.Create("INBOX", nil)
		user.Create("Trash", nil)
		user.Create("Archive", nil)
		memServer.AddUser(user)
		srv = imapserver.New(&imapserver.Options{
			NewSession: func(*imapserver.Conn) (imapserver.Session, *imapserver.GreetingData, error) {
				return memServer.NewSession(), nil, nil
			},
			InsecureAuth: true,
		})
		var err error
		ln, err = net.Listen("tcp", "127.0.0.1:0")
		Expect(err).NotTo(HaveOccurred())
		go srv.Serve(ln)

		host, port, _ := net.SplitHostPort(ln.Addr().String())
		acct = Account{
			Type: "imap", Hostname: host, Port: atoi(port), SocketType: 0,
			Username:   "alice@example.com",
			Identities: []Identity{{Email: "alice@example.com"}},
		}
		cfg := &Config{Accounts: []Account{acct}}
		creds := &Credentials{byHostUser: map[string]string{
			credKey(host, "alice@example.com"): "s3cret-imap",
		}}
		m = NewIMAP(cfg, creds)
	})
	AfterEach(func() { ln.Close() })

	It("appends and fetches a message body by UID", func() {
		raw := []byte("From: bob@example.com\r\nSubject: Hi\r\n\r\nbody text\r\n")
		Expect(m.Append("INBOX", raw, nil)).To(Succeed())
		// UID of the first appended message in a fresh mailbox is 1.
		ref := MessageRef{FolderURI: "imap://alice@example.com@" + acct.hostPort() + "/INBOX", MessageKey: 1}
		got, err := m.FetchBody(ref)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(got)).To(ContainSubstring("body text"))
	})

	It("sets and removes flags by UID", func() {
		raw := []byte("From: bob@example.com\r\nSubject: Flagme\r\n\r\nflag body\r\n")
		Expect(m.Append("INBOX", raw, nil)).To(Succeed())
		ref := MessageRef{FolderURI: "imap://alice@example.com@" + acct.hostPort() + "/INBOX", MessageKey: 1}
		Expect(m.SetFlags(ref, []imap.Flag{imap.FlagSeen}, nil)).To(Succeed())
		Expect(m.SetFlags(ref, nil, []imap.Flag{imap.FlagSeen})).To(Succeed())
	})

	It("resolves an account by folder URI hostname", func() {
		a, err := m.accountForFolderURI("imap://alice@example.com@" + acct.hostPort() + "/INBOX")
		Expect(err).NotTo(HaveOccurred())
		Expect(a.Hostname).To(Equal(acct.Hostname))
	})

	It("rejects local mailbox:// folders as read-only", func() {
		_, err := m.accountForFolderURI("mailbox://nobody@Local/Inbox")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("read-only"))
	})
})
