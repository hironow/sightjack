package sightjack

import "fmt"

// RunScanCommand represents the intent to run a sightjack scan.
// Independent of cobra — framework concerns are separated at the cmd layer.
type RunScanCommand struct {
	RepoPath   string
	Lang       string
	Strictness string
	DryRun     bool
}

// Validate checks that the command has valid required fields.
func (c *RunScanCommand) Validate() []error {
	var errs []error
	if c.RepoPath == "" {
		errs = append(errs, fmt.Errorf("RepoPath is required"))
	}
	if c.Lang != "" && !ValidLang(c.Lang) {
		errs = append(errs, fmt.Errorf("invalid lang %q (valid: ja, en)", c.Lang))
	}
	if c.Strictness != "" {
		if _, err := ParseStrictnessLevel(c.Strictness); err != nil {
			errs = append(errs, fmt.Errorf("invalid strictness %q", c.Strictness))
		}
	}
	return errs
}

// RunSessionCommand represents the intent to start an interactive session.
type RunSessionCommand struct {
	RepoPath    string
	DryRun      bool
	AutoApprove bool
	NotifyCmd   string
	ApproveCmd  string
	ReviewCmd   string
}

// Validate checks that the command has valid required fields.
func (c *RunSessionCommand) Validate() []error {
	var errs []error
	if c.RepoPath == "" {
		errs = append(errs, fmt.Errorf("RepoPath is required"))
	}
	return errs
}

// ResumeSessionCommand represents the intent to resume an existing session.
type ResumeSessionCommand struct {
	RepoPath  string
	SessionID string
}

// Validate checks that the command has valid required fields.
func (c *ResumeSessionCommand) Validate() []error {
	var errs []error
	if c.RepoPath == "" {
		errs = append(errs, fmt.Errorf("RepoPath is required"))
	}
	if c.SessionID == "" {
		errs = append(errs, fmt.Errorf("SessionID is required"))
	}
	return errs
}

// ApplyWaveCommand represents the intent to approve and apply a wave.
type ApplyWaveCommand struct {
	RepoPath    string
	SessionID   string
	ClusterName string
}

// Validate checks that the command has valid required fields.
func (c *ApplyWaveCommand) Validate() []error {
	var errs []error
	if c.RepoPath == "" {
		errs = append(errs, fmt.Errorf("RepoPath is required"))
	}
	if c.SessionID == "" {
		errs = append(errs, fmt.Errorf("SessionID is required"))
	}
	if c.ClusterName == "" {
		errs = append(errs, fmt.Errorf("ClusterName is required"))
	}
	return errs
}

// DiscussWaveCommand represents the intent to discuss a specific wave topic.
type DiscussWaveCommand struct {
	RepoPath    string
	SessionID   string
	ClusterName string
	Topic       string
}

// Validate checks that the command has valid required fields.
func (c *DiscussWaveCommand) Validate() []error {
	var errs []error
	if c.RepoPath == "" {
		errs = append(errs, fmt.Errorf("RepoPath is required"))
	}
	if c.SessionID == "" {
		errs = append(errs, fmt.Errorf("SessionID is required"))
	}
	if c.ClusterName == "" {
		errs = append(errs, fmt.Errorf("ClusterName is required"))
	}
	if c.Topic == "" {
		errs = append(errs, fmt.Errorf("Topic is required"))
	}
	return errs
}
