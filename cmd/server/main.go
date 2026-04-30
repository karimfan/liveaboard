package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/karimfan/liveaboard/internal/auth"
	"github.com/karimfan/liveaboard/internal/httpapi"
	"github.com/karimfan/liveaboard/internal/org"
	"github.com/karimfan/liveaboard/internal/store"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(log)

	dsn := os.Getenv("LIVEABOARD_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://localhost:5432/liveaboard?sslmode=disable"
	}
	addr := os.Getenv("LIVEABOARD_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	secure := strings.EqualFold(os.Getenv("LIVEABOARD_COOKIE_SECURE"), "true")

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := store.Migrate(ctx, dsn); err != nil {
		log.Error("migrate", "err", err)
		os.Exit(1)
	}
	pool, err := store.Open(ctx, dsn)
	if err != nil {
		log.Error("open store", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	srv := &httpapi.Server{
		Auth:   auth.New(pool, log),
		Org:    org.New(pool),
		Log:    log,
		Secure: secure,
	}

	httpServer := &http.Server{
		Addr:              addr,
		Handler:           srv.Router(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Info("listening", "addr", addr)
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
