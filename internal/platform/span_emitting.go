package platform

import "fmt"

// DefaultMaxValueLen is the default truncation limit for raw event values.
const DefaultMaxValueLen = 512

// TruncateValue truncates s to maxLen characters, appending "..." if truncated.
func TruncateValue(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// FormatRawEvent formats a stream event as "type:truncated_json" for storage
// in the stream.raw_events span attribute.
func FormatRawEvent(eventType, jsonData string, maxValueLen int) string {
	return eventType + ":" + TruncateValue(jsonData, maxValueLen)
}

// SyntheticToolID generates a fallback tool ID when the stream does not provide one.
func SyntheticToolID(seq int) string {
	return fmt.Sprintf("synthetic-%d", seq)
}
