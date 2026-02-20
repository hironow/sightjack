package sightjack

import "testing"

func TestDMailKind_Valid(t *testing.T) {
	kinds := []DMailKind{DMailSpecification, DMailReport, DMailFeedback}
	for _, k := range kinds {
		if k == "" {
			t.Errorf("kind constant should not be empty")
		}
	}
}

func TestValidateDMail_Valid(t *testing.T) {
	mail := &DMail{
		Name:        "spec-my-42",
		Kind:        DMailSpecification,
		Description: "Issue MY-42 ready for implementation",
	}
	if err := ValidateDMail(mail); err != nil {
		t.Errorf("expected valid, got: %v", err)
	}
}

func TestValidateDMail_MissingName(t *testing.T) {
	mail := &DMail{Kind: DMailSpecification, Description: "desc"}
	if err := ValidateDMail(mail); err == nil {
		t.Error("expected error for missing name")
	}
}

func TestValidateDMail_MissingKind(t *testing.T) {
	mail := &DMail{Name: "test", Description: "desc"}
	if err := ValidateDMail(mail); err == nil {
		t.Error("expected error for missing kind")
	}
}

func TestValidateDMail_InvalidKind(t *testing.T) {
	mail := &DMail{Name: "test", Kind: "invalid", Description: "desc"}
	if err := ValidateDMail(mail); err == nil {
		t.Error("expected error for invalid kind")
	}
}

func TestValidateDMail_MissingDescription(t *testing.T) {
	mail := &DMail{Name: "test", Kind: DMailFeedback}
	if err := ValidateDMail(mail); err == nil {
		t.Error("expected error for missing description")
	}
}
