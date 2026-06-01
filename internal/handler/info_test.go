// internal/handler/info_test.go
package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/thesouldev/goboxd/internal/languages"
	"github.com/thesouldev/goboxd/internal/worker"
)

func TestInfoHandler(t *testing.T) {
	reg, err := languages.Load("../../configs/languages/languages.yaml")
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}

	stats := &ServerStats{
		TotalRuns: 42,
	}
	pool := worker.NewConcurrencyPool(15, 500)
	ready := NewReadyHandler(reg)
	info := NewInfoHandler(reg, stats, ready, pool)

	req := httptest.NewRequest("GET", "/info", nil)
	rr := httptest.NewRecorder()

	info.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var resp InfoResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.BuildInfo.Version != "0.1.0" {
		t.Errorf("expected version 0.1.0, got %s", resp.BuildInfo.Version)
	}

	if resp.Stats.JobsTotal != 42 {
		t.Errorf("expected JobsTotal 42, got %d", resp.Stats.JobsTotal)
	}

	if len(resp.Languages) != len(reg.All()) {
		t.Errorf("expected %d languages, got %d", len(reg.All()), len(resp.Languages))
	}

	// Verify schema fields
	if resp.Nsjail.Path == "" {
		t.Error("expected non-empty nsjail path")
	}
}
