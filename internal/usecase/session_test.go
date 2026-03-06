package usecase

// Validation tests for RunSessionCommand, ResumeSessionCommand, and RescanSession
// have been moved to domain/primitives_test.go (parse-don't-validate).
// The usecase layer no longer calls Validate() — commands are always-valid by construction.
