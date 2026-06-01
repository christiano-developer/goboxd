// internal/handler/info.go
package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"runtime"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/thesouldev/goboxd/internal/languages"
	"github.com/thesouldev/goboxd/internal/sandbox"
	"github.com/thesouldev/goboxd/internal/validate"
	"github.com/thesouldev/goboxd/internal/worker"
)

// ServerStats tracks service-level execution statistics.
type ServerStats struct {
	InFlightJobs        int64  // atomic
	TotalRuns           uint64 // atomic (jobs_total)
	JobsFailedInternal  uint64 // atomic
	LastInternalErrorAt int64  // atomic Unix timestamp (0 if none)
}

type InfoHandler struct {
	Registry     *languages.Registry
	Stats        *ServerStats
	ReadyHandler *ReadyHandler
	Pool         *worker.ConcurrencyPool
}

type InfoResponse struct {
	BuildInfo BuildInfoResponse    `json:"build_info"`
	Nsjail    NsjailInfoResponse   `json:"nsjail"`
	Languages []LangInfoResponse   `json:"languages"`
	Limits    ServerLimitsResponse `json:"limits"`
	Stats     StatsResponse        `json:"stats"`
}

type BuildInfoResponse struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	GoVersion string `json:"go_version"`
}

type NsjailInfoResponse struct {
	Path    string `json:"path"`
	Version string `json:"version"`
}

type LangInfoResponse struct {
	ID               string           `json:"id"`
	Name             string           `json:"name"`
	Version          string           `json:"version"`
	DefaultRunLimits languages.Limits `json:"default_run_limits"`
}

type ServerLimitsResponse struct {
	MaxSourceBytes    int `json:"max_source_bytes"`
	MaxTests          int `json:"max_tests"`
	MaxConcurrentJobs int `json:"max_concurrent_jobs"`
	MaxQueueSize      int `json:"max_queue_size"`
}

type StatsResponse struct {
	InFlightJobs         int64  `json:"in_flight_jobs"`
	QueuedJobs           int    `json:"queued_jobs"`
	JobsTotal            uint64 `json:"jobs_total"`
	JobsFailedInternal   uint64 `json:"jobs_failed_internal"`
	JobsShedTotal        uint64 `json:"jobs_shed_total"`
	LastInternalErrorAt  string `json:"last_internal_error_at,omitempty"`
	DiskFreeBytesJailDir uint64 `json:"disk_free_bytes_jail_dir"`
}

func NewInfoHandler(registry *languages.Registry, stats *ServerStats, ready *ReadyHandler, pool *worker.ConcurrencyPool) *InfoHandler {
	return &InfoHandler{
		Registry:     registry,
		Stats:        stats,
		ReadyHandler: ready,
		Pool:         pool,
	}
}

func (h *InfoHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// Fetch language version mappings cached by ReadyHandler
	langs := h.Registry.All()
	respLangs := make([]LangInfoResponse, 0, len(langs))
	for _, lang := range langs {
		version := "unknown"
		if status, ok := h.ReadyHandler.GetLanguageStatus(lang.ID); ok && status.OK {
			version = status.Version
		}
		respLangs = append(respLangs, LangInfoResponse{
			ID:               lang.ID,
			Name:             lang.Name,
			Version:          version,
			DefaultRunLimits: lang.Run.Limits,
		})
	}

	nsjailVersion := "unknown"
	if status := h.ReadyHandler.GetNsjailStatus(); status.OK {
		nsjailVersion = status.Version
	}

	var lastErrStr string
	if lastErrUnix := atomic.LoadInt64(&h.Stats.LastInternalErrorAt); lastErrUnix > 0 {
		lastErrStr = time.Unix(lastErrUnix, 0).UTC().Format(time.RFC3339)
	}

	active, queued, shed := h.Pool.GetStats()

	resp := InfoResponse{
		BuildInfo: BuildInfoResponse{
			Version:   "0.1.0",
			Commit:    "abc1234", // Default placeholder
			GoVersion: runtime.Version(),
		},
		Nsjail: NsjailInfoResponse{
			Path:    sandbox.NsjailPath,
			Version: nsjailVersion,
		},
		Languages: respLangs,
		Limits: ServerLimitsResponse{
			MaxSourceBytes:    validate.MaxSourceBytes,
			MaxTests:          validate.MaxTests,
			MaxConcurrentJobs: h.Pool.GetMaxActive(),
			MaxQueueSize:      h.Pool.GetMaxQueue(),
		},
		Stats: StatsResponse{
			InFlightJobs:         int64(active),
			QueuedJobs:           queued,
			JobsTotal:            atomic.LoadUint64(&h.Stats.TotalRuns),
			JobsFailedInternal:   atomic.LoadUint64(&h.Stats.JobsFailedInternal),
			JobsShedTotal:        shed,
			LastInternalErrorAt:  lastErrStr,
			DiskFreeBytesJailDir: diskFreeBytes(os.TempDir()),
		},
	}

	_ = json.NewEncoder(w).Encode(resp)
}

func diskFreeBytes(path string) uint64 {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0
	}
	return uint64(stat.Bavail) * uint64(stat.Bsize)
}


