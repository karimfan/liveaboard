// Command scrape_boat seeds a single boat and its upcoming trips for
// an organization by scraping the boat's listing on liveaboard.com.
//
// Usage:
//
//	go run ./scripts/scrape_boat \
//	    --url https://www.liveaboard.com/diving/indonesia/gaia-love \
//	    --org "Acme Diving" \
//	    [--months 18] [--rate-ms 1000] [--user-agent ...] [--dry-run]
//
//	make scrape-boat URL='...' ORG='Acme Diving'
//
// The CLI refuses to run when LIVEABOARD_MODE=production and requires
// the target organization to already exist (created via the SPA's
// signup flow).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/karimfan/liveaboard/internal/config"
	"github.com/karimfan/liveaboard/internal/scrape/liveaboard"
	"github.com/karimfan/liveaboard/internal/store"
)

func main() {
	var (
		urlFlag       = flag.String("url", os.Getenv("URL"), "boat detail URL on liveaboard.com")
		orgFlag       = flag.String("org", os.Getenv("ORG"), "organization name or uuid")
		monthsFlag    = flag.Int("months", 18, "number of months to scrape")
		rateMSFlag    = flag.Int("rate-ms", 0, "min interval between requests (overrides config)")
		userAgentFlag = flag.String("user-agent", "", "User-Agent override")
		dryRunFlag    = flag.Bool("dry-run", false, "scrape and print, do not write the DB")
	)
	flag.Parse()

	if err := run(*urlFlag, *orgFlag, *monthsFlag, *rateMSFlag, *userAgentFlag, *dryRunFlag); err != nil {
		fmt.Fprintf(os.Stderr, "scrape-boat: %v\n", err)
		os.Exit(1)
	}
}

func run(boatURL, orgArg string, months, rateMS int, userAgent string, dryRun bool) error {
	if strings.TrimSpace(boatURL) == "" {
		return errors.New("--url is required (or set URL=)")
	}
	if strings.TrimSpace(orgArg) == "" {
		return errors.New("--org is required (or set ORG=)")
	}

	mode, err := config.ResolveMode("dev", nil)
	if err != nil {
		return err
	}
	cfg, err := config.Load(mode, "")
	if err != nil {
		return err
	}
	if cfg.Mode == config.ModeProduction {
		return errors.New("refusing to run in production mode")
	}

	ua := userAgent
	if ua == "" {
		ua = cfg.ScraperUserAgent
	}
	interval := time.Duration(rateMS) * time.Millisecond
	if rateMS == 0 {
		interval = time.Duration(cfg.ScraperMinIntervalMS) * time.Millisecond
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	client, err := liveaboard.NewClient(liveaboard.ClientConfig{
		UserAgent:   ua,
		MinInterval: interval,
		MaxRetries:  cfg.ScraperMaxRetries,
		Timeout:     cfg.ScraperHTTPTimeout,
		Log:         log,
	})
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Best-effort robots.txt politeness check (logs only).
	client.CheckRobots(ctx, "/diving/")

	res, err := liveaboard.RunBoat(ctx, liveaboard.RunBoatOptions{
		BaseURL: boatURL,
		Months:  months,
		Client:  client,
		Log:     log,
	})
	if err != nil {
		return err
	}

	fmt.Printf("Boat: %s (%s)\n", res.Boat.Name, res.Boat.Slug)
	if res.Boat.Country != "" {
		fmt.Printf("  Country: %s\n", res.Boat.Country)
	}
	if res.Boat.ImageURL != "" {
		fmt.Printf("  Image: %s\n", res.Boat.ImageURL)
	}
	fmt.Printf("  URL: %s\n", res.Boat.URL)
	fmt.Printf("  Trips parsed: %d (months fetched: %d)\n", len(res.Trips), res.MonthsFetched)
	if dryRun {
		for _, t := range res.Trips {
			fmt.Printf("    %s -> %s | %s (%s -> %s) | %s | %s\n",
				t.StartDate.Format("2006-01-02"), t.EndDate.Format("2006-01-02"),
				t.Itinerary, t.DeparturePort, t.ReturnPort,
				t.PriceText, t.AvailabilityText)
		}
		fmt.Println()
		fmt.Println("--dry-run: no DB writes.")
		return nil
	}

	// Open Postgres and persist.
	pool, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer pool.Close()

	org, err := resolveOrg(ctx, pool, orgArg)
	if err != nil {
		return err
	}
	fmt.Printf("  Org:   %s (%s)\n", org.Name, org.ID)

	syncedAt := time.Now().UTC()
	today := syncedAt.Truncate(24 * time.Hour)

	boat, err := pool.UpsertBoat(ctx, org.ID, liveaboard.SourceProvider, store.BoatScrape{
		Slug:       res.Boat.Slug,
		Name:       res.Boat.Name,
		URL:        res.Boat.URL,
		ImageURL:   res.Boat.ImageURL,
		ExternalID: res.Boat.ExternalID,
	}, syncedAt)
	if err != nil {
		return fmt.Errorf("upsert boat: %w", err)
	}

	scrapes := make([]store.TripScrape, 0, len(res.Trips))
	for _, t := range res.Trips {
		scrapes = append(scrapes, store.TripScrape{
			StartDate:        t.StartDate,
			EndDate:          t.EndDate,
			Itinerary:        t.Itinerary,
			DeparturePort:    t.DeparturePort,
			ReturnPort:       t.ReturnPort,
			PriceText:        t.PriceText,
			AvailabilityText: t.AvailabilityText,
			SourceURL:        t.SourceURL,
			SourceTripKey:    t.SourceTripKey,
		})
	}
	rep, err := pool.ReplaceFutureScrapedTrips(ctx, org.ID, boat.ID, liveaboard.SourceProvider, scrapes, syncedAt, today)
	if err != nil {
		return fmt.Errorf("replace trips: %w", err)
	}

	fmt.Printf("  Trips: %d total (%d inserts, %d updates, %d stale-deletes)\n",
		rep.Inserts+rep.Updates, rep.Inserts, rep.Updates, rep.StaleDeletes)
	if len(res.Trips) > 0 {
		fmt.Printf("  Range: %s -> %s\n",
			res.Trips[0].StartDate.Format("2006-01-02"),
			res.Trips[len(res.Trips)-1].StartDate.Format("2006-01-02"))
	}
	return nil
}

// resolveOrg accepts a UUID string or an organization name. Names are
// matched case-insensitively (exact). Ambiguous matches return a clear
// error directing the operator to the uuid form.
func resolveOrg(ctx context.Context, pool *store.Pool, arg string) (*store.Organization, error) {
	arg = strings.TrimSpace(arg)
	if id, err := uuid.Parse(arg); err == nil {
		org, err := pool.OrganizationByID(ctx, id)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return nil, fmt.Errorf("organization %s not found", id)
			}
			return nil, err
		}
		return org, nil
	}
	org, err := pool.OrganizationByName(ctx, arg)
	switch {
	case errors.Is(err, store.ErrOrgAmbiguous):
		return nil, fmt.Errorf("multiple organizations named %q; pass --org <uuid> instead", arg)
	case errors.Is(err, store.ErrNotFound):
		return nil, fmt.Errorf("organization %q not found; create it via the SPA signup flow first", arg)
	case err != nil:
		return nil, err
	}
	return org, nil
}
