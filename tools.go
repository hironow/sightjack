package sightjack

// Base tools (e.g. filesystem access, basic bash) that
// are generally useful and safe to
var BaseAllowedTools = []string{
	"Read",
	"Write",
	"Bash(ls:*)",
	"Bash(wc:*)",
	"Bash(find:*)",
	"Bash(echo:*)",
	"Bash(sort:*)",
	"Bash(grep:*)",
	"Bash(awk:*)",
	"Bash(sed:*)",
	"Bash(head:*)",
	"Bash(cat:*)",
}

// GitHub CLI tools (git, gh) and GitHub WebFetch tools that
// sightjack commonly uses for GitHub-related tasks.
var GHAllowedTools = []string{
	"Bash(git:*)",
	"Bash(gh:*)",
	"WebFetch(domain:github.com)",
	"WebFetch(domain:raw.githubusercontent.com)",
}

// LinearMCPAllowedTools lists the official Linear MCP server tools that
// sightjack needs. Passing this via WithAllowedTools prevents context
// explosion from unrelated plugins loading hundreds of tool definitions
// (see anthropics/claude-code#25857).
var LinearMCPAllowedTools = []string{
	"mcp__linear__list_issues",
	"mcp__linear__get_issue",
	"mcp__linear__create_issue",
	"mcp__linear__update_issue",
	"mcp__linear__list_issue_statuses",
	"mcp__linear__get_issue_status",
	"mcp__linear__list_issue_labels",
	"mcp__linear__create_issue_label",
	"mcp__linear__list_comments",
	"mcp__linear__create_comment",
	"mcp__linear__list_projects",
	"mcp__linear__get_project",
	"mcp__linear__save_project",
	"mcp__linear__list_project_labels",
	"mcp__linear__list_teams",
	"mcp__linear__get_team",
	"mcp__linear__list_users",
	"mcp__linear__get_user",
	"mcp__linear__list_cycles",
	// "mcp__linear__list_documents",
	// "mcp__linear__get_document",
	// "mcp__linear__create_document",
	// "mcp__linear__update_document",
	// "mcp__linear__list_milestones",
	// "mcp__linear__get_milestone",
	// "mcp__linear__create_milestone",
	// "mcp__linear__update_milestone",
	// "mcp__linear__get_attachment",
	// "mcp__linear__create_attachment",
	// "mcp__linear__delete_attachment",
	// "mcp__linear__extract_images",
	"mcp__linear__search_documentation",
}
