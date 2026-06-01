// internal/model/response.go
// Christiano Fernadnes
// 31 May 26
// Model for the responses,
//

package model

type RunResponse struct {
	Status   string       `json:"status"`
	Build    *BuildResult `json:"build,omitempty"`
	Tests    []TestResult `json:"tests"`
	Warnings []string     `json:"warnings,omitempty"`
}

type BuildResult struct {
	Status     string `json:"status"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	DurationMs int64  `json:"duration_ms"`
}

type TestResult struct {
	Status       string `json:"status"`
	Stdout       string `json:"stdout"`
	Stderr       string `json:"stderr"`
	DurationMs   int64  `json:"duration_ms"`
	MemoryPeakKB int64  `json:"memory_peak_kb,omitempty"`
}

type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
