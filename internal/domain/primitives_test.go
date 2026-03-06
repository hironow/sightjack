package domain_test

import (
	"testing"

	"github.com/hironow/sightjack/internal/domain"
)

func TestNewRepoPath_Valid(t *testing.T) {
	rp, err := domain.NewRepoPath("/tmp/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rp.String() != "/tmp/repo" {
		t.Errorf("expected /tmp/repo, got %q", rp.String())
	}
}

func TestNewRepoPath_RejectsEmpty(t *testing.T) {
	_, err := domain.NewRepoPath("")
	if err == nil {
		t.Fatal("expected error for empty RepoPath")
	}
}

func TestNewSessionID_Valid(t *testing.T) {
	sid, err := domain.NewSessionID("session-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sid.String() != "session-123" {
		t.Errorf("expected session-123, got %q", sid.String())
	}
}

func TestNewSessionID_RejectsEmpty(t *testing.T) {
	_, err := domain.NewSessionID("")
	if err == nil {
		t.Fatal("expected error for empty SessionID")
	}
}

func TestNewClusterName_Valid(t *testing.T) {
	cn, err := domain.NewClusterName("C1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cn.String() != "C1" {
		t.Errorf("expected C1, got %q", cn.String())
	}
}

func TestNewClusterName_RejectsEmpty(t *testing.T) {
	_, err := domain.NewClusterName("")
	if err == nil {
		t.Fatal("expected error for empty ClusterName")
	}
}

func TestNewTopic_Valid(t *testing.T) {
	tp, err := domain.NewTopic("design question")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tp.String() != "design question" {
		t.Errorf("expected 'design question', got %q", tp.String())
	}
}

func TestNewTopic_RejectsEmpty(t *testing.T) {
	_, err := domain.NewTopic("")
	if err == nil {
		t.Fatal("expected error for empty Topic")
	}
}

func TestNewLang_Valid(t *testing.T) {
	for _, lang := range []string{"ja", "en"} {
		l, err := domain.NewLang(lang)
		if err != nil {
			t.Fatalf("unexpected error for %q: %v", lang, err)
		}
		if l.String() != lang {
			t.Errorf("expected %q, got %q", lang, l.String())
		}
	}
}

func TestNewLang_RejectsInvalid(t *testing.T) {
	for _, lang := range []string{"", "jp", "fr", "invalid"} {
		_, err := domain.NewLang(lang)
		if err == nil {
			t.Errorf("expected error for invalid lang %q", lang)
		}
	}
}
