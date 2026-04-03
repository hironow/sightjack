package filter

import (
	"fmt"
	"strings"

	"github.com/hironow/sightjack/internal/domain"
)

// promptRegistry is the package-level singleton loaded from embedded YAML.
var promptRegistry = MustNewRegistry()

// DefaultRegistry returns the package-level prompt registry.
func DefaultRegistry() *Registry {
	return promptRegistry
}

// --- Section builders: pre-compute conditional template sections ---

// BuildClassifyModeSection returns the mode-specific introduction for classify.
func BuildClassifyModeSection(isWaveMode bool, lang string) string {
	if lang == "ja" {
		if isWaveMode {
			return "`gh` CLI を使って現在のリポジトリの GitHub Issues を取得し、\n論理的なクラスタ（機能グループ）に分類してください。"
		}
		return "Linear MCP Server を使って指定プロジェクトの Issue を取得し、\n論理的なクラスタ（機能グループ）に分類してください。"
	}
	if isWaveMode {
		return "Use the `gh` CLI to fetch GitHub Issues from the current repository\nand classify them into logical clusters (functional groups)."
	}
	return "Use the Linear MCP Server to fetch Issues from the specified project\nand classify them into logical clusters (functional groups)."
}

// BuildClassifyFilterSection returns the pre-computed filter criteria section.
func BuildClassifyFilterSection(isWaveMode bool, teamFilter, projectFilter, cycleFilter, lang string) string {
	var b strings.Builder
	if isWaveMode {
		if teamFilter != "" || projectFilter != "" {
			if lang == "ja" {
				b.WriteString("- 注意: wave modeではチーム/プロジェクトフィルターは参考情報です。必要に応じてGitHubラベルでフィルタしてください。\n")
			} else {
				b.WriteString("- Note: Team/Project filters are informational in wave mode. Use GitHub labels to filter if needed.\n")
			}
			if teamFilter != "" {
				if lang == "ja" {
					b.WriteString("- チーム: " + teamFilter + "\n")
				} else {
					b.WriteString("- Team: " + teamFilter + "\n")
				}
			}
			if projectFilter != "" {
				if lang == "ja" {
					b.WriteString("- プロジェクト: " + projectFilter + "\n")
				} else {
					b.WriteString("- Project: " + projectFilter + "\n")
				}
			}
		}
	} else {
		if lang == "ja" {
			b.WriteString("- チーム: " + teamFilter + "\n")
			b.WriteString("- プロジェクト: " + projectFilter + "\n")
		} else {
			b.WriteString("- Team: " + teamFilter + "\n")
			b.WriteString("- Project: " + projectFilter + "\n")
		}
	}
	if cycleFilter != "" {
		if lang == "ja" {
			b.WriteString("- サイクル: " + cycleFilter + "\n")
		} else {
			b.WriteString("- Cycle: " + cycleFilter + "\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// BuildClassifyStepsSection returns the mode-specific steps section.
func BuildClassifyStepsSection(isWaveMode bool, lang string) string {
	if lang == "ja" {
		if isWaveMode {
			return `1. ` + "`gh issue list --state open --json number,title,labels,body,state --limit 500`" + ` でオープンIssueを取得してください。ページネーションが必要な場合は全件取得すること。注意: ` + "`gh`" + `が正しいリポジトリを対象にするよう、プロジェクトディレクトリから実行すること
2. Issue のタイトル・ラベル・説明から論理的なクラスタに分類してください
3. 各クラスタの名前と所属する Issue ID（GitHub issue番号）のリストを作成してください
4. 各クラスタについて、所属する全Issueの GitHub ラベルの和集合を "labels" 配列に含めてください
5. 下記の屍人チェック用に、クローズ済みIssueも取得: ` + "`gh issue list --state closed --json number,title,labels,body,state --limit 200`" + `
6. **古いラベルの清掃**: ` + "`paintress:pr-open`" + ` ラベルがついたIssueについて、まだオープンなPRが存在するか確認してください（` + "`gh pr list --state open --search \"issue番号\" --limit 5`" + `）。オープンPRがなければ古いラベルを除去: ` + "`gh issue edit <番号> --remove-label \"paintress:pr-open\"`" + `（PRがマージまたはクローズされた場合）`
		}
		return `1. Linear MCP で Issue 一覧を取得してください
2. Issue のタイトル・ラベル・説明から論理的なクラスタに分類してください
3. 各クラスタの名前と所属する Issue ID のリストを作成してください
4. 各クラスタについて、所属する全Issueの Linear ラベルの和集合を "labels" 配列に含めてください`
	}
	if isWaveMode {
		return `1. Fetch OPEN issues using ` + "`gh issue list --state open --json number,title,labels,body,state --limit 500`" + `. Paginate if needed. Ensure ` + "`gh`" + ` targets the correct repository by running from the project directory
2. Classify open Issues into logical clusters based on title, labels, and description
3. Create a list of cluster names with their associated Issue IDs (use GitHub issue numbers as IDs)
4. For each cluster, collect the union of all GitHub labels from its issues into the "labels" array
5. For the Shibito check below, also fetch closed issues: ` + "`gh issue list --state closed --json number,title,labels,body,state --limit 200`" + `
6. **Stale label cleanup**: For issues with the ` + "`paintress:pr-open`" + ` label, check if they still have an OPEN pull request (use ` + "`gh pr list --state open --search \"issue_number\" --limit 5`" + `). If no open PR references the issue, remove the stale label: ` + "`gh issue edit <number> --remove-label \"paintress:pr-open\"`" + ` (the PR was merged or closed)`
	}
	return `1. Fetch the Issue list via Linear MCP
2. Classify Issues into logical clusters based on title, labels, and description
3. Create a list of cluster names with their associated Issue IDs
4. For each cluster, collect the union of all Linear labels from its issues into the "labels" array`
}

// BuildDeepScanModeNote returns the mode-specific note for deep scan.
func BuildDeepScanModeNote(isWaveMode bool, lang string) string {
	if lang == "ja" {
		if isWaveMode {
			return "各Issueについて、GitHubの現在のissue stateを \"status\" フィールドに、全てのGitHubラベルを \"labels\" 配列に含めてください。"
		}
		return "各Issueについて、Linearの現在のステータスを \"status\" フィールドに含めてください。"
	}
	if isWaveMode {
		return "For each issue, include the current GitHub issue state in the \"status\" field and all GitHub labels in the \"labels\" array."
	}
	return "For each issue, include the current Linear status in the \"status\" field."
}

// BuildWaveApplyModeIntro returns the mode-specific introduction for wave apply.
func BuildWaveApplyModeIntro(isWaveMode bool, lang string) string {
	if lang == "ja" {
		if isWaveMode {
			return "承認されたWaveのアクションを `gh` CLI 経由でIssueに適用してください。"
		}
		return "承認されたWaveのアクションをLinear MCP Server経由でIssueに適用してください。"
	}
	if isWaveMode {
		return "Apply the approved Wave actions to Issues via the `gh` CLI."
	}
	return "Apply the approved Wave actions to Issues via the Linear MCP Server."
}

// BuildWaveApplyStepsSection returns the mode-specific steps for wave apply.
func BuildWaveApplyStepsSection(isWaveMode bool, lang string) string {
	if lang == "ja" {
		if isWaveMode {
			return `1. ` + "`add_dod`" + `: ` + "`gh issue edit <番号> --body`" + ` でIssueのbodyにDoD項目を追記する
2. ` + "`add_dependency`" + `: Issue bodyに依存関係を記録する
3. ` + "`add_label`" + `: ` + "`gh issue edit <番号> --add-label`" + ` でラベルを付与する
4. ` + "`update_description`" + `: ` + "`gh issue edit <番号> --body`" + ` でIssueのbodyを更新する
5. ` + "`create`" + `: ` + "`gh issue create --title ... --body ...`" + ` で新しいサブIssueを作成する。bodyに "Parent: #<親Issue番号>" を含めて親子関係を保持すること
6. ` + "`cancel`" + `: ` + "`gh issue close <番号>`" + ` でIssueをクローズする。必須事項:
   a. Issueがオープンであることを確認（既にクローズ済みならREJECT）
   b. ` + "`gh issue comment`" + ` でキャンセル理由をコメント追記
   c. Issueをクローズ`
		}
		return `1. ` + "`add_dod`" + `: IssueのDescriptionにDoD項目を追記する
2. ` + "`add_dependency`" + `: Linear MCPでIssue間の関連を設定する
3. ` + "`add_label`" + `: Linear MCPでラベルを付与する
4. ` + "`update_description`" + `: IssueのDescriptionを更新する
5. ` + "`create`" + `: 親Issueの下に新しいサブIssueをLinear MCP経由で作成する (mcp__linear__create_issue)
6. ` + "`cancel`" + `: Linear MCP経由でIssueをキャンセルする。必須事項:
   a. IssueステータスがBacklogまたはTodoであることを確認（In Progress以降はREJECT）
   b. キャンセル理由（アクションのdetailフィールドから）をコメントとして追記
   c. Issueステータスを Cancelled に設定`
	}
	if isWaveMode {
		return `1. ` + "`add_dod`" + `: Append DoD items to the Issue body using ` + "`gh issue edit <number> --body`" + `
2. ` + "`add_dependency`" + `: Record dependency relationship in the Issue body
3. ` + "`add_label`" + `: Add a label using ` + "`gh issue edit <number> --add-label`" + `
4. ` + "`update_description`" + `: Update the Issue body using ` + "`gh issue edit <number> --body`" + `
5. ` + "`create`" + `: Create a new sub-issue using ` + "`gh issue create --title ... --body ...`" + `. Include "Parent: #<parent-number>" in the body to preserve the parent-child relationship
6. ` + "`cancel`" + `: Close the Issue using ` + "`gh issue close <number>`" + `. MUST:
   a. Verify issue is open (REJECT if already closed)
   b. Add a comment with the cancellation reason using ` + "`gh issue comment`" + `
   c. Close the issue`
	}
	return `1. ` + "`add_dod`" + `: Append DoD items to the Issue description
2. ` + "`add_dependency`" + `: Set Issue relationships via Linear MCP
3. ` + "`add_label`" + `: Add a label via Linear MCP
4. ` + "`update_description`" + `: Update the Issue description
5. ` + "`create`" + `: Create a new sub-issue under the parent issue via Linear MCP (mcp__linear__create_issue)
6. ` + "`cancel`" + `: Cancel the Issue via Linear MCP. MUST:
   a. Verify issue status is Backlog or Todo (REJECT if In Progress or beyond)
   b. Add a comment with the cancellation reason (from action detail field)
   c. Set issue status to Cancelled`
}

// BuildDoDSection wraps a DoD section with a header if non-empty.
func BuildDoDSection(dodSection, lang string) string {
	if dodSection == "" {
		return ""
	}
	if lang == "ja" {
		return "## 完成基準（DoD）ガイドライン\n\nこのクラスタには以下のDoD基準が適用されます:\n\n" + dodSection + "適用するアクションがこれらの基準に沿っていることを確認してください。"
	}
	return "## Definition of Done Guidelines\n\nThe following DoD standards apply to this cluster:\n\n" + dodSection + "Ensure proposed wave actions align with these standards."
}

// BuildExistingADRsSection formats existing ADRs into a prompt section.
func BuildExistingADRsSection(adrs []domain.ExistingADR, lang string) string {
	if len(adrs) == 0 {
		return ""
	}
	var b strings.Builder
	if lang == "ja" {
		b.WriteString("\n## 既存ADR（尊重すべき設計上の決定）\n")
	} else {
		b.WriteString("\n## Existing ADRs (design decisions to respect)\n")
	}
	for _, adr := range adrs {
		b.WriteString("### " + adr.Filename + "\n")
		b.WriteString(adr.Content + "\n")
	}
	return b.String()
}

// BuildRejectedActionsSection formats rejected actions into a prompt section.
func BuildRejectedActionsSection(rejectedActions, lang string) string {
	if rejectedActions == "" {
		return ""
	}
	if lang == "ja" {
		return "\n## 前回拒否されたアクション\n以下のアクションは前回のWaveでユーザーに拒否されました。同じアクションを再提案しないでください。\n" + rejectedActions
	}
	return "\n## Previously Rejected Actions\nThe user rejected these actions in a previous wave. Do NOT re-propose the same actions.\n" + rejectedActions
}

// BuildFeedbackSection formats feedback into a prompt section.
func BuildFeedbackSection(feedback, lang string) string {
	if feedback == "" {
		return ""
	}
	if lang == "ja" {
		return "\n## 受信フィードバック\n協働エージェントから以下のフィードバックを受信しました。関連する場合はWave提案に反映してください。\n" + feedback
	}
	return "\n## Received Feedback\nThe following feedback was received from collaborating agents. Incorporate these observations into your wave proposals where relevant.\n" + feedback
}

// BuildReportSection formats cross-tool reports into a prompt section.
func BuildReportSection(report, lang string) string {
	if report == "" {
		return ""
	}
	if lang == "ja" {
		return "\n## クロスツールレポート\n他ツールから以下のレポートを受信しました。これらはチェック結果やdrift検出など、自動分析の結果です。\nWave提案に反映してください。\n" + report
	}
	return "\n## Cross-Tool Reports\nThe following reports were received from other tools. These are automated analysis results such as check outcomes and drift detection.\nIncorporate them into your wave proposals.\n" + report
}

// BuildAutoDiscussFeedbackSection formats design feedback for auto-discuss architect.
func BuildAutoDiscussFeedbackSection(feedback, lang string) string {
	if feedback == "" {
		return ""
	}
	if lang == "ja" {
		return "## 受信したデザインフィードバック\n" + feedback
	}
	return "## Design Feedback Received\n" + feedback
}

// BuildAutoDiscussPriorSection formats prior Devil's Advocate content for architect.
func BuildAutoDiscussPriorSection(priorContent, lang string) string {
	if priorContent == "" {
		return ""
	}
	if lang == "ja" {
		return "## Devil's Advocateからの指摘\n以下の懸念点が提起されました。それぞれに対応してください:\n\n" + priorContent
	}
	return "## Devil's Advocate Challenges\nThe Devil's Advocate raised these concerns. Address each one:\n\n" + priorContent
}

// BuildAutoDiscussInstructionsSection returns instructions based on whether prior content exists.
func BuildAutoDiscussInstructionsSection(hasPrior bool, lang string) string {
	if lang == "ja" {
		if hasPrior {
			return `1. Devil's Advocateが提起した各懸念点に対応する
2. 各ポイントについて明確な根拠で防御または譲歩する
3. 懸念が妥当な場合はそれを認め、トレードオフを説明する`
		}
		return `1. このwaveのアクションが正しいアプローチである理由を説明する
2. 設計根拠と制約を記述する
3. 認識しているトレードオフやリスクを特定する`
	}
	if hasPrior {
		return `1. Address each concern raised by the Devil's Advocate
2. Defend or concede each point with clear reasoning
3. If a concern is valid, acknowledge it and explain the trade-off`
	}
	return `1. Explain why this wave's actions are the right approach
2. Describe the design rationale and constraints
3. Identify any trade-offs or risks you're aware of`
}

// BuildDAExistingADRsSection formats ADRs for the Devil's Advocate prompt.
func BuildDAExistingADRsSection(adrs []domain.ExistingADR, lang string) string {
	if len(adrs) == 0 {
		return ""
	}
	var b strings.Builder
	if lang == "ja" {
		b.WriteString("## 既存のアーキテクチャ決定記録\n以下の確立された決定に照らしてwaveに異議を唱えてください:\n\n")
	} else {
		b.WriteString("## Existing Architecture Decision Records\nChallenge the wave against these established decisions:\n\n")
	}
	for _, adr := range adrs {
		b.WriteString("### " + adr.Filename + "\n")
		b.WriteString(adr.Content + "\n\n")
	}
	return b.String()
}

// BuildDAClaudeMDSection formats CLAUDE.md content for the Devil's Advocate prompt.
func BuildDAClaudeMDSection(claudeMD, lang string) string {
	if claudeMD == "" {
		return ""
	}
	if lang == "ja" {
		return "## プロジェクト原則 (CLAUDE.md)\n以下のプロジェクト原則に照らしてwaveに異議を唱えてください:\n\n" + claudeMD
	}
	return "## Project Principles (CLAUDE.md)\nChallenge the wave against these project principles:\n\n" + claudeMD
}

// BuildDARoundInfo formats the round info string.
func BuildDARoundInfo(roundIndex, totalRounds int, lang string) string {
	if lang == "ja" {
		return fmt.Sprintf("ラウンド %d / %d", roundIndex, totalRounds)
	}
	return fmt.Sprintf("round %d of %d", roundIndex, totalRounds)
}

// BuildDAFinalRoundInstructions returns final round instructions if applicable.
func BuildDAFinalRoundInstructions(isFinal bool, lang string) string {
	if !isFinal {
		return ""
	}
	if lang == "ja" {
		return `これは最終ラウンドです。以下を提供してください:
- 未解決の全懸念事項のサマリー
- 総合評価: このwaveは新しいADRを必要とするか？その理由は？`
	}
	return `This is the final round. Provide:
- A summary of all unresolved concerns
- Your overall assessment: does this wave warrant a new ADR? Why or why not?`
}

// BuildDAOutputFormat returns the output JSON format for Devil's Advocate.
func BuildDAOutputFormat(isFinal bool, lang string) string {
	if lang == "ja" {
		if isFinal {
			return `{
  "content": "あなたの指摘と懸念事項をテキストで記述",
  "open_issues": ["未解決の懸念1", "未解決の懸念2"],
  "adr_recommended": true,
  "adr_recommendation_reason": "ADRが必要/不要な理由"
}`
		}
		return `{
  "content": "あなたの指摘と懸念事項をテキストで記述"
}`
	}
	if isFinal {
		return `{
  "content": "Your challenges and concerns as clear text",
  "open_issues": ["unresolved concern 1", "unresolved concern 2"],
  "adr_recommended": true,
  "adr_recommendation_reason": "Why an ADR is or is not needed"
}`
	}
	return `{
  "content": "Your challenges and concerns as clear text"
}`
}

// BuildScribeExistingADRsSection formats existing ADRs for the Scribe prompt.
func BuildScribeExistingADRsSection(adrs []domain.ExistingADR, lang string) string {
	if len(adrs) == 0 {
		if lang == "ja" {
			return "\n既存のADRはありません。"
		}
		return "\nNo existing ADRs found."
	}
	var b strings.Builder
	for _, adr := range adrs {
		b.WriteString("\n### " + adr.Filename + "\n")
		b.WriteString(adr.Content + "\n")
	}
	return b.String()
}
