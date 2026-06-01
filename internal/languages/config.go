// internal/languages/config.go
// Christiano Fernadnes
// 31 May 26
package languages

type Language struct {
	ID                       string       `yaml:"id" json:"id"`
	Name                     string       `yaml:"name" json:"name"`
	SourceFilename           string       `yaml:"source_filename,omitempty" json:"source_filename,omitempty"`
	SourceFilenameStrategy   string       `yaml:"source_filename_strategy,omitempty" json:"source_filename_strategy,omitempty"`
	Artifact                 string       `yaml:"artifact,omitempty" json:"artifact,omitempty"`
	ArtifactFilenameStrategy string       `yaml:"artifact_filename_strategy,omitempty" json:"artifact_filename_strategy,omitempty"`
	Build                    *PhaseConfig `yaml:"build,omitempty" json:"build,omitempty"`
	Run                      PhaseConfig  `yaml:"run" json:"run"`
}

type PhaseConfig struct {
	Cmd           string   `yaml:"cmd" json:"cmd"`
	Args          []string `yaml:"args" json:"args"`
	Limits        Limits   `yaml:"limits" json:"limits"`
	FlagAllowlist []string `yaml:"flag_allowlist,omitempty" json:"flag_allowlist,omitempty"`
}

type Limits struct {
	WallTimeS    int `yaml:"wall_time_s" json:"wall_time_s"`
	MemoryKB     int `yaml:"memory_kb" json:"memory_kb"`
	MaxProcesses int `yaml:"max_processes" json:"max_processes"`
}

type RegistryFile struct {
	Languages []Language `yaml:"languages" json:"languages"`
}
