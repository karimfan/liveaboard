package spreadsheet_test

import (
	"os"
	"strings"
	"testing"

	"github.com/karimfan/liveaboard/internal/imports/spreadsheet"
)

func TestParseCSVHappyPath(t *testing.T) {
	f, err := os.Open("testdata/ok.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	prev, err := spreadsheet.ParseCSV("ok.csv", f)
	if err != nil {
		t.Fatalf("ParseCSV: %v", err)
	}
	if len(prev.Rows) != 3 {
		t.Fatalf("len(rows) = %d, want 3", len(prev.Rows))
	}
	if len(prev.Warnings) != 0 {
		t.Errorf("warnings: %+v", prev.Warnings)
	}
	if got := prev.VesselNames; len(got) != 2 || got[0] != "Gaia Love" || got[1] != "Seahorse" {
		t.Errorf("vessel names: %v", got)
	}
	if prev.Rows[0].NumGuests == nil || *prev.Rows[0].NumGuests != 12 {
		t.Errorf("row 0 num_guests: %v", prev.Rows[0].NumGuests)
	}
	if prev.Rows[1].NumGuests != nil {
		t.Errorf("row 1 num_guests: want nil, got %v", *prev.Rows[1].NumGuests)
	}
	if prev.SourceFingerprint == "" {
		t.Errorf("expected fingerprint")
	}
}

func TestParseCSVMissingRequiredColumn(t *testing.T) {
	f, err := os.Open("testdata/missing_vessel_col.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	_, err = spreadsheet.ParseCSV("missing_vessel_col.csv", f)
	if err == nil {
		t.Fatal("expected error for missing column")
	}
	if !strings.Contains(err.Error(), "vessel name") {
		t.Errorf("error should mention vessel name: %v", err)
	}
}

func TestParseCSVWarnings(t *testing.T) {
	f, err := os.Open("testdata/bad_dates.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	prev, err := spreadsheet.ParseCSV("bad_dates.csv", f)
	if err != nil {
		t.Fatalf("ParseCSV: %v", err)
	}

	codes := map[string]int{}
	for _, w := range prev.Warnings {
		codes[w.Code]++
	}
	if codes[spreadsheet.WarnBadDate] < 2 {
		t.Errorf("bad_date warnings: want >=2 (start+end on row 2), got %d", codes[spreadsheet.WarnBadDate])
	}
	if codes[spreadsheet.WarnEndBeforeStart] != 1 {
		t.Errorf("end_before_start: %d", codes[spreadsheet.WarnEndBeforeStart])
	}
	if codes[spreadsheet.WarnDuplicateRow] != 1 {
		t.Errorf("duplicate_row: %d", codes[spreadsheet.WarnDuplicateRow])
	}
	if codes[spreadsheet.WarnEmptyVessel] != 1 {
		t.Errorf("empty_vessel: %d", codes[spreadsheet.WarnEmptyVessel])
	}
}

func TestParseXLSXHappyPath(t *testing.T) {
	f, err := os.Open("testdata/ok.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	prev, err := spreadsheet.ParseXLSX("ok.xlsx", f)
	if err != nil {
		t.Fatalf("ParseXLSX: %v", err)
	}
	if len(prev.Rows) != 3 {
		t.Fatalf("len(rows) = %d, want 3", len(prev.Rows))
	}
	if len(prev.Warnings) != 0 {
		t.Errorf("warnings: %+v", prev.Warnings)
	}
	if prev.Rows[0].VesselName != "Gaia Love" {
		t.Errorf("row 0 vessel: %q", prev.Rows[0].VesselName)
	}
	if prev.Rows[2].StartDate.Year() != 2026 || prev.Rows[2].StartDate.Month() != 8 {
		t.Errorf("row 2 start date wrong: %v", prev.Rows[2].StartDate)
	}
}

func TestParseCSVEmptyTrailingRows(t *testing.T) {
	csv := strings.NewReader(
		"vessel name,trip start date,trip end date,itinerary\n" +
			"Gaia Love,2026-06-02,2026-06-09,Komodo\n" +
			",,,,\n" +
			"\n" +
			",,,\n",
	)
	prev, err := spreadsheet.ParseCSV("trail.csv", csv)
	if err != nil {
		t.Fatalf("ParseCSV: %v", err)
	}
	if len(prev.Rows) != 1 {
		t.Errorf("len(rows) = %d, want 1 (trailing blanks should be skipped)", len(prev.Rows))
	}
}
