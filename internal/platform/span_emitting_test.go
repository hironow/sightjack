package platform_test

import (
	"testing"

	"github.com/hironow/sightjack/internal/platform"
)

func TestTruncateValue_short_string(t *testing.T) {
	result := platform.TruncateValue("hello", 512)
	if result != "hello" {
		t.Errorf("got %q, want %q", result, "hello")
	}
}

func TestTruncateValue_exact_limit(t *testing.T) {
	result := platform.TruncateValue("abc", 3)
	if result != "abc" {
		t.Errorf("got %q, want %q", result, "abc")
	}
}

func TestTruncateValue_over_limit(t *testing.T) {
	result := platform.TruncateValue("abcdef", 3)
	if result != "abc..." {
		t.Errorf("got %q, want %q", result, "abc...")
	}
}

func TestTruncateValue_zero_limit(t *testing.T) {
	result := platform.TruncateValue("hello", 0)
	if result != "..." {
		t.Errorf("got %q, want %q", result, "...")
	}
}

func TestFormatRawEvent(t *testing.T) {
	result := platform.FormatRawEvent("tool_use", `{"name":"Read","id":"toolu_01X"}`, 512)
	want := `tool_use:{"name":"Read","id":"toolu_01X"}`
	if result != want {
		t.Errorf("got %q, want %q", result, want)
	}
}

func TestFormatRawEvent_truncates_long_json(t *testing.T) {
	result := platform.FormatRawEvent("tool_result", "abcdefghij", 5)
	want := "tool_result:abcde..."
	if result != want {
		t.Errorf("got %q, want %q", result, want)
	}
}

func TestSyntheticToolID(t *testing.T) {
	if id := platform.SyntheticToolID(0); id != "synthetic-0" {
		t.Errorf("got %q, want synthetic-0", id)
	}
	if id := platform.SyntheticToolID(42); id != "synthetic-42" {
		t.Errorf("got %q, want synthetic-42", id)
	}
}
