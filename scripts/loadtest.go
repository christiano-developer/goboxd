// scripts/load_test.go
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"
)

type RunRequest struct {
	Language string     `json:"language"`
	Source   string     `json:"source"`
	Tests    []TestCase `json:"tests"`
}

type TestCase struct {
	Stdin          string `json:"stdin"`
	ExpectedStdout string `json:"expected_stdout"`
}

type RunResponse struct {
	Status string `json:"status"`
}

func main() {
	concurrency := flag.Int("c", 10, "Number of concurrent workers")
	totalReqs := flag.Int("n", 100, "Total number of requests to run")
	targetURL := flag.String("url", "http://localhost:8080/run", "Target endpoint URL")
	flag.Parse()

	if *concurrency <= 0 || *totalReqs <= 0 {
		fmt.Println("Concurrency and total requests must be positive numbers")
		os.Exit(1)
	}

	payload := RunRequest{
		Language: "py3",
		Source:   "print('hello')",
		Tests: []TestCase{
			{Stdin: "", ExpectedStdout: "hello\n"},
		},
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		fmt.Printf("failed to marshal payload: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Starting load test against %s\n", *targetURL)
	fmt.Printf("Concurrency: %d, Total Requests: %d\n", *concurrency, *totalReqs)

	jobs := make(chan int, *totalReqs)
	for i := 0; i < *totalReqs; i++ {
		jobs <- i
	}
	close(jobs)

	var wg sync.WaitGroup
	var mu sync.Mutex

	var latencies []time.Duration
	statusCounts := make(map[int]int)
	runStatuses := make(map[string]int)

	startTime := time.Now()

	for i := 0; i < *concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := &http.Client{
				Timeout: 15 * time.Second,
			}

			for range jobs {
				reqStart := time.Now()
				resp, err := client.Post(*targetURL, "application/json", bytes.NewReader(payloadBytes))
				reqDur := time.Since(reqStart)

				var code int
				var runStatus string

				if err != nil {
					code = 999 // connection error
				} else {
					code = resp.StatusCode
					if resp.StatusCode == http.StatusOK {
						var runResp RunResponse
						bodyBytes, _ := io.ReadAll(resp.Body)
						_ = json.Unmarshal(bodyBytes, &runResp)
						runStatus = runResp.Status
					}
					resp.Body.Close()
				}

				mu.Lock()
				latencies = append(latencies, reqDur)
				statusCounts[code]++
				if runStatus != "" {
					runStatuses[runStatus]++
				}
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	totalDuration := time.Since(startTime)

	if len(latencies) == 0 {
		fmt.Println("No requests completed.")
		return
	}

	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})

	p50 := latencies[len(latencies)*50/100]
	p95 := latencies[len(latencies)*95/100]
	p99 := latencies[len(latencies)*99/100]

	avg := time.Duration(0)
	for _, l := range latencies {
		avg += l
	}
	avg = avg / time.Duration(len(latencies))

	rps := float64(*totalReqs) / totalDuration.Seconds()

	fmt.Println("\n--- Results ---")
	fmt.Printf("Total Time:   %v\n", totalDuration)
	fmt.Printf("Throughput:   %.2f requests/sec\n", rps)
	fmt.Printf("Average:      %v\n", avg)
	fmt.Printf("p50 (median): %v\n", p50)
	fmt.Printf("p95:          %v\n", p95)
	fmt.Printf("p99:          %v\n", p99)

	fmt.Println("\n--- HTTP Status Counts ---")
	for code, count := range statusCounts {
		statusText := http.StatusText(code)
		if code == 999 {
			statusText = "Connection Error"
		}
		fmt.Printf("  %d (%s): %d\n", code, statusText, count)
	}

	if len(runStatuses) > 0 {
		fmt.Println("\n--- /run Outcomes ---")
		for status, count := range runStatuses {
			fmt.Printf("  %s: %d\n", status, count)
		}
	}
}
