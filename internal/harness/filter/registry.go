// Package filter defines LLM action spaces: prompt templates,
// response schemas, and variable specifications.
// Phase 2: PromptRegistry + YAML externalization (GEPA optimize ready).
package filter

import (
	"embed"
	"fmt"
	"io/fs"
	"slices"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

//go:embed prompts/*.yaml
var promptsFS embed.FS

// promptFile is the on-disk YAML schema for a prompt template.
type promptFile struct {
	Name        string            `yaml:"name"`
	Version     string            `yaml:"version"`
	Description string            `yaml:"description"`
	Variables   map[string]string `yaml:"variables"`
	Template    string            `yaml:"template"`
}

// PromptConfig is the read-only view of a loaded prompt template.
type PromptConfig struct {
	Name        string
	Version     string
	Description string
	Variables   map[string]string
	Template    string
}

// PromptRegistry holds all embedded prompt templates keyed by name.
type PromptRegistry struct {
	entries map[string]PromptConfig
}

// singleton registry (loaded once from embedded YAML).
var (
	defaultRegistry     *PromptRegistry
	defaultRegistryOnce sync.Once
	defaultRegistryErr  error
)

// Default returns the package-level PromptRegistry singleton.
// It is loaded once from embedded YAML files and safe for concurrent use.
func Default() (*PromptRegistry, error) {
	defaultRegistryOnce.Do(func() {
		defaultRegistry, defaultRegistryErr = NewRegistry()
	})
	return defaultRegistry, defaultRegistryErr
}

// MustDefault returns the package-level PromptRegistry singleton,
// panicking if the embedded YAML files cannot be loaded.
func MustDefault() *PromptRegistry {
	r, err := Default()
	if err != nil {
		panic("filter.MustDefault: " + err.Error())
	}
	return r
}

// NewRegistry loads all YAML files from the embedded prompts/ directory
// and returns a ready-to-use PromptRegistry.
func NewRegistry() (*PromptRegistry, error) {
	return NewRegistryFromFS(promptsFS)
}

// NewRegistryFromFS is the testable constructor that accepts any fs.FS.
func NewRegistryFromFS(fsys fs.FS) (*PromptRegistry, error) {
	r := &PromptRegistry{entries: make(map[string]PromptConfig)}

	entries, err := fs.ReadDir(fsys, "prompts")
	if err != nil {
		return nil, fmt.Errorf("read prompts dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		data, err := fs.ReadFile(fsys, "prompts/"+entry.Name())
		if err != nil {
			return nil, fmt.Errorf("read prompt %s: %w", entry.Name(), err)
		}

		var pf promptFile
		if err := yaml.Unmarshal(data, &pf); err != nil {
			return nil, fmt.Errorf("parse prompt %s: %w", entry.Name(), err)
		}

		if pf.Name == "" {
			return nil, fmt.Errorf("prompt %s: missing name field", entry.Name())
		}

		if pf.Template == "" {
			return nil, fmt.Errorf("prompt %s: missing template field", entry.Name())
		}

		if _, exists := r.entries[pf.Name]; exists {
			return nil, fmt.Errorf("duplicate prompt name %q in %s", pf.Name, entry.Name())
		}

		r.entries[pf.Name] = PromptConfig{
			Name:        pf.Name,
			Version:     pf.Version,
			Description: pf.Description,
			Variables:   pf.Variables,
			Template:    pf.Template,
		}
	}

	if len(r.entries) == 0 {
		return nil, fmt.Errorf("load prompt registry: no prompt files found")
	}

	return r, nil
}

// Get returns the PromptConfig for the given name.
// Returns an error if the prompt is not found.
func (r *PromptRegistry) Get(name string) (PromptConfig, error) {
	def, ok := r.entries[name]
	if !ok {
		return PromptConfig{}, fmt.Errorf("prompt %q not found", name)
	}
	return def, nil
}

// Expand retrieves the prompt template by name and replaces {key}
// placeholders with the corresponding values from vars.
// Unknown keys in the template are left as-is.
// Returns an error if the prompt name is not found.
func (r *PromptRegistry) Expand(name string, vars map[string]string) (string, error) {
	def, err := r.Get(name)
	if err != nil {
		return "", err
	}
	return ExpandTemplate(def.Template, vars), nil
}

// MustExpand is like Expand but panics on error.
func (r *PromptRegistry) MustExpand(name string, vars map[string]string) string {
	result, err := r.Expand(name, vars)
	if err != nil {
		panic("prompt expand " + name + ": " + err.Error())
	}
	return result
}

// Names returns all registered prompt names in sorted order.
func (r *PromptRegistry) Names() []string {
	names := make([]string, 0, len(r.entries))
	for name := range r.entries {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// ExpandTemplate performs simple {key} replacement on a template string.
func ExpandTemplate(tmpl string, vars map[string]string) string {
	result := processConditionals(tmpl, vars)
	const sentinel = "\x00PROMPT_VAR_"
	for k := range vars {
		result = strings.ReplaceAll(result, "{"+k+"}", sentinel+k+"\x00")
	}
	for k, v := range vars {
		result = strings.ReplaceAll(result, sentinel+k+"\x00", v)
	}
	return result
}

// processConditionals handles {#if key}...{#else}...{/if} blocks.
func processConditionals(tmpl string, vars map[string]string) string {
	for {
		start := strings.Index(tmpl, "{#if ")
		if start == -1 {
			return tmpl
		}
		closeTag := strings.Index(tmpl[start:], "}")
		if closeTag == -1 {
			return tmpl
		}
		key := tmpl[start+len("{#if ") : start+closeTag]
		endTag := "{/if}"
		endIdx := strings.Index(tmpl[start:], endTag)
		if endIdx == -1 {
			return tmpl
		}
		endIdx += start
		body := tmpl[start+closeTag+1 : endIdx]
		var ifBlock, elseBlock string
		if elseIdx := strings.Index(body, "{#else}"); elseIdx != -1 {
			ifBlock = body[:elseIdx]
			elseBlock = body[elseIdx+len("{#else}"):]
		} else {
			ifBlock = body
		}
		val, exists := vars[key]
		truthy := exists && val != "" && val != "false"
		var replacement string
		if truthy {
			replacement = ifBlock
		} else {
			replacement = elseBlock
		}
		tmpl = tmpl[:start] + replacement + tmpl[endIdx+len(endTag):]
	}
}
