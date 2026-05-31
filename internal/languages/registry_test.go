// internal/languages/registry_test.go
package languages_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/thesouldev/goboxd/internal/languages"
)

const testYAML = `
languages:
  - id: py3
    name: Python 3
    source_filename: solution.py
    run:
      cmd: /usr/bin/python3
      args: ["{{source}}"]
      limits:
        wall_time_s: 9
        memory_kb: 102400
        max_processes: 100
`

func TestLoad(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "languages.yaml")
	if err := os.WriteFile(path, []byte(testYAML), 0644); err != nil {
		t.Fatal(err)
	}

	reg, err := languages.Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	lang, err := reg.Get("py3")
	if err != nil {
		t.Fatalf("Get(py3) error: %v", err)
	}
	if lang.Name != "Python 3" {
		t.Errorf("expected name 'Python 3', got %q", lang.Name)
	}
	if lang.Run.Limits.WallTimeS != 9 {
		t.Errorf("expected wall_time_s 9, got %d", lang.Run.Limits.WallTimeS)
	}
}

func TestGetUnknown(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "languages.yaml")
	if err := os.WriteFile(path, []byte(testYAML), 0644); err != nil {
		t.Fatal(err)
	}

	reg, _ := languages.Load(path)
	if _, err := reg.Get("cobol"); err == nil {
		t.Error("expected error for unknown language")
	}
}

func TestLoadMissingFile(t *testing.T) {
	if _, err := languages.Load("/nonexistent/path.yaml"); err == nil {
		t.Error("expected error for missing file")
	}
}
