package liveaboard

import "time"

// BoatScrape carries the values produced by parsing a boat detail page.
// Stored into the boats table by the importer.
type BoatScrape struct {
	Slug       string // /diving/<country>/<slug>
	Country    string // /diving/<country>/<slug>; captured for future use
	Name       string
	URL        string // canonical URL (no ?m= query)
	ImageURL   string
	ExternalID string // numeric boat id from the image path, e.g., "5695"
}

// TripScrape carries the values produced by parsing one trip row.
// Stored into the trips table by the importer.
type TripScrape struct {
	StartDate        time.Time
	EndDate          time.Time
	Itinerary        string
	DeparturePort    string
	ReturnPort       string
	PriceText        string // raw, e.g., "$6,400"
	AvailabilityText string // raw, e.g., "FULL", "AVAILABLE"
	SourceURL        string // back-link, including the ?m=M/YYYY for that month
	SourceTripKey    string // deterministic fingerprint; see SourceTripKey()
}

// Result is what RunBoat returns to the CLI.
type Result struct {
	Boat            BoatScrape
	Trips           []TripScrape
	MonthsRequested int
	MonthsFetched   int
}
