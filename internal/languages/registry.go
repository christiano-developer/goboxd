// internal/languages/registry.go
// Christiano Fernadnes
// 31 May 26
// language registry source and validate choice
package languages

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Registry holds all loaded languages, keyed by ID.
type Registry struct {
	languages map[string]Language
}

// Load reads the YAML file at path and returns a Registry.
func Load(path string) (*Registry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read language config: %w", err)
	}

	var file RegistryFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parse language config: %w", err)
	}

	if len(file.Languages) == 0 {
		return nil, fmt.Errorf("language config has no entries")
	}

	m := make(map[string]Language, len(file.Languages))
	for _, lang := range file.Languages {
		if err := lang.Validate(); err != nil {
			return nil, fmt.Errorf("invalid language config: %w", err)
		}
		m[lang.ID] = lang
	}

	return &Registry{languages: m}, nil
}

// Get returns the Language for the given id, or an error if not found.
func (r *Registry) Get(id string) (Language, error) {
	lang, ok := r.languages[id]
	if !ok {
		return Language{}, fmt.Errorf("unknown language: %q", id)
	}
	return lang, nil
}

// All returns all registered languages.
func (r *Registry) All() []Language {
	out := make([]Language, 0, len(r.languages))
	for _, l := range r.languages {
		out = append(out, l)
	}
	return out
}

func (l Language) Validate() error {
	if l.ID == "" {
		return fmt.Errorf("language ID is required")
	}
	if l.Name == "" {
		return fmt.Errorf("language name is required for ID %q", l.ID)
	}
	if l.Run.Cmd == "" {
		return fmt.Errorf("run command is required for ID %q", l.ID)
	}
	if l.Run.Limits.WallTimeS <= 0 {
		return fmt.Errorf("run limits.wall_time_s must be positive for ID %q", l.ID)
	}
	if l.Run.Limits.MemoryKB <= 0 {
		return fmt.Errorf("run limits.memory_kb must be positive for ID %q", l.ID)
	}
	if l.Run.Limits.MaxProcesses <= 0 {
		return fmt.Errorf("run limits.max_processes must be positive for ID %q", l.ID)
	}

	if l.Build != nil {
		if l.Build.Cmd == "" {
			return fmt.Errorf("build command is required for ID %q", l.ID)
		}
		if l.Build.Limits.WallTimeS <= 0 {
			return fmt.Errorf("build limits.wall_time_s must be positive for ID %q", l.ID)
		}
		if l.Build.Limits.MemoryKB <= 0 {
			return fmt.Errorf("build limits.memory_kb must be positive for ID %q", l.ID)
		}
		if l.Build.Limits.MaxProcesses <= 0 {
			return fmt.Errorf("build limits.max_processes must be positive for ID %q", l.ID)
		}
	}

	if l.SourceFilename != "" {
		if err := validateFilename(l.SourceFilename); err != nil {
			return fmt.Errorf("invalid source_filename for ID %q: %w", l.ID, err)
		}
	}
	if l.Artifact != "" {
		if err := validateFilename(l.Artifact); err != nil {
			return fmt.Errorf("invalid artifact for ID %q: %w", l.ID, err)
		}
	}
	return nil
}

func validateFilename(s string) error {
	if s == "" {
		return fmt.Errorf("filename must not be empty")
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
	return nil
}
