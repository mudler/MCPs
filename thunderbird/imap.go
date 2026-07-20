package main

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

type IMAP struct {
	cfg   *Config
	creds *Credentials
}

func NewIMAP(cfg *Config, creds *Credentials) *IMAP { return &IMAP{cfg: cfg, creds: creds} }

func (a Account) hostPort() string { return net.JoinHostPort(a.Hostname, strconv.Itoa(a.Port)) }

func (m *IMAP) accountForFolderURI(uri string) (*Account, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("bad folder uri %q: %w", uri, err)
	}
	host := u.Hostname()
	for i := range m.cfg.Accounts {
		a := &m.cfg.Accounts[i]
		if a.Type == "imap" && a.Hostname == host {
			return a, nil
		}
	}
	if strings.HasPrefix(uri, "mailbox://") {
		return nil, fmt.Errorf("folder %q is in a local (non-IMAP) account and is read-only", uri)
	}
	return nil, fmt.Errorf("no IMAP account matches folder %q", uri)
}

func (m *IMAP) connect(a *Account) (*imapclient.Client, error) {
	if a.AuthMethod == 10 {
		return nil, fmt.Errorf("account %s uses OAuth2, which is not supported in this version", a.Hostname)
	}
	pw, ok := m.creds.Lookup(a.Hostname, a.Username)
	if !ok {
		return nil, fmt.Errorf("no stored password for %s@%s", a.Username, a.Hostname)
	}
	addr := a.hostPort()
	opts := &imapclient.Options{TLSConfig: &tls.Config{ServerName: a.Hostname}}
	var (
		c   *imapclient.Client
		err error
	)
	switch a.SocketType {
	case 3: // SSL/TLS
		c, err = imapclient.DialTLS(addr, opts)
	case 2: // STARTTLS
		c, err = imapclient.DialStartTLS(addr, opts)
	default: // 0 plain (also used by the in-memory test server)
		c, err = imapclient.DialInsecure(addr, opts)
	}
	if err != nil {
		return nil, fmt.Errorf("connecting to %s: %w", addr, err)
	}
	if err := c.Login(a.Username, pw).Wait(); err != nil {
		c.Close()
		return nil, fmt.Errorf("IMAP login failed for %s: %w", a.Username, err)
	}
	return c, nil
}

func folderNameFromURI(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		return strings.TrimPrefix(uri, "/")
	}
	name, _ := url.PathUnescape(strings.TrimPrefix(u.Path, "/"))
	return name
}

// withFolder connects, selects the folder for ref, and runs fn with the client and mailbox name.
func (m *IMAP) withFolder(uri string, readOnly bool, fn func(c *imapclient.Client, mailbox string) error) error {
	a, err := m.accountForFolderURI(uri)
	if err != nil {
		return err
	}
	c, err := m.connect(a)
	if err != nil {
		return err
	}
	defer c.Close()
	mailbox := folderNameFromURI(uri)
	if _, err := c.Select(mailbox, &imap.SelectOptions{ReadOnly: readOnly}).Wait(); err != nil {
		return fmt.Errorf("selecting %s: %w", mailbox, err)
	}
	if err := fn(c, mailbox); err != nil {
		return err
	}
	return c.Logout().Wait()
}

func (m *IMAP) FetchBody(ref MessageRef) ([]byte, error) {
	var raw []byte
	err := m.withFolder(ref.FolderURI, true, func(c *imapclient.Client, _ string) error {
		uidSet := imap.UIDSetNum(imap.UID(ref.MessageKey))
		section := &imap.FetchItemBodySection{Peek: true} // whole body, no \Seen
		opts := &imap.FetchOptions{BodySection: []*imap.FetchItemBodySection{section}}
		msgs, err := c.Fetch(uidSet, opts).Collect()
		if err != nil {
			return err
		}
		if len(msgs) == 0 {
			return fmt.Errorf("message UID %d not found in %s", ref.MessageKey, ref.FolderURI)
		}
		raw = msgs[0].FindBodySection(section)
		return nil
	})
	return raw, err
}

func (m *IMAP) SetFlags(ref MessageRef, add, remove []imap.Flag) error {
	return m.withFolder(ref.FolderURI, false, func(c *imapclient.Client, _ string) error {
		uidSet := imap.UIDSetNum(imap.UID(ref.MessageKey))
		store := func(op imap.StoreFlagsOp, flags []imap.Flag) error {
			if len(flags) == 0 {
				return nil
			}
			cmd := c.Store(uidSet, &imap.StoreFlags{Op: op, Silent: true, Flags: flags}, nil)
			return cmd.Close()
		}
		if err := store(imap.StoreFlagsAdd, add); err != nil {
			return err
		}
		return store(imap.StoreFlagsDel, remove)
	})
}

func (m *IMAP) Move(ref MessageRef, destFolder string) error {
	return m.withFolder(ref.FolderURI, false, func(c *imapclient.Client, _ string) error {
		uidSet := imap.UIDSetNum(imap.UID(ref.MessageKey))
		_, err := c.Move(uidSet, destFolder).Wait()
		return err
	})
}

func (m *IMAP) Delete(ref MessageRef, permanent bool) error {
	if !permanent {
		return m.Move(ref, "Trash")
	}
	return m.withFolder(ref.FolderURI, false, func(c *imapclient.Client, _ string) error {
		uidSet := imap.UIDSetNum(imap.UID(ref.MessageKey))
		if err := c.Store(uidSet, &imap.StoreFlags{Op: imap.StoreFlagsAdd, Silent: true, Flags: []imap.Flag{imap.FlagDeleted}}, nil).Close(); err != nil {
			return err
		}
		return c.Expunge().Close()
	})
}

func (m *IMAP) Append(folder string, raw []byte, flags []imap.Flag) error {
	// Append needs an account; resolve from the first IMAP account if folder is a bare name (tests),
	// else from the folder URI.
	a, err := m.appendAccount(folder)
	if err != nil {
		return err
	}
	c, err := m.connect(a)
	if err != nil {
		return err
	}
	defer c.Close()
	mailbox := folder
	if strings.Contains(folder, "://") {
		mailbox = folderNameFromURI(folder)
	}
	cmd := c.Append(mailbox, int64(len(raw)), &imap.AppendOptions{Flags: flags})
	if _, err := cmd.Write(raw); err != nil {
		return err
	}
	if err := cmd.Close(); err != nil {
		return err
	}
	if _, err := cmd.Wait(); err != nil {
		return err
	}
	return c.Logout().Wait()
}

func (m *IMAP) appendAccount(folder string) (*Account, error) {
	if strings.Contains(folder, "://") {
		return m.accountForFolderURI(folder)
	}
	for i := range m.cfg.Accounts {
		if m.cfg.Accounts[i].Type == "imap" {
			return &m.cfg.Accounts[i], nil
		}
	}
	return nil, fmt.Errorf("no IMAP account available")
}
