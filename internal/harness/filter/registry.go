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

	"gopkg.in/yaml.v3"
)

//go:embed prompts/*.yaml
var promptFS embed.FS

// PromptDefinition represents a single prompt loaded from a YAML file.
type PromptDefinition struct {
	Name        string            `yaml:"name"`
	Version     string            `yaml:"version"`
	Description string            `yaml:"description"`
	Variables   map[string]string `yaml:"variables"`
	Template    string            `yaml:"template"`
}

// Registry loads and serves prompt definitions from embedded YAML files.
type Registry struct {
	prompts map[string]PromptDefinition
}

// NewRegistry parses all YAML files in prompts/ and returns a Registry.
// It returns an error if any YAML file cannot be parsed or if duplicate
// prompt names are detected.
func NewRegistry() (*Registry, error) {
	return NewRegistryFromFS(promptFS)
}

// MustNewRegistry returns a Registry or panics. Safe with embed.FS.
func MustNewRegistry() *Registry {
	r, err := NewRegistry()
	if err != nil {
		panic("prompt registry: " + err.Error())
	}
	return r
}

// NewRegistryFromFS is the testable constructor that accepts any fs.FS.
func NewRegistryFromFS(fsys fs.FS) (*Registry, error) {
	r := &Registry{prompts: make(map[string]PromptDefinition)}

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

		var def PromptDefinition
		if err := yaml.Unmarshal(data, &def); err != nil {
			return nil, fmt.Errorf("parse prompt %s: %w", entry.Name(), err)
		}

		if def.Name == "" {
			return nil, fmt.Errorf("prompt %s: missing name field", entry.Name())
		}

		if _, exists := r.prompts[def.Name]; exists {
			return nil, fmt.Errorf("duplicate prompt name %q in %s", def.Name, entry.Name())
		}

		r.prompts[def.Name] = def
	}

	return r, nil
}

// Get returns the PromptDefinition for the given name.
// Returns an error if the prompt is not found.
func (r *Registry) Get(name string) (PromptDefinition, error) {
	def, ok := r.prompts[name]
	if !ok {
		return PromptDefinition{}, fmt.Errorf("prompt %q not found", name)
	}
	return def, nil
}

// Expand retrieves the prompt template by name and replaces {key}
// placeholders with the corresponding values from vars.
// Unknown keys in the template are left as-is.
// Returns an error if the prompt name is not found.
func (r *Registry) Expand(name string, vars map[string]string) (string, error) {
	def, err := r.Get(name)
	if err != nil {
		return "", err
	}
	return ExpandTemplate(def.Template, vars), nil
}

// MustExpand is like Expand but panics on error.
func (r *Registry) MustExpand(name string, vars map[string]string) string {
	result, err := r.Expand(name, vars)
	if err != nil {
		panic("prompt expand " + name + ": " + err.Error())
	}
	return result
}

// Names returns all registered prompt names in sorted order.
func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.prompts))
	for name := range r.prompts {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// ExpandTemplate performs simple {key} replacement on a template string.
func ExpandTemplate(tmpl string, vars map[string]string) string {
	result := processConditionals(tmpl, vars)
	for key, val := range vars {
		result = strings.ReplaceAll(result, "{"+key+"}", val)
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
