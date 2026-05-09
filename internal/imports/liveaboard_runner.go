// Package imports orchestrates trip imports from the SPA. Sprint 012
// adds two paths: an async wrapper around the liveaboard.com scraper
// (this file) and a synchronous spreadsheet commit driven by the
// spreadsheet sub-package.
//
// The Runner spawns one goroutine per kicked job. Concurrent kicks
// are serialized at the HTTP layer by liveaboard.Client's rate
// limiter (1 req/sec by default), so a worker pool is unnecessary
// at this scale. A sync.WaitGroup tracks in-flight goroutines so
// cmd/server can block on shutdown until they finish (or time out
// and mark them failed).
package imports

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/karimfan/liveaboard/internal/scrape/liveaboard"
	"github.com/karimfan/liveaboard/internal/store"
)

// scrapeRunner is the small surface our package needs from
// liveaboard.RunBoat — exposed as a function field so tests can
// inject a fake without spinning up an HTTP server.
type scrapeRunner func(ctx context.Context, opts liveaboard.RunBoatOptions) (*liveaboard.Result, error)

// Runner kicks off liveaboard.com import jobs. One *Runner is
// constructed in cmd/server/main.go and held for the process
// lifetime.
type Runner struct {
	Store  *store.Pool
	Client *liveaboard.Client
	Log    *slog.Logger

	// Now / RunBoat are overrideable for tests. Production wiring
	// uses time.Now and liveaboard.RunBoat.
	Now     func() time.Time
	RunBoat scrapeRunner

	// Months is the scrape window passed to liveaboard.RunBoat.
	// Defaults to 18 (the engine's hard cap).
	Months int

	wg sync.WaitGroup
}

// New returns a Runner with sensible defaults. Pass a *liveaboard.Client
// constructed with the operator's User-Agent / rate limit / timeout.
//
// Months defaults to 36. The 18-month cap from the original CLI was
// arbitrary and excluded operators who post their schedules a year+
// in advance. liveaboard.com's pagination naturally returns empty
// months past the boat's last published trip, so over-iterating just
// means a few extra rate-limited HTTP fetches that yield zero rows
// — no correctness risk.
func New(p *store.Pool, client *liveaboard.Client, log *slog.Logger) *Runner {
	return &Runner{
		Store:   p,
		Client:  client,
		Log:     log,
		Now:     func() time.Time { return time.Now().UTC() },
		RunBoat: liveaboard.RunBoat,
		Months:  36,
	}
}

// Kick persists a queued import_jobs row, spawns a goroutine, and
// returns immediately. The caller responds to its HTTP client with
// the job id; the SPA polls /api/admin/import/jobs/{id} until the
// status is terminal.
//
// The goroutine recovers panics into status=failed. A canceled
// context (e.g., server shutdown) terminates the in-flight
// liveaboard.RunBoat call cooperatively; the deferred update marks
// the job failed with the context error message.
func (r *Runner) Kick(ctx context.Context, orgID, userID uuid.UUID, url string) (*store.ImportJob, error) {
	job, err := r.Store.CreateImportJob(ctx, orgID, userID, store.ImportSourceLiveaboard, url)
	if err != nil {
		return nil, fmt.Errorf("create import job: %w", err)
	}

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		// Detach from the request context so the HTTP request can
		// return immediately. Use a fresh background context with a
		// generous timeout (the existing RunBoat default).
		bg, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		r.runJob(bg, job, url)
	}()
	return job, nil
}

// Wait blocks up to deadline for in-flight jobs to land. Any job
// still 'queued' or 'running' at deadline is best-effort marked
// failed with reason `msg`. cmd/server invokes this during graceful
// shutdown.
func (r *Runner) Wait(deadline time.Duration, shutdownMsg string) {
	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return
	case <-time.After(deadline):
		// Best-effort sweep. Use a short bg context so we don't
		// block shutdown on this call too.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if n, err := r.Store.MarkInFlightImportJobsFailed(ctx, shutdownMsg); err != nil {
			r.Log.Error("mark in-flight import jobs failed", "err", err)
		} else if n > 0 {
			r.Log.Warn("import jobs orphaned by shutdown", "count", n)
		}
	}
}

func (r *Runner) runJob(ctx context.Context, job *store.ImportJob, url string) {
	// Recover panics from the parser or persistence layers into a
	// clean failure rather than crashing the server.
	defer func() {
		if rec := recover(); rec != nil {
			msg := fmt.Sprintf("panic: %v", rec)
			r.Log.Error("import job panic", "job_id", job.ID, "err", msg)
			if err := r.Store.MarkImportJobFailed(context.Background(), job.ID, msg); err != nil {
				r.Log.Error("mark failed after panic", "err", err)
			}
		}
	}()

	if err := r.Store.MarkImportJobRunning(ctx, job.ID); err != nil {
		r.Log.Error("mark running", "err", err, "job_id", job.ID)
		return
	}

	res, err := r.RunBoat(ctx, liveaboard.RunBoatOptions{
		BaseURL: url,
		Months:  r.Months,
		Client:  r.Client,
		Log:     r.Log,
	})
	if err != nil {
		r.fail(job.ID, err)
		return
	}

	syncedAt := r.Now()
	today := syncedAt.Truncate(24 * time.Hour)

	boat, err := r.Store.UpsertBoat(ctx, job.OrganizationID, liveaboard.SourceProvider, store.BoatScrape{
		Slug:       res.Boat.Slug,
		Name:       res.Boat.Name,
		URL:        res.Boat.URL,
		ImageURL:   res.Boat.ImageURL,
		ExternalID: res.Boat.ExternalID,
	}, syncedAt)
	if err != nil {
		r.fail(job.ID, fmt.Errorf("upsert boat: %w", err))
		return
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
	rep, err := r.Store.ReplaceFutureScrapedTrips(ctx, job.OrganizationID, boat.ID, liveaboard.SourceProvider, scrapes, syncedAt, today)
	if err != nil {
		r.fail(job.ID, fmt.Errorf("replace trips: %w", err))
		return
	}

	if err := r.Store.MarkImportJobSucceeded(ctx, job.ID, store.ImportResult{
		BoatsInserted: 0, // UpsertBoat doesn't surface insert/update; we treat boats as a single touched row.
		BoatsUpdated:  1,
		TripsInserted: rep.Inserts,
		TripsUpdated:  rep.Updates,
		TripsDeleted:  rep.RemovedFromSource,
	}); err != nil {
		r.Log.Error("mark succeeded", "err", err, "job_id", job.ID)
	}
}

func (r *Runner) fail(jobID uuid.UUID, cause error) {
	msg := cause.Error()
	r.Log.Error("import job failed", "job_id", jobID, "err", msg)
	// Use a fresh context so a canceled scrape context doesn't
	// also kill the bookkeeping write.
	if err := r.Store.MarkImportJobFailed(context.Background(), jobID, msg); err != nil {
		r.Log.Error("mark failed", "err", err)
	}
}
