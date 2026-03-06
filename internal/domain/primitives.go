package domain

import "fmt"

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
