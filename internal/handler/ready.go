// internal/handler/ready.go
package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/thesouldev/goboxd/internal/languages"
	"github.com/thesouldev/goboxd/internal/sandbox"
)

type ReadyHandler struct {
	Registry *languages.Registry
	response ReadyResponse
	healthy  bool
}

// GetNsjailStatus returns the cached status of nsjail.
func (h *ReadyHandler) GetNsjailStatus() NsjailStatus {
	return h.response.Nsjail
}

// GetLanguageStatus returns the cached status of a language by its ID.
func (h *ReadyHandler) GetLanguageStatus(id string) (LangStatus, bool) {
	status, ok := h.response.Languages[id]
	return status, ok
}

type ReadyResponse struct {
	Status    string                `json:"status"`
	Nsjail    NsjailStatus          `json:"nsjail"`
	Languages map[string]LangStatus `json:"languages"`
}

type NsjailStatus struct {
	OK      bool   `json:"ok"`
	Version string `json:"version,omitempty"`
	Error   string `json:"error,omitempty"`
}

type LangStatus struct {
	OK      bool   `json:"ok"`
	Version string `json:"version,omitempty"`
	Error   string `json:"error,omitempty"`
}

func NewReadyHandler(registry *languages.Registry) *ReadyHandler {
	h := &ReadyHandler{
		Registry: registry,
		healthy:  true,
	}
	h.response.Languages = make(map[string]LangStatus)
	h.runChecks()
	return h
}

func (h *ReadyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if h.healthy {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	_ = json.NewEncoder(w).Encode(h.response)
}

func (h *ReadyHandler) runChecks() {
	// 1. Check nsjail
	_, err := exec.LookPath(sandbox.NsjailPath)
	if err != nil {
		h.healthy = false
		h.response.Nsjail = NsjailStatus{OK: false, Error: err.Error()}
	} else {
		h.response.Nsjail = NsjailStatus{OK: true, Version: "3.4"}
	}

	// 2. Check each language in registry
	langs := h.Registry.All()
	for _, lang := range langs {
		var checkCmd string
		var checkArgs []string

		if lang.SmokeCheckCmd != "" {
			checkCmd = lang.SmokeCheckCmd
		} else if lang.Build != nil {
			checkCmd = lang.Build.Cmd
		} else {
			checkCmd = lang.Run.Cmd
		}

		if len(lang.SmokeCheckArgs) > 0 {
			checkArgs = lang.SmokeCheckArgs
		} else {
			// Tailor version arguments for typical runtimes
			switch lang.ID {
			case "py3", "c", "cpp", "bash":
				checkArgs = []string{"--version"}
			case "java":
				checkArgs = []string{"-version"}
			case "js":
				checkArgs = []string{"--version"}
			case "verilog":
				checkArgs = []string{"-V"}
			default:
				checkArgs = []string{"--version"}
			}
		}

		ver, err := getCmdVersion(checkCmd, checkArgs)
		if err != nil {
			h.healthy = false
			h.response.Languages[lang.ID] = LangStatus{OK: false, Error: err.Error()}
		} else {
			h.response.Languages[lang.ID] = LangStatus{OK: true, Version: cleanVersionString(ver)}
		}
	}

	if h.healthy {
		h.response.Status = "ok"
	} else {
		h.response.Status = "degraded"
	}
}

func getCmdVersion(cmdName string, args []string) (string, error) {
	// First check if path is resolvable
	path, err := exec.LookPath(cmdName)
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, path, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Some utilities exit with error on version check (e.g. java -version exits with status, or iverilog outputs to stderr)
		// If we got some output in stdout or stderr, we can still parse it
		combined := stdout.String() + stderr.String()
		if len(strings.TrimSpace(combined)) > 0 {
			return combined, nil
		}
		return "", err
	}

	combined := stdout.String() + stderr.String()
	return combined, nil
}

func cleanVersionString(s string) string {
	lines := strings.Split(s, "\n")
	if len(lines) == 0 {
		return "unknown"
	}
	// Take the first non-empty line
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			return trimmed
		}
	}
	return "unknown"
}
