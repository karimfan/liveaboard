// Package spreadsheet parses operator-uploaded schedules into a
// canonical row shape the import handler can persist. Two formats are
// supported: CSV (stdlib encoding/csv) and XLSX (xuri/excelize/v2).
//
// The parser is deliberately strict on date formats — only ISO 8601
// (`2006-01-02`) and named-month forms (`Jan 2, 2006`, `2 Jan 2006`)
// are accepted. Locale-ambiguous slash-separated formats (`5/6/2026`)
// would otherwise force a disambiguation UI we explicitly chose not
// to build for Sprint 012; offending rows surface as warnings.
package spreadsheet

import "time"

// Row is one parsed schedule row. LineNumber is 1-based; the header
// row is line 1, so the first data row is line 2.
type Row struct {
	LineNumber int       `json:"line_number"`
	VesselName string    `json:"vessel_name"`
	StartDate  time.Time `json:"start_date"`
	EndDate    time.Time `json:"end_date"`
	Itinerary  string    `json:"itinerary"`
	NumGuests  *int      `json:"num_guests,omitempty"`
}

// Warning describes a non-fatal problem with a row. The row is still
// included in the preview but flagged so the operator can decide
// whether to skip it before commit.
type Warning struct {
	LineNumber int    `json:"line_number"`
	Code       string `json:"code"`
	Message    string `json:"message"`
}

// Warning codes — kept here so handler tests can assert against them.
const (
	WarnBadDate        = "bad_date"
	WarnEndBeforeStart = "end_before_start"
	WarnDuplicateRow   = "duplicate_row"
	WarnEmptyVessel    = "empty_vessel"
	WarnEmptyItinerary = "empty_itinerary"
)

// Preview is the full result of parsing a file. The handler persists
// it as JSON in `import_previews.payload` and returns it to the SPA;
// the wizard renders Rows + Warnings, and the commit handler resolves
// VesselNames against existing boats via a per-vessel mapping.
type Preview struct {
	Filename          string    `json:"filename"`
	SourceFingerprint string    `json:"source_fingerprint"`
	Headers           []string  `json:"headers"`
	Rows              []Row     `json:"rows"`
	Warnings          []Warning `json:"warnings"`
	VesselNames       []string  `json:"vessel_names"`
}
