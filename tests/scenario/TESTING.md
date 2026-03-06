# sightjack scenario tests

## Prerequisites

- Go 1.26+ (all 4 repos must use the same toolchain)
- Sibling repos at the same parent directory:
  - `phonewave/`, `sightjack/`, `paintress/`, `amadeus/`
  - Override with env vars: `PHONEWAVE_REPO`, `SIGHTJACK_REPO`, `PAINTRESS_REPO`, `AMADEUS_REPO`

## Running

```bash
# L1 minimal (single closed loop, ~12s)
just test-scenario-min

# L2 small (~14s)
just test-scenario-small

# L3 middle (~60s)
just test-scenario-middle

# L4 hard (~45s)
just test-scenario-hard

# L1+L2 (CI default)
just test-scenario

# All scenario tests (nightly)
just test-scenario-all
```

Or directly with `go test`:

```bash
go test -tags scenario ./tests/scenario/ -run TestScenario_L1 -count=1 -v -timeout=120s
```

## Test levels

| Level | Test | Focus |
|-------|------|-------|
| L1 | `TestScenario_L1_Minimal` | Single closed loop: issue → scan → wave plan |
| L2 | `TestScenario_L2_Small` | Multi-issue, wave scheduling |
| L3 | `TestScenario_L3_Middle` | Convergence detection, multi-scan |
| L3 | `TestScenario_L3_Interactive` | Interactive wave selection (go-expect PTY) |
| L4 | `TestScenario_L4_Hard` | Fault injection, recovery |
| - | `TestScenario_ApproveCmdPath` | `--approve-cmd` / `--notify-cmd` hooks + go-expect PTY (human-on-the-loop) |

## Human-on-the-loop

sightjack uses `--approve-cmd` and `--notify-cmd` flags for external approval
gates, plus `StdinApprover` with go-expect PTY for interactive approval.
`TestScenario_ApproveCmdPath` verifies both modes:

- `auto_approve` subtest: CmdApprover with `--approve-cmd` (non-interactive)
- `approve_cmd` subtest: CmdApprover + CmdNotifier + go-expect PTY convergence gate

sightjack is the only tool that uses go-expect PTY interaction for scenario
tests because its `select` and `discuss` subcommands require terminal input.

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

## Build tag

All scenario tests use `//go:build scenario`. They are excluded from regular
`go test ./...` runs and require `-tags scenario`.

## Troubleshooting

### `compile: version "go1.26.0" does not match go tool version "go1.19.3"`

GOROOT or GOTOOLDIR points to a different Go installation than the `go` binary in PATH.

```bash
go version
go tool compile -V
go env GOROOT
go env GOTOOLDIR

# Fix (mise users)
unset GOROOT GOTOOLDIR
mise install go
mise reshim
```

All 4 repos pin `go = "1.26"` in `mise.toml` and `go 1.26` in `go.mod`.
