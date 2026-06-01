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
	stats := &handler.ServerStats{}
	mux.Handle("GET /readyz", handler.NewReadyHandler(reg))
	mux.Handle("GET /info", handler.NewInfoHandler(reg, stats))
	mux.Handle("POST /run", &handler.RunHandler{Registry: reg, Stats: stats})
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

func TestReadyzEndpoint(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/readyz")
	if err != nil {
		t.Fatalf("GET /readyz failed: %v", err)
	}
	defer resp.Body.Close()

	// Inside tests (host has no nsjail unless inside Docker), readyz might be degraded or OK.
	// But it must return a valid JSON response with status.
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected 200 or 503, got %d", resp.StatusCode)
	}

	var data map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		t.Fatalf("decode readyz: %v", err)
	}
	if _, ok := data["status"]; !ok {
		t.Error("missing status field in readyz response")
	}
}

func TestInfoEndpoint(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/info")
	if err != nil {
		t.Fatalf("GET /info failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var data map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		t.Fatalf("decode info: %v", err)
	}
	if _, ok := data["nsjail_version"]; !ok {
		t.Error("missing nsjail_version in info response")
	}
	if _, ok := data["languages"]; !ok {
		t.Error("missing languages list in info response")
	}
}

func TestCppAccepted(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	result := postRun(t, srv, model.RunRequest{
		Language: "cpp",
		Source: `#include <iostream>
int main() {
    std::cout << "hello C++" << std::endl;
    return 0;
}`,
		Tests: []model.TestCase{
			{Stdin: "", ExpectedStdout: "hello C++\n"},
		},
	})

	if result.Status != "accepted" {
		t.Errorf("expected accepted, got %q (build error: %v)", result.Status, result.Build)
	}
}

func TestJavaAccepted(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	result := postRun(t, srv, model.RunRequest{
		Language:         "java",
		Source: `public class Main {
    public static void main(String[] args) {
        System.out.println("hello Java");
    }
}`,
		SourceFilename:   "Main.java",
		ArtifactFilename: "Main",
		Tests: []model.TestCase{
			{Stdin: "", ExpectedStdout: "hello Java\n"},
		},
	})

	if result.Status != "accepted" {
		t.Errorf("expected accepted, got %q (build error: %v)", result.Status, result.Build)
	}
}

func TestBashAccepted(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	result := postRun(t, srv, model.RunRequest{
		Language: "bash",
		Source:   "echo 'hello Bash'",
		Tests: []model.TestCase{
			{Stdin: "", ExpectedStdout: "hello Bash\n"},
		},
	})

	if result.Status != "accepted" {
		t.Errorf("expected accepted, got %q", result.Status)
	}
}

func TestJsAccepted(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	result := postRun(t, srv, model.RunRequest{
		Language: "js",
		Source:   "console.log('hello Node');",
		Tests: []model.TestCase{
			{Stdin: "", ExpectedStdout: "hello Node\n"},
		},
	})

	if result.Status != "accepted" {
		t.Errorf("expected accepted, got %q", result.Status)
	}
}

func TestVerilogAccepted(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	result := postRun(t, srv, model.RunRequest{
		Language: "verilog",
		Source: `module Main;
    initial begin
        $display("hello Verilog");
        $finish;
    end
endmodule`,
		Tests: []model.TestCase{
			{Stdin: "", ExpectedStdout: "hello Verilog\n"},
		},
	})

	if result.Status != "accepted" {
		t.Errorf("expected accepted, got %q (build error: %v)", result.Status, result.Build)
	}
}
