//go:build ignore

// gen_xlsx.go writes ok.xlsx — a minimal fixture with the same shape
// as ok.csv. Run with `go run testdata/gen_xlsx.go` from inside this
// package directory if the fixture needs regenerating.
package main

import (
	"log"

	"github.com/xuri/excelize/v2"
)

func main() {
	f := excelize.NewFile()
	defer f.Close()

	sh := f.GetSheetName(0) // default "Sheet1"
	rows := [][]any{
		{"vessel name", "trip start date", "trip end date", "itinerary", "number of guests"},
		{"Gaia Love", "2026-06-02", "2026-06-09", "Komodo North", 12},
		{"Seahorse", "2026-07-04", "2026-07-11", "Raja Ampat", ""},
		{"Gaia Love", "Aug 14, 2026", "Aug 21, 2026", "Banda Sea", 8},
	}
	for i, row := range rows {
		cell, _ := excelize.CoordinatesToCellName(1, i+1)
		if err := f.SetSheetRow(sh, cell, &row); err != nil {
			log.Fatalf("set row %d: %v", i, err)
		}
	}
	if err := f.SaveAs("ok.xlsx"); err != nil {
		log.Fatalf("save: %v", err)
	}
	log.Println("wrote ok.xlsx")
}
