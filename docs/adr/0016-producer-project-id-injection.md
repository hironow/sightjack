# 0016. Producer-side `project_id` injection (Phase α↔β bridge)

**Date:** 2026-05-07
**Status:** Accepted (2026-05-09, producer 4 ツール main merged: sj #203 `1998d182` / pt #205 `1eaa881d` / am #206 `bd82a1c4` / dom #19 `cfed12c4`)
**Linked plan:** `/Users/nino/.claude-work-a/plans/2026-05-07-phase-alpha-bridge-producer-project-id.md` (v3)
**Linked spec:** `/Users/nino/tap/refs/docs/dmail-metadata-v1-1.md`
**Linked audit:** `/Users/nino/tap/refs/docs/audit-multiplex-readiness.md` §軸 5

## Context

Phase α (gateway-side multiplex 6 issue + workspace-VM-side 2 issue) が 2026-05-07 に完了した。 multiplex 経路は以下 3 経路で実走可能:

1. **gateway 経路** (Slack `/runops --project=foo`): receiver 側で frontmatter 注入済 → 4 ツール parser は unknown key ignore で安全 pass
2. **agent local 経路** (cdr-job / 4 ツール CLI 直接、 operator が `cd ~/projects/foo/...`): gateway を通らない → producer 側で書かないと下流で project 識別が消える
3. **AI agent dispatch**: 経路 2 と同形

経路 2/3 で producer 側 (= sightjack 自身が D-Mail を新規生成する箇所すべて) が `metadata.project_id` を frontmatter に書き込む必要がある。

dmail-metadata-v1-1.md §「project_id 設定主体」は 2026-05-07 に「gateway / dmail-receiver / 4 ツール producer (sj/pt/am/dom) — phonewave は transport-only」 に訂正された。 4 ツール copy-sync byte-identical の同形原則 (S0037 substrate canonical lock + Rival Contract Phase 0 copy-sync) を維持しながら producer 側書込を実装する。

## Decision

### 1. 共通 helper 採用

`internal/platform/projectid/projectid.go` に **4 ツール copy-sync byte-identical** な helper を配置:

```go
package projectid

// Resolve returns the project_id for the current producer context.
// Priority: env var RUNOPS_PROJECT_ID > CWD path inference > empty.
//
// CWD inference rule: if cwd matches `<homeDir>/projects/<id>/...`,
// returns <id>. Otherwise empty.
//
// The returned source is one of "env", "cwd", or "" (legacy single-mode).
func Resolve(cwd string) (id, source string)

// IsValidProjectID returns true if id matches the gateway-side
// domain.ValidateProjectID regex (^[a-zA-Z0-9_-]+$, max 64 chars).
func IsValidProjectID(id string) bool

// InjectProjectID resolves project_id and writes it into mail.Metadata.
// No-op when project_id cannot be resolved or is invalid.
func InjectProjectID(mail *domain.DMail)
```

### 2. 注入位置

各 caller の `domain.DMail{Metadata: ...}` 構築直後に 1 行 `projectid.InjectProjectID(&mail)` を追加。 emit 経路完全リストは audit 軸 5 で pin 済 (sightjack 6 caller)。

### 3. legacy single-mode 互換

env var `RUNOPS_PROJECT_ID` 未設定 + CWD が `~/projects/<id>/` 配下でない場合、 `InjectProjectID` は no-op (= frontmatter から `project_id` 行が出ない)。 これにより legacy single-mode は byte-identical で互換維持。

### 4. 同形境界 (同形保証対象)

| Layer | 同形 | 理由 |
|---|---|---|
| `internal/platform/projectid/projectid.go` (helper) | byte-identical (package 名のみ差) | infrastructure layer、 S0037 substrate canonical lock 対象 |
| `internal/platform/projectid/projectid_test.go` (test) | byte-identical (package 名のみ差) | helper と co-located、 同形維持 |
| caller boilerplate stub (`projectid.InjectProjectID(&mail)`) | byte-identical 1 行 | 注入位置は domain 固有だが 1 行 stub は再利用 |
| caller 周辺 (use case main flow) | ツール固有 OK | wave generation / expedition / convergence / NFR は domain logic |

### 5. Validation policy

- 規約違反値 (`^[a-zA-Z0-9_-]+$` / 最大 64 文字違反) は frontmatter に書かず stderr に warn
- 違反値の出力先: stderr direct (telemetry-standards 準拠の stdout/stderr 分離)
- OTel span attribute 化は将来別 ADR で扱う (Phase α 既存 emit span への追加可能性)

### 6. Implementation phases

- Phase 0: refs spec 訂正 + audit 軸 5 pin + 本 ADR draft (Phase 0 完了 = mandatory gate)
- Phase 1: sightjack TDD パイロット (helper Red/Green → caller 統合)
- Phase 2: paintress / amadeus / dominator copy-sync (byte-identical + caller stub 注入)
- Phase 3: gap-check / 4 PR per tool / squash merge

## Consequences

### Positive

- agent local / cdr-job 経路で producer 側 project_id が下流に伝搬され、 multiplex 経路 2/3 で routing / 追跡が機能する
- 4 ツール同形原則を維持 (byte-identical helper + 1 行 stub)
- legacy single-mode 互換 byte-identical (env / CWD 非該当時は no-op)
- 規約違反値の defensive validation で systemd / 下流の事故を防ぐ

### Negative

- 4 ツール copy-sync drift のリスク (mitigation: gap-check ガード + 各 ADR で copy-sync 明記)
- caller 注入位置は domain 固有のため、 emit 経路の追加 (新 DMail kind 等) があれば 1 行 stub の追加忘れリスクあり (mitigation: regression test で frontmatter 出力を確認)

### Neutral

- worktree_id / notify_slack の producer 側書込は本 ADR のスコープ外 (project_id のみ、 将来別 ADR で扱う)
- gateway 経路 1 (Slack `/runops --project=foo`) は既に Phase α で receiver 側 frontmatter 注入済 → producer 側書込が後から走っても idempotent (helper は env > cwd > empty 優先順位、 既存値が env と一致するなら no-op、 一致しないなら overwrite)

## References

- plan v3: `/Users/nino/.claude-work-a/plans/2026-05-07-phase-alpha-bridge-producer-project-id.md`
- spec: `/Users/nino/tap/refs/docs/dmail-metadata-v1-1.md`
- audit 軸 5: `/Users/nino/tap/refs/docs/audit-multiplex-readiness.md`
- 4 ツール copy-sync 原則: shared ADR S0037 (substrate canonical lock)
- gateway-side `domain.ValidateProjectID`: regex `^[a-zA-Z0-9_-]+$` + max 64 chars
