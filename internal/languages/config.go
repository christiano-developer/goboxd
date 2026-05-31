// internal/languages/config.go
// Christiano Fernadnes
// 31 May 26
// Model for the responses
package languages

type Language struct {
	ID                       string       `yaml:"id"`
	Name                     string       `yaml:"name"`
	SourceFilename           string       `yaml:"source_filename,omitempty"`
	SourceFilenameStrategy   string       `yaml:"source_filename_strategy,omitempty"`
	Artifact                 string       `yaml:"artifact,omitempty"`
	ArtifactFilenameStrategy string       `yaml:"artifact_filename_strategy,omitempty"`
	Build                    *PhaseConfig `yaml:"build,omitempty"`
	Run                      PhaseConfig  `yaml:"run"`
}

type PhaseConfig struct {
	Cmd           string   `yaml:"cmd"`
	Args          []string `yaml:"args"`
	Limits        Limits   `yaml:"limits"`
	FlagAllowlist []string `yaml:"flag_allowlist,omitempty"`
}

type Limits struct {
	WallTimeS    int `yaml:"wall_time_s"`
	MemoryKB     int `yaml:"memory_kb"`
	MaxProcesses int `yaml:"max_processes"`
}

type RegistryFile struct {
	Languages []Language `yaml:"languages"`
}
