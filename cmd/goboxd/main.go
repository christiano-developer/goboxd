// cmd/goboxd/main.go
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/thesouldev/goboxd/internal/executor"
	"github.com/thesouldev/goboxd/internal/handler"
	"github.com/thesouldev/goboxd/internal/languages"
	"github.com/thesouldev/goboxd/internal/worker"
)

func main() {
	// Configure global structured JSON logger
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	// Clean up stale orphan directories from previous runs/crashes
	if err := executor.SweepOrphans(5 * time.Minute); err != nil {
		slog.Warn("failed to sweep orphan directories at startup", "err", err)
	} else {
		slog.Info("startup orphan directory sweep completed")
	}

	// Load language registry
	configPath := envOrDefault("LANGUAGE_CONFIG", "configs/languages/languages.yaml")
	registry, err := languages.Load(configPath)
	if err != nil {
		slog.Error("failed to load language config", "err", err)
		os.Exit(1)
	}
	slog.Info("language registry loaded")

	// Instantiate stats and pool
	stats := &handler.ServerStats{}
	pool := worker.NewConcurrencyPool(15, 500)

	mux := http.NewServeMux()

	// GET /healthz
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"ok"}`)
	})

	// GET /readyz
	readyHandler := handler.NewReadyHandler(registry)
	mux.Handle("GET /readyz", readyHandler)

	// GET /info
	mux.Handle("GET /info", handler.NewInfoHandler(registry, stats, readyHandler, pool))

	// POST /run
	mux.Handle("POST /run", &handler.RunHandler{
		Registry: registry,
		Stats:    stats,
		Pool:     pool,
	})

	srv := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	// Graceful shutdown channel
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		slog.Info("server starting", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "err", err)
			os.Exit(1)
		}
	}()

	<-stop
	slog.Info("shutting down server gracefully...")

	// 15 seconds window to drain in-flight requests
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("graceful shutdown failed", "err", err)
	} else {
		slog.Info("server stopped cleanly")
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
