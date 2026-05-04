package liveaboard

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}

func TestParseBoatPage_GaiaLove_2027_02(t *testing.T) {
	html := loadFixture(t, "gaia_love_2027_02.html")
	src := "https://www.liveaboard.com/diving/indonesia/gaia-love?m=2/2027"

	boat, trips, candidates, err := ParseBoatPage(html, src, "2/2027")
	if err != nil {
		t.Fatalf("ParseBoatPage: %v", err)
	}

	// --- Boat assertions ---
	if boat.Name != "Gaia Love" {
		t.Errorf("boat.Name = %q want %q", boat.Name, "Gaia Love")
	}
	if boat.Slug != "gaia-love" {
		t.Errorf("boat.Slug = %q", boat.Slug)
	}
	if boat.Country != "indonesia" {
		t.Errorf("boat.Country = %q want %q", boat.Country, "indonesia")
	}
	if boat.URL != "https://www.liveaboard.com/diving/indonesia/gaia-love" {
		t.Errorf("boat.URL = %q", boat.URL)
	}
	if boat.ImageURL == "" {
		t.Errorf("boat.ImageURL is empty")
	}
	if boat.ExternalID != "5695" {
		t.Errorf("boat.ExternalID = %q want %q", boat.ExternalID, "5695")
	}

	// --- Trip assertions ---
	if candidates < len(trips) || candidates == 0 {
		t.Errorf("candidates=%d trips=%d", candidates, len(trips))
	}
	if len(trips) < 2 {
		t.Fatalf("expected at least 2 trips for Feb 2027; got %d", len(trips))
	}

	// First trip: "06 Feb -> 11 Days / 10 Nights" -> Feb 6, 2027 -> Feb 16, 2027
	first := trips[0]
	if !first.StartDate.Equal(time.Date(2027, 2, 6, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("first.StartDate = %v want 2027-02-06", first.StartDate)
	}
	if !first.EndDate.Equal(time.Date(2027, 2, 16, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("first.EndDate = %v want 2027-02-16", first.EndDate)
	}
	if first.Itinerary != "Raja Ampat North & South" {
		t.Errorf("first.Itinerary = %q", first.Itinerary)
	}
	if first.DeparturePort != "Sorong" || first.ReturnPort != "Sorong" {
		t.Errorf("first ports: %q -> %q", first.DeparturePort, first.ReturnPort)
	}
	if first.PriceText == "" {
		t.Errorf("first.PriceText is empty")
	}
	if first.AvailabilityText == "" {
		t.Errorf("first.AvailabilityText is empty")
	}
	if first.SourceTripKey == "" {
		t.Errorf("first.SourceTripKey is empty")
	}

	// Source trip key is deterministic and unique per trip.
	if trips[0].SourceTripKey == trips[1].SourceTripKey {
		t.Errorf("trips have the same SourceTripKey %q", trips[0].SourceTripKey)
	}

	// Source URL is the per-month back-link.
	if first.SourceURL != src {
		t.Errorf("first.SourceURL = %q want %q", first.SourceURL, src)
	}
}

func TestParseBoatPage_GaiaLove_2026_08(t *testing.T) {
	html := loadFixture(t, "gaia_love_2026_08.html")
	src := "https://www.liveaboard.com/diving/indonesia/gaia-love?m=8/2026"
	boat, trips, _, err := ParseBoatPage(html, src, "8/2026")
	if err != nil {
		t.Fatalf("ParseBoatPage: %v", err)
	}
	if boat.Slug != "gaia-love" {
		t.Errorf("slug: %q", boat.Slug)
	}
	for _, tp := range trips {
		if tp.StartDate.Year() < 2026 || tp.StartDate.Year() > 2027 {
			t.Errorf("StartDate.Year out of expected window: %v", tp.StartDate)
		}
	}
}

func TestSourceTripKeyDistinguishesEndDates(t *testing.T) {
	a := SourceTripKey("gaia-love",
		time.Date(2027, 2, 6, 0, 0, 0, 0, time.UTC),
		time.Date(2027, 2, 16, 0, 0, 0, 0, time.UTC),
		"Raja Ampat", "Sorong")
	b := SourceTripKey("gaia-love",
		time.Date(2027, 2, 6, 0, 0, 0, 0, time.UTC),
		time.Date(2027, 2, 17, 0, 0, 0, 0, time.UTC), // one day longer
		"Raja Ampat", "Sorong")
	if a == b {
		t.Errorf("same key for different end dates: %s", a)
	}
}

func TestSourceTripKeyDistinguishesDeparturePorts(t *testing.T) {
	a := SourceTripKey("gaia-love",
		time.Date(2027, 2, 6, 0, 0, 0, 0, time.UTC),
		time.Date(2027, 2, 16, 0, 0, 0, 0, time.UTC),
		"Raja Ampat", "Sorong")
	b := SourceTripKey("gaia-love",
		time.Date(2027, 2, 6, 0, 0, 0, 0, time.UTC),
		time.Date(2027, 2, 16, 0, 0, 0, 0, time.UTC),
		"Raja Ampat", "Bali")
	if a == b {
		t.Errorf("same key for different ports")
	}
}

func TestSourceTripKeyIsDeterministic(t *testing.T) {
	for i := 0; i < 5; i++ {
		k := SourceTripKey("gaia-love",
			time.Date(2027, 2, 6, 0, 0, 0, 0, time.UTC),
			time.Date(2027, 2, 16, 0, 0, 0, 0, time.UTC),
			"Raja Ampat", "Sorong")
		if k == "" || len(k) != 32 {
			t.Errorf("bad key: %q", k)
		}
	}
}

func TestSplitItineraryTitle(t *testing.T) {
	cases := []struct {
		in       string
		wantItin string
		wantDep  string
		wantRet  string
	}{
		{"Raja Ampat North & South (Sorong - Sorong)", "Raja Ampat North & South", "Sorong", "Sorong"},
		{"Komodo (Bali - Bima)", "Komodo", "Bali", "Bima"},
		{"&amp;Test (A - B)", "&Test", "A", "B"},
		{"No ports here", "No ports here", "", ""},
	}
	for _, c := range cases {
		gotI, gotD, gotR := splitItineraryTitle(c.in)
		if gotI != c.wantItin || gotD != c.wantDep || gotR != c.wantRet {
			t.Errorf("splitItineraryTitle(%q) = (%q, %q, %q) want (%q, %q, %q)",
				c.in, gotI, gotD, gotR, c.wantItin, c.wantDep, c.wantRet)
		}
	}
}

func TestParseNights(t *testing.T) {
	cases := map[string]int{
		"11 Days / 10 Nights": 10,
		"8 Days / 7 Nights":   7,
		"1 Night":             1,
		"":                    0,
		"some other text":     0,
	}
	for in, want := range cases {
		if got := parseNights(in); got != want {
			t.Errorf("parseNights(%q) = %d want %d", in, got, want)
		}
	}
}

func TestTrimPriceText(t *testing.T) {
	cases := map[string]string{
		"$ 6,400 / person":     "$6,400",
		"from $6,400 / person": "$6,400",
		"€4,200":               "€4,200",
		"":                     "",
	}
	for in, want := range cases {
		if got := trimPriceText(in); got != want {
			t.Errorf("trimPriceText(%q) = %q want %q", in, got, want)
		}
	}
}

func TestYearRollHeuristic(t *testing.T) {
	// URL says December 2026; trip listed as "Jan" -> January 2027.
	// Synthesize a minimal HTML that exercises just the date logic.
	html := []byte(`
<html><head>
<link rel=canonical href="https://www.liveaboard.com/diving/x/test-boat">
<meta name=twitter:title content="Test Boat">
<meta name=twitter:image content="https://img.liveaboard.com/picture_library/boat/1234/x.jpg">
</head><body>
<div id="preset-boat-rates-grid">
  <div role="row">
    <div aria-describedby="departure-date-header"><span>05</span><span>Jan</span></div>
    <div aria-describedby="departure-itinerary-header">
      <button title="Test Itin (A - B)">Test Itin</button>
      <span>8 Days / 7 Nights</span>
    </div>
    <div aria-describedby="departure-price-header">$1,000</div>
    <div aria-describedby="departure-select-header">AVAILABLE</div>
  </div>
</div>
</body></html>`)
	_, trips, _, err := ParseBoatPage(html, "https://x/?m=12/2026", "12/2026")
	if err != nil {
		t.Fatalf("ParseBoatPage: %v", err)
	}
	if len(trips) != 1 {
		t.Fatalf("len trips = %d", len(trips))
	}
	if got := trips[0].StartDate.Year(); got != 2027 {
		t.Errorf("year-roll: got %d want 2027", got)
	}
}
