// cmd/goboxd/main.go
// Christiano Fernandes
// 31 May 26
// http service root, healthz route
package main

import (
	"encoding/json"
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
	// GET /healthz
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"ok"}`)
	})
	// POST /run
	mux.Handle("POST /run", &handler.RunHandler{Registry: registry})
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

// keep json import used via indirect reference in healthz
var _ = json.Marshal
