package sightjack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

// normalizeJSONFile reads a JSON file, re-encodes it with raw UTF-8
// (no \uXXXX for non-ASCII characters), and writes it back.
// This normalizes Claude's output so that human-readable characters
// (e.g. Japanese) appear as raw UTF-8 instead of escape sequences.
func normalizeJSONFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("normalize read: %w", err)
	}
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return fmt.Errorf("normalize unmarshal: %w", err)
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("normalize encode: %w", err)
	}
	return os.WriteFile(path, buf.Bytes(), 0644)
}
