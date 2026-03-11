package session

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

// NormalizeJSONFile reads a JSON file, re-encodes it with raw UTF-8
// (no \uXXXX for non-ASCII characters), and writes it back.
// This normalizes Claude's output so that human-readable characters
// (e.g. Japanese) appear as raw UTF-8 instead of escape sequences.
//
// It also handles non-JSON text wrapping that Claude may produce:
// markdown code blocks (```json ... ```) and natural language
// prefixes/suffixes around JSON content.
func NormalizeJSONFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("normalize read: %w", err)
	}
	cleaned := stripMarkdownCodeBlock(data)
	var v any
	if err := json.Unmarshal(cleaned, &v); err != nil {
		// Fallback: extract JSON object from mixed text
		extracted := extractJSON(cleaned)
		if err2 := json.Unmarshal(extracted, &v); err2 != nil {
			return fmt.Errorf("normalize unmarshal: %w", err)
		}
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

// stripMarkdownCodeBlock removes markdown code block wrappers (```json ... ```)
// from data. Claude may wrap JSON output in markdown fences.
func stripMarkdownCodeBlock(data []byte) []byte {
	trimmed := bytes.TrimSpace(data)
	if !bytes.HasPrefix(trimmed, []byte("```")) {
		return trimmed
	}
	if idx := bytes.IndexByte(trimmed, '\n'); idx >= 0 {
		trimmed = trimmed[idx+1:]
	} else {
		return trimmed
	}
	if idx := bytes.LastIndex(trimmed, []byte("```")); idx >= 0 {
		trimmed = trimmed[:idx]
	}
	return bytes.TrimSpace(trimmed)
}

// extractJSON finds the first top-level JSON object ({...}) in data by
// scanning for the opening brace and matching it with the closing brace.
// Handles cases where Claude wraps JSON in natural language text.
func extractJSON(data []byte) []byte {
	start := bytes.IndexByte(data, '{')
	if start < 0 {
		return data
	}
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(data); i++ {
		if escaped {
			escaped = false
			continue
		}
		c := data[i]
		if c == '\\' && inString {
			escaped = true
			continue
		}
		if c == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		switch c {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return data[start : i+1]
			}
		}
	}
	return data[start:]
}
