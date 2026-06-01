// internal/handler/run.go
// Christiano Fernandes
// 31 May 26
package handler

import (
	"encoding/json"
	"net/http"
	"sync/atomic"

	"github.com/thesouldev/goboxd/internal/executor"
	"github.com/thesouldev/goboxd/internal/languages"
	"github.com/thesouldev/goboxd/internal/model"
	"github.com/thesouldev/goboxd/internal/validate"
)

// RunHandler handles POST /run.
type RunHandler struct {
	Registry *languages.Registry
	Stats    *ServerStats
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

	// Execute
	resp, err := executor.Run(&req, lang)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	if h.Stats != nil {
		atomic.AddUint64(&h.Stats.TotalRuns, 1)
	}

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
