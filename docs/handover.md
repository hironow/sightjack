# Handover

**Last updated:** 2026-05-20 (asia/tokyo, Phase 2a kickoff)
**Updated by:** Claude Opus 4.7 session

## Current State

`feat/jun15-mcp-pivot` long-lived branch を切り、 refs/issues/0027
(jun15 MCP pivot v4) の Phase 2a (= sightjack horizontal expansion)
を着手。 paintress Phase 1 で確立した 9 commit pattern (= PR #213,
squash-merged at 9b884c6, ADR paintress 0017) を copy する。

本 commit (= scaffold) で配置済:

- `.semgrep/jun15-no-headless-llm.yaml`: 5 rule + transitional
  exclude on `internal/session/claude_adapter.go` および
  `internal/session/doctor.go` (= 現状 `claude --print` exec を
  保持しているため、 sub-B で MCP 移行と一緒に削除予定)

## In Progress

- branch: `feat/jun15-mcp-pivot` (= long-lived feature branch、 main
  merge は Phase 2a 全完了後)
- linked issue: `refs/HTMLification/docs/issues/0027-jun15-mcp-pivot.html`
- canonical pattern: paintress ADR 0017 (= LLM owner inversion、
  Go CLI を MCP server data plane に縮約)
- Phase 2a MVP scope (= refs 0027 §8 を sightjack 用に adapt):
  - [x] feat/jun15-mcp-pivot branch 作成 + scaffold commit (= 本 commit)
  - [ ] MCP server endpoint (= `internal/session/mcp_server.go`) skeleton + `sightjack mcp` cobra subcommand
  - [ ] sightjack.next_wave / get_scan_result / update_strictness 等の MCP tool **interface fixed + stub**
  - [ ] `/sightjack-scan` slash command の skill definition (= `plugins/sightjack/skills/sightjack-scan/SKILL.md`)
  - [ ] D-Mail envelope schema 参照 (= paintress canonical を import か repo 内 copy)
  - [ ] **sub-A**: `internal/session/claude_adapter.go` の `claude --print` invocation を deprecate stub に置換
  - [ ] **sub-B**: semgrep transitional exclude 削除 + skipped test 完全削除
  - [ ] **sub-C**: `docs/adr/0010-mcp-pivot.md` 起票 (= sightjack 内 ADR 連番継続) + handover finalize

## Next Actions

次 session で sub-A 着手:
1. `internal/session/mcp_server.go` を新規実装 (= paintress 9dcccd6 を copy)
2. `internal/cmd/mcp.go` cobra subcommand
3. root.go に `newMCPCommand()` register
4. test 配置

## Known Risks / Blockers

- sightjack は既に **stdin pattern 移行済** (= paintress と異なり argv 渡しではなく stdin 経由)、 そのため E2BIG 経由の deprecate motivation が paintress より弱い
- ただし billing boundary 観点 (= 2026-06-15 credit pool 分離) は同じ、 architecturally inversion が必要
- `wave_generator.go` / `cluster_generator.go` 等の LLM-using 主要 entry point が複数あり、 paintress の `Expedition.Run()` よりも refactor scope 大
- doctor.go の `claude --print --max-turns 1 \"1+1=\"` health check は sub-B で MCP server ping に置換予定

## Context the Next Actor Needs

- **canonical plan**: `refs/HTMLification/docs/issues/0027-jun15-mcp-pivot.html`
- **paintress ADR 0017**: `~/tap/paintress/docs/adr/0017-mcp-pivot.md` (= LLM owner inversion 設計、 sightjack も同 pattern を踏襲)
- **paintress 9 commit history**: `~/tap/paintress` 内 PR #213 (squash-merged at 9b884c6) を参照、 sightjack の 9 commit はこの順序を copy
- **billing boundary 原則**: LLM 発火は常に human-initiated、 daemon は route まで、 consume 側は明示 slash command で trigger
- **semgrep gate**: `.semgrep/jun15-no-headless-llm.yaml` 5 rule、 production path に `permanent` nosemgrep 例外禁止

## Relevant Files and Commands

- `.semgrep/jun15-no-headless-llm.yaml` - billing-boundary gate (5 rule、 transitional exclude on claude_adapter.go + doctor.go)
- `internal/session/claude_adapter.go` - 現状の LLM invocation entry point (= sub-A で deprecate 予定)
- `internal/session/doctor.go` - health check で `claude --print` 利用 (= sub-B で MCP ping に置換)
- `just lint-go` - golangci-lint v2
- `just semgrep` - semgrep gate (= 0 finding 維持目標)
- `go test -count=1 -timeout=120s ./internal/...` - sightjack test suite
