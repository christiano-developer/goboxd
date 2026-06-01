// internal/validate/validate_test.go
// Christiano Fernandes
// 31 May 26
package validate_test

import (
	"strings"
	"testing"

	"github.com/thesouldev/goboxd/internal/model"
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

func TestResourceLimits(t *testing.T) {
	// 1. Test Java under-allocation clamp (MemoryKB 256 -> 1048576)
	limitsJava := &model.Limits{
		MemoryKB:  256,
		WallTimeS: 0, // no change
	}
	warns := validate.ResourceLimits(limitsJava, "java", 0.0)
	if limitsJava.MemoryKB != 1048576 {
		t.Errorf("expected MemoryKB to be clamped to 1048576, got %d", limitsJava.MemoryKB)
	}
	if len(warns) != 1 || !strings.Contains(warns[0], "clamped to minimum") {
		t.Errorf("expected 1 warning regarding minimum clamp, got: %v", warns)
	}

	// 2. Test Java over-allocation clamp (MemoryKB 4GB -> 2GB)
	limitsJavaOver := &model.Limits{
		MemoryKB:  4194304,
		WallTimeS: 50, // exceeds 15s max
	}
	warnsOver := validate.ResourceLimits(limitsJavaOver, "java", 0.0)
	if limitsJavaOver.MemoryKB != 2097152 {
		t.Errorf("expected MemoryKB to be clamped to 2097152, got %d", limitsJavaOver.MemoryKB)
	}
	if limitsJavaOver.WallTimeS != 15 {
		t.Errorf("expected WallTimeS to be clamped to 15, got %d", limitsJavaOver.WallTimeS)
	}
	if len(warnsOver) != 2 {
		t.Errorf("expected 2 warnings, got: %v", warnsOver)
	}

	// 3. Test Load-Adaptive Capping under high request rate (e.g. 15.0 req/sec)
	// At rate 15.0:
	// maxWallTime = 15 - 0.5 * (15 - 5) = 10s
	// maxMem for python = 512MB - 50MB * (15 - 5) = 512MB - 500MB = 12MB -> minimum cap baseline 16MB (16384 KB)
	limitsPy := &model.Limits{
		MemoryKB:  524288, // requests standard 512MB
		WallTimeS: 12,     // requests 12s
	}
	warnsLoad := validate.ResourceLimits(limitsPy, "py3", 15.0)
	if limitsPy.WallTimeS != 10 {
		t.Errorf("expected load-adaptive WallTimeS 10, got %d", limitsPy.WallTimeS)
	}
	if limitsPy.MemoryKB != 16384 {
		t.Errorf("expected load-adaptive MemoryKB 16384 (16MB), got %d", limitsPy.MemoryKB)
	}
	if len(warnsLoad) != 2 {
		t.Errorf("expected 2 load-adaptive warnings, got: %v", warnsLoad)
	}
}

