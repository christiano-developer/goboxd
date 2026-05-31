// tests/integration/run_test.go
//go:build integration

package integration_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/thesouldev/goboxd/internal/handler"
	"github.com/thesouldev/goboxd/internal/languages"
	"github.com/thesouldev/goboxd/internal/model"
)

func setupServer(t *testing.T) *httptest.Server {
	t.Helper()
	configPath := os.Getenv("LANGUAGE_CONFIG")
	if configPath == "" {
		if _, err := os.Stat("../../configs/languages/languages.yaml"); err == nil {
			configPath = "../../configs/languages/languages.yaml"
		} else {
			configPath = "configs/languages/languages.yaml"
		}
	}
	reg, err := languages.Load(configPath)
	if err != nil {
		t.Fatalf("load registry: %v", err)
	}
	mux := http.NewServeMux()
	mux.Handle("POST /run", &handler.RunHandler{Registry: reg})
	return httptest.NewServer(mux)
}

func postRun(t *testing.T, srv *httptest.Server, req model.RunRequest) model.RunResponse {
	t.Helper()
	body, _ := json.Marshal(req)
	resp, err := http.Post(srv.URL+"/run", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /run: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result model.RunResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return result
}

func TestPythonAccepted(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	result := postRun(t, srv, model.RunRequest{
		Language: "py3",
		Source:   "print(input())",
		Tests: []model.TestCase{
			{Stdin: "hello\n", ExpectedStdout: "hello\n"},
		},
	})

	if result.Status != "accepted" {
		var stderr string
		if len(result.Tests) > 0 {
			stderr = result.Tests[0].Stderr
		}
		t.Errorf("expected accepted, got %q (stderr: %q)", result.Status, stderr)
	}
	if len(result.Tests) > 0 && result.Tests[0].Status != "accepted" {
		t.Errorf("test[0] expected accepted, got %q (stderr: %q)", result.Tests[0].Status, result.Tests[0].Stderr)
	}
}

func TestPythonWrongOutput(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	result := postRun(t, srv, model.RunRequest{
		Language: "py3",
		Source:   "print('wrong')",
		Tests: []model.TestCase{
			{Stdin: "", ExpectedStdout: "right\n"},
		},
	})

	if result.Status != "wrong_output" {
		t.Errorf("expected wrong_output, got %q", result.Status)
	}
}

func TestCAccepted(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	result := postRun(t, srv, model.RunRequest{
		Language: "c",
		Source: `#include <stdio.h>
int main() {
    printf("hello\n");
    return 0;
}`,
		Tests: []model.TestCase{
			{Stdin: "", ExpectedStdout: "hello\n"},
		},
	})

	if result.Build == nil {
		t.Fatal("expected build result for C")
	}
	if result.Build.Status != "ok" {
		t.Errorf("build expected ok, got %q (stderr: %s)", result.Build.Status, result.Build.Stderr)
	}
	if result.Status != "accepted" {
		var stderr string
		if len(result.Tests) > 0 {
			stderr = result.Tests[0].Stderr
		}
		t.Errorf("expected accepted, got %q (stderr: %q)", result.Status, stderr)
	}
}

func TestCBuildFailed(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	result := postRun(t, srv, model.RunRequest{
		Language: "c",
		Source:   `this is not valid C`,
		Tests: []model.TestCase{
			{Stdin: "", ExpectedStdout: ""},
		},
	})

	if result.Status != "build_failed" {
		t.Errorf("expected build_failed, got %q", result.Status)
	}
	if result.Tests[0].Status != "not_executed" {
		t.Errorf("test[0] expected not_executed, got %q", result.Tests[0].Status)
	}
}

func TestUnknownLanguage(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	body, _ := json.Marshal(model.RunRequest{
		Language: "cobol",
		Source:   "x",
		Tests:    []model.TestCase{{Stdin: "", ExpectedStdout: ""}},
	})
	resp, _ := http.Post(srv.URL+"/run", "application/json", bytes.NewReader(body))
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}
