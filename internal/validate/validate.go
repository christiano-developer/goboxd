// internal/validate/validate.go
// Christiano Fernadnes
// 31 May 26
// Model for the responses
package validate

import (
	"fmt"
	"path/filepath"
	"strings"
)

const (
	MaxSourceBytes = 256 * 1024 // 256 KiB
	MaxTests       = 50
	MaxFilenameLen = 128
	MaxStdinBytes  = 64 * 1024 // 64 KiB per test stdin
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
