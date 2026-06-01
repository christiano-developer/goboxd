// cmd/goboxd/main.go
// Christiano Fernandes
// 31 May 26
// http service root, healthz route
package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/thesouldev/goboxd/internal/handler"
	"github.com/thesouldev/goboxd/internal/languages"
)

func main() {
	// Load language registry
	configPath := envOrDefault("LANGUAGE_CONFIG", "configs/languages/languages.yaml")
	registry, err := languages.Load(configPath)
	if err != nil {
		slog.Error("failed to load language config", "err", err)
		os.Exit(1)
	}
	slog.Info("language registry loaded")
	mux := http.NewServeMux()
	// Initialize stats tracking
	stats := &handler.ServerStats{}

	// GET /healthz
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"ok"}`)
	})
	// GET /readyz
	mux.Handle("GET /readyz", handler.NewReadyHandler(registry))
	// GET /info
	mux.Handle("GET /info", handler.NewInfoHandler(registry, stats))
	// POST /run
	mux.Handle("POST /run", &handler.RunHandler{Registry: registry, Stats: stats})
	addr := ":8080"
	slog.Info("server starting", "addr", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		slog.Error("server failed", "err", err)
		os.Exit(1)
	}
}
func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
