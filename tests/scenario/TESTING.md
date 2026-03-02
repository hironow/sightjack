# sightjack scenario tests

## go-expect Usage Policy

- **Non-interactive tests** (`TestScenario_L1_Minimal`, `TestScenario_L2_Small`, `TestScenario_L3_Middle`, `TestScenario_L4_Hard`, `TestScenario_ApproveCmdPath/auto_approve`):
  Use `--auto-approve` to bypass interactive prompts. No PTY, no go-expect dependency.

- **Interactive tests** (`TestScenario_L3_Interactive`, `TestScenario_ApproveCmdPath/approve_cmd`):
  Use `github.com/Netflix/go-expect` to drive PTY-based wave selection and approval prompts.
  Limited to the minimum operations: select wave (`"1"`), approve all (`"a"`).
  `approve_cmd` subtest additionally verifies that `--approve-cmd` handles the convergence gate
  via CmdApprover (not AutoApprover) and that `--notify-cmd` fires.

### When to use go-expect

Only when testing the interactive code path itself (wave selection, approval prompts)
or when verifying `--approve-cmd`/`--notify-cmd` hooks without `--auto-approve`.
All other tests must remain non-interactive (`--auto-approve`).

### When NOT to use go-expect

- Routing verification (use `--auto-approve` + `WaitForDMail`)
- Convergence gate testing (use `--approve-cmd` with a shell script)
- Multi-tool closed loop tests (use convenience helpers with `--auto-approve`)
