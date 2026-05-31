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
	"github.com/karimfan/liveaboard/internal/fxauto"
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

	if cfg.EmailTransport == "smtp" && (cfg.SMTPHost == "" || cfg.SMTPUsername == "" || cfg.SMTPPassword == "" || cfg.SMTPFrom == "") {
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

	var sender email.Sender
	senderFrom := cfg.SMTPFrom
	switch cfg.EmailTransport {
	case "filesystem":
		log.Warn("email transport: filesystem (no SMTP delivery)", "inbox_dir", cfg.EmailFilesystemDir)
		if senderFrom == "" {
			senderFrom = "Liveaboard <noreply@filesystem.local>"
		}
		sender = email.NewFilesystemSender(cfg.EmailFilesystemDir, log)
	default:
		sender = &email.SMTPSender{
			Host:     cfg.SMTPHost,
			Port:     cfg.SMTPPort,
			Username: cfg.SMTPUsername,
			Password: cfg.SMTPPassword,
		}
	}

	authSvc := auth.New(pool, sender, log, cfg.AppBaseURL, senderFrom)
	authSvc.BcryptCost = cfg.BcryptCost
	authSvc.SessionDuration = cfg.SessionDuration
	authSvc.VerificationDuration = cfg.VerificationDuration

	session := &auth.SessionMiddleware{
		Store: pool,
		Log:   log,
	}
	guestSession := &auth.GuestSessionMiddleware{
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

	// Sprint 024 — automated FX refresher (Frankfurter / ECB). We
	// construct + start it in every mode except `test`; tests run
	// against testdb and must not make real HTTP egress. The PATCH
	// handler also calls RefreshOnce(only=[newCurrency]) when an
	// org adds a quote, so the page never has to wait a full tick.
	var fxRefresher *fxauto.Refresher
	if cfg.Mode != config.ModeTest {
		fxRefresher = &fxauto.Refresher{
			Fetcher: &fxauto.Client{},
			Store:   pool,
			Log:     log,
		}
		go fxRefresher.Run(ctx)
		log.Info("fxauto refresher started", "interval", fxauto.DefaultInterval, "provider", fxauto.Provider)
	}

	srv := &httpapi.Server{
		Org:          org.New(pool),
		Log:          log,
		Auth:         authSvc,
		Session:      session,
		GuestSession: guestSession,
		AdminAPI:     &httpapi.AdminHandlers{Store: pool},
		ImportRunner: importRunner,
		DocumentsDir: cfg.DocumentsDir,
		CookieSecure: cfg.CookieSecure,
		IsDev:        cfg.Mode == config.ModeDev,
	}
	if fxRefresher != nil {
		srv.FXRefresher = fxRefresher
	}
	if cfg.Mode == config.ModeDev && cfg.EmailTransport == "filesystem" {
		srv.DevInboxDir = cfg.EmailFilesystemDir
		log.Info("dev inbox viewer mounted", "path", "/dev/inbox", "inbox_dir", cfg.EmailFilesystemDir)
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
