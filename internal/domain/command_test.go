package domain_test

import (
	"testing"

	"github.com/hironow/sightjack/internal/domain"
)

func TestNewInitCommand(t *testing.T) {
	rp, _ := domain.NewRepoPath("/tmp/repo")
	cmd := domain.NewInitCommand(rp, "Eng", "Hades", "en", "alert")

	if cmd.BaseDir().String() != "/tmp/repo" {
		t.Errorf("expected /tmp/repo, got %q", cmd.BaseDir().String())
	}
	if cmd.Team() != "Eng" {
		t.Errorf("expected Eng, got %q", cmd.Team())
	}
	if cmd.Project() != "Hades" {
		t.Errorf("expected Hades, got %q", cmd.Project())
	}
	if cmd.Lang() != "en" {
		t.Errorf("expected en, got %q", cmd.Lang())
	}
	if cmd.Strictness() != "alert" {
		t.Errorf("expected alert, got %q", cmd.Strictness())
	}
}

func TestNewRunScanCommand(t *testing.T) {
	rp, _ := domain.NewRepoPath("/tmp/repo")
	cmd := domain.NewRunScanCommand(rp, true)

	if cmd.RepoPath().String() != "/tmp/repo" {
		t.Errorf("expected /tmp/repo, got %q", cmd.RepoPath().String())
	}
	if !cmd.DryRun() {
		t.Error("expected DryRun to be true")
	}
}

func TestNewRunSessionCommand(t *testing.T) {
	rp, _ := domain.NewRepoPath("/tmp/repo")
	cmd := domain.NewRunSessionCommand(rp, false)

	if cmd.RepoPath().String() != "/tmp/repo" {
		t.Errorf("expected /tmp/repo, got %q", cmd.RepoPath().String())
	}
	if cmd.DryRun() {
		t.Error("expected DryRun to be false")
	}
}

func TestNewResumeSessionCommand(t *testing.T) {
	rp, _ := domain.NewRepoPath("/tmp/repo")
	sid, _ := domain.NewSessionID("session-123")
	cmd := domain.NewResumeSessionCommand(rp, sid)

	if cmd.RepoPath().String() != "/tmp/repo" {
		t.Errorf("expected /tmp/repo, got %q", cmd.RepoPath().String())
	}
	if cmd.SessionID().String() != "session-123" {
		t.Errorf("expected session-123, got %q", cmd.SessionID().String())
	}
}

func TestNewApplyWaveCommand(t *testing.T) {
	rp, _ := domain.NewRepoPath("/tmp/repo")
	sid, _ := domain.NewSessionID("session-123")
	cn, _ := domain.NewClusterName("C1")
	cmd := domain.NewApplyWaveCommand(rp, sid, cn)

	if cmd.RepoPath().String() != "/tmp/repo" {
		t.Errorf("expected /tmp/repo, got %q", cmd.RepoPath().String())
	}
	if cmd.SessionID().String() != "session-123" {
		t.Errorf("expected session-123, got %q", cmd.SessionID().String())
	}
	if cmd.ClusterName().String() != "C1" {
		t.Errorf("expected C1, got %q", cmd.ClusterName().String())
	}
}

func TestNewDiscussWaveCommand(t *testing.T) {
	rp, _ := domain.NewRepoPath("/tmp/repo")
	sid, _ := domain.NewSessionID("session-123")
	cn, _ := domain.NewClusterName("C1")
	tp, _ := domain.NewTopic("design question")
	cmd := domain.NewDiscussWaveCommand(rp, sid, cn, tp)

	if cmd.RepoPath().String() != "/tmp/repo" {
		t.Errorf("expected /tmp/repo, got %q", cmd.RepoPath().String())
	}
	if cmd.SessionID().String() != "session-123" {
		t.Errorf("expected session-123, got %q", cmd.SessionID().String())
	}
	if cmd.ClusterName().String() != "C1" {
		t.Errorf("expected C1, got %q", cmd.ClusterName().String())
	}
	if cmd.Topic().String() != "design question" {
		t.Errorf("expected 'design question', got %q", cmd.Topic().String())
	}
}
