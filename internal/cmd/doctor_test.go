package cmd

// white-box-reason: cobra command construction: NewRootCommand and CLI routing are unexported

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/hironow/sightjack/internal/domain"
)

func TestPrintDoctorJSON_Parseable(t *testing.T) {
	// given
	results := []domain.DoctorCheck{
		{Name: "config", Status: domain.CheckOK, Message: "valid"},
		{Name: "claude", Status: domain.CheckFail, Message: "not found", Hint: "install claude"},
		{Name: "linear", Status: domain.CheckSkip, Message: "no API key"},
	}

	// when
	var buf bytes.Buffer
	_ = printDoctorJSON(&buf, results) // returns error because of CheckFail

	// then — must be valid JSON
	var parsed struct {
		Checks []doctorJSONCheck `json:"checks"`
	}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if len(parsed.Checks) != 3 {
		t.Fatalf("checks = %d, want 3", len(parsed.Checks))
	}
	if parsed.Checks[0].Status != "OK" {
		t.Errorf("checks[0].status = %q, want OK", parsed.Checks[0].Status)
	}
	if parsed.Checks[1].Hint != "install claude" {
		t.Errorf("checks[1].hint = %q, want 'install claude'", parsed.Checks[1].Hint)
	}
}

func TestPrintDoctorJSON_FailReturnsError(t *testing.T) {
	// given — results with a failure
	results := []domain.DoctorCheck{
		{Name: "claude", Status: domain.CheckFail, Message: "not found"},
	}

	// when
	var buf bytes.Buffer
	err := printDoctorJSON(&buf, results)

	// then
	if err == nil {
		t.Error("expected error when checks fail")
	}
}

func TestPrintDoctorJSON_NoFailNoError(t *testing.T) {
	// given — all passing
	results := []domain.DoctorCheck{
		{Name: "config", Status: domain.CheckOK, Message: "valid"},
	}

	// when
	var buf bytes.Buffer
	err := printDoctorJSON(&buf, results)

	// then
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
