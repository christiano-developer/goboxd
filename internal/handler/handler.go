// internal/handler/handler.go
package handler

import (
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/thesouldev/goboxd/internal/executor"
	"github.com/thesouldev/goboxd/internal/languages"
	"github.com/thesouldev/goboxd/internal/model"
	"github.com/thesouldev/goboxd/internal/validate"
	"github.com/thesouldev/goboxd/internal/worker"
)

// RunHandler handles POST /run.
type RunHandler struct {
	Registry *languages.Registry
	Stats    *ServerStats
	Pool     *worker.ConcurrencyPool
}

func (h *RunHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Cap request body to 256 KiB
	r.Body = http.MaxBytesReader(w, r.Body, validate.MaxSourceBytes)

	var req model.RunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	// Validate language
	if req.Language == "" {
		writeError(w, http.StatusBadRequest, "missing_language", "language is required")
		return
	}

	lang, err := h.Registry.Get(req.Language)
	if err != nil {
		writeError(w, http.StatusBadRequest, "unknown_language",
			"language "+req.Language+" is not supported")
		return
	}

	// Validate source
	if err := validate.SourceSize(req.Source); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_source", err.Error())
		return
	}

	// Validate test count
	if err := validate.TestCount(len(req.Tests)); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_tests", err.Error())
		return
	}

	// Validate optional filenames
	if req.SourceFilename != "" {
		if err := validate.Filename(req.SourceFilename); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_filename", err.Error())
			return
		}
	}
	if req.ArtifactFilename != "" {
		if err := validate.Filename(req.ArtifactFilename); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_filename", err.Error())
			return
		}
	}

	// Validate flags against per-language allowlist
	if req.Build != nil && lang.Build != nil {
		if err := validate.Flags(req.Build.Flags, lang.Build.FlagAllowlist); err != nil {
			writeError(w, http.StatusBadRequest, "disallowed_flag", err.Error())
			return
		}
	}

	// Apply load-adaptive limit checks & clamp overrides
	var warnings []string
	rate := h.Pool.RateTracker.GetRate()
	if req.Build != nil && req.Build.Limits != nil {
		warns := validate.ResourceLimits(req.Build.Limits, lang.ID, rate)
		warnings = append(warnings, warns...)
	}
	if req.Run != nil && req.Run.Limits != nil {
		warns := validate.ResourceLimits(req.Run.Limits, lang.ID, rate)
		warnings = append(warnings, warns...)
	}

	// Estimate cost and memory to acquire slot in the priority queue
	buildTime := 0
	if lang.Build != nil {
		buildTime = lang.Build.Limits.WallTimeS
		if req.Build != nil && req.Build.Limits != nil && req.Build.Limits.WallTimeS > 0 {
			buildTime = req.Build.Limits.WallTimeS
		}
	}
	runTime := lang.Run.Limits.WallTimeS
	if req.Run != nil && req.Run.Limits != nil && req.Run.Limits.WallTimeS > 0 {
		runTime = req.Run.Limits.WallTimeS
	}
	cost := int64(buildTime + len(req.Tests)*runTime)

	memKB := lang.Run.Limits.MemoryKB
	if req.Run != nil && req.Run.Limits != nil && req.Run.Limits.MemoryKB > 0 {
		memKB = req.Run.Limits.MemoryKB
	}

	// Acquire concurrency slot (block in priority queue or reject if queue is saturated)
	err = h.Pool.Acquire(r.Context(), cost, memKB)
	if err != nil {
		if err == worker.ErrQueueFull {
			_, queued, _ := h.Pool.GetStats()
			avgDur := h.Pool.ExecutionTracker.GetAverage()
			maxActive := h.Pool.GetMaxActive()

			// Estimated Wait Time = (QueueSize * AvgDuration) / MaxConcurrency
			estWaitSecs := int(math.Ceil(float64(queued) * avgDur.Seconds() / float64(maxActive)))
			if estWaitSecs < 1 {
				estWaitSecs = 1
			}

			w.Header().Set("Retry-After", strconv.Itoa(estWaitSecs))
			writeError(w, http.StatusServiceUnavailable, "service_unavailable", "server overload: queue is full")
			return
		}
		// Request context cancelled / timeout
		writeError(w, http.StatusRequestTimeout, "request_timeout", err.Error())
		return
	}
	defer h.Pool.Release()

	// Track active execution state
	atomic.AddInt64(&h.Stats.InFlightJobs, 1)
	startExec := time.Now()

	resp, err := executor.Run(&req, lang)

	execDur := time.Since(startExec)
	atomic.AddInt64(&h.Stats.InFlightJobs, -1)

	if err != nil {
		atomic.AddUint64(&h.Stats.JobsFailedInternal, 1)
		atomic.StoreInt64(&h.Stats.LastInternalErrorAt, time.Now().Unix())
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	// Success tracking
	atomic.AddUint64(&h.Stats.TotalRuns, 1)
	h.Pool.ExecutionTracker.AddDuration(execDur)

	resp.Warnings = warnings

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(model.ErrorResponse{
		Error: model.ErrorDetail{Code: code, Message: message},
	})
}

