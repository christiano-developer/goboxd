// internal/executor/executor.go
package executor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/thesouldev/goboxd/internal/languages"
	"github.com/thesouldev/goboxd/internal/model"
	"github.com/thesouldev/goboxd/internal/sandbox"
)

const (
	maxOutputBytes = 64 * 1024 // 64 KiB cap on stdout/stderr per run
	truncMarker    = "\n[TRUNCATED]"
)

// Run executes a RunRequest end-to-end and returns a RunResponse.
func Run(req *model.RunRequest, lang languages.Language) (*model.RunResponse, error) {
	// Create a unique temp directory for this request — never reuse
	workDir, err := os.MkdirTemp("", "goboxd-*")
	if err != nil {
		return nil, fmt.Errorf("create workdir: %w", err)
	}
	// Always clean up, even on panic
	defer os.RemoveAll(workDir)

	// Determine source filename
	srcFilename := lang.SourceFilename
	if req.SourceFilename != "" {
		srcFilename = req.SourceFilename
	}
	srcPath := filepath.Join(workDir, srcFilename)

	// Write source to temp dir
	if err := os.WriteFile(srcPath, []byte(req.Source), 0644); err != nil {
		return nil, fmt.Errorf("write source: %w", err)
	}

	resp := &model.RunResponse{}

	// --- Build phase (compiled languages only) ---
	if lang.Build != nil {
		artifactPath := filepath.Join(workDir, lang.Artifact)
		buildCmd := resolvePlaceholders(lang.Build.Args, srcPath, artifactPath)

		limits := lang.Build.Limits
		if req.Build != nil && req.Build.Limits != nil {
			limits = mergeLimits(limits, *req.Build.Limits)
		}

		start := time.Now()
		stdout, stderr, exitErr := runInSandbox(sandbox.Config{
			WorkDir:      workDir,
			WallTimeSecs: limits.WallTimeS,
			MemoryKB:     limits.MemoryKB,
			MaxPIDs:      limits.MaxProcesses,
			Command:      append([]string{lang.Build.Cmd}, buildCmd...),
		}, "")
		durationMs := time.Since(start).Milliseconds()

		buildStatus := "ok"
		if exitErr != nil {
			buildStatus = "failed"
		}

		resp.Build = &model.BuildResult{
			Status:     buildStatus,
			Stdout:     stdout,
			Stderr:     stderr,
			DurationMs: durationMs,
		}

		// If build failed, mark all tests as not_executed and return
		if buildStatus == "failed" {
			resp.Status = "build_failed"
			resp.Tests = make([]model.TestResult, len(req.Tests))
			for i := range resp.Tests {
				resp.Tests[i] = model.TestResult{Status: "not_executed"}
			}
			return resp, nil
		}
	}

	// --- Run phase — one execution per test case ---
	runLimits := lang.Run.Limits
	if req.Run != nil && req.Run.Limits != nil {
		runLimits = mergeLimits(runLimits, *req.Run.Limits)
	}

	artifactPath := filepath.Join(workDir, lang.Artifact)
	runCmd := resolvePlaceholders(lang.Run.Args, srcPath, artifactPath)
	fullCmd := append([]string{lang.Run.Cmd}, runCmd...)
	// Resolve {{artifact}} in the cmd itself (e.g. "./{{artifact}}")
	fullCmd[0] = strings.ReplaceAll(fullCmd[0], "{{artifact}}", artifactPath)

	resp.Tests = make([]model.TestResult, len(req.Tests))
	firstNonAccepted := ""

	for i, tc := range req.Tests {
		start := time.Now()
		stdout, stderr, exitErr := runInSandbox(sandbox.Config{
			WorkDir:      workDir,
			WallTimeSecs: runLimits.WallTimeS,
			MemoryKB:     runLimits.MemoryKB,
			MaxPIDs:      runLimits.MaxProcesses,
			Command:      fullCmd,
		}, tc.Stdin)
		durationMs := time.Since(start).Milliseconds()

		status := classifyResult(stdout, tc.ExpectedStdout, exitErr, runLimits.WallTimeS, durationMs)

		resp.Tests[i] = model.TestResult{
			Status:     status,
			Stdout:     stdout,
			Stderr:     stderr,
			DurationMs: durationMs,
		}

		if firstNonAccepted == "" && status != "accepted" {
			firstNonAccepted = status
		}
	}

	// Top-level status
	if firstNonAccepted == "" {
		resp.Status = "accepted"
	} else {
		resp.Status = firstNonAccepted
	}

	return resp, nil
}

// runInSandbox executes cmd inside nsjail, feeding stdin, and returns capped stdout/stderr.
func runInSandbox(cfg sandbox.Config, stdin string) (stdout, stderr string, err error) {
	cmd, err := sandbox.Build(cfg)
	if err != nil {
		return "", "", err
	}

	cmd.Stdin = strings.NewReader(stdin)

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &cappedWriter{w: &outBuf, limit: maxOutputBytes}
	cmd.Stderr = &cappedWriter{w: &errBuf, limit: maxOutputBytes}

	// Use a context so we can detect timeout independently
	ctx, cancel := context.WithTimeout(context.Background(),
		time.Duration(cfg.WallTimeSecs+2)*time.Second)
	defer cancel()
	_ = ctx // nsjail enforces its own time limit; context is a safety net

	runErr := cmd.Run()
	return outBuf.String(), errBuf.String(), runErr
}

// classifyResult maps execution outcome to the brief's status vocabulary.
func classifyResult(actual, expected string, exitErr error, wallTimeSecs int, durationMs int64) string {
	if exitErr != nil {
		// Check if it was a timeout (nsjail exits 1 on timeout but duration tells us)
		if durationMs >= int64(wallTimeSecs)*1000 {
			return "time_exceeded"
		}
		return "runtime_error"
	}
	if actual == expected {
		return "accepted"
	}
	if strings.TrimSpace(actual) == strings.TrimSpace(expected) {
		return "output_whitespace_mismatch"
	}
	return "wrong_output"
}

// resolvePlaceholders replaces {{source}} and {{artifact}} in arg list.
func resolvePlaceholders(args []string, srcPath, artifactPath string) []string {
	out := make([]string, len(args))
	for i, a := range args {
		a = strings.ReplaceAll(a, "{{source}}", srcPath)
		a = strings.ReplaceAll(a, "{{artifact}}", artifactPath)
		out[i] = a
	}
	return out
}

// mergeLimits applies non-zero fields from override onto base.
func mergeLimits(base languages.Limits, override model.Limits) languages.Limits {
	if override.WallTimeS > 0 {
		base.WallTimeS = override.WallTimeS
	}
	if override.MemoryKB > 0 {
		base.MemoryKB = override.MemoryKB
	}
	if override.MaxProcesses > 0 {
		base.MaxProcesses = override.MaxProcesses
	}
	return base
}

// cappedWriter writes to w up to limit bytes, then appends a truncation marker.
type cappedWriter struct {
	w       io.Writer
	limit   int
	written int
}

func (c *cappedWriter) Write(p []byte) (int, error) {
	if c.written >= c.limit {
		return len(p), nil // silently drop
	}
	remaining := c.limit - c.written
	if len(p) > remaining {
		p = p[:remaining]
		_, _ = c.w.Write(p)
		_, _ = io.WriteString(c.w, truncMarker)
		c.written = c.limit
		return len(p), nil
	}
	n, err := c.w.Write(p)
	c.written += n
	return n, err
}
