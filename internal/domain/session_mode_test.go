package domain_test

import (
	"testing"

	"github.com/hironow/sightjack/internal/domain"
)

func TestParseSessionMode(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    domain.ResumeChoice
		wantErr bool
	}{
		{name: "resume", input: "resume", want: domain.ResumeChoiceResume},
		{name: "new", input: "new", want: domain.ResumeChoiceNew},
		{name: "rescan", input: "rescan", want: domain.ResumeChoiceRescan},
		{name: "empty returns error", input: "", wantErr: true},
		{name: "invalid returns error", input: "invalid", wantErr: true},
		{name: "uppercase is rejected", input: "Resume", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domain.ParseSessionMode(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseSessionMode(%q) expected error, got %v", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseSessionMode(%q) unexpected error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("ParseSessionMode(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
