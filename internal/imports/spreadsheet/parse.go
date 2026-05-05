package spreadsheet

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// fingerprint returns a stable hex prefix of sha256(payload). Used to
// link the persisted preview back to the bytes the operator uploaded
// so the SPA can warn if a re-upload doesn't match the previewed file.
func fingerprint(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:8]) // 16 hex chars; collision-resistant enough.
}

// buildPreview is the format-agnostic core. CSV and XLSX adapters
// produce a string-of-cells matrix and call this. The first row is
// the header.
//
// Strict-but-friendly philosophy: a structurally invalid file
// (missing column, no data rows) returns an error that the handler
// surfaces as a 4xx without a preview. A row with bad data
// (unparseable date, end before start, empty vessel/itinerary)
// becomes a warning attached to the row but stays in the preview so
// the operator can decide.
func buildPreview(filename string, raw []byte, table [][]string) (*Preview, error) {
	if len(table) == 0 {
		return nil, fmt.Errorf("file is empty")
	}
	headers := table[0]
	idx, err := detectColumns(headers)
	if err != nil {
		return nil, err
	}
	if len(table) < 2 {
		return nil, fmt.Errorf("file has no data rows")
	}

	prev := &Preview{
		Filename:          filename,
		SourceFingerprint: fingerprint(raw),
		Headers:           append([]string(nil), headers...),
	}

	// Row dedup tracking: vessel|start|end|itinerary -> first line
	// number we saw it on. Subsequent matches yield WarnDuplicateRow.
	seen := map[string]int{}

	uniqueVessels := map[string]struct{}{}

	for rowI, rec := range table[1:] {
		lineNumber := rowI + 2 // header is line 1

		vesselRaw := safeCell(rec, idx[colVessel])
		startRaw := safeCell(rec, idx[colStart])
		endRaw := safeCell(rec, idx[colEnd])
		itinRaw := safeCell(rec, idx[colItinerary])
		guestsRaw := ""
		if idx[colNumGuests] >= 0 {
			guestsRaw = safeCell(rec, idx[colNumGuests])
		}

		// Empty rows (often trailing blanks in CSVs) are silently
		// skipped — no warning, no row.
		if strings.TrimSpace(vesselRaw)+strings.TrimSpace(startRaw)+strings.TrimSpace(endRaw)+strings.TrimSpace(itinRaw) == "" {
			continue
		}

		row := Row{LineNumber: lineNumber}
		row.VesselName = strings.TrimSpace(vesselRaw)
		row.Itinerary = strings.TrimSpace(itinRaw)

		if row.VesselName == "" {
			prev.Warnings = append(prev.Warnings, Warning{
				LineNumber: lineNumber, Code: WarnEmptyVessel,
				Message: "vessel name is empty",
			})
		} else {
			uniqueVessels[row.VesselName] = struct{}{}
		}
		if row.Itinerary == "" {
			prev.Warnings = append(prev.Warnings, Warning{
				LineNumber: lineNumber, Code: WarnEmptyItinerary,
				Message: "itinerary is empty",
			})
		}

		startD, errStart := parseDate(startRaw)
		endD, errEnd := parseDate(endRaw)
		if errStart != nil {
			prev.Warnings = append(prev.Warnings, Warning{
				LineNumber: lineNumber, Code: WarnBadDate,
				Message: fmt.Sprintf("start date %q does not match an accepted format. Use %s.", strings.TrimSpace(startRaw), acceptedDateFormatsHumanList()),
			})
		} else {
			row.StartDate = startD
		}
		if errEnd != nil {
			prev.Warnings = append(prev.Warnings, Warning{
				LineNumber: lineNumber, Code: WarnBadDate,
				Message: fmt.Sprintf("end date %q does not match an accepted format. Use %s.", strings.TrimSpace(endRaw), acceptedDateFormatsHumanList()),
			})
		} else {
			row.EndDate = endD
		}
		if errStart == nil && errEnd == nil && row.EndDate.Before(row.StartDate) {
			prev.Warnings = append(prev.Warnings, Warning{
				LineNumber: lineNumber, Code: WarnEndBeforeStart,
				Message: "end date is before start date",
			})
		}

		if guestsRaw = strings.TrimSpace(guestsRaw); guestsRaw != "" {
			n, err := strconv.Atoi(guestsRaw)
			if err == nil && n >= 0 {
				row.NumGuests = &n
			}
			// We deliberately don't warn on a bad guest count — the
			// column is optional and "couldn't parse" is fine to
			// quietly ignore at MVP. Operators can fix and re-upload.
		}

		// Duplicate detection — same vessel + dates + itinerary.
		key := strings.ToLower(row.VesselName) + "|" +
			startRaw + "|" + endRaw + "|" +
			strings.ToLower(row.Itinerary)
		if prevLine, hit := seen[key]; hit {
			prev.Warnings = append(prev.Warnings, Warning{
				LineNumber: lineNumber, Code: WarnDuplicateRow,
				Message: fmt.Sprintf("duplicate of line %d", prevLine),
			})
		} else {
			seen[key] = lineNumber
		}

		prev.Rows = append(prev.Rows, row)
	}

	prev.VesselNames = make([]string, 0, len(uniqueVessels))
	for v := range uniqueVessels {
		prev.VesselNames = append(prev.VesselNames, v)
	}
	sort.Strings(prev.VesselNames)

	return prev, nil
}

// safeCell returns the cell at column i, or "" if the row is short
// (Excel often emits ragged rows when later columns are blank).
func safeCell(rec []string, i int) string {
	if i < 0 || i >= len(rec) {
		return ""
	}
	return rec[i]
}
