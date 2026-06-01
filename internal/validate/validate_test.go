// internal/validate/validate_test.go
// Christiano Fernandes
// 31 May 26
package validate_test

import (
	"strings"
	"testing"

	"github.com/thesouldev/goboxd/internal/validate"
)

func TestFilename(t *testing.T) {
	valid := []string{"solution.py", "main.go", "Solution.java"}
	for _, name := range valid {
		if err := validate.Filename(name); err != nil {
			t.Errorf("Filename(%q) unexpected error: %v", name, err)
		}
	}

	invalid := []string{
		"",
		"../etc/passwd",
		"/etc/passwd",
		".hidden",
		"a/b.py",
		strings.Repeat("a", 200),
	}
	for _, name := range invalid {
		if err := validate.Filename(name); err == nil {
			t.Errorf("Filename(%q) expected error, got nil", name)
		}
	}
}

func TestSourceSize(t *testing.T) {
	if err := validate.SourceSize("print('hello')"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := validate.SourceSize(""); err == nil {
		t.Error("expected error for empty source")
	}
	big := strings.Repeat("a", validate.MaxSourceBytes+1)
	if err := validate.SourceSize(big); err == nil {
		t.Error("expected error for oversized source")
	}
}

func TestTestCount(t *testing.T) {
	if err := validate.TestCount(1); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := validate.TestCount(0); err == nil {
		t.Error("expected error for 0 tests")
	}
	if err := validate.TestCount(validate.MaxTests + 1); err == nil {
		t.Error("expected error for too many tests")
	}
}

func TestFlags(t *testing.T) {
	allowlist := []string{"-O0", "-O1", "-O2", "-std=*", "-Wall"}

	// Allowed exact match
	if err := validate.Flags([]string{"-O2"}, allowlist); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	// Allowed glob
	if err := validate.Flags([]string{"-std=c11"}, allowlist); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	// Disallowed
	if err := validate.Flags([]string{"-fplugin=evil.so"}, allowlist); err == nil {
		t.Error("expected error for disallowed flag")
	}
	// Empty flags — always ok
	if err := validate.Flags(nil, allowlist); err != nil {
		t.Errorf("unexpected error for nil flags: %v", err)
	}
	// No allowlist + flags supplied
	if err := validate.Flags([]string{"-O2"}, nil); err == nil {
		t.Error("expected error when allowlist is nil but flags supplied")
	}
}
