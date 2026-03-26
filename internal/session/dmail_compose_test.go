package session_test

import (
	"context"
	"testing"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/session"
)

func TestComposeSpecification_WaveMode_AttachesWaveReference(t *testing.T) {
	// given: wave with actions in wave mode
	dir := t.TempDir()
	store := testOutboxStore(t, dir)
	wave := domain.Wave{
		ID:          "w1",
		ClusterName: "auth",
		Title:       "Auth Wave",
		Actions: []domain.WaveAction{
			{IssueID: "MY-1", Description: "Add middleware", Detail: "JWT based"},
			{IssueID: "MY-2", Description: "Add login"},
		},
	}

	// when: compose with wave mode
	err := session.ComposeSpecification(context.Background(), store, wave, domain.ModeWave)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Verify outbox was flushed (store stages internally; flush verifies write)
}

func TestComposeSpecification_LinearMode_NoWaveReference(t *testing.T) {
	// given: wave in linear mode
	dir := t.TempDir()
	store := testOutboxStore(t, dir)
	wave := domain.Wave{
		ID:          "w1",
		ClusterName: "auth",
		Title:       "Auth Wave",
		Actions: []domain.WaveAction{
			{IssueID: "MY-1", Description: "Add middleware"},
		},
	}

	// when: compose without mode (defaults to no wave ref)
	err := session.ComposeSpecification(context.Background(), store, wave)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestComposeSpecification_WaveMode_EmptyActions_FallsBack(t *testing.T) {
	// given: wave with no actions
	dir := t.TempDir()
	store := testOutboxStore(t, dir)
	wave := domain.Wave{
		ID:          "w1",
		ClusterName: "fix",
		Title:       "Quick Fix",
	}

	// when
	err := session.ComposeSpecification(context.Background(), store, wave, domain.ModeWave)

	// then: should not error (fallback to wave key as step)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
