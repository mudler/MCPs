package main

import (
	"database/sql"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type Contacts struct{ db *sql.DB }

func OpenContacts(profileDir string) (*Contacts, error) {
	p := filepath.Join(profileDir, "abook.sqlite")
	if _, err := os.Stat(p); err != nil {
		return nil, nil
	}
	return openContactsAt(p)
}

func openContactsAt(path string) (*Contacts, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		return nil, err
	}
	return &Contacts{db: db}, nil
}

func (c *Contacts) Close() error {
	if c == nil {
		return nil
	}
	return c.db.Close()
}

const contactPivot = `
SELECT
  MAX(CASE WHEN name='DisplayName'  THEN value END) AS name,
  MAX(CASE WHEN name='PrimaryEmail' THEN value END) AS email,
  MAX(CASE WHEN name='NickName'     THEN value END) AS nick
FROM properties GROUP BY card`

func (c *Contacts) Search(term string, limit int) ([]Contact, error) {
	if c == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 25
	}
	q := "SELECT name, email, nick FROM (" + contactPivot + ") WHERE " +
		"(name LIKE ? OR email LIKE ? OR nick LIKE ?) LIMIT ?"
	like := "%" + term + "%"
	rows, err := c.db.Query(q, like, like, like, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanContacts(rows)
}

func (c *Contacts) Get(email string) (Contact, bool, error) {
	if c == nil {
		return Contact{}, false, nil
	}
	q := "SELECT name, email, nick FROM (" + contactPivot + ") WHERE email = ? LIMIT 1"
	rows, err := c.db.Query(q, email)
	if err != nil {
		return Contact{}, false, err
	}
	defer rows.Close()
	list, err := scanContacts(rows)
	if err != nil || len(list) == 0 {
		return Contact{}, false, err
	}
	return list[0], true, nil
}

func scanContacts(rows *sql.Rows) ([]Contact, error) {
	var out []Contact
	for rows.Next() {
		var name, email, nick sql.NullString
		if err := rows.Scan(&name, &email, &nick); err != nil {
			return nil, err
		}
		out = append(out, Contact{Name: name.String, Email: email.String, Nickname: nick.String})
	}
	return out, rows.Err()
}
