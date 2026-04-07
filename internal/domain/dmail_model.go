package domain

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// DMailKind is the message type for D-Mails.
type DMailKind string

const (
	KindSpecification  DMailKind = "specification"
	KindReport         DMailKind = "report"
	KindDesignFeedback DMailKind = "design-feedback"
	KindImplFeedback   DMailKind = "implementation-feedback"
	KindConvergence    DMailKind = "convergence"
	KindCIResult       DMailKind = "ci-result"
	KindStallEscalation DMailKind = "stall-escalation"
)

// ValidDMailKinds is the canonical set of allowed D-Mail kinds per schema v1.
var ValidDMailKinds = map[DMailKind]bool{
	KindSpecification:   true,
	KindReport:          true,
	KindDesignFeedback:  true,
	KindImplFeedback:    true,
	KindConvergence:     true,
	KindCIResult:        true,
	KindStallEscalation: true,
}

// IsValidDMailKind returns true if the given kind is in the canonical set.
func IsValidDMailKind(kind DMailKind) bool {
	return ValidDMailKinds[kind]
}

// ValidateKind checks that kind is one of the allowed D-Mail kinds.
func ValidateKind(kind DMailKind) error {
	if !IsValidDMailKind(kind) {
		return fmt.Errorf("invalid D-Mail kind %q: %w", kind, ErrDMailKindInvalid)
	}
	return nil
}

// DMail represents the domain-owned D-Mail model.
// Core type and invariants belong here; I/O and orchestration stay in session.
type DMail struct {
	Name          string            `yaml:"name"`
	Kind          DMailKind         `yaml:"kind"`
	Description   string            `yaml:"description"`
	SchemaVersion string            `yaml:"dmail-schema-version,omitempty"`
	Issues        []string          `yaml:"issues,omitempty"`
	Severity      string            `yaml:"severity,omitempty"`
	Action        string            `yaml:"action,omitempty"`
	Priority      int               `yaml:"priority,omitempty"`
	Wave          *WaveReference    `yaml:"wave,omitempty"`
	Metadata      map[string]string `yaml:"metadata,omitempty"`
	Context       *InsightContext   `yaml:"context,omitempty" json:"context,omitempty"`
	Body          string            `yaml:"-"`
}

// Filename returns the canonical filename: "<name>.md".
func (d *DMail) Filename() string {
	return d.Name + ".md"
}

// ValidateDMail checks that a DMail conforms to D-Mail schema v1.
// Send-side strict validation (Postel's law).
func ValidateDMail(d *DMail) error {
	if d.SchemaVersion == "" {
		return ErrDMailSchemaRequired
	}
	if d.SchemaVersion != DMailSchemaVersion {
		return ErrDMailSchemaUnsupported
	}
	if d.Name == "" {
		return ErrDMailNameRequired
	}
	if d.Kind == "" {
		return ErrDMailKindRequired
	}
	if err := ValidateKind(d.Kind); err != nil {
		return err
	}
	if d.Description == "" {
		return ErrDMailDescriptionRequired
	}
	if d.Action != "" && !validDMailActions[d.Action] {
		return ErrDMailActionInvalid
	}
	return nil
}

var validDMailActions = map[string]bool{
	"retry":    true,
	"escalate": true,
	"resolve":  true,
}

// ParseDMail parses a D-Mail from raw bytes (YAML frontmatter + Markdown body).
// Postel-liberal: accepts any valid YAML frontmatter regardless of schema version
// or unknown fields. Validation is a separate step (ValidateDMail).
func ParseDMail(data []byte) (DMail, error) {
	content := string(data)
	if !strings.HasPrefix(content, "---\n") {
		return DMail{}, fmt.Errorf("dmail: missing frontmatter delimiter")
	}
	rest := content[4:]
	idx := strings.Index(rest, "\n---\n")
	if idx < 0 {
		if strings.HasSuffix(rest, "\n---") {
			idx = len(rest) - 4
		} else {
			return DMail{}, fmt.Errorf("dmail: missing closing frontmatter delimiter")
		}
	}
	yamlPart := rest[:idx]
	bodyPart := rest[idx+5:]

	var mail DMail
	if err := yaml.Unmarshal([]byte(yamlPart), &mail); err != nil {
		return DMail{}, fmt.Errorf("dmail parse frontmatter: %w", err)
	}
	mail.Body = strings.TrimPrefix(bodyPart, "\n")
	return mail, nil
}
