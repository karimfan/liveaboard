package spreadsheet

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
)

// ParseCSV consumes a CSV file and returns the canonical preview.
// FieldsPerRecord is set to -1 so ragged rows (common in
// hand-edited CSVs) don't fail outright; the parser pads short rows
// to the header length internally.
func ParseCSV(filename string, r io.Reader) (*Preview, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	reader := csv.NewReader(bytes.NewReader(raw))
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true

	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parse CSV: %w", err)
	}
	return buildPreview(filename, raw, records)
}
