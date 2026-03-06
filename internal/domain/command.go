package domain

// InitCommand represents the intent to initialize a sightjack project.
type InitCommand struct {
	baseDir    RepoPath
	team       string
	project    string
	lang       string
	strictness string
}

func NewInitCommand(baseDir RepoPath, team, project, lang, strictness string) InitCommand {
	return InitCommand{baseDir: baseDir, team: team, project: project, lang: lang, strictness: strictness}
}

func (c InitCommand) BaseDir() RepoPath  { return c.baseDir }
func (c InitCommand) Team() string       { return c.team }
func (c InitCommand) Project() string    { return c.project }
func (c InitCommand) Lang() string       { return c.lang }
func (c InitCommand) Strictness() string { return c.strictness }

// RunScanCommand represents the intent to run a sightjack scan.
type RunScanCommand struct {
	repoPath RepoPath
	dryRun   bool
}

func NewRunScanCommand(repoPath RepoPath, dryRun bool) RunScanCommand {
	return RunScanCommand{repoPath: repoPath, dryRun: dryRun}
}

func (c RunScanCommand) RepoPath() RepoPath { return c.repoPath }
func (c RunScanCommand) DryRun() bool       { return c.dryRun }

// RunSessionCommand represents the intent to start an interactive session.
type RunSessionCommand struct {
	repoPath RepoPath
	dryRun   bool
}

func NewRunSessionCommand(repoPath RepoPath, dryRun bool) RunSessionCommand {
	return RunSessionCommand{repoPath: repoPath, dryRun: dryRun}
}

func (c RunSessionCommand) RepoPath() RepoPath { return c.repoPath }
func (c RunSessionCommand) DryRun() bool       { return c.dryRun }

// ResumeSessionCommand represents the intent to resume an existing session.
type ResumeSessionCommand struct {
	repoPath  RepoPath
	sessionID SessionID
}

func NewResumeSessionCommand(repoPath RepoPath, sessionID SessionID) ResumeSessionCommand {
	return ResumeSessionCommand{repoPath: repoPath, sessionID: sessionID}
}

func (c ResumeSessionCommand) RepoPath() RepoPath   { return c.repoPath }
func (c ResumeSessionCommand) SessionID() SessionID { return c.sessionID }

// ApplyWaveCommand represents the intent to approve and apply a wave.
type ApplyWaveCommand struct {
	repoPath    RepoPath
	sessionID   SessionID
	clusterName ClusterName
}

func NewApplyWaveCommand(repoPath RepoPath, sessionID SessionID, clusterName ClusterName) ApplyWaveCommand {
	return ApplyWaveCommand{repoPath: repoPath, sessionID: sessionID, clusterName: clusterName}
}

func (c ApplyWaveCommand) RepoPath() RepoPath       { return c.repoPath }
func (c ApplyWaveCommand) SessionID() SessionID     { return c.sessionID }
func (c ApplyWaveCommand) ClusterName() ClusterName { return c.clusterName }

// DiscussWaveCommand represents the intent to discuss a specific wave topic.
type DiscussWaveCommand struct {
	repoPath    RepoPath
	sessionID   SessionID
	clusterName ClusterName
	topic       Topic
}

func NewDiscussWaveCommand(repoPath RepoPath, sessionID SessionID, clusterName ClusterName, topic Topic) DiscussWaveCommand {
	return DiscussWaveCommand{repoPath: repoPath, sessionID: sessionID, clusterName: clusterName, topic: topic}
}

func (c DiscussWaveCommand) RepoPath() RepoPath       { return c.repoPath }
func (c DiscussWaveCommand) SessionID() SessionID     { return c.sessionID }
func (c DiscussWaveCommand) ClusterName() ClusterName { return c.clusterName }
func (c DiscussWaveCommand) Topic() Topic             { return c.topic }
