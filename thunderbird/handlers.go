package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func parseDate(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	return time.Parse("2006-01-02", s)
}

func ListAccounts(_ context.Context, _ *mcp.CallToolRequest, _ ListAccountsInput) (*mcp.CallToolResult, ListAccountsOutput, error) {
	var out ListAccountsOutput
	for _, a := range app.Config.Accounts {
		info := AccountInfo{
			Key: a.Key, Type: a.Type, Name: a.Name, Hostname: a.Hostname, Local: a.IsLocal(),
		}
		for _, id := range a.Identities {
			if id.Email != "" {
				info.Identities = append(info.Identities, id.Email)
			}
		}
		out.Accounts = append(out.Accounts, info)
	}
	return nil, out, nil
}

func SearchMessages(_ context.Context, _ *mcp.CallToolRequest, in SearchMessagesInput) (*mcp.CallToolResult, SearchMessagesOutput, error) {
	since, err := parseDate(in.Since)
	if err != nil {
		return nil, SearchMessagesOutput{}, fmt.Errorf("invalid since: %w", err)
	}
	until, err := parseDate(in.Until)
	if err != nil {
		return nil, SearchMessagesOutput{}, fmt.Errorf("invalid until: %w", err)
	}
	res, err := app.Gloda.Search(SearchQuery{
		Text: in.Query, From: in.From, To: in.To, Subject: in.Subject,
		FolderURI: in.Folder, Since: since, Until: until,
		Limit: in.Limit, Offset: in.Offset,
	})
	if err != nil {
		return nil, SearchMessagesOutput{}, err
	}
	return nil, SearchMessagesOutput{Messages: res, Count: len(res)}, nil
}

func GetMessage(_ context.Context, _ *mcp.CallToolRequest, in GetMessageInput) (*mcp.CallToolResult, GetMessageOutput, error) {
	ref, err := ParseMessageRef(in.MessageRef)
	if err != nil {
		return nil, GetMessageOutput{}, err
	}
	var raw []byte
	if app.folderIsLocal(ref.FolderURI) {
		a, err := app.localAccountFor(ref.FolderURI)
		if err != nil {
			return nil, GetMessageOutput{}, err
		}
		raw, err = readMboxAt(localMboxPath(a, ref.FolderURI), ref.MessageKey)
		if err != nil {
			return nil, GetMessageOutput{}, fmt.Errorf("reading local message: %w", err)
		}
	} else {
		raw, err = app.IMAP.FetchBody(ref)
		if err != nil {
			return nil, GetMessageOutput{}, err
		}
	}
	detail, err := ParseMessage(in.MessageRef, raw)
	if err != nil {
		return nil, GetMessageOutput{}, err
	}
	if in.BodyFormat != "html" {
		detail.BodyHTML = "" // keep response small unless html requested
	}
	return nil, GetMessageOutput{Message: detail}, nil
}

func ListRecent(_ context.Context, _ *mcp.CallToolRequest, in ListRecentInput) (*mcp.CallToolResult, ListRecentOutput, error) {
	limit := in.Limit
	if limit <= 0 {
		limit = 20
	}
	res, err := app.Gloda.Recent(in.Folder, limit)
	if err != nil {
		return nil, ListRecentOutput{}, err
	}
	return nil, ListRecentOutput{Messages: res, Count: len(res)}, nil
}

func ListFolders(_ context.Context, _ *mcp.CallToolRequest, in ListFoldersInput) (*mcp.CallToolResult, ListFoldersOutput, error) {
	var out ListFoldersOutput
	for i := range app.Config.Accounts {
		a := &app.Config.Accounts[i]
		if in.Account != "" && a.Key != in.Account {
			continue
		}
		if a.Type == "imap" {
			fs, err := app.IMAP.listIMAPFolders(a)
			if err != nil {
				// Degrade: report the account with an error-free empty list rather than failing all.
				continue
			}
			out.Folders = append(out.Folders, fs...)
		} else {
			out.Folders = append(out.Folders, listLocalFolders(a)...)
		}
	}
	return nil, out, nil
}

func listLocalFolders(a *Account) []FolderInfo {
	var out []FolderInfo
	_ = filepath.Walk(a.Directory, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		name := info.Name()
		if strings.HasSuffix(name, ".msf") || strings.HasSuffix(name, ".dat") {
			return nil
		}
		out = append(out, FolderInfo{
			Name:    name,
			URI:     "mailbox://" + a.Username + "@" + a.Hostname + "/" + name,
			Account: a.Key, Local: true,
		})
		return nil
	})
	return out
}
