// internal/handler/info.go
package handler

import (
	"encoding/json"
	"net/http"
	"os/exec"
	"strings"
	"sync/atomic"

	"github.com/thesouldev/goboxd/internal/languages"
	"github.com/thesouldev/goboxd/internal/sandbox"
	"github.com/thesouldev/goboxd/internal/validate"
)

type ServerStats struct {
	TotalRuns uint64 `json:"total_runs"`
}

type InfoHandler struct {
	Registry      *languages.Registry
	Stats         *ServerStats
	nsjailVersion string
}

type InfoResponse struct {
	NsjailVersion string               `json:"nsjail_version"`
	Limits        ServerLimits         `json:"limits"`
	Languages     []languages.Language `json:"languages"`
	Stats         StatsResponse        `json:"stats"`
}

type ServerLimits struct {
	MaxSourceBytes int `json:"max_source_bytes"`
	MaxTests       int `json:"max_tests"`
	MaxFilenameLen int `json:"max_filename_len"`
}

type StatsResponse struct {
	TotalRuns uint64 `json:"total_runs"`
}

func NewInfoHandler(registry *languages.Registry, stats *ServerStats) *InfoHandler {
	h := &InfoHandler{
		Registry: registry,
		Stats:    stats,
	}
	h.nsjailVersion = getNsjailVersion()
	return h
}

func (h *InfoHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	resp := InfoResponse{
		NsjailVersion: h.nsjailVersion,
		Limits: ServerLimits{
			MaxSourceBytes: validate.MaxSourceBytes,
			MaxTests:       validate.MaxTests,
			MaxFilenameLen: validate.MaxFilenameLen,
		},
		Languages: h.Registry.All(),
		Stats: StatsResponse{
			TotalRuns: atomic.LoadUint64(&h.Stats.TotalRuns),
		},
	}

	_ = json.NewEncoder(w).Encode(resp)
}

func getNsjailVersion() string {
	path, err := exec.LookPath(sandbox.NsjailPath)
	if err != nil {
		return "unknown"
	}
	out, err := exec.Command(path, "--version").CombinedOutput()
	if err != nil {
		return "unknown"
	}
	lines := strings.Split(string(out), "\n")
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed != "" {
			return trimmed
		}
	}
	return "unknown"
}
