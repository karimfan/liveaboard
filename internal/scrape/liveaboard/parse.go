package liveaboard

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// ErrSelectorDrift is returned when the parser saw trip-shaped DOM
// nodes but failed to extract a single complete trip from them. The
// caller (RunBoat / CLI) treats this as a non-zero exit so a markup
// change on the source site is loud rather than silent.
var ErrSelectorDrift = errors.New("liveaboard: selector drift; parsed zero trips from a non-empty trip grid")

// ParseBoatPage extracts a BoatScrape and all trip rows present on a
// single boat detail page. monthYear ("M/YYYY") is the value of the
// ?m= query parameter used to fetch this page; it is the year context
// for date cells that show only "DD Mon".
//
// candidatesSeen reports how many trip-shaped DOM nodes were found
// (whether or not they parsed). The caller uses it to distinguish a
// legitimately empty month from selector drift.
func ParseBoatPage(htmlBody []byte, sourceURL, monthYear string) (boat BoatScrape, trips []TripScrape, candidatesSeen int, err error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(htmlBody))
	if err != nil {
		return BoatScrape{}, nil, 0, fmt.Errorf("parse html: %w", err)
	}

	boat, err = parseBoat(doc, sourceURL)
	if err != nil {
		return BoatScrape{}, nil, 0, err
	}

	defaultYear, urlMonth := parseMonthYear(monthYear)
	trips, candidatesSeen = parseTrips(doc, boat.Slug, sourceURL, defaultYear, urlMonth)
	return boat, trips, candidatesSeen, nil
}

// SourceTripKey is the deterministic fingerprint used as the trip
// uniqueness key in the database. Itinerary marketing copy alone is
// not identity; this fingerprint pins on (slug, dates, itinerary,
// departure port).
func SourceTripKey(slug string, startDate, endDate time.Time, itinerary, departurePort string) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s|%s|%s|%s|%s",
		strings.ToLower(strings.TrimSpace(slug)),
		startDate.Format("2006-01-02"),
		endDate.Format("2006-01-02"),
		strings.ToLower(strings.TrimSpace(itinerary)),
		strings.ToLower(strings.TrimSpace(departurePort)))
	return hex.EncodeToString(h.Sum(nil))[:32]
}

// --- boat ---

func parseBoat(doc *goquery.Document, sourceURL string) (BoatScrape, error) {
	canonical := strings.TrimSpace(doc.Find(`link[rel="canonical"]`).AttrOr("href", ""))
	if canonical == "" {
		canonical = sourceURL
	}
	canonical = stripQueryString(canonical)

	slug, country := slugAndCountryFromURL(canonical)
	if slug == "" {
		return BoatScrape{}, fmt.Errorf("liveaboard: could not derive slug from URL %q", canonical)
	}

	// Boat name: prefer the og:title / twitter:title meta which is the
	// cleanest "Gaia Love" form. Fall back to <title> with the trailing
	// ", Country - LiveAboard.com" stripped.
	name := strings.TrimSpace(doc.Find(`meta[name="twitter:title"]`).AttrOr("content", ""))
	if name == "" {
		name = strings.TrimSpace(doc.Find(`meta[property="og:title"]`).AttrOr("content", ""))
	}
	if name == "" {
		t := strings.TrimSpace(doc.Find("title").Text())
		// "Gaia Love, Indonesia - LiveAboard.com" -> "Gaia Love"
		if idx := strings.Index(t, ","); idx > 0 {
			name = strings.TrimSpace(t[:idx])
		} else if idx := strings.Index(t, " - "); idx > 0 {
			name = strings.TrimSpace(t[:idx])
		} else {
			name = t
		}
	}
	if name == "" {
		return BoatScrape{}, fmt.Errorf("liveaboard: could not extract boat name from %q", canonical)
	}

	imageURL := strings.TrimSpace(doc.Find(`meta[name="twitter:image"]`).AttrOr("content", ""))
	if imageURL == "" {
		imageURL = strings.TrimSpace(doc.Find(`meta[property="og:image"]`).AttrOr("content", ""))
	}
	imageURL = stripImageTransformQuery(imageURL)

	return BoatScrape{
		Slug:       slug,
		Country:    country,
		Name:       name,
		URL:        canonical,
		ImageURL:   imageURL,
		ExternalID: externalIDFromImageURL(imageURL),
	}, nil
}

// slugAndCountryFromURL extracts ("gaia-love", "indonesia") from
// "https://www.liveaboard.com/diving/indonesia/gaia-love".
func slugAndCountryFromURL(rawURL string) (slug, country string) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", ""
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	// /diving/<country>/<slug>
	if len(parts) >= 3 && parts[0] == "diving" {
		return parts[2], parts[1]
	}
	if len(parts) > 0 {
		return parts[len(parts)-1], ""
	}
	return "", ""
}

// stripImageTransformQuery removes the ?tr=w-1200,h-628 image-transform
// query from the meta image URL so we record the canonical asset URL.
func stripImageTransformQuery(s string) string { return stripQueryString(s) }

func stripQueryString(s string) string {
	if i := strings.IndexByte(s, '?'); i >= 0 {
		return s[:i]
	}
	return s
}

// externalIDFromImageURL extracts "5695" from
// "https://img.liveaboard.com/picture_library/boat/5695/gaia-main.jpg".
var imageBoatIDRegexp = regexp.MustCompile(`/boat/(\d+)/`)

func externalIDFromImageURL(s string) string {
	if m := imageBoatIDRegexp.FindStringSubmatch(s); len(m) == 2 {
		return m[1]
	}
	return ""
}

// --- trips ---

// parseTrips walks every trip row inside #preset-boat-rates-grid (and
// also #boat-rates-grid for the structured view) and returns the
// successfully parsed TripScrapes plus the number of candidate rows
// seen.
//
// monthFromURL is 1..12 derived from the page's ?m= query; trips with
// a month abbreviation that's earlier than monthFromURL are assumed to
// roll into the next year (e.g. URL m=12/2026, trip "05 Jan" -> 2027).
func parseTrips(
	doc *goquery.Document,
	slug, sourceURL string,
	defaultYear, monthFromURL int,
) ([]TripScrape, int) {
	var trips []TripScrape
	var candidates int

	// The page has two grids ("preset-boat-rates-grid" is the
	// pre-rendered list; "boat-rates-grid" is hydrated by JS for the
	// price-detail card). The first one carries every row in
	// server-rendered HTML; we parse only that.
	doc.Find(`#preset-boat-rates-grid div[role="row"]`).Each(func(_ int, row *goquery.Selection) {
		candidates++
		t, ok := parseTripRow(row, slug, sourceURL, defaultYear, monthFromURL)
		if ok {
			trips = append(trips, t)
		}
	})
	return trips, candidates
}

// parseTripRow extracts one trip from a single row. Returns ok=false if
// the row is too incomplete to record (missing date or itinerary).
func parseTripRow(
	row *goquery.Selection,
	slug, sourceURL string,
	defaultYear, monthFromURL int,
) (TripScrape, bool) {
	dateCell := row.Find(`[aria-describedby="departure-date-header"]`).First()
	if dateCell.Length() == 0 {
		return TripScrape{}, false
	}
	itinCell := row.Find(`[aria-describedby="departure-itinerary-header"]`).First()
	if itinCell.Length() == 0 {
		return TripScrape{}, false
	}
	priceCell := row.Find(`[aria-describedby="departure-price-header"]`).First()
	selectCell := row.Find(`[aria-describedby="departure-select-header"]`).First()

	// Date: two <span>s, "06" and "Feb".
	spans := dateCell.Find("span")
	if spans.Length() < 2 {
		return TripScrape{}, false
	}
	dayStr := strings.TrimSpace(spans.Eq(0).Text())
	monStr := strings.TrimSpace(spans.Eq(1).Text())
	day, err := strconv.Atoi(dayStr)
	if err != nil || day < 1 || day > 31 {
		return TripScrape{}, false
	}
	month, ok := parseMonthAbbrev(monStr)
	if !ok {
		return TripScrape{}, false
	}
	year := defaultYear
	// Year-roll heuristic: if the URL is e.g. December and we see
	// "Jan", it must be next year. If the URL is e.g. January and we
	// see "Dec", it's last year (rare; only for trips spanning the
	// new year and whose start date is in the previous month). We use
	// a 6-month window to disambiguate.
	if monthFromURL > 0 {
		diff := int(month) - monthFromURL
		switch {
		case diff < -6:
			year++
		case diff > 6:
			year--
		}
	}
	startDate := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)

	// Itinerary: the <button> with a title attribute holds the canonical
	// "Itinerary (Departure - Return)" string.
	titleAttr := strings.TrimSpace(itinCell.Find("button[title]").First().AttrOr("title", ""))
	if titleAttr == "" {
		// Fall back to button text content.
		titleAttr = strings.TrimSpace(itinCell.Find("button").First().Text())
	}
	itinerary, departurePort, returnPort := splitItineraryTitle(titleAttr)
	if itinerary == "" {
		return TripScrape{}, false
	}

	// Duration: the cell contains a span like "11 Days / 10 Nights".
	durText := compactWhitespace(itinCell.Text())
	nights := parseNights(durText)
	endDate := startDate
	if nights > 0 {
		endDate = startDate.AddDate(0, 0, nights)
	}
	if !endDate.After(startDate) {
		// Some trips might list "8 Days / 7 Nights"; if we somehow
		// failed to read nights, fall back to a plausible 1-day floor
		// so the database CHECK doesn't reject. The dedup key still
		// includes endDate so this row is still distinguishable.
		endDate = startDate.AddDate(0, 0, 1)
	}

	priceText := compactWhitespace(priceCell.Text())
	priceText = trimPriceText(priceText)

	availability := normalizeAvailability(row, selectCell)

	return TripScrape{
		StartDate:        startDate,
		EndDate:          endDate,
		Itinerary:        itinerary,
		DeparturePort:    departurePort,
		ReturnPort:       returnPort,
		PriceText:        priceText,
		AvailabilityText: availability,
		SourceURL:        sourceURL,
		SourceTripKey:    SourceTripKey(slug, startDate, endDate, itinerary, departurePort),
	}, true
}

// splitItineraryTitle splits "Raja Ampat North & South (Sorong - Sorong)"
// into ("Raja Ampat North & South", "Sorong", "Sorong").
var itinTitleRegexp = regexp.MustCompile(`^(.+?)\s*\(([^)]+?)\s*[-–]\s*([^)]+?)\)\s*$`)

func splitItineraryTitle(s string) (itinerary, depPort, retPort string) {
	s = decodeHTMLEntities(strings.TrimSpace(s))
	if m := itinTitleRegexp.FindStringSubmatch(s); m != nil {
		return strings.TrimSpace(m[1]), strings.TrimSpace(m[2]), strings.TrimSpace(m[3])
	}
	return s, "", ""
}

// parseNights pulls "10" from "11 Days / 10 Nights" (or returns 0).
var nightsRegexp = regexp.MustCompile(`(?i)(\d+)\s*Nights?`)

func parseNights(s string) int {
	if m := nightsRegexp.FindStringSubmatch(s); len(m) == 2 {
		n, _ := strconv.Atoi(m[1])
		return n
	}
	return 0
}

// trimPriceText keeps the leading currency symbol + amount and drops
// trailing "/ person" / "from" decorations the source markup includes.
var pricePrefixRegexp = regexp.MustCompile(`(\$|€|£|AUD|EUR|USD|GBP)\s*\d[\d,]*`)

func trimPriceText(s string) string {
	s = strings.TrimSpace(s)
	if m := pricePrefixRegexp.FindString(s); m != "" {
		return strings.ReplaceAll(m, " ", "")
	}
	// Last-resort: collapse whitespace and return the first 32 chars.
	if len(s) > 32 {
		s = s[:32]
	}
	return s
}

// parseMonthYear pulls (year, month) from "M/YYYY" (no zero padding).
// monthYear == "" yields (0, 0); the caller can use those zero-values
// as "ignore the URL year hint".
func parseMonthYear(monthYear string) (year, month int) {
	if monthYear == "" {
		return 0, 0
	}
	parts := strings.SplitN(monthYear, "/", 2)
	if len(parts) != 2 {
		return 0, 0
	}
	m, err1 := strconv.Atoi(parts[0])
	y, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || m < 1 || m > 12 {
		return 0, 0
	}
	return y, m
}

var monthAbbrevs = map[string]time.Month{
	"jan": time.January, "feb": time.February, "mar": time.March,
	"apr": time.April, "may": time.May, "jun": time.June,
	"jul": time.July, "aug": time.August, "sep": time.September,
	"sept": time.September,
	"oct":  time.October, "nov": time.November, "dec": time.December,
}

func parseMonthAbbrev(s string) (time.Month, bool) {
	m, ok := monthAbbrevs[strings.ToLower(strings.TrimSpace(s))]
	return m, ok
}

// normalizeAvailability extracts a clean status string from the row.
// The source markup mixes a modal-trigger button labeled "Itinerary"
// with the actual status indicator (a `.text-red-600` span carrying
// "FULL", or a "Select Cabin" / "Request to Book" CTA when bookable).
// We prefer the explicit indicator and fall back to "AVAILABLE" when
// the select cell holds a CTA but no negative status is present.
func normalizeAvailability(row *goquery.Selection, selectCell *goquery.Selection) string {
	// Negative statuses live in red text.
	if neg := compactWhitespace(row.Find(".text-red-600").Text()); neg != "" {
		return strings.ToUpper(neg)
	}
	// A "Select Cabin" or "Request to Book" CTA implies availability.
	cta := compactWhitespace(selectCell.Find("button, a").Not(`[aria-controls*="itinerary-modal"]`).First().Text())
	if cta != "" {
		// Keep the CTA verb tidy; strip the "Itinerary" decoration if it
		// sneaks in. Also normalize common variants.
		cta = strings.TrimSpace(strings.ReplaceAll(cta, "Itinerary", ""))
		switch strings.ToLower(cta) {
		case "select cabin", "select cabin available", "available":
			return "AVAILABLE"
		case "request to book", "request":
			return "ON REQUEST"
		}
		return strings.ToUpper(cta)
	}
	// Last resort: raw select-cell text minus the modal-button label.
	raw := compactWhitespace(selectCell.Text())
	raw = strings.TrimSpace(strings.ReplaceAll(raw, "Itinerary", ""))
	return strings.ToUpper(raw)
}

// compactWhitespace flattens runs of whitespace into a single space.
var wsRegexp = regexp.MustCompile(`\s+`)

func compactWhitespace(s string) string {
	return strings.TrimSpace(wsRegexp.ReplaceAllString(s, " "))
}

// decodeHTMLEntities is a tiny replacement for the handful of entities
// that appear in liveaboard.com titles (`&amp;`, `&#39;`). goquery's
// .Text() decodes most entities already; this catches the cases where
// we read an attribute value via .AttrOr().
func decodeHTMLEntities(s string) string {
	r := strings.NewReplacer(
		"&amp;", "&",
		"&#39;", "'",
		"&apos;", "'",
		"&quot;", `"`,
		"&lt;", "<",
		"&gt;", ">",
		"&nbsp;", " ",
	)
	return r.Replace(s)
}
