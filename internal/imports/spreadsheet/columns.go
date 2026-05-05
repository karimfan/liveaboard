package spreadsheet

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// columnRole names a logical column. Header strings in the upload are
// normalized and matched against the alias maps below to recover this
// role.
type columnRole int

const (
	colVessel columnRole = iota
	colStart
	colEnd
	colItinerary
	colNumGuests
)

var aliasMap = map[columnRole][]string{
	colVessel:    {"vessel", "vessel name", "boat", "boat name"},
	colStart:     {"start date", "trip start date", "start", "from", "departure date"},
	colEnd:       {"end date", "trip end date", "end", "to", "return date"},
	colItinerary: {"itinerary", "route", "destination"},
	colNumGuests: {"number of guests", "guests", "num guests", "guest count", "guests on board"},
}

// columnIndex maps each known role to the 0-based column position in
// the header row. -1 means "not present in the file." Roles other
// than colNumGuests are required.
type columnIndex map[columnRole]int

// detectColumns walks the header row and returns the column index. An
// error is returned if any required column is missing — the caller
// should surface this as a top-level upload error (no preview shown).
func detectColumns(headers []string) (columnIndex, error) {
	idx := columnIndex{}
	for role := range aliasMap {
		idx[role] = -1
	}

	for i, raw := range headers {
		key := normalizeHeader(raw)
		if key == "" {
			continue
		}
		for role, aliases := range aliasMap {
			for _, a := range aliases {
				if key == a {
					// First match wins — operators occasionally
					// duplicate columns; we honor the leftmost.
					if idx[role] == -1 {
						idx[role] = i
					}
					break
				}
			}
		}
	}

	var missing []string
	for _, role := range []columnRole{colVessel, colStart, colEnd, colItinerary} {
		if idx[role] == -1 {
			missing = append(missing, columnLabel(role))
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required column(s): %s", strings.Join(missing, ", "))
	}
	return idx, nil
}

func columnLabel(r columnRole) string {
	switch r {
	case colVessel:
		return "vessel name"
	case colStart:
		return "trip start date"
	case colEnd:
		return "trip end date"
	case colItinerary:
		return "itinerary"
	case colNumGuests:
		return "number of guests"
	}
	return "unknown"
}

func normalizeHeader(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	// Collapse runs of whitespace and underscores to a single space.
	var b strings.Builder
	prevSpace := false
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '_' || r == '-' {
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
			continue
		}
		b.WriteRune(r)
		prevSpace = false
	}
	return strings.TrimSpace(b.String())
}

// acceptedDateFormats are the only date layouts the parser will
// recognize. Anything else triggers a `bad_date` warning that names
// the accepted formats.
var acceptedDateFormats = []string{
	"2006-01-02", // ISO 8601
	"Jan 2, 2006",
	"2 Jan 2006",
}

var errBadDate = errors.New("date does not match an accepted format")

// parseDate tries each accepted format in order. Any match wins; nil
// returned when none do.
func parseDate(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, errBadDate
	}
	for _, layout := range acceptedDateFormats {
		t, err := time.Parse(layout, raw)
		if err == nil {
			return t, nil
		}
	}
	return time.Time{}, errBadDate
}

// acceptedDateFormatsHumanList renders the accepted formats for use
// in the warning message.
func acceptedDateFormatsHumanList() string {
	return `"YYYY-MM-DD", "Jan 2, 2026", or "2 Jan 2026"`
}
