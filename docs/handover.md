# Handover

**Last updated:** 2026-06-10 (JST)
**Updated by:** claude (AI draft from git history — review before trusting)

## Current State

The MCP pivot is reflected throughout the repo: sightjack is a pure data
plane (`sightjack mcp`) serving scan/wave read models and persisting
strictness, with the headless designer pipeline retired (README). Recent work
on `main` aligned MCP pivot wording/docs (#244–#247), hardened session close
handling (#233), kept MCP session wiring active (`1a3f7cb`), suppressed
pre-existing lint findings (#234–#237), and migrated e2e tests to
testcontainers-go (#230). Last commit: `0397cbb` "docs: add decision queue
for human-review items (#249)" on 2026-06-10.

## In Progress

不明 (git 履歴からは判別できず) — no open feature branch is evident in the
shallow clone; the most recent code change is `fix(sessions): keep mcp
session wiring active`.

## Next Actions

1. requester による docs/intent.md ドラフトのレビューと確定
2. Work through the human-review items in `docs/decision-queue.md` (added 2026-06-10, #249)

## Known Risks / Blockers

- `docs/intent.md` / `docs/handover.md` are in `.gitignore`; this PR adds them with `git add -f`. Decide whether to track them or drop the ignore entries.

## Context the Next Actor Needs

- Task runner is `just` (12k justfile); `just check` runs fmt + vet + golangci-lint + semgrep + root-guard + tests + docs-check
- Project-specific semgrep rules live under `.semgrep/`; pre-commit hooks via `.pre-commit-config.yaml`; toolchain pinned in `mise.toml`
- Naming/design concepts come from the game SIREN (sightjack, waves, clusters, shibito) — see README before touching domain terms
- The Claude Code skill that drives the workflow lives at `plugins/sightjack/skills/sightjack-scan/SKILL.md`
- Releases via GoReleaser; e2e tests use testcontainers-go

## Relevant Files and Commands

- `README.md` — MCP tools, SIREN concept mapping, strictness model
- `docs/decision-queue.md` — open human-review items
- `plugins/sightjack/skills/sightjack-scan/SKILL.md` — the scan/wave skill workflow
- `justfile` — `just check` (full gate), `just test`, `just lint`, `just semgrep`
- `cmd/` and `internal/` — CLI entrypoints and core implementation
