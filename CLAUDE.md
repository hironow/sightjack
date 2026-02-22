# sightjack

## Workflow

- Do NOT use git worktrees (`EnterWorktree`, `isolation: "worktree"`). Work directly on the current branch.

## Repository Structure

- Entry: `cmd/sightjack/main.go` (signal.NotifyContext + DefaultToScan)
- CLI: `internal/cmd/` (cobra v1.10.2, `NewRootCommand()` exported for testability)
- Library: root package `sightjack` (scan, wave, dmail, feedback, gate, notify, approve, telemetry, logger)
- OTel: `telemetry.go` (noop default + OTLP HTTP exporter, shutdown via cobra.OnFinalize)
- Docker: `docker/compose.yaml` + `docker/jaeger-v2-config.yaml` (Jaeger v2)
- Semgrep: `.semgrep/cobra.yaml` (canonical source is phonewave)
- Release: `.goreleaser.yaml`
- E2E: `tests/e2e/compose-e2e.yaml`

## CLI Design

- `cobra.EnableTraverseRunHooks = true` in `init()` (not constructor)
- All commands use `RunE` (not `Run`)
- `--config`, `--lang`, `--verbose`, `--dry-run` are PersistentFlags on root
- Default subcommand: `sightjack [flags] <repo>` → prepends `scan` via `DefaultToScan`
- OTel tracer shutdown: `cobra.OnFinalize` + `sync.Once`
- `run` subcommand: `--notify-cmd`, `--approve-cmd`, `--auto-approve` local flags (convergence gate)

## Build & Test

```bash
just build        # build with version from git tags
just install      # build + install to /usr/local/bin
just test         # all tests, 300s timeout
just test-race    # with race detector
just test-e2e     # Docker E2E tests
just check        # fmt + vet + test
just semgrep      # cobra semgrep rules
just lint         # vet + markdown lint + gofmt check
```
