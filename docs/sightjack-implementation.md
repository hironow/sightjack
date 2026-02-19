# Sightjack - Implementation Design

アーキテクチャ設計書・UXコンセプトを踏まえた実装設計。以下の3つの設計判断が前提。

1. **Claude Codeに対話を任せきる** — Go側はオーケストレーター。対話ロジックはClaude Codeのプロンプトに閉じる
2. **Linear Issueが永続化層** — 木構造・依存関係はLinear上に存在する。Sightjack側で二重管理しない
3. **状態ファイルは1つだけ** — 「今どこまで進んだか」の薄いstate。複雑な永続化はしない

---

## 設計原則: Sightjack = Claude Code セッションのオーケストレーター

```
┌─────────────────────────────────────────────────────┐
│  Sightjack (Go binary)                               │
│                                                       │
│  役割:                                                │
│  - Claude Codeプロセスの起動・中断・再開              │
│  - goroutineによる並行・並列処理                      │
│  - 薄い状態ファイルの読み書き                         │
│  - CLI表示（Link Navigator、Wave結果、波及）          │
│                                                       │
│  やらないこと:                                        │
│  - 対話のロジック（Claude Codeに任せる）              │
│  - Issue木構造の管理（Linearに任せる）                │
│  - ADRの構造化保存（Linearドキュメントに任せる）      │
│  - 複雑な状態管理（状態ファイル1つで十分）            │
└─────────────────────────────────────────────────────┘
```

---

## Claude Code セッション管理

### ralph loop パターンの適用

Claude Codeを `--resume` で再開することで、対話コンテキストを引き継ぐ。

```go
type ClaudeSession struct {
    SessionID string   // Claude Codeの--session-id
    MCP       string   // MCP config path
}

// 新規セッション開始
func (cs *ClaudeSession) Start(ctx context.Context, prompt string) (string, error) {
    cs.SessionID = uuid.New().String()

    cmd := exec.CommandContext(ctx, "claude",
        "--print",
        "--output-format", "json",
        "--session-id", cs.SessionID,
        "--mcp-config", cs.MCP,
        "-p", prompt,
    )
    return cs.run(cmd)
}

// セッション再開（中断した対話の続き）
func (cs *ClaudeSession) Resume(ctx context.Context, prompt string) (string, error) {
    cmd := exec.CommandContext(ctx, "claude",
        "--print",
        "--output-format", "json",
        "--session-id", cs.SessionID,
        "--resume",
        "--mcp-config", cs.MCP,
        "-p", prompt,
    )
    return cs.run(cmd)
}
```

### Wave実行の流れ

1つのWaveは以下のようにClaude Codeセッションとして実行される：

```go
func (s *Sightjack) ExecuteWave(ctx context.Context, wave Wave) (*WaveResult, error) {
    session := &ClaudeSession{MCP: s.mcpConfig}

    // Step 1: Wave分析を開始（Claude Codeに全コンテキストを渡す）
    analysisPrompt := s.buildWavePrompt(wave)
    analysis, err := session.Start(ctx, analysisPrompt)
    if err != nil {
        return nil, err
    }

    // Step 2: 分析結果をCLIに表示
    proposal := parseWaveProposal(analysis)
    s.cli.DisplayProposal(proposal)

    // Step 3: 人間の選択を待つ
    choice := s.cli.PromptChoice()

    switch choice {
    case Approve:
        // Step 4a: 承認 → Claude Codeセッションを再開して適用指示
        applyResult, err := session.Resume(ctx,
            "提案を承認します。Linear上のIssueを更新してください。")
        return parseWaveResult(applyResult), err

    case Discuss:
        // Step 4b: 議論 → Claude Codeセッションを再開して議論開始
        return s.runDiscussion(ctx, session, wave, proposal)

    case Skip:
        return nil, nil
    }
}
```

### 議論（Architect Agent対話）の実装

議論はClaude Codeセッションの `--resume` を繰り返すことで実現する。**対話のロジックはClaude Code側のプロンプトに閉じている。** Go側はユーザー入力を受け取ってClaude Codeに渡すだけ。

```go
func (s *Sightjack) runDiscussion(
    ctx context.Context,
    session *ClaudeSession,
    wave Wave,
    proposal *WaveProposal,
) (*WaveResult, error) {

    // 議論開始
    topic := s.cli.PromptFreeText("何について議論しますか？")
    response, _ := session.Resume(ctx, topic)
    s.cli.DisplayResponse(response)

    for {
        input := s.cli.PromptFreeText("（続ける / 'done' で判断確定 / 'back' で戻る）")

        if input == "back" {
            return nil, nil
        }

        if input == "done" {
            // 判断確定 → ADR生成 + Issue更新を指示
            result, err := session.Resume(ctx,
                "議論を終了します。設計判断が確定したものについて："+
                "1. ADRを生成してLinearドキュメントとして保存"+
                "2. 関連IssueのDoDを更新"+
                "3. 波及する他Clusterへの影響を報告"+
                "JSON形式で結果を返してください。")
            return parseWaveResult(result), err
        }

        // 対話を続ける
        response, _ := session.Resume(ctx, input)
        s.cli.DisplayResponse(response)
    }
}
```

### Claude Codeプロセスの抽象化

プロセス分割戦略は後から変更可能にする。初期実装は以下の方針：

```go
type ProcessStrategy interface {
    // スキャン（読み取り）: 並列可能
    Scan(ctx context.Context, clusters []IssueCluster) ([]ScanResult, error)

    // Wave実行（対話あり）: シーケンシャル
    ExecuteWave(ctx context.Context, wave Wave) (*WaveResult, error)
}

// 初期実装: シンプル版
type SimpleStrategy struct {
    mcpConfig string
    semaphore chan struct{} // スキャン時の並行数制御
}

// スキャンはgoroutineで並列
func (s *SimpleStrategy) Scan(ctx context.Context, clusters []IssueCluster) ([]ScanResult, error) {
    results := make([]ScanResult, len(clusters))
    g, gctx := errgroup.WithContext(ctx)

    for i, cluster := range clusters {
        i, cluster := i, cluster
        g.Go(func() error {
            s.semaphore <- struct{}{}
            defer func() { <-s.semaphore }()

            session := &ClaudeSession{MCP: s.mcpConfig}
            result, err := session.Start(gctx, buildScanPrompt(cluster))
            if err != nil {
                return err
            }
            results[i] = parseScanResult(result)
            return nil
        })
    }

    return results, g.Wait()
}

// Wave実行は1プロセスでシーケンシャル
func (s *SimpleStrategy) ExecuteWave(ctx context.Context, wave Wave) (*WaveResult, error) {
    session := &ClaudeSession{MCP: s.mcpConfig}
    // ... 上記のExecuteWaveと同じ
}
```

---

## 永続化設計: Linear = 真実の源泉

### 何をどこに保存するか

| データ | 保存先 | 理由 |
|---|---|---|
| Issue（タイトル、description、DoD） | Linear Issue | 元々Linearにある。二重管理しない |
| Issue間の依存関係 | Linear relation (blocks/blocked by) | Linearの機能をそのまま使う |
| 親子関係 | Linear sub-issue | Linearの機能をそのまま使う |
| ラベル（sightjack:wave1等） | Linear label | Sightjackの進捗をLinear上で追跡可能 |
| ADR | Linear document | プロジェクトに紐づくドキュメントとして自然 |
| ADRとIssueの紐付け | Issue descriptionにADRリンク | シンプル。別管理不要 |
| Wave完了状態 | Linear label + 状態ファイル | ラベルが真実、状態ファイルはキャッシュ |
| セッションの現在位置 | 状態ファイル | Sightjack固有の情報 |

### Linear上の表現

```
Linear Project: my-project
│
├── [Label: sightjack:cluster:auth]     ← Cluster識別
│   ├── ENG-101: ユーザー認証API実装
│   │   ├── [Label: sightjack:wave1:done]
│   │   ├── [Label: sightjack:wave2:done]
│   │   ├── [DoD] ADR-007に準拠したJWT実装
│   │   └── [Relation] blocks → ENG-102
│   ├── ENG-102: JWTトークン管理
│   │   └── ...
│   └── ENG-108: OAuth2プロバイダー連携
│       └── ...
│
├── [Label: sightjack:cluster:api]
│   └── ...
│
├── [Document] ADR-001: 認証APIとミドルウェアの実装単位
├── [Document] ADR-007: JWT + Refresh Token方式の採用
└── ...
```

### Cluster検出ロジック

Clusterの検出もLinear上の情報から行う。独自のグラフ構造は持たない。

```go
func DetectClusters(issues []Issue) []IssueCluster {
    // 方法1: Linear上の既存ラベルを使う
    //   sightjack:cluster:auth → Auth Cluster
    //   （人間が事前にラベル付けしている場合）

    // 方法2: AIが分析して自動クラスタリング
    //   Issue descriptionの類似度、親子関係、依存関係から推定
    //   結果をLinearラベルとして付与

    // 方法3: 親Issueベース
    //   同じ親Issueを持つIssue群を1 Clusterとする
}
```

---

## 状態ファイル: 薄い1ファイル

### 設計方針

- **JSONまたはYAML 1ファイルのみ**
- **Linear上の状態のキャッシュに過ぎない**。状態ファイルが消えてもLinearから復元可能
- **複雑な構造は持たない**。フラット。

### 構造

```json
{
  "version": 1,
  "project": "my-project",
  "session_id": "sj-2026-02-15-001",
  "claude_sessions": {
    "scan_auth": "cs-abc123",
    "scan_api": "cs-def456",
    "wave_auth_w2": "cs-ghi789"
  },
  "completeness": 0.62,
  "clusters": [
    {
      "id": "auth",
      "label": "sightjack:cluster:auth",
      "completeness": 0.65,
      "current_wave": 3
    },
    {
      "id": "api",
      "label": "sightjack:cluster:api",
      "completeness": 0.58,
      "current_wave": 2
    }
  ],
  "completed_waves": [
    {"cluster": "auth", "wave": 1, "completed_at": "2026-02-15T10:30:00Z"},
    {"cluster": "auth", "wave": 2, "completed_at": "2026-02-15T11:15:00Z"},
    {"cluster": "api",  "wave": 1, "completed_at": "2026-02-15T10:45:00Z"}
  ],
  "adr_count": 4
}
```

### 復元ロジック

状態ファイルが消えた場合、Linearから復元する：

```go
func RecoverState(ctx context.Context, linear *LinearClient, project string) (*State, error) {
    issues, _ := linear.GetIssuesWithLabel(ctx, "sightjack:*")

    state := &State{Project: project}

    for _, issue := range issues {
        // ラベルからCluster、Wave進捗を復元
        for _, label := range issue.Labels {
            if strings.HasPrefix(label, "sightjack:cluster:") {
                clusterID := strings.TrimPrefix(label, "sightjack:cluster:")
                state.EnsureCluster(clusterID)
            }
            if strings.HasPrefix(label, "sightjack:wave") {
                // wave1:done, wave2:done 等からWave進捗を復元
                state.MarkWaveFromLabel(label)
            }
        }
    }

    // ADRはLinearドキュメントから数える
    docs, _ := linear.GetDocuments(ctx, project)
    state.ADRCount = countADRs(docs)

    return state, nil
}
```

---

## メインループ実装

```go
func main() {
    cfg := loadConfig("sightjack.yaml")

    // 状態ファイルのロード or 新規作成
    state, err := LoadOrCreateState(cfg)
    if err != nil {
        state = RecoverState(ctx, linear, cfg.Project)
    }

    sj := &Sightjack{
        config:   cfg,
        state:    state,
        strategy: &SimpleStrategy{
            mcpConfig: cfg.MCPConfig,
            semaphore: make(chan struct{}, cfg.MaxConcurrency),
        },
        cli: NewCLI(),
    }

    // メインループ
    for {
        // 1. スキャン（初回 or 差分）
        if state.NeedsScan() {
            scanResults := sj.strategy.Scan(ctx, state.Clusters)
            waves := sj.generateWaves(scanResults)
            state.UpdateWaves(waves)
        }

        // 2. Link Navigator表示
        sj.cli.DisplayNavigator(state)

        // 3. 人間のアクション待ち
        action := sj.cli.PromptAction()

        switch action.Type {
        case SelectWave:
            result, err := sj.strategy.ExecuteWave(ctx, action.Wave)
            if err != nil {
                sj.cli.DisplayError(err)
                continue
            }
            if result != nil {
                state.CompleteWave(action.Wave, result)
                sj.cli.DisplayRipple(result.Ripples) // 波及の演出
            }

        case GoBack:
            wave := sj.cli.PromptSelectCompletedWave(state)
            result, _ := sj.strategy.ExecuteWave(ctx, wave) // 再訪
            if result != nil {
                state.UpdateWave(wave, result)
            }

        case ViewSummary:
            sj.cli.DisplaySummary(state)

        case Quit:
            state.Save()
            return
        }

        // 4. 状態ファイル保存
        state.Save()

        // 5. 完成度チェック
        if state.Completeness >= 0.85 {
            sj.cli.DisplayCompletion(state)
            if sj.cli.ConfirmFinish() {
                sj.applyFinalLabels(ctx, state)
                break
            }
        }

        // 6. 新しいWaveの動的生成（Wave完了で見えてくるもの）
        sj.strategy.Scan(ctx, state.ClustersNeedingNewWaves())
    }
}
```

---

## プロンプト設計

### Wave分析プロンプト（Scanner Agent）

Claude Codeに渡すプロンプト。Linear MCP Serverを使って直接Issueを読み書きする。

```markdown
あなたはSightjackのScanner Agentです。

## タスク
以下のLinear Issue Cluster を分析し、Wave提案を生成してください。

## Cluster情報
- Cluster名: {cluster_name}
- 対象Issue: {issue_ids}
- 現在の完成度: {completeness}%
- 完了済みWave: {completed_waves}
- 既存ADR: {adr_list}

## やること
1. Linear MCP Serverを使って各Issueの詳細を取得してください
2. 以下を分析してください:
   - DoDの充足度（何が足りないか）
   - 隠れた依存関係（明示されていないblocks/blocked byがないか）
   - 設計判断が必要な箇所（ADR候補）
   - 過去の類似Issueとのパターンマッチ（屍人の蘇生リスク）
3. 「次にこのClusterでやるべきこと」をWaveとして提案してください

## 出力形式
JSON形式で返してください:
{
  "wave_title": "...",
  "wave_description": "...",
  "completeness_delta": {"before": 55, "after": 75},
  "actions": [
    {"type": "add_dod", "issue_id": "...", "dod": "...", "priority": "must"},
    {"type": "add_dependency", "from": "...", "to": "...", "reason": "..."},
    {"type": "needs_decision", "topic": "...", "options": [...], "adr_candidate": true},
    {"type": "resurrection_risk", "issue_id": "...", "past_issue": "...", "description": "..."}
  ],
  "prerequisites": [{"cluster": "...", "wave": ...}]
}
```

### Wave議論プロンプト（Architect Agent）

Wave内で人間が「議論したい」を選んだ時のプロンプト。

```markdown
あなたはSightjackのArchitect Agentです。開発者との設計判断の議論パートナーです。

## コンテキスト
- プロジェクト: {project}
- 現在のCluster: {cluster_name}
- このWaveの提案: {wave_proposal}
- 既存ADR: {adr_list}
- セッション中の判断履歴: {past_decisions}

## ルール
- 選択肢を提示する際は、各選択のトレードオフを明確にする
- Cluster全体と他Clusterへの波及を考慮した推奨を出す
- 過去のADRと矛盾する判断にはフラグを立てる
- 判断を強制しない。情報を提供して人間が決める
- 設計判断が確定したら、以下のJSON形式で報告する

## 設計判断が確定した場合の出力
人間が判断を確定したと判断できたら、通常の応答に加えて
以下のJSONブロックを末尾に付与してください:

```json
{"decision_detected": true, "adr": {"title": "...", "context": "...", "decision": "...", "consequences": "..."}}
```

まだ議論中の場合は付与しないでください。

## 人間への最初の質問
「何について議論しますか？」
```

---

## ディレクトリ構成（フラット）

v0.1の規模では `internal/` は過剰。フラット構造で始め、ファイルが増えて見通しが悪くなった時点でパッケージに切り出す。

```
sightjack/
├── cmd/
│   └── sightjack/
│       └── main.go                  # エントリポイント + メインループ
├── claude.go                        # Claude Codeセッション管理（Start/Resume）
├── strategy.go                      # ProcessStrategy interface + SimpleStrategy
├── navigator.go                     # Link Navigator表示ロジック
├── state.go                         # 状態ファイル読み書き + Linear復元
├── cli.go                           # CLI表示・入力受付
├── config.go                        # sightjack.yaml パーサー
├── model.go                         # Wave/Cluster/WaveResult等
├── prompt.go                        # Go template レンダラー
├── prompts/
│   └── templates/
│       ├── scanner.md.tmpl          # Scanner Agent用（Go template）
│       ├── architect.md.tmpl        # Architect Agent用（Go template）
│       └── scribe.md.tmpl           # Scribe Agent用（Go template）
├── sightjack.yaml
├── .sightjack/
│   └── state.json                   # 薄い状態ファイル（1つだけ）
├── go.mod
├── go.sum
├── justfile
└── README.md
```

---

## v0.1 実装スコープ

最小限で「体験のループ」を確認できるもの:

1. **Linear MCP Server接続 + Issue取得**
2. **1 Clusterのスキャン（Claude Code 1プロセス）**
3. **Wave提案の表示（CLI）**
4. **承認 → Linear Issue更新（DoD追記、依存関係設定）**
5. **状態ファイル保存・ロード**
6. **Link Navigator表示（1 Cluster分）**

v0.1では「議論」「ADR生成」「波及の演出」「複数Cluster並列」は含まない。まず「Scan → 提案 → 承認 → 適用 → Navigator更新」の最小ループを回す。
