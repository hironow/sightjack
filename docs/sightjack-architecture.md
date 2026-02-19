# Sightjack - Architecture Design Document v3

## 概要

**Sightjack** は、完成度30%のIssue群を、AI Coding Agentが並列で実行可能な完成度85%の状態へ引き上げる **Issue Architecturize Tool** である。

残りの15%は開発の進行に伴い動的に発見される非機能要件・UI/UXの変化として許容する（「進みながら考える」領域）。

### 他のツールとの違い

Sightjackは単なるバッチ処理ツールではない。**CLI上でAI Agent Teamと人間がWave（塊）単位で合意形成を繰り返しながら、Issue群の整備とADR（Architectural Decision Records）の蓄積を段階的に進めるセッション型ツール**である。

- **Sightjack** — Issueを「実行可能にする」Agent（見抜く者）
- **下流の実行ツール** — Issueを「実行する」Agent（描く者）

### 名前の由来

PS2ホラーゲーム『SIREN』のコアメカニクス「Sightjack（視界ジャック）」に由来する。原作では敵の視覚を乗っ取り、自分の死角を知る能力。プレイヤーは見えたものに対して判断し行動する。

このツールでも同様に、AIが各Issueの「視界」を横断的に乗っ取って欠落を発見し、人間がその結果に対してWave単位で設計判断を下していく。

---

## SIRENメカニクス → Sightjack機能マッピング

| SIRENメカニクス | ゲームでの役割 | Sightjackでの抽象 | 具体的な機能 |
|---|---|---|---|
| **Sightjack** | 敵/味方の視覚を乗っ取る | Issue群の横断分析 | AIがIssue間の死角（DoD不足・隠れた依存）を見抜く |
| **Link Navigator** | 時系列×人物のマトリクス | **Cluster × Wave のマトリクス** | 人間が「次にどのClusterのどのWaveをやるか」を選ぶUI |
| **シナリオ** | 人物ごとのプレイ可能な章 | **Wave** | AIがClusterごとに動的に生成する作業塊 |
| **人物** | プレイアブルキャラクター | **Issue Cluster** | 同じコード領域・機能群に属するIssueのまとまり |
| **終了条件1** | ステージクリア（脱出） | 機能要件の充足 | Issueの基本的なAcceptance Criteria |
| **終了条件2** | 隠し目標 | 非機能要件・DoD補完 | AIが検出する不足しているDoD項目 |
| **シナリオ解放** | 別人物のクリアで解放 | **Wave解放条件** | Cluster AのWave完了がCluster BのWave解放条件になる |
| **Archive** | 世界観を補完する収集物 | ADR（設計判断記録） | 対話中の設計判断をADRとして自動生成・蓄積 |
| **Shibito蘇生** | 倒した敵が復活する | 技術負債の再発 | 過去にクローズされた類似Issue/パターンの検知 |
| **Fog of War** | 初期は見えない領域 | 未発見の非機能要件 | Waveの進行に伴い動的に出現するDoD（15%の領域） |

---

## コアコンセプト: Link Navigator + Wave Model

### SIRENのLink Navigator

SIRENのLink Navigatorは**人物（縦軸）× 時系列（横軸）**のマトリクスで、プレイヤーが「次にどの人物のどのシナリオをプレイするか」を選ぶ。あるシナリオのクリアが別のシナリオの解放条件になる。

### SightjackのLink Navigator

SightjackではこれをIssue群の整備に転用する。

- **縦軸: Issue Cluster**（人物に相当）— Auth系、API系、DB系のようなまとまり
- **横軸: Wave**（シナリオに相当）— AIがClusterごとに動的に判断する「次にやるべきこと」

```
SightjackのLink Navigator:

                Wave 1          Wave 2          Wave 3          Wave 4
  ┌───────────┬───────────────┬───────────────┬───────────────┬──────────┐
  │ Auth      │ ██ 依存関係   │ ░░ DoD補完    │               │          │
  │ Cluster   │    整理       │    + ADR      │               │          │
  │ (4 issues)│               │               │               │          │
  ├───────────┼───────────────┼───────────────┼───────────────┼──────────┤
  │ API       │ ██ エンドポイ │ ░░ エラー     │               │          │
  │ Cluster   │    ント分割   │    ハンドリング│               │          │
  │ (6 issues)│               │    DoD        │               │          │
  ├───────────┼───────────────┼───────────────┼───────────────┼──────────┤
  │ DB        │ ██ スキーマ   │               │               │          │
  │ Cluster   │    依存整理   │ Auth Wave 2   │               │          │
  │ (3 issues)│               │ が前提        │               │          │
  ├───────────┼───────────────┼───────────────┼───────────────┼──────────┤
  │ Frontend  │ ██ コンポー   │ ░░ API       │               │          │
  │ Cluster   │    ネント分割 │    Cluster    │               │          │
  │ (7 issues)│               │    Wave 1が前提│               │          │
  ├───────────┼───────────────┼───────────────┼───────────────┼──────────┤
  │ Infra     │ ██ 環境構築   │ ░░ 全Cluster  │               │          │
  │ Cluster   │    Issue整理  │    Wave 1完了  │               │          │
  │ (3 issues)│               │    が前提     │               │          │
  └───────────┴───────────────┴───────────────┴───────────────┴──────────┘

  ██ = 着手可能（解放済）    ░░ = 条件付き解放    空 = 未生成
```

### Waveの動的生成

**WaveはAIがClusterごとに動的に決める。** 固定の4段階ではなく、各Clusterの状態に応じて「今このClusterに最も必要なこと」をAIが判断してWaveを生成する。

```go
type Wave struct {
    ID            string
    ClusterID     string
    Title         string          // AIが生成: 「依存関係整理」「DoD補完 + ADR」等
    Description   string          // このWaveで何をやるか
    Actions       []WaveAction    // このWaveで実行する変更群
    Prerequisites []WaveRef       // このWaveの解放条件（他のWave）
    Completeness  CompleteDelta   // このWave完了で期待される完成度変化
    Status        WaveStatus      // Locked | Available | InProgress | Completed
}

type WaveAction struct {
    Type        ActionType  // AddDoD, AddSubIssue, AddDependency, ModifyIssue, CreateADR
    IssueID     string
    Description string
    Payload     interface{}
}

type CompleteDelta struct {
    Before float64  // Wave開始前の推定完成度
    After  float64  // Wave完了後の推定完成度
}

type WaveRef struct {
    ClusterID string
    WaveID    string
}
```

### Waveの生成例

AIがScanner結果を基に、各Clusterに対して以下のように判断する：

```
Auth Cluster (4 issues, 現在25%完成):
  → Wave 1: 「依存関係整理」
    理由: ENG-101→102→108の実行順が未定義。まず構造を固める。
    完成度: 25% → 40%

  → Wave 2: 「認証方式のDoD補完 + ADR」
    理由: JWT vs Session の判断がない。ADRが必要。
    前提: Auth Wave 1
    完成度: 40% → 65%

API Cluster (6 issues, 現在30%完成):
  → Wave 1: 「エンドポイント分割と責務整理」
    理由: 1つのIssueに複数エンドポイントが混在。分割が必要。
    完成度: 30% → 45%

  → Wave 2: 「エラーハンドリング・レスポンス定義DoD」
    理由: 4xx/5xxの定義が全Issue欠落。
    前提: API Wave 1
    完成度: 45% → 70%

DB Cluster (3 issues, 現在35%完成):
  → Wave 1: 「スキーマ依存整理」
    理由: マイグレーション順が未定義。
    完成度: 35% → 50%

  → Wave 2: 「トランザクション境界とロールバックDoD」
    理由: Auth ClusterのJWT判断がDB設計に影響する。
    前提: Auth Wave 2 (ADRが必要)
    完成度: 50% → 75%
```

---

## セッションフロー

### 全体の進行

```
$ sightjack session --project my-project

╔══════════════════════════════════════════════════════════╗
║  Sightjack v1.0 - Link Navigator                        ║
║  Project: my-project | Clusters: 5 | Completeness: 30%   ║
╚══════════════════════════════════════════════════════════╝

[Scan] Issue 23件を取得。5つのClusterを検出。
[Sightjack] 各Clusterを分析中...（goroutine × 5 並列）
[Sightjack] Wave生成完了。Link Navigatorを表示します。

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  LINK NAVIGATOR                    全体完成度: 30%
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  Cluster          │ Wave 1              │ Wave 2
  ─────────────────┼─────────────────────┼──────────────────
  Auth (4)    25%  │ ✅ 依存関係整理     │ 🔒 DoD + ADR
  API  (6)    30%  │ ✅ エンドポイント分割│ 🔒 エラー定義
  DB   (3)    35%  │ ✅ スキーマ依存整理  │ 🔒 Auth W2が前提
  Front(7)    28%  │ ✅ コンポーネント分割│ 🔒 API W1が前提
  Infra(3)    40%  │ ✅ 環境構築整理     │ 🔒 全W1完了が前提

  ✅ = 着手可能  🔒 = 前提条件あり  ⬜ = 未生成

? どのWaveに着手しますか？（番号 or Cluster名）
  1. Auth  - Wave 1: 依存関係整理        (25% → 40%)
  2. API   - Wave 1: エンドポイント分割   (30% → 45%)
  3. DB    - Wave 1: スキーマ依存整理     (35% → 50%)
  4. Front - Wave 1: コンポーネント分割   (28% → 42%)
  5. Infra - Wave 1: 環境構築整理        (40% → 55%)

> 1
```

### Wave実行フロー

```
━━━ Auth Cluster - Wave 1: 依存関係整理 ━━━

  対象Issue:
    ENG-101: ユーザー認証API実装
    ENG-102: JWTトークン管理
    ENG-103: パスワードリセットフロー
    ENG-108: OAuth2プロバイダー連携

[Sightjack] このClusterの分析結果:

  📊 提案する変更 (7件):

  【依存関係の追加】
    1. ENG-101 → ENG-102 (認証APIが先、トークン管理が後)
    2. ENG-101 → ENG-103 (認証APIが先、パスワードリセットが後)
    3. ENG-102 → ENG-108 (トークン管理が先、OAuth2が後)

  【Issue分割の提案】
    4. ENG-101 を分割:
       - ENG-101a: 認証エンドポイント（/login, /logout）
       - ENG-101b: ミドルウェア（認証チェック）

  【DoD追記（構造レベル）】
    5. ENG-101: 「認証フローのシーケンス図または擬似コード」
    6. ENG-102: 「トークンライフサイクルの状態遷移定義」
    7. ENG-108: 「対応するOAuth2プロバイダーの一覧」

  🔄 屍人の蘇生リスク:
    過去のENG-045（旧認証基盤）でトークン管理の
    循環依存が発生した履歴あり。今回の依存順に注意。

  推定完成度変化: 25% → 40%

? この提案をどうしますか？
  ❯ 1. すべて承認して適用
    2. 個別に承認/却下を選ぶ
    3. 提案について議論したい
    4. Wave全体をスキップ

> 3

? 何について議論しますか？
> ENG-101の分割、エンドポイントとミドルウェアを
> 別Issueにする必要あるかな？一緒でよくない？

[Architect] Auth Cluster全体を見ると...

  分割する場合:
  - ミドルウェアはAPI Clusterの全エンドポイントから参照される
  - 別Issueにすると、API ClusterのWave 1で依存を明示できる
  - 下流ツールで並列実行する際、別Agentに割り当て可能

  分割しない場合:
  - Issue数が増えて管理コストが上がる
  - 認証の実装者が一貫してやる方が設計の整合性を保ちやすい

  このプロジェクトの規模（23 issues）だと、分割しない方が
  シンプルかもしれません。ただしミドルウェアのインターフェース定義は
  DoDに追加すべきです。

> じゃあ分割しない。ミドルウェアのインターフェース定義をDoDに入れよう

[Scribe] 設計判断を検知しました:
  「認証APIとミドルウェアは分割せず1 Issueで実装。
   ミドルウェアのインターフェース定義をDoDに追加。」

? ADR-001として記録しますか？
  ❯ 1. はい
    2. いいえ（メモだけ残す）
    3. 内容を修正してから記録

> 1

[Scribe] ADR-001 生成:
  Title: 認証APIとミドルウェアの実装単位
  Status: Accepted
  Context: 23 issueのプロジェクト規模。ミドルウェアはAPI全体から参照される。
  Decision: 分割せずENG-101で一括実装。ミドルウェアのインターフェース定義をDoDに追加。
  Consequences: API Clusterの各IssueがENG-101のミドルウェア仕様に依存する形になる。

  📝 ADR-001をLinearに保存
  📝 ENG-101のDoDに「ミドルウェアのインターフェース定義」を追加

[Sightjack] 提案を修正しました。（分割提案を除外、DoD修正を反映）

  📊 修正後の変更 (7件):

  【依存関係の追加】 ← 変更なし
    1. ENG-101 → ENG-102
    2. ENG-101 → ENG-103
    3. ENG-102 → ENG-108

  【DoD追記】 ← 修正
    4. ENG-101: 「認証フローのシーケンス図」
    5. ENG-101: 「ミドルウェアのインターフェース定義」← ADR-001
    6. ENG-102: 「トークンライフサイクルの状態遷移定義」
    7. ENG-108: 「対応するOAuth2プロバイダーの一覧」

? 修正後の提案を承認しますか？
  ❯ 1. すべて承認して適用
    2. さらに修正
> 1

[Apply] Auth Cluster Wave 1 を適用中...
  ✅ 依存関係 3件 設定完了
  ✅ DoD 4件 追記完了
  ✅ ADR-001 保存完了
  ✅ ラベル sightjack:wave1 付与完了

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  LINK NAVIGATOR 更新                全体完成度: 36%
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  Cluster          │ Wave 1              │ Wave 2
  ─────────────────┼─────────────────────┼──────────────────
  Auth (4)    40%  │ ✅✅ 完了           │ ✅ DoD + ADR ← 解放！
  API  (6)    30%  │ ✅ エンドポイント分割│ 🔒 エラー定義
  DB   (3)    35%  │ ✅ スキーマ依存整理  │ 🔒 Auth W2が前提
  Front(7)    28%  │ ✅ コンポーネント分割│ 🔒 API W1が前提
  Infra(3)    40%  │ ✅ 環境構築整理     │ 🔒 全W1完了が前提

? 次のWaveを選んでください...
```

---

## システムアーキテクチャ

### 全体構成

```
┌────────────────────────────────────────────────────────────┐
│                   Sightjack (Go binary)                     │
│                                                             │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐     │
│  │ CLI          │  │ Link         │  │ Session      │     │
│  │ Dialog       │  │ Navigator    │  │ Manager      │     │
│  │              │  │ Engine       │  │              │     │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘     │
│         │                  │                  │              │
│  ┌──────┴──────────────────┴──────────────────┴──────┐     │
│  │              AI Agent Team                         │     │
│  │                                                    │     │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐        │     │
│  │  │ Scanner  │  │ Architect│  │ Scribe   │        │     │
│  │  │ Agent    │  │ Agent    │  │ Agent    │        │     │
│  │  └────┬─────┘  └────┬─────┘  └────┬─────┘        │     │
│  │       │              │              │               │     │
│  │  ┌────┴──────────────┴──────────────┴────┐         │     │
│  │  │    Claude Code subprocess Pool         │         │     │
│  │  └───────────────┬────────────────────────┘         │     │
│  └──────────────────┼──────────────────────────────────┘     │
│                     │                                        │
└─────────────────────┼────────────────────────────────────────┘
                      │
               ┌──────┴──────────┐
               │ Linear MCP      │
               │ Server          │
               └────────┬────────┘
                        │
               ┌────────┴────────┐
               │   Linear API    │
               └─────────────────┘
```

### コンポーネント詳細

#### Link Navigator Engine

セッションの中核。Cluster × Waveのマトリクスを管理し、解放条件の評価とWave間の依存関係を追跡する。

```go
type LinkNavigator struct {
    clusters   []IssueCluster
    waves      map[string][]Wave     // ClusterID → Waves
    matrix     *NavigatorMatrix      // 表示用マトリクス
    completeness *CompletenessTracker
}

// Wave解放条件の評価
func (ln *LinkNavigator) AvailableWaves() []Wave {
    var available []Wave
    for _, waves := range ln.waves {
        for _, w := range waves {
            if w.Status == Locked && ln.prerequisitesMet(w) {
                w.Status = Available
            }
            if w.Status == Available {
                available = append(available, w)
            }
        }
    }
    return available
}

// Link Navigator完了後に新しいWaveを動的生成
func (ln *LinkNavigator) GenerateNextWaves(ctx context.Context, scanner *ScannerAgent) {
    for _, cluster := range ln.clusters {
        if ln.needsMoreWaves(cluster) {
            newWave := scanner.ProposeNextWave(ctx, cluster, ln.completedWaves(cluster))
            ln.waves[cluster.ID] = append(ln.waves[cluster.ID], newWave)
        }
    }
}
```

#### Session Manager

セッション全体のライフサイクルを管理する。中断・再開をサポートする。

```go
type Session struct {
    ID          string
    navigator   *LinkNavigator
    agents      *AgentTeam
    state       *SessionState
    config      *Config
}

type SessionState struct {
    CompletedWaves []WaveRef
    PendingActions []WriteAction
    ADRs           []ADR
    Decisions      []Decision
    DialogHistory  []DialogEntry
    Completeness   float64
}

func (s *Session) Run(ctx context.Context) error {
    // Phase 1: Initial Scan（自動・並列）
    scanResult := s.agents.Scanner.ScanAll(ctx)
    s.navigator = s.buildNavigator(scanResult)

    // Phase 2: Wave Loop（人間が選択→AI提示→合意→適用→繰り返し）
    for !s.isComplete() {
        s.displayNavigator()
        selectedWave := s.promptWaveSelection()

        proposal := s.agents.Scanner.AnalyzeWave(ctx, selectedWave)
        s.displayProposal(proposal)

        choice := s.promptHumanChoice()

        switch choice {
        case ApproveAll:
            s.applyWave(ctx, proposal)
        case ApproveSelective:
            filtered := s.promptSelectiveApproval(proposal)
            s.applyWave(ctx, filtered)
        case Discuss:
            s.runArchitectDialog(ctx, selectedWave, proposal)
        case SkipWave:
            continue
        }

        // Wave完了後: Link Navigator更新 + 新Wave動的生成
        s.navigator.MarkCompleted(selectedWave)
        s.navigator.GenerateNextWaves(ctx, s.agents.Scanner)
    }

    // Phase 3: Final Summary
    s.displayFinalSummary()
    return nil
}
```

### AI Agent Team

#### Scanner Agent（視界ジャック担当）

```go
type ScannerAgent struct {
    runner *ClaudeCodeRunner
}

// セッション開始時の全体スキャン
func (a *ScannerAgent) ScanAll(ctx context.Context) *ScanResult

// Clusterに対する次のWaveを提案
func (a *ScannerAgent) ProposeNextWave(ctx context.Context, cluster IssueCluster, completed []Wave) Wave

// 選択されたWaveの詳細分析と変更提案
func (a *ScannerAgent) AnalyzeWave(ctx context.Context, wave Wave) *WaveProposal
```

**起動タイミング**: セッション開始時（全体スキャン）、Wave完了時（次Wave生成）、Wave選択時（詳細分析）。

#### Architect Agent（設計判断サポート担当）

```go
type ArchitectAgent struct {
    runner  *ClaudeCodeRunner
    context *SessionContext
}

// 人間との自由対話。セッション全体の判断履歴・ADRをコンテキストに持つ。
func (a *ArchitectAgent) Discuss(ctx context.Context, topic string, waveContext *WaveContext) *ArchitectResponse

// 設計判断が必要かどうかの判定
func (a *ArchitectAgent) NeedsDecision(proposal *WaveProposal) []DecisionPoint
```

**起動タイミング**: 人間が「議論したい」を選択した時。またはScanner Agentの提案に設計判断が必要な箇所が含まれる時。

#### Scribe Agent（Archive = ADR生成担当）

```go
type ScribeAgent struct {
    runner     *ClaudeCodeRunner
    adrStore   *ADRStore
    sessionLog *SessionLog
}

// 対話ログから設計判断を検知してADR生成
func (a *ScribeAgent) DetectAndGenerate(ctx context.Context, dialog []DialogEntry) *ADR

// ADR生成後、関連IssueへのDoD追加等のアクションを生成
func (a *ScribeAgent) ADRToActions(adr *ADR, cluster IssueCluster) []WaveAction

// 新ADRと既存ADRの矛盾チェック
func (a *ScribeAgent) CheckConsistency(newADR *ADR) []Conflict
```

**起動タイミング**: Architect Agentとの対話中に設計判断が確定した瞬間。

---

## ADR自動生成システム

### ADRが生まれるタイミング

Wave内でArchitect Agentと議論→設計判断が確定→Scribe AgentがADRを自動生成。SIRENのArchiveが「ステージ内の行動の結果として自然に集まる」のと同じ構造。

```
Wave内の対話 → 設計判断確定 → ADR生成 → 関連Issue更新 → Link Navigator更新
```

### ADR構造

```go
type ADR struct {
    ID            string
    Title         string
    Status        string    // Proposed | Accepted | Deprecated | Superseded
    Context       string    // なぜこの判断が必要だったか
    Decision      string    // 何を決めたか
    Consequences  string    // この判断の結果、何が変わるか
    RelatedIssues []string  // 影響を受けるIssue群
    RelatedWaves  []WaveRef // この判断が影響するWave
    CreatedAt     time.Time
    SessionID     string
    WaveID        string    // どのWave実行中に生まれたか
}
```

### ADR → Wave フィードバック

ADRが生まれると、Link Navigatorに波及する:

- 関連IssueのDoDが更新される
- 新しいサブIssueが生成される場合がある
- 他のClusterのWave解放条件や内容が変わる場合がある
- まだ生成されていないWaveの内容にADRが影響する

```
ADR-001: 認証とミドルウェアは分割しない
  ↓
  Auth Cluster:
    ENG-101 DoD追加: 「ミドルウェアのインターフェース定義」
  API Cluster:
    Wave 2の内容が変化: 「ENG-101のミドルウェア仕様を前提としたエラーハンドリング」
  Link Navigator:
    API Cluster Wave 2 の前提条件にAuth Wave 2が追加される可能性
```

---

## SIREN難易度システム → DoD厳格度レベル

| レベル | SIRENでの対応 | Sightjack での挙動 | 適用タイミング |
|---|---|---|---|
| **Level 1: Fog** | 霧の中を進む | DoDの不足を **Warning** として表示のみ | プロトタイプ / Spike |
| **Level 2: Alert** | 屍人の気配を感じる | Must級DoD欠落で **サブIssue提案** | 機能開発中期 |
| **Level 3: Lockdown** | 終了条件2未達で進行不可 | 全DoD必須。依存Issueを **Blocked** に | リリース前 |

厳格度はWave生成にも影響する:

- **Fog**: Waveは構造整理とDoD補完が中心。NFRは提案のみ。
- **Alert**: NFR用のWaveも生成。DoDにMust要件を含む。
- **Lockdown**: 最終整合性チェックWaveが自動追加。ADR間矛盾は即座にブロック。

---

## Claude Code subprocess 設計

### Claude Code subprocess パターン

```go
type ClaudeCodeRunner struct {
    maxConcurrency int
    semaphore      chan struct{}
    mcpConfig      string
}

// バッチ分析用（Scanner Agent）
func (r *ClaudeCodeRunner) Run(ctx context.Context, prompt string) (*Result, error) {
    r.semaphore <- struct{}{}
    defer func() { <-r.semaphore }()

    cmd := exec.CommandContext(ctx, "claude",
        "--print",
        "--output-format", "json",
        "--mcp-config", r.mcpConfig,
        "-p", prompt,
    )
    // stdout/stderr capture → JSON parse → return
}

// 対話用（Architect Agent）: ストリーミング出力
func (r *ClaudeCodeRunner) RunStreaming(ctx context.Context, prompt string, out chan<- string) error {
    cmd := exec.CommandContext(ctx, "claude",
        "--output-format", "stream-json",
        "--mcp-config", r.mcpConfig,
        "-p", prompt,
    )
    // line-by-line streaming to channel
}
```

---

## 設定ファイル

```yaml
# sightjack.yaml
version: "1"

linear:
  team: "ENG"
  project: "my-project"
  cycle: "Q1 2026 Sprint 3"
  filters:
    states: ["backlog", "todo", "in_progress"]
    exclude_labels: ["sightjack:analyzed", "spike"]

execution:
  max_concurrency: 5
  timeout_per_issue: "120s"

strictness:
  default: fog
  overrides:
    - label: "release-candidate"
      level: lockdown
    - label: "spike"
      level: fog

adr:
  storage: "linear_document"     # linear_document | local_markdown | both
  prefix: "ADR"
  auto_detect: true
  consistency_check: true

dod_templates:
  api_endpoint:
    must:
      - "エラーレスポンス（4xx/5xx）の定義"
      - "認証/認可の要件明記"
      - "レスポンスタイム目標値"
    should:
      - "OpenAPI specの更新"
      - "レート制限の考慮"
  database_migration:
    must:
      - "ロールバック手順"
      - "既存データへの影響範囲"
    should:
      - "パフォーマンス影響の計測方法"

session:
  auto_save: true               # セッション状態の自動保存
  save_path: ".siren/sessions"
```

---

## ディレクトリ構成

フラット構造で始める。ファイル数が増えて見通しが悪くなった時点でパッケージに切り出す。

```
sightjack/
├── cmd/
│   └── sightjack/
│       └── main.go                  # エントリポイント
├── claude.go                        # Claude Codeセッション管理
├── strategy.go                      # ProcessStrategy + 実装
├── navigator.go                     # Link Navigator Engine
├── session.go                       # セッション管理（メインループ）
├── state.go                         # 状態ファイル + Linear復元
├── scanner.go                       # Scanner Agent
├── architect.go                     # Architect Agent
├── scribe.go                        # Scribe Agent + ADR生成
├── cli.go                           # CLI表示・入力受付
├── config.go                        # sightjack.yaml パーサー
├── model.go                         # Wave/Cluster/ADR等のモデル
├── prompt.go                        # Go template レンダラー
├── prompts/
│   └── templates/
│       ├── scanner_scan.md.tmpl     # 全体スキャン用（Go template）
│       ├── scanner_wave.md.tmpl     # Wave分析・提案用（Go template）
│       ├── architect.md.tmpl        # 設計判断対話用（Go template）
│       ├── scribe_detect.md.tmpl    # 設計判断検知用（Go template）
│       └── scribe_adr.md.tmpl       # ADR生成用（Go template）
├── sightjack.yaml
├── .siren/
│   └── state.json
├── go.mod
├── go.sum
├── justfile
└── README.md
```

---

## 将来の拡張（下流ツール連携）

Sightjack単体完結後、以下の接続面で下流の実行ツールと連携可能:

1. **Sightjack → 下流ツール**: Wave完了後に `sightjack:ready` ラベルのIssueを下流ツールが自動PickUp
2. **下流ツール → Sightjack**: 実装中に発見した非機能要件で新Waveを動的追加（Fog of Warの晴れ）
3. **共通ADRストア**: 下流ツールがPR作成時にADRを参照し設計判断との整合性を確認
4. **共通学習DB**: 下流ツールの学習パターンDBをSightjackが参照し実装知見をWave提案に反映

---

## 開発ロードマップ

### v0.1: Scanner + Link Navigator Skeleton
- Linear MCP Server接続
- Issue一括取得・Cluster検出
- 基本的なLink Navigator表示（CLI）
- Scanner Agentの全体スキャン

### v0.2: Wave Generation + Execution
- AIによるWave動的生成
- Wave単位の提案・承認・適用フロー
- Wave解放条件の評価

### v0.3: Architect Agent + Discussion
- Wave内での設計判断対話モード
- Cluster全体を考慮した推奨

### v0.4: Scribe Agent + ADR
- 設計判断の自動検知
- ADR生成・Linear保存
- ADR → Issue/Wave フィードバック

### v0.5: Session Persistence + SIREN Mechanics
- セッション中断・再開
- 厳格度レベルシステム（Fog/Alert/Lockdown）
- 屍人の蘇生チェック
- ADR間の矛盾検知

### v1.0: Production Ready
- 安定したセッション管理
- エラーハンドリング・リトライ
- 完成度トラッキングの精度向上
- 下流ツール連携インターフェース定義
