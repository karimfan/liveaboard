package store

import "testing"

func TestNormalizeCabinLayoutInputParsesPasteAndCSV(t *testing.T) {
	preview, err := NormalizeCabinLayoutInput(CabinLayoutInput{
		Source: "paste",
		Paste:  "1,A,B\nSuite 4,Port,Starboard",
	})
	if err != nil {
		t.Fatalf("paste preview: %v", err)
	}
	if len(preview.Cabins) != 2 || len(preview.Cabins[0].Berths) != 2 {
		t.Fatalf("paste preview = %+v", preview.Cabins)
	}

	preview, err = NormalizeCabinLayoutInput(CabinLayoutInput{
		Source: "csv",
		CSV:    "cabin_label,berth_label,deck,sort_order,notes\n1,A,Lower,10,\n1,B,Lower,11,\n",
	})
	if err != nil {
		t.Fatalf("csv preview: %v", err)
	}
	if len(preview.Cabins) != 1 || preview.Cabins[0].Deck == nil || *preview.Cabins[0].Deck != "Lower" {
		t.Fatalf("csv preview = %+v", preview.Cabins)
	}
}

func TestNormalizeCabinLayoutInputRejectsAmbiguousPaste(t *testing.T) {
	if _, err := NormalizeCabinLayoutInput(CabinLayoutInput{Source: "paste", Paste: "1 AB"}); err == nil {
		t.Fatalf("ambiguous paste was accepted")
	}
}
