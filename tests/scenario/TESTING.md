# sightjack scenario tests

## go-expect Usage Policy

- **Non-interactive tests** (`TestScenario_L1_Minimal`, `TestScenario_L2_Small`, `TestScenario_L3_Middle`, `TestScenario_L4_Hard`, `TestScenario_ApproveCmdPath`):
  Use `--auto-approve` to bypass interactive prompts. No PTY, no go-expect dependency.

- **Interactive test** (`TestScenario_L3_Interactive`):
  Uses `github.com/Netflix/go-expect` to drive PTY-based wave selection and approval prompts.
  Limited to the minimum operations: select wave (`"1"`), approve all (`"a"`).

### When to use go-expect

Only when testing the interactive code path itself (wave selection, approval prompts).
All other tests must remain non-interactive (`--auto-approve`).

### When NOT to use go-expect

- Routing verification (use `--auto-approve` + `WaitForDMail`)
- Convergence gate testing (use `--approve-cmd` with a shell script)
- Multi-tool closed loop tests (use convenience helpers with `--auto-approve`)
