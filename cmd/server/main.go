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
	"github.com/karimfan/liveaboard/internal/httpapi"
	"github.com/karimfan/liveaboard/internal/org"
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

	provider, err := auth.NewClerkProvider(cfg.ClerkSecretKey, "")
	if err != nil {
		log.Error("init clerk provider", "err", err)
		os.Exit(1)
	}

	exchange := &auth.Exchanger{
		Provider:     provider,
		Store:        pool,
		Log:          log,
		SessionTTL:   cfg.SessionDuration,
		CookieSecure: cfg.CookieSecure,
	}
	session := &auth.SessionMiddleware{
		Store: pool,
		Log:   log,
	}
	admin := &auth.AdminHandlers{
		Provider: provider,
		Store:    pool,
		Log:      log,
	}
	webhook, err := auth.NewWebhookReceiver(provider, pool, log, cfg.ClerkWebhookSecret)
	if err != nil {
		log.Error("init webhook receiver", "err", err)
		os.Exit(1)
	}

	srv := &httpapi.Server{
		Org:      org.New(pool),
		Log:      log,
		Exchange: exchange,
		Session:  session,
		Admin:    admin,
		Webhook:  webhook,
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
}
