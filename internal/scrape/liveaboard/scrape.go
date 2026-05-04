package liveaboard

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"
)

// RunBoatOptions configures a single scrape pass.
type RunBoatOptions struct {
	// BaseURL is the boat detail URL without the ?m= query, e.g.
	// https://www.liveaboard.com/diving/indonesia/gaia-love. Any
	// existing query string is stripped.
	BaseURL string

	// Months is how many months of trips to scrape, starting at the
	// current month. Default 18.
	Months int

	// Today anchors the month iteration. Lets tests pin the date
	// without playing with time.Now. nil -> time.Now().UTC().
	Today func() time.Time

	// Client is the HTTP fetcher. Required.
	Client *Client

	// Log receives orchestration events. nil -> discard.
	Log *slog.Logger
}

// RunBoat fetches the boat detail page across `Months` consecutive
// months starting at Today(), parses each, and returns the merged
// Result. Trips are deduplicated across months by SourceTripKey
// (multi-month trips appear on both pages).
//
// On a parser failure for any month with `candidatesSeen > 0 &&
// len(parsed) == 0`, RunBoat returns ErrSelectorDrift. The CLI exits
// non-zero so the operator notices the markup change.
func RunBoat(ctx context.Context, opts RunBoatOptions) (*Result, error) {
	if opts.Client == nil {
		return nil, errors.New("liveaboard: RunBoatOptions.Client is required")
	}
	if strings.TrimSpace(opts.BaseURL) == "" {
		return nil, errors.New("liveaboard: RunBoatOptions.BaseURL is required")
	}
	if opts.Months <= 0 {
		opts.Months = 18
	}
	if opts.Today == nil {
		opts.Today = func() time.Time { return time.Now().UTC() }
	}
	log := opts.Log
	if log == nil {
		log = slog.Default()
	}

	canonicalURL := stripQueryString(opts.BaseURL)
	if _, err := url.Parse(canonicalURL); err != nil {
		return nil, fmt.Errorf("invalid BaseURL %q: %w", opts.BaseURL, err)
	}

	today := opts.Today()
	var (
		boat        BoatScrape
		boatSet     bool
		merged      = map[string]TripScrape{}
		monthsTried int
		monthsOK    int
	)

	for offset := 0; offset < opts.Months; offset++ {
		ym := today.AddDate(0, offset, 0)
		monthYear := fmt.Sprintf("%d/%d", int(ym.Month()), ym.Year())
		pageURL, err := MonthURL(canonicalURL, monthYear)
		if err != nil {
			return nil, fmt.Errorf("build month URL: %w", err)
		}
		monthsTried++

		body, err := opts.Client.Get(ctx, pageURL)
		if err != nil {
			return nil, fmt.Errorf("fetch %s: %w", pageURL, err)
		}

		b, trips, candidates, err := ParseBoatPage(body, pageURL, monthYear)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", pageURL, err)
		}
		if candidates > 0 && len(trips) == 0 {
			return nil, fmt.Errorf("%w: %s", ErrSelectorDrift, pageURL)
		}

		if !boatSet {
			boat = b
			boatSet = true
		}
		monthsOK++

		for _, t := range trips {
			if existing, ok := merged[t.SourceTripKey]; ok {
				// If the same key appears in two months, prefer the row
				// that includes a price; otherwise keep the first seen.
				if existing.PriceText == "" && t.PriceText != "" {
					merged[t.SourceTripKey] = t
				}
				continue
			}
			merged[t.SourceTripKey] = t
		}

		log.Debug("liveaboard: month scraped",
			"url", pageURL, "candidates", candidates, "kept", len(trips), "merged_total", len(merged))
	}

	out := &Result{
		Boat:            boat,
		MonthsRequested: opts.Months,
		MonthsFetched:   monthsOK,
	}
	out.Trips = sortedTrips(merged)
	_ = monthsTried
	return out, nil
}

// sortedTrips flattens the dedup map into a slice ordered by start
// date for stable, human-readable output.
func sortedTrips(m map[string]TripScrape) []TripScrape {
	out := make([]TripScrape, 0, len(m))
	for _, t := range m {
		out = append(out, t)
	}
	// Insertion sort is fine for the small N (<= ~50 trips per boat).
	for i := 1; i < len(out); i++ {
		j := i
		for j > 0 && out[j].StartDate.Before(out[j-1].StartDate) {
			out[j], out[j-1] = out[j-1], out[j]
			j--
		}
	}
	return out
}
