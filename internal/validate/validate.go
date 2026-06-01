// internal/validate/validate.go
// Christiano Fernadnes
// 31 May 26

package validate

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/thesouldev/goboxd/internal/model"
)

const (
	MaxSourceBytes      = 256 * 1024 // 256 KiB
	MaxTests            = 50
	MaxFilenameLen      = 128
	MaxStdinBytes       = 64 * 1024 // 64 KiB per test stdin
	MaxRequestBodyBytes = 4 * 1024 * 1024 // 4 MiB max HTTP body limit
)

func Filename(s string) error {
	if s == "" {
		return fmt.Errorf("filename must not be empty")
	}
	if len(s) > MaxFilenameLen {
		return fmt.Errorf("filename too long (max %d chars)", MaxFilenameLen)
	}
	if strings.ContainsAny(s, `/\`) {
		return fmt.Errorf("filename must be a single path component")
	}
	if strings.HasPrefix(s, ".") {
		return fmt.Errorf("filename must not start with a dot")
	}
	if s == ".." {
		return fmt.Errorf("filename must not be ..")
	}
	// Ensure it has no directory component at all
	if filepath.Base(s) != s {
		return fmt.Errorf("filename must be a single path component")
	}
	return nil
}

func SourceSize(source string) error {
	if len(source) == 0 {
		return fmt.Errorf("source must not be empty")
	}
	if len(source) > MaxSourceBytes {
		return fmt.Errorf("source exceeds maximum size of %d bytes", MaxSourceBytes)
	}
	return nil
}

func TestCount(n int) error {
	if n == 0 {
		return fmt.Errorf("at least one test case is required")
	}
	if n > MaxTests {
		return fmt.Errorf("too many test cases (max %d)", MaxTests)
	}
	return nil
}

func Flags(supplied []string, allowlist []string) error {
	if len(supplied) == 0 {
		return nil
	}
	if len(allowlist) == 0 {
		return fmt.Errorf("this language does not accept custom flags")
	}
	for _, flag := range supplied {
		if !flagAllowed(flag, allowlist) {
			return fmt.Errorf("flag %q is not allowed for this language", flag)
		}
	}
	return nil
}

func flagAllowed(flag string, allowlist []string) bool {
	for _, pattern := range allowlist {
		if pattern == flag {
			return true
		}
		// Handle glob patterns like "-std=*"
		if strings.HasSuffix(pattern, "*") {
			prefix := strings.TrimSuffix(pattern, "*")
			if strings.HasPrefix(flag, prefix) {
				return true
			}
		}
	}
	return false
}

// ResourceLimits validates and dynamically clamps resource overrides.
// It returns a slice of warnings if any values were adjusted.
func ResourceLimits(limits *model.Limits, langID string, reqRate float64) []string {
	if limits == nil {
		return nil
	}

	var warnings []string

	// 1. Calculate load-adaptive maximum caps based on the current req rate
	maxWallTime := 15
	if reqRate > 5.0 {
		// Reduce wall time by 0.5s per req/sec above 5, down to min 1s
		reduced := 15 - int(0.5*(reqRate-5.0))
		if reduced < 1 {
			maxWallTime = 1
		} else {
			maxWallTime = reduced
		}
	}

	maxProc := 128

	// Memory caps depend on language compile/run requirements
	maxMem := 524288 // 512 MB standard max
	if langID == "java" || langID == "js" || langID == "c" || langID == "cpp" {
		// JVM, Node, GCC/G++ build can use up to 2 GB
		maxMem = 2097152
	}
	if reqRate > 5.0 {
		// Reduce max memory by 50MB (51200KB) per req/sec above 5
		reductionKB := int(51200 * (reqRate - 5.0))
		if langID == "java" || langID == "js" {
			// For Java/JS, never reduce below 1 GB (1048576 KB)
			clamped := 2097152 - reductionKB
			if clamped < 1048576 {
				maxMem = 1048576
			} else {
				maxMem = clamped
			}
		} else {
			clamped := maxMem - reductionKB
			if clamped < 16384 { // absolute baseline 16 MB for other languages under load
				maxMem = 16384
			} else {
				maxMem = clamped
			}
		}
	}

	// 2. Minimum baseline caps (under-allocation protection)
	minMem := 16384 // 16 MB
	if langID == "java" || langID == "js" {
		minMem = 1048576 // 1 GB boot min
	} else if langID == "c" || langID == "cpp" {
		minMem = 262144 // 256 MB compile min
	}

	// Clamp WallTimeS
	if limits.WallTimeS > 0 {
		if limits.WallTimeS < 1 {
			limits.WallTimeS = 1
			warnings = append(warnings, "wall_time_s limit override is too low; clamped to minimum (1s)")
		} else if limits.WallTimeS > maxWallTime {
			limits.WallTimeS = maxWallTime
			warnings = append(warnings, fmt.Sprintf("wall_time_s limit override exceeds load-adaptive cap; clamped to maximum (%ds)", maxWallTime))
		}
	}

	// Clamp MemoryKB
	if limits.MemoryKB > 0 {
		if limits.MemoryKB < minMem {
			limits.MemoryKB = minMem
			warnings = append(warnings, fmt.Sprintf("memory_kb limit override is too low for runtime initialization; clamped to minimum (%d KB)", minMem))
		} else if limits.MemoryKB > maxMem {
			limits.MemoryKB = maxMem
			warnings = append(warnings, fmt.Sprintf("memory_kb limit override exceeds load-adaptive cap; clamped to maximum (%d KB)", maxMem))
		}
	}

	// Clamp MaxProcesses
	if limits.MaxProcesses > 0 {
		if limits.MaxProcesses < 1 {
			limits.MaxProcesses = 1
			warnings = append(warnings, "max_processes limit override is too low; clamped to minimum (1)")
		} else if limits.MaxProcesses > maxProc {
			limits.MaxProcesses = maxProc
			warnings = append(warnings, fmt.Sprintf("max_processes limit override exceeds maximum cap; clamped to (%d)", maxProc))
		}
	}

	return warnings
}

// TestInputs validates that the stdin and expected output sizes for all test cases 
// do not exceed MaxStdinBytes to prevent memory exhaustion attacks.
func TestInputs(tests []model.TestCase) error {
	for i, tc := range tests {
		if len(tc.Stdin) > MaxStdinBytes {
			return fmt.Errorf("test case %d stdin exceeds max size of %d bytes", i, MaxStdinBytes)
		}
		if len(tc.ExpectedStdout) > MaxStdinBytes {
			return fmt.Errorf("test case %d expected stdout exceeds max size of %d bytes", i, MaxStdinBytes)
		}
	}
	return nil
}


