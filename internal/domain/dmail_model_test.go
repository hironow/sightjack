package domain_test

import (
	"errors"
	"testing"

	"github.com/hironow/sightjack/internal/domain"
)

func TestValidateDMail_Valid(t *testing.T) {
	d := &domain.DMail{
		SchemaVersion: domain.DMailSchemaVersion,
		Name:          "spec-001",
		Kind:          domain.KindSpecification,
		Description:   "test",
	}
	if err := domain.ValidateDMail(d); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestValidateDMail_Errors(t *testing.T) {
	tests := []struct {
		name    string
		dmail   *domain.DMail
		wantErr error
	}{
		{
			name:    "missing schema",
			dmail:   &domain.DMail{Name: "a", Kind: "report", Description: "d"},
			wantErr: domain.ErrDMailSchemaRequired,
		},
		{
			name:    "bad schema",
			dmail:   &domain.DMail{SchemaVersion: "99", Name: "a", Kind: "report", Description: "d"},
			wantErr: domain.ErrDMailSchemaUnsupported,
		},
		{
			name:    "missing name",
			dmail:   &domain.DMail{SchemaVersion: "1", Kind: "report", Description: "d"},
			wantErr: domain.ErrDMailNameRequired,
		},
		{
			name:    "missing kind",
			dmail:   &domain.DMail{SchemaVersion: "1", Name: "a", Description: "d"},
			wantErr: domain.ErrDMailKindRequired,
		},
		{
			name:    "invalid kind",
			dmail:   &domain.DMail{SchemaVersion: "1", Name: "a", Kind: "foo", Description: "d"},
			wantErr: domain.ErrDMailKindInvalid,
		},
		{
			name:    "missing description",
			dmail:   &domain.DMail{SchemaVersion: "1", Name: "a", Kind: "report"},
			wantErr: domain.ErrDMailDescriptionRequired,
		},
		{
			name:    "invalid action",
			dmail:   &domain.DMail{SchemaVersion: "1", Name: "a", Kind: "report", Description: "d", Action: "nope"},
			wantErr: domain.ErrDMailActionInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := domain.ValidateDMail(tt.dmail)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("got %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestIsValidDMailKind(t *testing.T) {
	if !domain.IsValidDMailKind(domain.KindSpecification) {
		t.Error("specification should be valid")
	}
	if !domain.IsValidDMailKind(domain.KindStallEscalation) {
		t.Error("stall-escalation should be valid")
	}
	if domain.IsValidDMailKind("unknown") {
		t.Error("unknown should be invalid")
	}
}
