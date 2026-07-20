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
		return nil, err // includes ErrMasterPassword — fail hard with a clear message
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
	for i := range a.Config.Accounts {
		acct := &a.Config.Accounts[i]
		if acct.IsLocal() {
			return acct, nil
		}
	}
	return nil, fmt.Errorf("no local account for %q", folderURI)
}
