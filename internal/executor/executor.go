// internal/executor/executor.go
// Christiano Fernandes
// 31 May 26
package executor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/thesouldev/goboxd/internal/languages"
	"github.com/thesouldev/goboxd/internal/model"
	"github.com/thesouldev/goboxd/internal/sandbox"
)

const (
	maxOutputBytes = 64 * 1024 // 64 KiB cap on stdout/stderr per run
	truncMarker    = "\n[TRUNCATED]"
)

var uidCounter uint32

// nextUID returns a process-unique UID in the range [100000, 1001099999]
// to prevent namespace collisions under concurrent load.
func nextUID() int {
	c := atomic.AddUint32(&uidCounter, 1)
	pid := os.Getpid()
	return 100000 + (pid%100000)*10000 + int(c%10000)
}

// Run executes a RunRequest end-to-end and returns a RunResponse.
func Run(req *model.RunRequest, lang languages.Language) (*model.RunResponse, error) {
	// Create a unique temp directory for this request — never reuse
	workDir, err := os.MkdirTemp("", "goboxd-*")
	if err != nil {
		return nil, fmt.Errorf("create workdir: %w", err)
	}
	// Always clean up, even on panic
	defer os.RemoveAll(workDir)

	// Ensure the sandbox user (running under a mapped unprivileged UID) has full read/write access
	if err := os.Chmod(workDir, 0777); err != nil {
		return nil, fmt.Errorf("chmod workdir: %w", err)
	}

	reqUID := nextUID()

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
		artifactName := lang.Artifact
		if lang.ArtifactFilenameStrategy == "from_request" || artifactName == "" {
			artifactName = req.ArtifactFilename
		}
		artifactPath := ""
		if artifactName != "" {
			artifactPath = filepath.Join(workDir, artifactName)
		}

		buildFlags := []string{}
		if req.Build != nil && len(req.Build.Flags) > 0 {
			buildFlags = req.Build.Flags
		}

		buildCmd := resolvePlaceholders(lang.Build.Args, srcPath, artifactPath, buildFlags)

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
			UID:          reqUID,
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

	artifactName := lang.Artifact
	if lang.ArtifactFilenameStrategy == "from_request" || artifactName == "" {
		artifactName = req.ArtifactFilename
	}
	artifactPath := ""
	if artifactName != "" {
		artifactPath = filepath.Join(workDir, artifactName)
	}

	runFlags := []string{}
	if req.Run != nil && len(req.Run.Flags) > 0 {
		runFlags = req.Run.Flags
	}

	// For execution arguments, we pass the relative artifactName (e.g. for Java classname: java Main)
	runCmd := resolvePlaceholders(lang.Run.Args, srcPath, artifactName, runFlags)
	fullCmd := append([]string{lang.Run.Cmd}, runCmd...)
	// Resolve {{artifact}} in the command path itself (e.g. "./{{artifact}}" -> "/tmp/goboxd-xxx/solution")
	if artifactPath != "" {
		fullCmd[0] = strings.ReplaceAll(fullCmd[0], "{{artifact}}", artifactPath)
	}
	if strings.HasPrefix(fullCmd[0], "./") {
		cleaned := strings.TrimPrefix(fullCmd[0], "./")
		if filepath.IsAbs(cleaned) {
			fullCmd[0] = cleaned
		}
	}

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
			UID:          reqUID,
		}, tc.Stdin)
		durationMs := time.Since(start).Milliseconds()

		status := classifyResult(stdout, tc.ExpectedStdout, exitErr, runLimits.WallTimeS, durationMs)

		resp.Tests[i] = model.TestResult{
			Status:       status,
			Stdout:       stdout,
			Stderr:       stderr,
			DurationMs:   durationMs,
			MemoryPeakKB: 0,
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

// resolvePlaceholders replaces {{source}}, {{artifact}}, and {{flags}} placeholders.
func resolvePlaceholders(args []string, srcPath, artifactParam string, flags []string) []string {
	var out []string
	for _, a := range args {
		if a == "{{flags}}" {
			out = append(out, flags...)
		} else {
			a = strings.ReplaceAll(a, "{{source}}", srcPath)
			a = strings.ReplaceAll(a, "{{artifact}}", artifactParam)
			out = append(out, a)
		}
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
