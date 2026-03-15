package testdata

import "path/filepath"

// ==========================================================================
// layers-local.yaml test fixture
// Covers sightjack-specific rules:
//   - gate-config-raw-field-access
//   - config-no-computed-field-in-set
//   - config-no-direct-computed-write
//   - no-state-dir-literal-in-path-join
// ==========================================================================

// --- Rule: gate-config-raw-field-access ---

func badGateRawFieldLocal(cfg struct {
	Gate struct {
		ReviewCmd   string
		AutoApprove bool
	}
}) {
	// ruleid: gate-config-raw-field-access
	_ = cfg.Gate.ReviewCmd
	// ruleid: gate-config-raw-field-access
	_ = cfg.Gate.AutoApprove
}

func goodGateIntentMethodLocal(gate struct {
	HasReviewCmd  func() bool
	IsAutoApprove func() bool
}) {
	// ok: gate-config-raw-field-access
	gate.HasReviewCmd()
	// ok: gate-config-raw-field-access
	gate.IsAutoApprove()
}

// --- Rule: no-state-dir-literal-in-path-join ---

func badStateDirLiteral() {
	// ruleid: no-state-dir-literal-in-path-join
	filepath.Join("/home", ".siren")
}

func badStateDirLiteralWithSuffix() {
	// ruleid: no-state-dir-literal-in-path-join
	filepath.Join("/home", ".siren", "events")
}

func goodStateDirConst(stateDir string) {
	// ok: no-state-dir-literal-in-path-join
	filepath.Join("/home", stateDir)
}

var _ = filepath.Join
