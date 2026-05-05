package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/karimfan/liveaboard/internal/auth"
	"github.com/karimfan/liveaboard/internal/config"
	"github.com/karimfan/liveaboard/internal/email"
	"github.com/karimfan/liveaboard/internal/httpapi"
	"github.com/karimfan/liveaboard/internal/imports"
	"github.com/karimfan/liveaboard/internal/org"
	"github.com/karimfan/liveaboard/internal/scrape/liveaboard"
	"github.com/karimfan/liveaboard/internal/store"
)

func main() {
	modeFlag := flag.String("mode", "", "runtime mode: dev, test, or production")
	addrFlag := flag.String("addr", "", "listen address (overrides config)")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(log)

	mode, err := config.ResolveMode(*modeFlag, nil)
	if err != nil {
		log.Error("resolve mode", "err", err)
		os.Exit(2)
	}

	cfg := config.MustLoad(mode, "")
	if *addrFlag != "" {
		cfg.Addr = *addrFlag
	}
	log.Info("config loaded", "mode", cfg.Mode, "addr", cfg.Addr, "cookie_secure", cfg.CookieSecure)

	if cfg.SMTPHost == "" || cfg.SMTPUsername == "" || cfg.SMTPPassword == "" || cfg.SMTPFrom == "" {
		log.Error("SMTP not configured", "host_set", cfg.SMTPHost != "", "user_set", cfg.SMTPUsername != "", "from_set", cfg.SMTPFrom != "")
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := store.Migrate(ctx, cfg.DatabaseURL); err != nil {
		log.Error("migrate", "err", err)
		os.Exit(1)
	}
	pool, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("open store", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	sender := &email.SMTPSender{
		Host:     cfg.SMTPHost,
		Port:     cfg.SMTPPort,
		Username: cfg.SMTPUsername,
		Password: cfg.SMTPPassword,
	}

	authSvc := auth.New(pool, sender, log, cfg.AppBaseURL, cfg.SMTPFrom)
	authSvc.BcryptCost = cfg.BcryptCost
	authSvc.SessionDuration = cfg.SessionDuration
	authSvc.VerificationDuration = cfg.VerificationDuration

	session := &auth.SessionMiddleware{
		Store: pool,
		Log:   log,
	}

	// Sprint 012 — liveaboard.com import runner. The same Client
	// constructor used by the scrape CLI; rate-limited and
	// politeness-aware. Concurrent kicks serialize at the HTTP
	// layer via the client's rate limiter, so we don't need a
	// worker pool.
	scrapeClient, err := liveaboard.NewClient(liveaboard.ClientConfig{
		UserAgent:   cfg.ScraperUserAgent,
		MinInterval: time.Duration(cfg.ScraperMinIntervalMS) * time.Millisecond,
		MaxRetries:  cfg.ScraperMaxRetries,
		Timeout:     cfg.ScraperHTTPTimeout,
		Log:         log,
	})
	if err != nil {
		log.Error("init scrape client", "err", err)
		os.Exit(1)
	}
	importRunner := imports.New(pool, scrapeClient, log)

	// Best-effort cleanup of orphaned import-jobs and expired
	// previews from a prior shutdown. Both are small queries, run
	// once at startup.
	if n, err := pool.MarkInFlightImportJobsFailed(ctx, "server restart"); err == nil && n > 0 {
		log.Warn("orphaned import jobs cleared", "count", n)
	}
	if n, err := pool.DeleteExpiredImportPreviews(ctx, time.Now().UTC()); err == nil && n > 0 {
		log.Info("expired import previews cleared", "count", n)
	}

	srv := &httpapi.Server{
		Org:          org.New(pool),
		Log:          log,
		Auth:         authSvc,
		Session:      session,
		AdminAPI:     &httpapi.AdminHandlers{Store: pool},
		ImportRunner: importRunner,
		CookieSecure: cfg.CookieSecure,
	}

	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           srv.Router(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Info("listening", "addr", cfg.Addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("listen", "err", err)
			cancel()
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	_ = httpServer.Shutdown(shutdownCtx)

	// Wait up to 30s for in-flight import jobs to land. Anything
	// still running at the deadline is marked failed.
	importRunner.Wait(30*time.Second, "server shutdown")
}
