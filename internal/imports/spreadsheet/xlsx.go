package spreadsheet

import (
	"bytes"
	"fmt"
	"io"

	"github.com/xuri/excelize/v2"
)

// ParseXLSX consumes an .xlsx file via excelize. The first worksheet
// is treated as the schedule; later sheets are ignored.
//
// excelize accepts an io.Reader through OpenReader (it buffers
// internally). We also keep the raw bytes so the preview's
// SourceFingerprint matches what the operator uploaded.
func ParseXLSX(filename string, r io.Reader) (*Preview, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	f, err := excelize.OpenReader(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("open xlsx: %w", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("xlsx has no sheets")
	}
	sheet := sheets[0]

	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, fmt.Errorf("read rows: %w", err)
	}

	// excelize sometimes returns a single empty row trailing the
	// data; buildPreview already skips fully-empty rows so we don't
	// need to filter here.
	return buildPreview(filename, raw, rows)
}
