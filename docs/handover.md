# Handover

**Last updated:** 2026-05-21 (asia/tokyo, Phase 2a finalize)
**Updated by:** Claude Opus 4.7 session

## Current State

`feat/jun15-mcp-pivot` long-lived branch 上で refs/issues/0027
(jun15 MCP pivot v4) の Phase 2a (= sightjack horizontal expansion)
を 8 commit (scaffold + sub-A + sub-B + sub-C) 完了。 paintress
Phase 1 (PR #213, squash-merged at 9b884c6, ADR paintress 0017) で
確立した 9 commit pattern を sightjack 用に adapt し、 `claude --print`
exec を全 production path から削除、 `sightjack mcp` stdio MCP server
+ `/sightjack-scan` skill + 9-field D-Mail envelope schema を追加、
sightjack ADR 0018 を発行。

Phase 2a 完了内容:

1. **`.semgrep/jun15-no-headless-llm.yaml`** (= 5 rule, scaffold で
   transitional exclude を設定し sub-B で削除済。 残る exclude は
   `tests/**` のみ = fake-claude binary を呼ぶ test fixture 用)。
2. **`sightjack mcp` MCP server** (`internal/session/mcp_server.go`)
   = JSON-RPC 2.0 stdio、 4 MiB scanner buffer、 Phase 2a MVP として
   `sightjack.ping` / `sightjack.next_wave` /
   `sightjack.get_scan_result` / `sightjack.update_strictness` を
   advertise + dispatch。 後 3 つは contract 固定 + stub。 7 test
   pass (= ListsAllPhase2aTools / CallsPingTool / RejectsUnknownTool
   / NextWaveStub / GetScanResultStub_EchoesSessionID /
   UpdateStrictnessStub_EchoesLevel / RejectsUnknownMethod)。
3. **`/sightjack-scan` skill** (= `plugins/sightjack/skills/sightjack-scan/SKILL.md`)
   + plugin README。 `--plugin-dir ./plugins/sightjack` で claude code
   session に load、 `mcp__sightjack__*` tools を allowed-tools に
   宣言。
4. **D-Mail 9-field envelope** (`internal/domain/dmail_envelope.go`)
   = paintress canonical の symmetric copy (= paintress → sightjack
   方向 + sightjack → paintress 方向どちらも parser 経由で正規化可)。
   `tests/fixtures/dmail/dmail-2026-06-01T11-00-00Z-def456.{yaml,body.md}`
   で fixture pair を配置 + 5 test pass (= FixtureSchemaIsHonored /
   RejectsMissingRequiredFields / DetectsConsumedEnvelope /
   BodyPathPointsAtPairedFile / IdempotencyKey_DistinguishesFixtures)。
5. **sub-A** (= claude_adapter.go + doctor.go の `claude --print`
   invocation を `ErrMCPPivotDeprecated` stub に置換)。
   `ClaudeAdapter.RunDetailed` body 280 行 + `effectiveDir` helper
   削除、 struct field は composition root 互換のため保持。 doctor の
   inference / context-budget check は `Skip` 結果 (`post jun15 MCP
   pivot` reason) に置換。 33 件の dependent test に t.Skip 付与。
6. **sub-B** (= semgrep transitional excludes 削除 + deprecated test
   33 件物理削除 + canonical assertion 1 件追加)。 file-level
   削除: `streambus_wiring_test.go` / `auto_discuss_no_adr_test.go` /
   `auto_discuss_test.go`。 partial 削除: claude_test 7 件 +
   review_gate_test 2 件 + scanner_test 3 件 (+ writeRecorder helper)
   + telemetry_test 1 件 + rival_contract_produce_test 1 件
   (+ updateGolden / dmailUUIDPattern / canonicalSpecWave /
   produceCanonicalSpecBytes helpers) + lifecycle_test 19 件
   (+ 22 helpers)。 残った canonical assertion:
   `TestClaudeAdapter_RunDetailedReturnsErrMCPPivotDeprecated`
   (`errors.Is` で `session.ErrMCPPivotDeprecated` を検証)。
7. **sub-C** (= 本 commit、 sightjack ADR 0018 起票 + handover finalize)。

## In Progress

- branch: `feat/jun15-mcp-pivot` (= scaffold + 7 commit、 8 commit 目
  が本 sub-C、 9 commit 目は必要時 post-merge fixup)
- main merge は Phase 2a 完了後の PR 作成 + CI green + squash-merge
  待ち (= paintress PR #213 pattern)
- 次 phase: 2b dominator / 2c amadeus (= phonewave は LLM 非使用
  のため対象外)

## Next Actions

1. `feat/jun15-mcp-pivot` に PR 作成 (= title: `Phase 2a: sightjack
   jun15 MCP pivot (refs/issues/0027)`)
2. CI を green まで監視 (= paintress PR #213 と同様、 docs-check と
   test-fail が発生する可能性あり)
3. squash-merge 完了後、 Phase 2b dominator 着手
4. cost monitoring: OTel MCP invocation count を sightjack でも計測、
   Anthropic dashboard で credit 0 維持を手動検証

## Known Risks / Blockers

- `paintress` PR #213 では post-merge で docs-check と 14 test の
  ErrMCPPivotDeprecated fail が出て e781429 で fixup した。 sightjack
  は事前に 33 test 削除済 + docs 同期済なので post-merge fixup は
  最小限の想定。
- `sightjack scan` / `run` / `waves` / `discuss` / `apply` 全 5
  subcommand が `ErrMCPPivotDeprecated` 返却に倒れたため、 schedulers
  / CI / 既存 runbook で `sightjack run` を呼ぶ全箇所が
  `human-in-the-loop` (= `/sightjack-scan` skill) 必須になる。 移行
  期間は handover で明示。
- `sightjack mcp-config` (legacy `.mcp.json` 管理) と `sightjack mcp`
  (= MCP server) は名称が紛らわしい。 plugin README + skill SKILL.md
  で role 違いを明示済だが、 ユーザ向け doc 更新が将来必要。
- Phase 2b/c では amadeus / dominator が同様の `claude --print`
  invocation を持つ場合は同 9 commit pattern を copy する。

## Context the Next Actor Needs

- **canonical plan**: `refs/HTMLification/docs/issues/0027-jun15-mcp-pivot.html`
- **paintress ADR 0017**: `~/tap/paintress/docs/adr/0017-mcp-pivot.md`
  (= 9 commit pattern + enforcement inventory + bypass candidates)
- **sightjack ADR 0018**: `docs/adr/0018-mcp-pivot.md`
  (= 本 phase の architectural pin、 paintress 0017 の symmetric
  counterpart)
- **paintress 9 commit history**: `~/tap/paintress` 内 PR #213
  (squash-merged at 9b884c6) を参照、 sightjack はこの順序を copy
- **billing boundary 原則**: LLM 発火は常に human-initiated、 daemon
  は route まで、 consume 側は明示 slash command で trigger
- **semgrep gate**: `.semgrep/jun15-no-headless-llm.yaml` 5 rule、
  production path に `permanent` nosemgrep 例外禁止、 残る exclude は
  `tests/**` (fake-claude binary) のみ
- **MCP server tool 命名規約**: `<tool_name>.<verb>` (= dot 区切り、
  paintress の `paintress.ping` / `paintress.next_issue` / etc と
  対称)。 claude code 側の `mcp__<server>__<tool>` 自動 mapping に
  対応する。

## Relevant Files and Commands

- `docs/adr/0018-mcp-pivot.md` - 本 phase の architectural pin
- `.semgrep/jun15-no-headless-llm.yaml` - billing-boundary gate (5
  rule、 production scope 完全 enforced、 `tests/**` のみ exclude)
- `internal/session/mcp_server.go` - sightjack MCP server (= Phase 2a
  MVP scope、 4 tool stub)
- `internal/session/claude_adapter.go` - `ClaudeAdapter.Run` /
  `RunDetailed` = `ErrMCPPivotDeprecated` stub
- `internal/session/doctor.go` - `claude-inference` /
  `context-budget` = Skip (post jun15 MCP pivot)
- `internal/domain/dmail_envelope.go` - 9-field envelope symmetric
  copy
- `internal/cmd/mcp.go` - `sightjack mcp` cobra subcommand
- `plugins/sightjack/skills/sightjack-scan/SKILL.md` - human-driven
  entry point
- `tests/fixtures/dmail/dmail-2026-06-01T11-00-00Z-def456.{yaml,body.md}`
  - synthetic D-Mail contract fixture (paintress → sightjack 方向)
- `just lint-go` - golangci-lint v2 (= 0 issues 維持)
- `just semgrep` - semgrep gate (= 0 findings 維持、 78 rules)
- `go test -count=1 -short ./...` - sightjack test suite (= 全 pkg ok)
