//internal/model/request.go
//Christiano Fernadnes
// 31 May 26
// Model for the requsts,

package model

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
