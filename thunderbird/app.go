package main

import (
	"fmt"
	"log"
	"strings"
)

// App holds all runtime state for the Thunderbird MCP server.
type App struct {
	Config    *Config
	ReadOnly  bool
	AllowSend bool

	Gloda    *Gloda
	Contacts *Contacts
	Calendar *Calendar
	IMAP     *IMAP
	Sender   *Sender
}

// app is the process-wide server state, populated in main().
var app *App

// LoadApp discovers/loads the profile and opens every data source.
func LoadApp(profileDir string, readOnly, allowSend bool) (*App, error) {
	cfg, err := loadProfile(profileDir)
	if err != nil {
		return nil, err
	}
	creds, err := LoadCredentials(profileDir)
	if err != nil {
		// A credential/decryption failure (including ErrMasterPassword) must not be
		// fatal: it only disables IMAP/SMTP. The read-only gloda/contacts/calendar/local
		// tools need no credentials, so continue with an empty credential store — its
		// Lookup returns ("", false), so IMAP.connect/Sender.Send fail cleanly later.
		log.Printf("thunderbird: WARNING: could not load credentials (%v); IMAP/SMTP (fetch, mutate, send) disabled — read-only search/contacts/calendar still available", err)
		creds = &Credentials{}
	}
	return loadAppFromConfig(cfg, creds, readOnly, allowSend)
}

func loadAppFromConfig(cfg *Config, creds *Credentials, readOnly, allowSend bool) (*App, error) {
	a := &App{Config: cfg, ReadOnly: readOnly, AllowSend: allowSend}
	a.IMAP = NewIMAP(cfg, creds)
	a.Sender = NewSender(cfg, creds)

	if g, err := OpenGloda(cfg.ProfileDir); err != nil {
		log.Printf("thunderbird: gloda unavailable: %v", err)
	} else {
		a.Gloda = g
	}
	if c, err := OpenContacts(cfg.ProfileDir); err != nil {
		log.Printf("thunderbird: contacts unavailable: %v", err)
	} else {
		a.Contacts = c
	}
	if c, err := OpenCalendar(cfg.ProfileDir, cfg.Calendars); err != nil {
		log.Printf("thunderbird: calendar unavailable: %v", err)
	} else {
		a.Calendar = c
	}
	return a, nil
}

func (a *App) folderIsLocal(folderURI string) bool {
	return strings.HasPrefix(folderURI, "mailbox://")
}

func (a *App) localAccountFor(folderURI string) (*Account, error) {
	host := mailboxAuthorityHost(folderURI)
	var first *Account
	for i := range a.Config.Accounts {
		acct := &a.Config.Accounts[i]
		if !acct.IsLocal() {
			continue
		}
		if first == nil {
			first = acct
		}
		if host != "" && acct.Hostname == host {
			return acct, nil
		}
	}
	if first != nil {
		return first, nil
	}
	return nil, fmt.Errorf("no local account for %q", folderURI)
}
