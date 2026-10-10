// Command survey serves the OSS-AGENT01 onboarding survey and stores answers
// in Supabase. It uses only the Go standard library.
//
// The Supabase secret key never leaves this server: the browser talks only to
// this service, and the database table has row-level security switched on with
// no public policies, so the key is the only way in.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	web "oss-agent01-survey"
	"oss-agent01-survey/internal/config"
	"oss-agent01-survey/internal/httpapi"
	"oss-agent01-survey/internal/ratelimit"
	"oss-agent01-survey/internal/store"
)

// shutdownTimeout is how long in-flight requests get to finish on shutdown.
const shutdownTimeout = 10 * time.Second

func main() {
	cfg, err := config.Load(os.Getenv)
	log := newLogger(os.Stdout, cfg.LogFormat)
	if err != nil {
		log.Error("config", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	err = run(ctx, log, cfg)
	stop()
	if err != nil {
		log.Error("server", "err", err)
		os.Exit(1)
	}
}

// run serves until ctx is cancelled, then shuts down gracefully.
func run(ctx context.Context, log *slog.Logger, cfg config.Config) error {
	site, err := web.Site()
	if err != nil {
		return err
	}

	limiter := ratelimit.New(cfg.RateLimit, cfg.RateWindow, nil)
	limiterCtx, stopLimiter := context.WithCancel(context.Background())
	defer stopLimiter()
	go limiter.Run(limiterCtx)

	saver := store.NewSupabase(cfg.SupabaseURL, cfg.SupabaseKey, cfg.Table, &http.Client{Timeout: 10 * time.Second})
	handler, err := httpapi.New(log, saver, limiter, site, httpapi.Options{
		AllowedOrigin: cfg.AllowedOrigin,
		TrustProxy:    cfg.TrustProxy,
	})
	if err != nil {
		return err
	}

	srv := &http.Server{
		Addr:              net.JoinHostPort("", cfg.Port),
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    64 << 10,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelWarn),
	}

	serveErr := make(chan error, 1)
	go func() {
		log.Info("survey server listening", "addr", srv.Addr)
		serveErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		return fmt.Errorf("listen: %w", err)
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	if err := <-serveErr; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("listen: %w", err)
	}
	log.Info("server stopped")
	return nil
}

// newLogger returns a JSON logger for format "json" and a text logger otherwise.
func newLogger(w io.Writer, format string) *slog.Logger {
	if format == "json" {
		return slog.New(slog.NewJSONHandler(w, nil))
	}
	return slog.New(slog.NewTextHandler(w, nil))
}
