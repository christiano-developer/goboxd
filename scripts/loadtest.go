// scripts/loadtest.go
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"
)

type RunRequest struct {
	Language         string     `json:"language"`
	Source           string     `json:"source"`
	SourceFilename   string     `json:"source_filename,omitempty"`
	ArtifactFilename string     `json:"artifact_filename,omitempty"`
	Build            *PhaseOpts `json:"build,omitempty"`
	Run              *PhaseOpts `json:"run,omitempty"`
	Tests            []TestCase `json:"tests"`
}

type PhaseOpts struct {
	Limits *Limits  `json:"limits,omitempty"`
	Flags  []string `json:"flags,omitempty"`
}

type Limits struct {
	WallTimeS    int `json:"wall_time_s,omitempty"`
	MemoryKB     int `json:"memory_kb,omitempty"`
	MaxProcesses int `json:"max_processes,omitempty"`
}

type TestCase struct {
	Stdin          string `json:"stdin"`
	ExpectedStdout string `json:"expected_stdout"`
}

type RunResponse struct {
	Status   string   `json:"status"`
	Warnings []string `json:"warnings,omitempty"`
}

type PayloadConfig struct {
	Language         string
	Source           string
	SourceFilename   string
	ArtifactFilename string
	ExpectedStdout   string
}

var payloads = map[string]PayloadConfig{
	"py3": {
		Language:       "py3",
		Source:         "print('hello')",
		ExpectedStdout: "hello\n",
	},
	"c": {
		Language:       "c",
		Source:         "#include <stdio.h>\nint main() { printf(\"hello\\n\"); return 0; }",
		ExpectedStdout: "hello\n",
	},
	"cpp": {
		Language:       "cpp",
		Source:         "#include <iostream>\nint main() { std::cout << \"hello\\n\"; return 0; }",
		ExpectedStdout: "hello\n",
	},
	"java": {
		Language:         "java",
		Source:           "public class Main { public static void main(String[] args) { System.out.println(\"hello\"); } }",
		SourceFilename:   "Main.java",
		ArtifactFilename: "Main",
		ExpectedStdout:   "hello\n",
	},
	"bash": {
		Language:       "bash",
		Source:         "echo 'hello'",
		ExpectedStdout: "hello\n",
	},
	"js": {
		Language:       "js",
		Source:         "console.log('hello')",
		ExpectedStdout: "hello\n",
	},
	"verilog": {
		Language:       "verilog",
		Source:         "module Main; initial begin $display(\"hello\"); $finish; end endmodule",
		ExpectedStdout: "hello\n",
	},
}

func main() {
	concurrency := flag.Int("c", 10, "Number of concurrent workers")
	totalReqs := flag.Int("n", 100, "Total number of requests to run")
	targetURL := flag.String("url", "http://localhost:8080/run", "Target endpoint URL")
	lang := flag.String("lang", "py3", "Language payload to run (py3, c, cpp, java, bash, js, verilog, mixed)")
	flag.Parse()

	if *concurrency <= 0 || *totalReqs <= 0 {
		fmt.Println("Concurrency and total requests must be positive numbers")
		os.Exit(1)
	}

	var staticConfig PayloadConfig
	isMixed := (*lang == "mixed")

	if !isMixed {
		var ok bool
		staticConfig, ok = payloads[*lang]
		if !ok {
			fmt.Printf("Unsupported language: %s. Supported: py3, c, cpp, java, bash, js, verilog, mixed\n", *lang)
			os.Exit(1)
		}
	}

	fmt.Printf("Starting load test against %s\n", *targetURL)
	fmt.Printf("Language: %s, Concurrency: %d, Total Requests: %d\n", *lang, *concurrency, *totalReqs)

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
	warningsReceived := 0

	startTime := time.Now()

	for i := 0; i < *concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := &http.Client{
				Timeout: 20 * time.Second,
			}

			for jobIdx := range jobs {
				var reqPayload RunRequest
				var selectedLang string

				if isMixed {
					keys := []string{"py3", "c", "cpp", "java", "bash", "js", "verilog"}
					selectedLang = keys[rand.Intn(len(keys))]
					config := payloads[selectedLang]

					reqPayload = RunRequest{
						Language:         config.Language,
						Source:           config.Source,
						SourceFilename:   config.SourceFilename,
						ArtifactFilename: config.ArtifactFilename,
						Tests: []TestCase{
							{Stdin: "", ExpectedStdout: config.ExpectedStdout},
						},
					}

					// Randomly inject overrides to test clamping
					overrideType := rand.Intn(3) // 0: under-allocate, 1: over-allocate, 2: standard
					if overrideType == 0 {
						reqPayload.Run = &PhaseOpts{
							Limits: &Limits{
								MemoryKB:  256, // Under-allocated memory
								WallTimeS: 0,   // Under-allocated wall time
							},
						}
						// If Java/C/C++ compile exists, also under-allocate build phase
						if selectedLang == "c" || selectedLang == "cpp" || selectedLang == "java" {
							reqPayload.Build = &PhaseOpts{
								Limits: &Limits{
									MemoryKB: 512,
								},
							}
						}
					} else if overrideType == 1 {
						reqPayload.Run = &PhaseOpts{
							Limits: &Limits{
								MemoryKB:  5242880, // 5 GB (over-allocated)
								WallTimeS: 45,      // 45s (over-allocated)
							},
						}
					}
				} else {
					selectedLang = *lang
					reqPayload = RunRequest{
						Language:         staticConfig.Language,
						Source:           staticConfig.Source,
						SourceFilename:   staticConfig.SourceFilename,
						ArtifactFilename: staticConfig.ArtifactFilename,
						Tests: []TestCase{
							{Stdin: "", ExpectedStdout: staticConfig.ExpectedStdout},
						},
					}
				}

				payloadBytes, err := json.Marshal(reqPayload)
				if err != nil {
					continue
				}

				reqStart := time.Now()
				resp, err := client.Post(*targetURL, "application/json", bytes.NewReader(payloadBytes))
				reqDur := time.Since(reqStart)

				var code int
				var runStatus string
				var warnings []string

				if err != nil {
					code = 999 // connection error
					runStatus = "connection_failed"
				} else {
					code = resp.StatusCode
					if resp.StatusCode == http.StatusOK {
						var runResp RunResponse
						bodyBytes, _ := io.ReadAll(resp.Body)
						_ = json.Unmarshal(bodyBytes, &runResp)
						runStatus = runResp.Status
						warnings = runResp.Warnings
					} else {
						// For non-200, try to parse error message
						bodyBytes, _ := io.ReadAll(resp.Body)
						runStatus = fmt.Sprintf("http_%d: %s", resp.StatusCode, string(bodyBytes))
					}
					resp.Body.Close()
				}

				mu.Lock()
				latencies = append(latencies, reqDur)
				statusCounts[code]++
				if runStatus != "" {
					runStatuses[runStatus]++
				}
				if len(warnings) > 0 {
					warningsReceived += len(warnings)
				}
				mu.Unlock()

				warningLog := ""
				if len(warnings) > 0 {
					warningLog = fmt.Sprintf(" | Warnings: %v", warnings)
				}
				fmt.Printf("[%d] Lang: %s | Code: %d | Status: %s | Latency: %v%s\n",
					jobIdx, selectedLang, code, runStatus, reqDur, warningLog)
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
	fmt.Printf("Warnings Rec: %d\n", warningsReceived)

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

