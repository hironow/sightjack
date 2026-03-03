# sightjack

## Workflow

- Do NOT use git worktrees (`EnterWorktree`, `isolation: "worktree"`). Work directly on the current branch.

## Repository Structure

- Entry: `cmd/sightjack/main.go` (signal.NotifyContext + NeedsDefaultScan)
- CLI: `internal/cmd/` (cobra v1.10.2, `NewRootCommand()` exported for testability)
- Root: `doc.go` only (root-zero: all code moved to internal/)
- Domain: `internal/domain/` (types, interfaces, constants, go:embed, pure functions — wave scheduling, scan utils, event projection)
- Session: `internal/session/` (I/O orchestration — Claude subprocess, file ops, scanner, dmail, archive, config loading, state I/O, outbox store)
    - `outbox_store.go`: SQLiteOutboxStore (transactional outbox pattern — Stage to SQLite, Flush with atomic writes to archive/ + outbox/)
- Event sourcing: `internal/eventsource/` (event store infrastructure; per-session directory storage, `os.RemoveAll` pruning)
- OTel: `internal/cmd/telemetry.go` (initTracer + OTLP HTTP exporter, shutdown via cobra.OnFinalize)
- Docker: `docker/compose.yaml` + `docker/jaeger-v2-config.yaml` (Jaeger v2)
- Semgrep: `.semgrep/cobra.yaml` (canonical source is phonewave)
- Release: `.goreleaser.yaml`

## CLI Design

- `cobra.EnableTraverseRunHooks = true` in `init()` (not constructor)
- All commands use `RunE` (not `Run`)
- `--config`, `--lang`, `--verbose`, `--dry-run` are PersistentFlags on root
- Default subcommand: `sightjack [flags] <repo>` → prepends `scan` via `NeedsDefaultScan`
- OTel tracer shutdown: `cobra.OnFinalize` + `sync.Once`
- `run` subcommand: `--notify-cmd`, `--approve-cmd`, `--auto-approve` local flags (convergence gate)
- `SIGHTJACK_TTY` env var: overrides `/dev/tty` in `openTTY()` for go-expect PTY injection in E2E tests
- Interactive input (`select`, `discuss`): reads from `openTTY()`, prompts to `cmd.ErrOrStderr()`

## Test Layout

- Unit tests: `*_test.go` colocated with source (Go convention)
    - Prefer `package sightjack_test` (external test package) — test exported API, not implementation details
    - `package sightjack` (in-package) only for test infrastructure (`newCmd` variable injection, etc.)
    - If an unexported function is worth testing individually, export it
- Integration tests: `internal/cmd/cobra_integration_test.go` (CLI integration)
- E2E tests: `tests/e2e/` (Docker-based, real Claude binary via fake-claude fixture)
    - `tests/e2e/compose-e2e.yaml` — Docker Compose for E2E environment
    - `tests/e2e/fake-claude/` — fixture-based Claude test double
    - `tests/e2e/fixtures/` — canned JSON for pipe tests

## Build & Test

```bash
just build              # build with version from git tags
just install            # build + install to /usr/local/bin
just test               # all tests, 300s timeout
just test-race          # with race detector
just test-e2e           # Docker E2E tests
just test-scenario-min  # L1 scenario test (minimal closed loop)
just test-scenario      # L1+L2 scenario tests (CI default)
just test-scenario-all  # all scenario tests (L1-L4)
just check              # fmt + vet + test
just semgrep            # cobra semgrep rules
just lint               # vet + markdown lint + gofmt check
```
