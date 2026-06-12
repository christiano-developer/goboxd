// scripts/memhog/memhog_loadtest.go
//
// Open-loop, rate-stepped load generator for the MemoryHog Java benchmark.
//
// For each target request rate it holds an OPEN-LOOP attack for a fixed
// duration (requests are issued on a fixed schedule regardless of whether
// prior ones have completed — this is what makes "offered RPS" meaningful),
// then records one CSV row. A request is a FAILURE if it returns a non-2xx
// status OR exceeds the per-request timeout (default 10s) — matching the
// challenge's definition. Between steps it drains the server queue so each
// step starts from a clean slate.
//
// Output CSV schema (exact):
//   target_rps,throughput_rps,duration_s,requests,success,failed,error_pct,p50_ms,p95_ms,p99_ms,max_ms
//
// Usage:
//   go run ./scripts/memhog \
//     -url http://localhost:8080/run -source docs/loadtest/MemoryHog.java \
//     -out docs/loadtest/results.csv
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ---- request/response shapes (mirror the goboxd API) ----

type runRequest struct {
	Language         string     `json:"language"`
	Source           string     `json:"source"`
	SourceFilename   string     `json:"source_filename,omitempty"`
	ArtifactFilename string     `json:"artifact_filename,omitempty"`
	Tests            []testCase `json:"tests"`
}

type testCase struct {
	Stdin          string `json:"stdin"`
	ExpectedStdout string `json:"expected_stdout"`
}

// ---- per-step result ----

type stepResult struct {
	targetRPS          float64
	throughput         float64
	durationS          float64
	requests           int
	success            int
	failed             int
	errorPct           float64
	p50, p95, p99, max float64 // milliseconds

	// server-side work accounting (from /info jobs_total delta)
	serverCompleted int // jobs the server actually finished this step
	wasted          int // finished but the client had already timed out (abandoned)

	// extra signals
	shed      int // 503 load-shed responses
	timeouts  int // client-side >timeout failures
	nonAccept int // HTTP 200 but run status != accepted
}

func main() {
	url := flag.String("url", "http://localhost:8080/run", "POST /run endpoint")
	infoURL := flag.String("info", "http://localhost:8080/info", "GET /info endpoint (queue drain + concurrency label)")
	source := flag.String("source", "docs/loadtest/MemoryHog.java", "path to MemoryHog.java")
	runsDir := flag.String("runs-dir", "docs/loadtest/runs", "directory for the per-run CSV + plots")
	latest := flag.String("out", "docs/loadtest/results.csv", "canonical 'latest' CSV (latest plots are written beside it)")
	label := flag.String("label", "", "run label for filenames; default auto = c<concurrency> from /info")
	tag := flag.String("tag", "2vcpu-2gb", "extra tag baked into the run label")
	ratesStr := flag.String("rates", "1,2,3,5,10,25,50,75,100,150,200,300,400", "comma-separated target RPS ladder")
	duration := flag.Duration("duration", 30*time.Second, "hold time per step")
	timeout := flag.Duration("timeout", 10*time.Second, "per-request timeout (>this counts as failed)")
	stopAfterFail := flag.Int("stop-after-fail", 3, "stop this many steps after the first step that has a failure")
	drainMax := flag.Duration("drain-max", 120*time.Second, "max time to wait for the queue to drain between steps")
	expected := flag.String("expected", "MemoryHog OK mb=150 checksum=-101888\n", "expected stdout (only used to mark runs accepted; does not affect pass/fail)")
	doPlot := flag.Bool("plot", true, "render plots after the run")
	python := flag.String("python", "python3", "python interpreter with matplotlib (for plotting)")
	plotScript := flag.String("plot-script", "docs/loadtest/plot.py", "matplotlib plot script")
	flag.Parse()

	srcBytes, err := os.ReadFile(*source)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read source %q: %v\n", *source, err)
		os.Exit(1)
	}

	body, err := json.Marshal(runRequest{
		Language:         "java",
		Source:           string(srcBytes),
		SourceFilename:   "MemoryHog.java",
		ArtifactFilename: "MemoryHog",
		Tests:            []testCase{{Stdin: "", ExpectedStdout: *expected}},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal request: %v\n", err)
		os.Exit(1)
	}

	rates, err := parseRates(*ratesStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse rates: %v\n", err)
		os.Exit(1)
	}

	// A transport tuned for many concurrent short connections so the client
	// itself is not the bottleneck.
	tr := &http.Transport{
		MaxIdleConns:        2000,
		MaxIdleConnsPerHost: 2000,
		MaxConnsPerHost:     0,
		IdleConnTimeout:     30 * time.Second,
	}
	client := &http.Client{Timeout: *timeout, Transport: tr}

	// Read the live concurrency limit so the run is auto-labelled (c<N>) and the
	// plot titles say which CONCURRENCY_LIMIT produced them.
	maxConc, maxQueue := fetchLimits(client, *infoURL)
	base := *label
	if base == "" {
		if maxConc > 0 {
			base = fmt.Sprintf("memhog_c%d_%s", maxConc, *tag)
		} else {
			base = "memhog_" + *tag
		}
	}
	// Timestamp keeps every run's files distinct (runs are sequential, never
	// simultaneous), e.g. memhog_c8_2vcpu-2gb_20060102-150405.
	runLabel := base + "_" + time.Now().Format("20060102-150405")
	title := "2 vCPU / 2 GB"
	if maxConc > 0 {
		title = fmt.Sprintf("2 vCPU / 2 GB · CONCURRENCY_LIMIT=%d", maxConc)
	}

	if err := os.MkdirAll(*runsDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir %q: %v\n", *runsDir, err)
		os.Exit(1)
	}
	runCSV := filepath.Join(*runsDir, runLabel+".csv")

	f, err := os.Create(runCSV)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create %q: %v\n", runCSV, err)
		os.Exit(1)
	}
	// First 11 columns are the challenge schema; the rest are extra work-accounting signals.
	fmt.Fprintln(f, "target_rps,throughput_rps,duration_s,requests,success,failed,error_pct,p50_ms,p95_ms,p99_ms,max_ms,server_completed,wasted,wasted_pct,timeouts,shed_503")

	fmt.Printf("MemoryHog load test → %s\n", *url)
	fmt.Printf("run=%s  concurrency_limit=%d  max_queue=%d\n", runLabel, maxConc, maxQueue)
	fmt.Printf("ladder=%v  duration=%s  timeout=%s  stop_after_fail=%d\n\n", rates, *duration, *timeout, *stopAfterFail)

	firstFailIdx := -1
	breakingPoint := 0.0
	for i, rate := range rates {
		// Start each step from an empty queue so the measurement is clean.
		drainQueue(client, *infoURL, *drainMax)
		jobsBefore := fetchJobsTotal(client, *infoURL)

		fmt.Printf("[step %d] offering %g rps for %s ...\n", i+1, rate, *duration)
		res := attack(client, *url, body, rate, *duration, *timeout)

		// Let any still-running (abandoned) jobs finish, then measure how much
		// work the server actually completed vs what the client received.
		drainQueue(client, *infoURL, *drainMax)
		res.serverCompleted = fetchJobsTotal(client, *infoURL) - jobsBefore
		res.wasted = res.serverCompleted - res.success
		if res.wasted < 0 {
			res.wasted = 0
		}

		writeRow(f, res)
		f.Sync()
		printStep(res)

		if res.failed > 0 && firstFailIdx == -1 {
			firstFailIdx = i
			breakingPoint = rate
			fmt.Printf("  >>> BREAKING POINT: first failure at %g rps\n", rate)
		}
		if firstFailIdx >= 0 && i-firstFailIdx >= *stopAfterFail {
			fmt.Printf("\nStopped %d steps past the breaking point.\n", *stopAfterFail)
			break
		}
	}
	f.Close()

	if firstFailIdx >= 0 {
		fmt.Printf("\nBreaking point (offered RPS of first failure): %g\n", breakingPoint)
	} else {
		fmt.Printf("\nNo failures across the whole ladder — breaking point not reached.\n")
	}

	// Mirror this run to the canonical "latest" CSV for the top-level deliverable.
	if err := copyFile(runCSV, *latest); err != nil {
		fmt.Fprintf(os.Stderr, "warn: copy to %q failed: %v\n", *latest, err)
	}
	fmt.Printf("CSV: %s  (latest copy: %s)\n", runCSV, *latest)

	// Render plots: per-run (in runsDir, prefixed with the label) and the
	// canonical "latest" pair beside the results CSV.
	if *doPlot {
		runPlot(*python, *plotScript, runCSV, *runsDir, runLabel, breakingPoint, title)
		runPlot(*python, *plotScript, *latest, filepath.Dir(*latest), "", breakingPoint, title)
	}
}

// fetchLimits reads the active concurrency limit and queue size from /info.
func fetchLimits(client *http.Client, infoURL string) (maxConcurrent, maxQueue int) {
	resp, err := client.Get(infoURL)
	if err != nil {
		return 0, 0
	}
	defer resp.Body.Close()
	var info struct {
		Limits struct {
			MaxConcurrentJobs int `json:"max_concurrent_jobs"`
			MaxQueueSize      int `json:"max_queue_size"`
		} `json:"limits"`
	}
	bb, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(bb, &info)
	return info.Limits.MaxConcurrentJobs, info.Limits.MaxQueueSize
}

// fetchJobsTotal reads stats.jobs_total (count of server-completed runs) from /info.
func fetchJobsTotal(client *http.Client, infoURL string) int {
	resp, err := client.Get(infoURL)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	var info struct {
		Stats struct {
			JobsTotal int `json:"jobs_total"`
		} `json:"stats"`
	}
	bb, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(bb, &info)
	return info.Stats.JobsTotal
}

// copyFile copies src to dst (used to refresh the canonical latest CSV).
func copyFile(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, b, 0o644)
}

// runPlot shells out to the matplotlib plot script. A plotting failure (e.g.
// matplotlib not installed) is reported but does not fail the run — the CSV is
// the critical artifact.
func runPlot(python, script, csv, outdir, prefix string, breakingPoint float64, title string) {
	args := []string{script, csv, "--outdir", outdir, "--title", title}
	if prefix != "" {
		args = append(args, "--prefix", prefix)
	}
	if breakingPoint > 0 {
		args = append(args, "--breaking-point", strconv.FormatFloat(breakingPoint, 'g', -1, 64))
	}
	cmd := exec.Command(python, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warn: plot (%s) failed: %v\n%s\n", python, err, out)
		fmt.Fprintf(os.Stderr, "      (install matplotlib, or run: python3 %s %s)\n", script, csv)
		return
	}
	fmt.Print(string(out))
}

// attack issues requests at `rate` per second for `dur` (open-loop), then waits
// for in-flight requests to finish (bounded by the client timeout).
func attack(client *http.Client, url string, body []byte, rate float64, dur, timeout time.Duration) stepResult {
	interval := time.Duration(float64(time.Second) / rate)

	var (
		mu        sync.Mutex
		lats      []float64 // ms
		wg        sync.WaitGroup
		success   int64
		failed    int64
		shed      int64
		timeouts  int64
		nonAccept int64
	)

	start := time.Now()
	deadline := start.Add(dur)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	launch := func() {
		defer wg.Done()
		t0 := time.Now()
		req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		lat := time.Since(t0).Seconds() * 1000.0

		ok := false
		if err != nil {
			// Treat client timeout (and any transport error) as a failure.
			if isTimeout(err) {
				atomic.AddInt64(&timeouts, 1)
			}
		} else {
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				ok = true
				// Peek at run status purely for insight (does not affect pass/fail).
				var rr struct {
					Status string `json:"status"`
				}
				bb, _ := io.ReadAll(resp.Body)
				if json.Unmarshal(bb, &rr) == nil && rr.Status != "accepted" {
					atomic.AddInt64(&nonAccept, 1)
				}
			} else {
				if resp.StatusCode == http.StatusServiceUnavailable {
					atomic.AddInt64(&shed, 1)
				}
				io.Copy(io.Discard, resp.Body)
			}
			resp.Body.Close()
		}

		if ok {
			atomic.AddInt64(&success, 1)
		} else {
			atomic.AddInt64(&failed, 1)
		}
		mu.Lock()
		lats = append(lats, lat)
		mu.Unlock()
	}

	for now := range ticker.C {
		if !now.Before(deadline) {
			break
		}
		wg.Add(1)
		go launch()
	}
	wg.Wait()
	elapsed := time.Since(start).Seconds()

	sort.Float64s(lats)
	reqs := int(success + failed)
	res := stepResult{
		targetRPS: rate,
		durationS: elapsed,
		requests:  reqs,
		success:   int(success),
		failed:    int(failed),
		shed:      int(shed),
		timeouts:  int(timeouts),
		nonAccept: int(nonAccept),
		p50:       pct(lats, 50),
		p95:       pct(lats, 95),
		p99:       pct(lats, 99),
		max:       pct(lats, 100),
	}
	if elapsed > 0 {
		res.throughput = float64(res.success) / elapsed
	}
	if reqs > 0 {
		res.errorPct = float64(res.failed) / float64(reqs) * 100.0
	}
	return res
}

// drainQueue blocks until the server reports no in-flight and no queued jobs,
// or until maxWait elapses, so the next step starts clean.
func drainQueue(client *http.Client, infoURL string, maxWait time.Duration) {
	type infoStats struct {
		Stats struct {
			InFlight int `json:"in_flight_jobs"`
			Queued   int `json:"queued_jobs"`
		} `json:"stats"`
	}
	deadline := time.Now().Add(maxWait)
	for time.Now().Before(deadline) {
		resp, err := client.Get(infoURL)
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		var s infoStats
		bb, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if json.Unmarshal(bb, &s) == nil && s.Stats.InFlight == 0 && s.Stats.Queued == 0 {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func writeRow(w io.Writer, r stepResult) {
	wastedPct := 0.0
	if r.serverCompleted > 0 {
		wastedPct = float64(r.wasted) / float64(r.serverCompleted) * 100.0
	}
	fmt.Fprintf(w, "%g,%.2f,%.1f,%d,%d,%d,%.2f,%.1f,%.1f,%.1f,%.1f,%d,%d,%.2f,%d,%d\n",
		r.targetRPS, r.throughput, r.durationS, r.requests, r.success, r.failed,
		r.errorPct, r.p50, r.p95, r.p99, r.max,
		r.serverCompleted, r.wasted, wastedPct, r.timeouts, r.shed)
}

func printStep(r stepResult) {
	fmt.Printf("  reqs=%d success=%d failed=%d err=%.1f%%  thr=%.2f/s  p50=%.0f p95=%.0f p99=%.0f max=%.0f ms\n",
		r.requests, r.success, r.failed, r.errorPct, r.throughput, r.p50, r.p95, r.p99, r.max)
	fmt.Printf("    server_completed=%d delivered=%d wasted=%d (work the server finished after the client gave up)\n",
		r.serverCompleted, r.success, r.wasted)
	if r.failed > 0 || r.nonAccept > 0 {
		fmt.Printf("    failure modes: timeouts=%d shed_503=%d  | non_accepted_200=%d\n", r.timeouts, r.shed, r.nonAccept)
	}
}

// pct returns the nearest-rank percentile (p in [0,100]) from sorted vals (ms).
func pct(sorted []float64, p float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if p <= 0 {
		return sorted[0]
	}
	if p >= 100 {
		return sorted[n-1]
	}
	rank := int(math.Ceil(p/100.0*float64(n))) - 1
	if rank < 0 {
		rank = 0
	}
	if rank >= n {
		rank = n - 1
	}
	return sorted[rank]
}

func parseRates(s string) ([]float64, error) {
	var out []float64
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		v, err := strconv.ParseFloat(part, 64)
		if err != nil {
			return nil, fmt.Errorf("bad rate %q: %w", part, err)
		}
		if v <= 0 {
			return nil, fmt.Errorf("rate must be positive, got %g", v)
		}
		out = append(out, v)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no rates given")
	}
	return out, nil
}

func isTimeout(err error) bool {
	type timeout interface{ Timeout() bool }
	if t, ok := err.(timeout); ok && t.Timeout() {
		return true
	}
	// url.Error wraps the underlying error.
	s := err.Error()
	return strings.Contains(s, "Client.Timeout") || strings.Contains(s, "context deadline exceeded")
}
