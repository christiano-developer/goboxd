// internal/languages/registry.go
// Christiano Fernadnes
// 31 May 26
// language registry source and validate choice
package languages

import (
	"fmt"
	"os"

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
