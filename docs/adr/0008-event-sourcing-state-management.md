# 0008. Event Sourcing State Management

**Date:** 2026-02-24
**Status:** Accepted

## Context

sightjack の状態管理はスナップショットモデル（`.siren/state.json` に最新の
`SessionState` を丸ごと上書き保存）を採用していた。この方式では：

- **履歴が残らない** — 最後の状態のみが記録され、何が起きたかの時系列が失われる
- **クラッシュ耐性が弱い** — 書き込み中にクラッシュすると直前の操作が失われる
- **意思決定過程の追跡が不可能** — セッション中の承認・却下・変更の経緯が残らない
- **Point-in-time recovery が不可能** — 任意時点への復元ができない

制約として AWS・DB は不使用。ファイルベースのみで実現する必要がある。

## Decision

Event Sourcing パターンを導入する。

- 不変イベント列（JSONL ファイル）を **唯一の source of truth** とする
- 現在の `SessionState` はイベント列の射影（Materialized View）として再構築する
- イベントストアは `.siren/events/{sessionID}.jsonl` に append-only で記録
- スナップショットは不要（典型的なセッションは 10〜50 イベント程度）
- 既存の `state.json` フォーマットとの後方互換は維持しない
- Pipe パターン（UNIX pipe chain）は維持する（JSON wire format は不変）

主要コンポーネント：

- `Event` 封筒型 + 17 種類の `EventType`（session_started, scan_completed, waves_generated 等）
- `FileEventStore` — JSONL append-only、`O_APPEND|O_CREATE|O_WRONLY` + `fsync`
- `SessionRecorder` — EventStore ラッパー、自動連番管理
- `ProjectState` / `LoadState` — イベント再生による状態復元

## Consequences

### Positive

- 完全な操作履歴が残り、セッションの意思決定過程を後から追跡可能
- append-only によりクラッシュ耐性が向上（部分書き込みはスキップ可能）
- Point-in-time recovery が可能（任意の sequence までリプレイ）
- デバッグ・監査が容易（`cat events/*.jsonl | jq` で全履歴参照）

### Negative

- 状態の取得にフルリプレイが必要（ただしイベント数が少ないため実質的なコストは無視できる）
- `WriteState` / `ReadState` に依存する全てのコードパスを更新する必要がある
- 既存の `state.json` との後方互換がなくなる（移行パスなし）

### Neutral

- Pipe コマンドの JSON wire format は不変のため、pipe ユーザーへの影響はない
- scan キャッシュ（`.siren/.run/` 配下の `scan_result.json`）は引き続き独立して機能する
