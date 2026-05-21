# Handover

**Last updated:** 2026-05-22 (asia/tokyo, Phase 4 #1 update_strictness landed)
**Updated by:** Claude Opus 4.7 session

## Current State

jun15 MCP pivot (refs/issues/0027) **全 phase 完了 + archive 入り**。
Phase 2a で確立した 9-commit pattern を起点に、 Phase 3 real impl と
Phase 4 follow-up #1 で sightjack の MCP server-first architecture が
main merged。

sightjack 固有の jun15 landmark:

- ADR 0018 (= `docs/adr/0018-mcp-pivot.md`) で architectural pin 固定
- 4 MCP tool 全 real impl (= ping / next_wave / get_scan_result /
  update_strictness)
- Phase 4 #1 (PR #217 `675ce8c`): `sightjack.update_strictness` を
  preview-only → `UpdateConfig` で `config.yaml` 書き戻しに昇格、
  persistence='config.yaml' or 'no-op'
- `.semgrep/jun15-no-headless-llm.yaml` 5 rule で headless LLM 経路を
  permanent block (= `tests/**` のみ exclude、 fake-claude binary 用)
- `internal/session/claude_adapter.go` の `Run` / `RunDetailed` は
  `ErrMCPPivotDeprecated` stub
- `/sightjack-scan` skill + plugin README で claude code session 経由の
  唯一の wave-driving 経路を提供
- D-Mail 9-field envelope schema cross-tool fixture が paintress と
  cross-repo contract 整合

## In Progress

なし。 jun15 MCP pivot に関する作業は完了し refs 0027 は archive (=
`tap/refs/HTMLification/docs/archive/0027-jun15-mcp-pivot.html`)。

## Next Actions

なし (= Phase 4 #1-#4 全完了)。 後続作業候補は別 issue で fork:

1. Phase 3 cost (c) Anthropic dashboard credit 0 verify (= 2026-06-15
   launch 以降の operational evidence)

## Known Risks / Blockers

- `sightjack scan` / `run` / `waves` / `discuss` / `apply` 全 5
  subcommand が `ErrMCPPivotDeprecated` 返却に倒れているため、
  scheduler / CI / 既存 runbook で `sightjack run` を呼ぶ全箇所が
  `human-in-the-loop` (= `/sightjack-scan` skill) 必須
- `sightjack mcp-config` (legacy `.mcp.json` 管理) と `sightjack mcp`
  (MCP server) は名称が紛らわしい。 plugin README + skill SKILL.md で
  role 違いを明示済

## Context the Next Actor Needs

- **canonical plan archive**: `tap/refs/HTMLification/docs/archive/0027-jun15-mcp-pivot.html`
  (= immutable、 status pill `✅ ARCHIVED`)
- **post-mortem**: `tap/refs/HTMLification/lessons/0027-jun15-mcp-pivot-post-mortem.html`
- **billing boundary 原則**: LLM 発火は常に human-initiated、 daemon は route まで
- **semgrep gate**: `.semgrep/jun15-no-headless-llm.yaml` 5 rule、 production
  path に `permanent` nosemgrep 例外禁止
- **MCP server tool 命名規約**: `<tool_name>.<verb>` (paintress / amadeus /
  dominator / phonewave と対称、 claude code の `mcp__<server>__<tool>` mapping に対応)

## Relevant Files and Commands

- `docs/adr/0018-mcp-pivot.md` - architectural pin
- `.semgrep/jun15-no-headless-llm.yaml` - billing-boundary gate (5 rule)
- `internal/session/mcp_server.go` - sightjack MCP server (4 tool real impl + config.yaml 書き戻し)
- `internal/session/claude_adapter.go` - `ClaudeAdapter.Run` / `RunDetailed` = `ErrMCPPivotDeprecated` stub
- `internal/session/doctor.go` - `claude-inference` / `context-budget` = Skip (post jun15 MCP pivot)
- `internal/domain/dmail_envelope.go` - 9-field envelope schema
- `internal/cmd/mcp.go` - `sightjack mcp` cobra subcommand (UpdateConfig 経由で config.yaml 書き戻し)
- `plugins/sightjack/skills/sightjack-scan/SKILL.md` - human-driven entry point
- `just lint-go` - golangci-lint v2 (0 issues 維持)
- `just semgrep` - semgrep gate (0 findings 維持)
- `go test -count=1 -short ./...` - sightjack test suite
