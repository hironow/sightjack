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
