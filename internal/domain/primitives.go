package domain

import "fmt"

// TrackingMode determines the issue tracking backend.
// Wave mode (default) uses D-Mail archive as event source.
// Linear mode uses Linear MCP for issue tracking (legacy).
type TrackingMode string

const (
	// ModeWave is the default mode: waves and steps drive expedition targeting.
	// D-Mail archive/ is the single source of truth for wave state.
	ModeWave TrackingMode = "wave"

	// ModeLinear uses Linear MCP for issue tracking (existing behavior).
	ModeLinear TrackingMode = "linear"
)

// NewTrackingMode returns ModeLinear when linear is true, ModeWave otherwise.
func NewTrackingMode(linear bool) TrackingMode {
	if linear {
		return ModeLinear
	}
	return ModeWave
}

// IsLinear returns true when operating in Linear MCP mode.
func (m TrackingMode) IsLinear() bool { return m == ModeLinear }

// IsWave returns true when operating in Wave-centric mode.
func (m TrackingMode) IsWave() bool { return m == ModeWave }

// String returns the mode name.
func (m TrackingMode) String() string { return string(m) }

// RepoPath is an always-valid, non-empty repository path.
type RepoPath struct{ v string }

func NewRepoPath(raw string) (RepoPath, error) {
	if raw == "" {
		return RepoPath{}, fmt.Errorf("RepoPath is required")
	}
	return RepoPath{v: raw}, nil
}

func (r RepoPath) String() string { return r.v }

// SessionID is an always-valid, non-empty session identifier.
type SessionID struct{ v string }

func NewSessionID(raw string) (SessionID, error) {
	if raw == "" {
		return SessionID{}, fmt.Errorf("SessionID is required")
	}
	return SessionID{v: raw}, nil
}

func (s SessionID) String() string { return s.v }

// ClusterName is an always-valid, non-empty cluster name.
type ClusterName struct{ v string }

func NewClusterName(raw string) (ClusterName, error) {
	if raw == "" {
		return ClusterName{}, fmt.Errorf("ClusterName is required")
	}
	return ClusterName{v: raw}, nil
}

func (c ClusterName) String() string { return c.v }

// Topic is an always-valid, non-empty discussion topic.
type Topic struct{ v string }

func NewTopic(raw string) (Topic, error) {
	if raw == "" {
		return Topic{}, fmt.Errorf("Topic is required")
	}
	return Topic{v: raw}, nil
}

func (t Topic) String() string { return t.v }

// Lang is an always-valid language code ("ja" or "en").
type Lang struct{ v string }

func NewLang(raw string) (Lang, error) {
	if !ValidLang(raw) {
		return Lang{}, fmt.Errorf("invalid lang %q (valid: ja, en)", raw)
	}
	return Lang{v: raw}, nil
}

func (l Lang) String() string { return l.v }
