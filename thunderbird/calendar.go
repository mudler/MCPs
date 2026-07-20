package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type Calendar struct {
	db    *sql.DB
	names map[string]string // cal_id -> display name
}

func OpenCalendar(profileDir string, cals []CalendarRef) (*Calendar, error) {
	p := filepath.Join(profileDir, "calendar-data", "local.sqlite")
	if _, err := os.Stat(p); err != nil {
		return nil, nil
	}
	names := map[string]string{}
	for _, c := range cals {
		names[c.UUID] = c.Name
	}
	return openCalendarAt(p, names)
}

func openCalendarAt(path string, names map[string]string) (*Calendar, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		return nil, err
	}
	return &Calendar{db: db, names: names}, nil
}

func (c *Calendar) Close() error {
	if c == nil {
		return nil
	}
	return c.db.Close()
}

const eventsQuery = `
SELECT e.cal_id, e.id, e.title, e.event_start, e.event_end, p.value
FROM cal_events e
LEFT JOIN cal_properties p ON p.item_id = e.id AND p.cal_id = e.cal_id AND p.key = 'LOCATION'
WHERE e.event_start >= ? AND e.event_start <= ?
ORDER BY e.event_start ASC LIMIT ?`

func (c *Calendar) Events(since, until time.Time, limit int) ([]CalendarEvent, error) {
	if c == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 50
	}
	rows, err := c.db.Query(eventsQuery, since.UnixMicro(), until.UnixMicro(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CalendarEvent
	for rows.Next() {
		var calID, id, title string
		var startUS, endUS int64
		var loc sql.NullString
		if err := rows.Scan(&calID, &id, &title, &startUS, &endUS, &loc); err != nil {
			return nil, err
		}
		name := c.names[calID]
		if name == "" {
			name = calID
		}
		out = append(out, CalendarEvent{
			ID: id, Calendar: name, Title: title, Location: loc.String,
			Start: time.UnixMicro(startUS).UTC(), End: time.UnixMicro(endUS).UTC(),
		})
	}
	return out, rows.Err()
}
