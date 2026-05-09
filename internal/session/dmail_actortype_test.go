package session_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/platform/actortype"
	"github.com/hironow/sightjack/internal/session"
)

func TestComposeDMail_EmitsActorType_Env(t *testing.T) {
	// given
	t.Setenv("RUNOPS_ACTOR_TYPE", "ai-agent")
	t.Setenv("RUNOPS_INITIATING_ACTOR_TYPE", "")

	dir := t.TempDir()
	if err := session.EnsureMailDirs(dir); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	store := testOutboxStore(t, dir)
	mail := &domain.DMail{
		Name:          "sj-spec-actortype-env_00000000",
		Kind:          domain.KindSpecification,
		Description:   "with actor type",
		SchemaVersion: "1",
		Body:          "# DoD\n- item 1\n",
	}

	// when
	if err := session.ComposeDMail(context.Background(), store, mail); err != nil {
		t.Fatalf("compose: %v", err)
	}

	// then
	outboxPath := filepath.Join(domain.MailDir(dir, "outbox"), mail.Filename())
	data, err := os.ReadFile(outboxPath)
	if err != nil {
		t.Fatalf("read outbox: %v", err)
	}
	parsed, err := session.ParseDMail(data)
	if err != nil {
		t.Fatalf("parse outbox: %v", err)
	}
	if parsed.Metadata["requester_actor_type"] != "ai-agent" {
		t.Errorf("requester_actor_type: got %q, want ai-agent", parsed.Metadata["requester_actor_type"])
	}
	if parsed.Metadata["requester_actor_source"] != "env" {
		t.Errorf("requester_actor_source: got %q, want env", parsed.Metadata["requester_actor_source"])
	}
	if _, ok := parsed.Metadata["initiating_actor_type"]; ok {
		t.Errorf("non-daemon actor must not carry initiating_actor_type")
	}
}

func TestComposeDMail_EmitsActorType_DaemonWithInitiating(t *testing.T) {
	// given
	t.Setenv("RUNOPS_ACTOR_TYPE", "workspace-daemon")
	t.Setenv("RUNOPS_INITIATING_ACTOR_TYPE", "human-operator")

	dir := t.TempDir()
	if err := session.EnsureMailDirs(dir); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	store := testOutboxStore(t, dir)
	mail := &domain.DMail{
		Name:          "sj-spec-actortype-daemon_00000000",
		Kind:          domain.KindSpecification,
		Description:   "daemon-driven",
		SchemaVersion: "1",
		Body:          "# DoD\n",
	}

	// when
	if err := session.ComposeDMail(context.Background(), store, mail); err != nil {
		t.Fatalf("compose: %v", err)
	}

	// then
	outboxPath := filepath.Join(domain.MailDir(dir, "outbox"), mail.Filename())
	data, err := os.ReadFile(outboxPath)
	if err != nil {
		t.Fatalf("read outbox: %v", err)
	}
	parsed, err := session.ParseDMail(data)
	if err != nil {
		t.Fatalf("parse outbox: %v", err)
	}
	if parsed.Metadata["requester_actor_type"] != "workspace-daemon" {
		t.Errorf("requester_actor_type: got %q, want workspace-daemon", parsed.Metadata["requester_actor_type"])
	}
	if parsed.Metadata["initiating_actor_type"] != "human-operator" {
		t.Errorf("initiating_actor_type: got %q, want human-operator", parsed.Metadata["initiating_actor_type"])
	}
}

func TestComposeDMail_NoActorType_LegacyCompat(t *testing.T) {
	// given — env unset (legacy compat path)
	t.Setenv("RUNOPS_ACTOR_TYPE", "")

	dir := t.TempDir()
	if err := session.EnsureMailDirs(dir); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	store := testOutboxStore(t, dir)
	mail := &domain.DMail{
		Name:          "sj-spec-actortype-legacy_00000000",
		Kind:          domain.KindSpecification,
		Description:   "legacy compat",
		SchemaVersion: "1",
		Body:          "# DoD\n",
	}

	// when
	if err := session.ComposeDMail(context.Background(), store, mail); err != nil {
		t.Fatalf("compose: %v", err)
	}

	// then
	outboxPath := filepath.Join(domain.MailDir(dir, "outbox"), mail.Filename())
	data, err := os.ReadFile(outboxPath)
	if err != nil {
		t.Fatalf("read outbox: %v", err)
	}
	parsed, err := session.ParseDMail(data)
	if err != nil {
		t.Fatalf("parse outbox: %v", err)
	}
	if v, ok := parsed.Metadata["requester_actor_type"]; ok {
		t.Errorf("requester_actor_type must be absent in legacy compat path, got %q", v)
	}
	if v, ok := parsed.Metadata["requester_actor_source"]; ok {
		t.Errorf("requester_actor_source must be absent in legacy compat path, got %q", v)
	}
}

func TestComposeDMail_InvalidEnv_FailsEmit(t *testing.T) {
	// given
	t.Setenv("RUNOPS_ACTOR_TYPE", "robot")

	dir := t.TempDir()
	if err := session.EnsureMailDirs(dir); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	store := testOutboxStore(t, dir)
	mail := &domain.DMail{
		Name:          "sj-spec-actortype-invalid_00000000",
		Kind:          domain.KindSpecification,
		Description:   "invalid env",
		SchemaVersion: "1",
		Body:          "# DoD\n",
	}

	// when
	err := session.ComposeDMail(context.Background(), store, mail)

	// then
	if err == nil {
		t.Fatal("expected error for invalid RUNOPS_ACTOR_TYPE env, got nil")
	}
	if !errors.Is(err, actortype.ErrInvalidActorType) {
		t.Errorf("expected ErrInvalidActorType wrapped, got %v", err)
	}

	// outbox MUST NOT have the file (silent escalation prevention).
	outboxPath := filepath.Join(domain.MailDir(dir, "outbox"), mail.Filename())
	if _, statErr := os.Stat(outboxPath); statErr == nil {
		t.Errorf("outbox file must not exist after emit fail, but %s does", outboxPath)
	}
}

func TestComposeDMail_DaemonInvalidInitiating_FailsEmit(t *testing.T) {
	// given
	t.Setenv("RUNOPS_ACTOR_TYPE", "workspace-daemon")
	t.Setenv("RUNOPS_INITIATING_ACTOR_TYPE", "robot")

	dir := t.TempDir()
	if err := session.EnsureMailDirs(dir); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	store := testOutboxStore(t, dir)
	mail := &domain.DMail{
		Name:          "sj-spec-actortype-daemoninvalid_00000000",
		Kind:          domain.KindSpecification,
		Description:   "daemon with invalid initiating",
		SchemaVersion: "1",
		Body:          "# DoD\n",
	}

	// when
	err := session.ComposeDMail(context.Background(), store, mail)

	// then
	if err == nil {
		t.Fatal("expected error for invalid RUNOPS_INITIATING_ACTOR_TYPE env, got nil")
	}
	if !errors.Is(err, actortype.ErrInvalidInitiatingActorType) {
		t.Errorf("expected ErrInvalidInitiatingActorType wrapped, got %v", err)
	}

	outboxPath := filepath.Join(domain.MailDir(dir, "outbox"), mail.Filename())
	if _, statErr := os.Stat(outboxPath); statErr == nil {
		t.Errorf("outbox file must not exist after emit fail, but %s does", outboxPath)
	}
}
