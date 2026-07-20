package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var prefLine = regexp.MustCompile(`^\s*user_pref\("([^"]+)",\s*(.*)\);\s*$`)

// parsePrefs reads a prefs.js and returns each user_pref value as a string.
// String values are unquoted/unescaped; numbers and booleans are kept literal.
func parsePrefs(r io.Reader) (map[string]string, error) {
	out := map[string]string{}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		m := prefLine.FindStringSubmatch(sc.Text())
		if m == nil {
			continue
		}
		key, raw := m[1], strings.TrimSpace(m[2])
		if len(raw) >= 2 && raw[0] == '"' && raw[len(raw)-1] == '"' {
			s := raw[1 : len(raw)-1]
			s = strings.ReplaceAll(s, `\"`, `"`)
			s = strings.ReplaceAll(s, `\\`, `\`)
			out[key] = s
		} else {
			out[key] = raw // int or bool literal
		}
	}
	return out, sc.Err()
}

func atoi(s string) int { n, _ := strconv.Atoi(s); return n }

func splitList(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

// buildConfig assembles a Config by walking the account -> server/identity graph.
func buildConfig(p map[string]string, profileDir string) *Config {
	cfg := &Config{ProfileDir: profileDir, SMTP: map[string]SMTPServer{}}

	for _, acctKey := range splitList(p["mail.accountmanager.accounts"]) {
		srv := p["mail.account."+acctKey+".server"]
		if srv == "" {
			continue
		}
		sp := "mail.server." + srv + "."
		acct := Account{
			Key:        acctKey,
			Type:       p[sp+"type"],
			Hostname:   p[sp+"hostname"],
			Port:       atoi(p[sp+"port"]),
			Username:   p[sp+"userName"],
			SocketType: atoi(p[sp+"socketType"]),
			AuthMethod: atoi(p[sp+"authMethod"]),
			Directory:  resolveDir(p[sp+"directory"], p[sp+"directory-rel"], profileDir),
			Name:       p[sp+"name"],
		}
		if acct.Type == "" {
			continue
		}
		for _, idKey := range splitList(p["mail.account."+acctKey+".identities"]) {
			ip := "mail.identity." + idKey + "."
			acct.Identities = append(acct.Identities, Identity{
				Key:         idKey,
				Email:       p[ip+"useremail"],
				FullName:    p[ip+"fullName"],
				SMTPKey:     p[ip+"smtpServer"],
				DraftFolder: p[ip+"draft_folder"],
				FccFolder:   p[ip+"fcc_folder"],
			})
		}
		cfg.Accounts = append(cfg.Accounts, acct)
	}

	for key := range smtpKeys(p) {
		sp := "mail.smtpserver." + key + "."
		cfg.SMTP[key] = SMTPServer{
			Key: key, Hostname: p[sp+"hostname"], Port: atoi(p[sp+"port"]),
			Username: p[sp+"username"], SocketType: atoi(p[sp+"try_ssl"]),
			AuthMethod: atoi(p[sp+"authMethod"]),
		}
	}

	for uuid := range calendarKeys(p) {
		cp := "calendar.registry." + uuid + "."
		cfg.Calendars = append(cfg.Calendars, CalendarRef{
			UUID: uuid, Name: p[cp+"name"], Type: p[cp+"type"], URI: p[cp+"uri"],
		})
	}
	return cfg
}

// smtpKeys / calendarKeys discover instance keys from the flat pref map.
func smtpKeys(p map[string]string) map[string]bool {
	keys := map[string]bool{}
	for k := range p {
		if rest, ok := strings.CutPrefix(k, "mail.smtpserver."); ok {
			keys[strings.SplitN(rest, ".", 2)[0]] = true
		}
	}
	return keys
}

func calendarKeys(p map[string]string) map[string]bool {
	keys := map[string]bool{}
	for k := range p {
		if rest, ok := strings.CutPrefix(k, "calendar.registry."); ok {
			keys[strings.SplitN(rest, ".", 2)[0]] = true
		}
	}
	return keys
}

func resolveDir(abs, rel, profileDir string) string {
	if abs != "" {
		return abs
	}
	if r, ok := strings.CutPrefix(rel, "[ProfD]"); ok {
		return filepath.Join(profileDir, filepath.FromSlash(r))
	}
	return rel
}

// discoverProfile returns THUNDERBIRD_PROFILE or the default profile from profiles.ini.
func discoverProfile() (string, error) {
	if p := os.Getenv("THUNDERBIRD_PROFILE"); p != "" {
		return p, nil
	}
	root, err := thunderbirdRoot()
	if err != nil {
		return "", err
	}
	return defaultProfileFromINI(filepath.Join(root, "profiles.ini"), root)
}

func loadProfile(profileDir string) (*Config, error) {
	f, err := os.Open(filepath.Join(profileDir, "prefs.js"))
	if err != nil {
		return nil, fmt.Errorf("opening prefs.js: %w", err)
	}
	defer f.Close()
	prefs, err := parsePrefs(f)
	if err != nil {
		return nil, err
	}
	return buildConfig(prefs, profileDir), nil
}

// thunderbirdRoot returns the platform Thunderbird data directory.
func thunderbirdRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	candidates := []string{
		filepath.Join(home, ".thunderbird"),
		filepath.Join(home, ".mozilla-thunderbird"),
		filepath.Join(home, "snap", "thunderbird", "common", ".thunderbird"),
		filepath.Join(home, ".var", "app", "org.mozilla.Thunderbird", ".thunderbird"),
		filepath.Join(home, "Library", "Thunderbird"),      // macOS
		filepath.Join(os.Getenv("APPDATA"), "Thunderbird"), // Windows
	}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(c, "profiles.ini")); err == nil {
			return c, nil
		}
	}
	return "", fmt.Errorf("no Thunderbird profiles.ini found; set THUNDERBIRD_PROFILE")
}

// defaultProfileFromINI picks the default profile path from profiles.ini.
// Honors [Install*] Default=, else the [Profile*] with Default=1, else the first.
func defaultProfileFromINI(iniPath, root string) (string, error) {
	f, err := os.Open(iniPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	type section struct{ kv map[string]string }
	var sections []section
	var cur *section
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "[") {
			sections = append(sections, section{kv: map[string]string{}})
			cur = &sections[len(sections)-1]
			continue
		}
		if cur == nil || !strings.Contains(line, "=") {
			continue
		}
		k, v, _ := strings.Cut(line, "=")
		cur.kv[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}

	resolve := func(path, rel string) string {
		if rel == "1" || filepath.IsAbs(path) {
			return path
		}
		return filepath.Join(root, path)
	}
	// Install section wins.
	for _, s := range sections {
		if d := s.kv["Default"]; d != "" && s.kv["Path"] == "" {
			return filepath.Join(root, d), nil
		}
	}
	var first string
	for _, s := range sections {
		if p := s.kv["Path"]; p != "" {
			full := resolve(p, s.kv["IsRelative"])
			if first == "" {
				first = full
			}
			if s.kv["Default"] == "1" {
				return full, nil
			}
		}
	}
	if first == "" {
		return "", fmt.Errorf("no profile found in %s", iniPath)
	}
	return first, nil
}
