package session_test

import (
	"strings"
	"testing"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/session"
)

func TestFormatADRConflictSection_WithConflicts(t *testing.T) {
	// given: a ScribeResponse with conflicts
	resp := &domain.ScribeResponse{
		ADRID:   "0005",
		Title:   "Use Redis for caching",
		Content: "# 0005. Use Redis for caching\n\n**Status:** Accepted\n",
		Conflicts: []domain.ADRConflict{
			{ExistingADRID: "0003", Description: "contradicts in-memory cache decision"},
			{ExistingADRID: "0004", Description: "conflicts with cache invalidation strategy"},
		},
	}

	// when
	section := session.FormatADRConflictSection(resp)

	// then
	if section == "" {
		t.Fatal("FormatADRConflictSection: expected non-empty section for response with conflicts")
	}
	if !strings.Contains(section, "0003") {
		t.Errorf("FormatADRConflictSection: expected section to reference ADR 0003, got:\n%s", section)
	}
	if !strings.Contains(section, "contradicts in-memory cache decision") {
		t.Errorf("FormatADRConflictSection: expected section to contain conflict description, got:\n%s", section)
	}
	if !strings.Contains(section, "0004") {
		t.Errorf("FormatADRConflictSection: expected section to reference ADR 0004, got:\n%s", section)
	}
}

func TestFormatADRConflictSection_NoConflicts(t *testing.T) {
	// given: a ScribeResponse with no conflicts
	resp := &domain.ScribeResponse{
		ADRID:     "0006",
		Title:     "Use PostgreSQL",
		Content:   "# 0006. Use PostgreSQL\n\n**Status:** Accepted\n",
		Conflicts: nil,
	}

	// when
	section := session.FormatADRConflictSection(resp)

	// then: empty section for no conflicts
	if section != "" {
		t.Errorf("FormatADRConflictSection: expected empty section for no conflicts, got %q", section)
	}
}

func TestFormatADRConflictSection_AppendedToContent(t *testing.T) {
	// given: a ScribeResponse with one conflict
	resp := &domain.ScribeResponse{
		ADRID:   "0007",
		Title:   "Switch to gRPC",
		Content: "# 0007. Switch to gRPC\n\n**Status:** Accepted\n\n## Context\n\nContext here.\n",
		Conflicts: []domain.ADRConflict{
			{ExistingADRID: "0002", Description: "REST-first approach was decided in 0002"},
		},
	}

	// when
	section := session.FormatADRConflictSection(resp)
	combined := resp.Content + section

	// then: combined output contains both original content and conflict section
	if !strings.Contains(combined, "Switch to gRPC") {
		t.Error("combined: expected original ADR title to remain")
	}
	if !strings.Contains(combined, "0002") {
		t.Error("combined: expected conflict section with ADR 0002")
	}
}
