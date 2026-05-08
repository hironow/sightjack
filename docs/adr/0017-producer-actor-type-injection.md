# 0017. Producer-side `requester_actor_type` injection (Phase β-1: AI-vs-human approval gating)

**Date:** 2026-05-09
**Status:** Accepted
**Linked ADR (gateway):** runops-gateway docs/adr/0035-ai-agent-cannot-approve-ai-agent.md (architectural pin)
**Linked ADR (gateway):** runops-gateway docs/adr/0036-phase-4a-approval-actor-validation.md (effective rule)
**Linked ADR (gateway):** runops-gateway docs/adr/0037-producer-actor-classification.md (4 axes contract、 §Tests proving coverage の per-tool 必須要件)
**Linked issue:** /Users/nino/tap/refs/docs/issues/0011-runops-gateway-ai-agent-identity-4-eyes.md
**Template precedent:** ADR 0016 (producer-side `project_id` injection)

## Context

gateway-side ADR 0035/0036/0037 で AI agent が AI agent の 4-eyes approval を実行できない architectural pin が確立した (system-level invariant 完成、 15 PR 着地、 2026-05-08〜2026-05-09)。 ただし producer rollout (= 4 ツール sightjack/paintress/amadeus/phonewave 各 repo で `metadata.requester_actor_type` を D-Mail に emit) は未完了。

migration window の責任分担は ADR 0037 §Migration で path 別に narrowing 済:

- **HIGH severity 4-eyes approval (= convergence)**: ADR 0037 acceptance 日 (2026-05-09) から **即 fail-closed**。 producer から `requester_actor_type` が来なければ approve は受理されない。 migration window フォールバックは適用されない
- **非 HIGH severity (= dispatch / canary)**: producer rollout 完了までは empty actor type を `CallerHumanOperator` フォールバック許可 (= 0038 候補 trigger 後に flip)

つまり producer rollout が遅れると **HIGH path 全部が止まる** (= 即座のオペレーション影響)、 一方で非 HIGH path は migration window で human として処理される。

ADR 0037 §Enforcement inventory の per-tool 必須要件に基づき、 producer 側 emit site で:

1. `RUNOPS_ACTOR_TYPE` env を 4 canonical 値 (`human-operator` / `gateway-service` / `ai-agent` / `workspace-daemon`) で読む
2. `metadata.requester_actor_type` + `metadata.requester_actor_source=env` を emit (gateway ADR 0037 Axis 1 二分割: producer 入力 enum `{env, unknown}`、 `broker` は gateway-only attestation)
3. invalid env value (= 4 canonical 外) は **producer 側で emit を fail させる** (= silent escalation 防止、 ADR 0037 §Producer-side validation 必須要件)
4. `workspace-daemon` の場合 `RUNOPS_INITIATING_ACTOR_TYPE` env も読み、 `metadata.initiating_actor_type` を carry。 invalid は **producer emit fail** (= silent escalation 防止)、 unset は producer 側で emit 続行 → gateway ADR 0036 4 layer fail-closed の "missing initiating" path で HIGH のみ reject (非 HIGH は migration window フォールバック)

本 ADR は sightjack pilot として 4 ツール copy-sync substrate を立ち上げる。 paintress / amadeus は同 helper を byte-identical で copy-sync (S0037 substrate canonical lock)、 phonewave は relay-preserve / daemon-generated 区別の都合で別 ADR で扱う。

## Decision

### 1. 共通 helper 採用

`internal/platform/actortype/actortype.go` に **4 ツール copy-sync byte-identical** な helper を配置 (ADR 0016 の `projectid` と同形 substrate):

```go
package actortype

// ErrInvalidActorType is returned when RUNOPS_ACTOR_TYPE env is set but
// not one of the 4 canonical values. Callers MUST fail the emit on this
// error per ADR 0037 §Producer-side validation: silent escalation via
// migration fallback is disallowed.
var ErrInvalidActorType = errors.New("invalid RUNOPS_ACTOR_TYPE")

// ErrInvalidInitiatingActorType is returned when RUNOPS_INITIATING_ACTOR_TYPE
// env is set but not one of the 4 canonical values. Callers MUST fail the
// emit on this error.
var ErrInvalidInitiatingActorType = errors.New("invalid RUNOPS_INITIATING_ACTOR_TYPE")

// Resolve reads RUNOPS_ACTOR_TYPE env and returns (actorType, source).
// Priority: env > empty.
//
//   - env unset                    → ("", "", nil)             [legacy compat path]
//   - env set + 4 canonical values → (canonical, "env", nil)
//   - env set + invalid value      → ("", "", ErrInvalidActorType)
//
// The returned source is "env" when actor type was resolved, "" otherwise.
// Producer "broker" attestation is forbidden — helpers always write "env".
func Resolve() (actorType, source string, err error)

// ResolveInitiating reads RUNOPS_INITIATING_ACTOR_TYPE env and returns the
// initiating actor type. Used only when the resolved actor type is
// "workspace-daemon" (per gateway ADR 0036 effective_requester_actor_type rule).
//
//   - env unset                    → ("", nil)             [allowed for non-HIGH; HIGH gateway will fail-closed]
//   - env set + 4 canonical values → (canonical, nil)
//   - env set + invalid value      → ("", ErrInvalidInitiatingActorType)
func ResolveInitiating() (string, error)

// IsValidActorType returns true if t is one of the 4 canonical values.
func IsValidActorType(t string) bool

// InjectActorType resolves actor type from the current process context
// and writes it into the provided metadata map. Returns the (possibly
// newly-allocated) map, or an error from Resolve / ResolveInitiating.
//
// Callers MUST propagate the error and fail the emit. No fallback.
//
// Wiring:
//
//	updated, err := actortype.InjectActorType(mail.Metadata)
//	if err != nil { return fmt.Errorf("dmail emit: %w", err) }
//	mail.Metadata = updated
func InjectActorType(metadata map[string]string) (map[string]string, error)
```

### 2. 注入位置

`internal/session/dmail.go` の `ComposeDMail` 内に追加 (ADR 0016 stub line 100 の直後)。 `InjectActorType` は error を返すため、 ここで emit を fail させる:

```go
mail.Metadata = projectid.InjectProjectID(mail.Metadata)
updated, err := actortype.InjectActorType(mail.Metadata)
if err != nil {
    return fmt.Errorf("dmail compose: invalid actor type env: %w", err)
}
mail.Metadata = updated
```

`ComposeDMail` が sightjack の **共通 D-Mail emit entry** であり、 全 caller (ADR 0016 で pin 済の 6 caller) がここを通過するため、 1 ヶ所追加で全 emit site をカバーする。

### 3. legacy compat (= env unset path のみ)

env var `RUNOPS_ACTOR_TYPE` が **未設定** の場合、 `InjectActorType` は no-op + nil error (= frontmatter から `requester_actor_type` 行が出ない)。 gateway 側の挙動は path 別:

- HIGH path: 即 fail-closed (ADR 0037 acceptance day から)。 producer 側で env を必ず set する責務 (= systemd unit / launcher / wrapper の構築仕様)
- 非 HIGH path: migration window で `CallerHumanOperator` フォールバック許可 (0038 候補 trigger 後に flip)

env var が **設定されているが invalid** の場合は legacy compat ではなく **emit fail** (= 上記 §1 §2)。 silent escalation を防ぐ。

### 4. Validation policy

| env state | helper return | ComposeDMail behavior |
|---|---|---|
| unset | `("", "", nil)` | emit 続行 (legacy compat path) |
| canonical 4 値 | `(value, "env", nil)` | emit に metadata 注入 |
| invalid (= 4 canonical 外、 空白文字、 `broker` 等) | `("", "", ErrInvalidActorType)` | **emit fail with error** |

daemon path (`RUNOPS_INITIATING_ACTOR_TYPE`) も同等:

| env state | helper return | ComposeDMail behavior |
|---|---|---|
| unset (when actor=daemon) | `("", nil)` | emit 続行 (= gateway 側で HIGH path fail-closed が catch、 非 HIGH は migration window) |
| canonical 4 値 | `(value, nil)` | emit に metadata 注入 |
| invalid | `("", ErrInvalidInitiatingActorType)` | **emit fail with error** |

OTel span attribute 化は将来別 ADR で扱う。

### 5. relay vs new-emit 区別 — sightjack 固有

sightjack は **producer-only** (= 既存 D-Mail を relay しない)。 すべての `ComposeDMail` 通過は new-emit。 relay-preserve 経路は phonewave (= courier daemon) の責務であり、 ADR 0037 Axis 3 の dual-actor 問題と組み合わせて phonewave 別 ADR で扱う。

### 6. broker source 不可侵

helper は `requester_actor_source` を **常に `"env"` 固定** で書く。 producer から `"broker"` を direct 書込することは禁止 (gateway ADR 0037 Axis 1 二分割: `broker` は gateway-only attestation)。 producer が `"broker"` を direct 書込すると gateway 側で `spoofed_broker` classification → fail-closed されるため、 helper レベルで「源は env」 と固定することで spoof attempt を構造的に防ぐ。

### 7. Implementation phases

- Phase 0: 本 ADR draft (本変更) + gateway ADR 0037 cross-ref pin
- Phase 1: actortype helper RED→GREEN (4-canonical-value matrix + invalid env error case + initiating env case + emit-fail propagation)
- Phase 2: `ComposeDMail` に stub 追加 + caller integration test (frontmatter 出力検証 + invalid env で emit fail 検証)
- Phase 3: gap-check + sightjack PR
- Phase 4 (別 session): paintress / amadeus copy-sync (substrate byte-identical 維持)
- Phase 5 (別 session / 別 ADR): phonewave (relay-preserve + daemon-generated 区別)

## Enforcement inventory

ADR 0037 §Enforcement inventory に基づく path 網羅:

### Entry points (sightjack producer caller)

- `internal/session/dmail.go` `ComposeDMail` (= 共通 emit entry、 6 caller がここを通過)

### Persistent / carried data needed at each enforcement point

- `metadata.requester_actor_type` (string, 4 canonical values)
- `metadata.requester_actor_source` (string, **`"env"` only** — producer から `broker` 書込禁止、 ADR 0037 Axis 1 二分割対応)
- `metadata.initiating_actor_type` (string, **only when actor=workspace-daemon**)

### Bypass candidates

- env var unset (legacy compat) → frontmatter 出ず → gateway 挙動は path 別: **HIGH = 即 fail-closed (ADR 0037 acceptance day から)**、 非 HIGH = migration window フォールバック (`CallerHumanOperator`、 0038 候補後に flip)
- env var invalid (canonical 4 値外) → **producer 側で emit fail** (= silent escalation 防止、 helper が `ErrInvalidActorType` を返す)
- daemon actor type で initiating env unset → frontmatter に `initiating_actor_type` 出ず → **HIGH path は gateway ADR 0036 4 layer fail-closed の "missing initiating" path で reject**、 非 HIGH path は migration window フォールバック
- daemon actor type で initiating env invalid → **producer 側で emit fail** (= helper が `ErrInvalidInitiatingActorType` を返す)
- daemon actor type で initiating env が daemon 自身を指す (= self-reference) → 本 helper は値検証のみ、 self-reference 拒否は gateway ADR 0036 effective rule の責務 (本 ADR スコープ外)
- producer が `requester_actor_source=broker` を direct 書込 → 本 helper は `"env"` 固定書込のため発生しない (構造的閉塞)

### Tests proving coverage

| Test | Layer | Verifies |
|---|---|---|
| `TestResolve_HumanOperator` | helper unit | env=human-operator → ("human-operator", "env", nil) |
| `TestResolve_GatewayService` | helper unit | env=gateway-service → ("gateway-service", "env", nil) |
| `TestResolve_AIAgent` | helper unit | env=ai-agent → ("ai-agent", "env", nil) |
| `TestResolve_WorkspaceDaemon` | helper unit | env=workspace-daemon → ("workspace-daemon", "env", nil) |
| `TestResolve_Empty` | helper unit | env unset → ("", "", nil) |
| `TestResolve_InvalidValue_Robot` | helper unit | env="robot" → ("", "", ErrInvalidActorType) |
| `TestResolve_InvalidValue_Whitespace` | helper unit | env="   " → ("", "", ErrInvalidActorType) |
| `TestResolve_RejectsBroker` | helper unit | env="broker" → ("", "", ErrInvalidActorType) (= producer から broker 書込禁止の構造的検査) |
| `TestResolveInitiating_HumanOperator` | helper unit | RUNOPS_INITIATING_ACTOR_TYPE=human-operator → ("human-operator", nil) |
| `TestResolveInitiating_Empty` | helper unit | env unset → ("", nil) |
| `TestResolveInitiating_Invalid` | helper unit | invalid → ("", ErrInvalidInitiatingActorType) |
| `TestInjectActorType_Daemon_WithInitiating` | helper unit | actor=daemon + initiating=human → metadata 両方 set + nil err |
| `TestInjectActorType_NonDaemon_NoInitiating` | helper unit | actor=ai-agent + initiating env set → initiating 書かれない (non-daemon は initiating 不要) |
| `TestInjectActorType_InvalidEnv_ReturnsError` | helper unit | env=robot → error returned + metadata unmodified |
| `TestInjectActorType_NilMap_LazyAlloc` | helper unit | nil metadata + valid env → 新規 map allocated |
| `TestComposeDMail_EmitsActorType_Env` | session integration | env set 時に frontmatter に `requester_actor_type` + `requester_actor_source=env` |
| `TestComposeDMail_NoActorType_LegacyCompat` | session integration | env unset 時に frontmatter から行が出ない (byte-identical legacy) |
| `TestComposeDMail_InvalidEnv_FailsEmit` | session integration | env=robot 時に `ComposeDMail` が error を返し、 outbox 書込が起きない (= silent escalation 防止) |

## Consequences

### Positive

- gateway ADR 0035/0036/0037 system-level invariant が producer 経由でも貫通 (AI dispatch が gateway で AI agent として正確に classify)
- 4 ツール copy-sync byte-identical (S0037 substrate canonical lock 維持、 ADR 0016 と同 substrate)
- helper レベルで `broker` source 構造的閉塞 (spoof attempt 防止)
- helper レベルで invalid env emit-fail (silent escalation 防止)
- ADR 0016 と同 helper pattern (= 学習コスト最小、 4 ツール展開時の認知負荷最小)
- legacy compat byte-identical (env unset 時は no-op、 既存 D-Mail と同じ frontmatter)

### Negative

- 4 ツール copy-sync drift リスク (mitigation: gap-check + helper byte-identical 保証)
- env misconfiguration が emit fail を引き起こすため、 daemon 起動 spec / launcher / wrapper の env 設定確認は構築側責務 (= 本 ADR スコープ外、 別 spec で pin)
- daemon HIGH path で initiating env 忘れは gateway 側 fail-closed で停止 (= 良い fail-closed だが daemon 起動 spec の整備必須)

### Neutral

- broker-verified actor classification (= JWT-attested) は gateway 側 token broker 経由で attach される (本 ADR スコープ外)
- 0038 候補 (gateway 側 非 HIGH path fail-closed flip) の trigger は producer rollout 完了 + 観測 telemetry zero (gateway ADR 0036 §Migration で扱う)
- env var の設定責任は producer 外部 (systemd unit / launcher / claude-code wrapper / cdr-job spec) に残る (= 本 ADR スコープ外)

## References

- gateway ADR 0035: AI agent cannot approve AI agent (architectural pin)
- gateway ADR 0036: Phase 4a approval actor validation (effective_requester_actor_type rule、 §Migration の path-split)
- gateway ADR 0037: producer-side actor classification (4 axes、 §Producer-side validation 必須要件、 §Enforcement inventory framework)
- ADR 0016: producer-side `project_id` injection (template precedent、 同 substrate pattern)
- 4 ツール copy-sync 原則: shared ADR S0037 (substrate canonical lock)
- gateway-side `MetadataKeyRequesterActorType`: `internal/core/domain/dmail.go`
- gateway-side `MetadataKeyRequesterActorSource`: `internal/core/domain/dmail.go`
- gateway-side `MetadataKeyInitiatingActorType`: `internal/core/domain/dmail.go`
