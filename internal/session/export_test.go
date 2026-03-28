package session

// white-box-reason: bridge constructor: exposes unexported symbols for external test packages

import (
	"context"
	"database/sql"
	"os/exec"

	"github.com/hironow/sightjack/internal/domain"
)

// SetNewCmd replaces the command constructor for testing and returns a cleanup function.
// This is test infrastructure for injecting fake commands, not a logic shim.
func SetNewCmd(fn func(ctx context.Context, name string, args ...string) *exec.Cmd) func() {
	old := newCmd
	newCmd = fn
	return func() { newCmd = old }
}

// NewLocalNotifierForTest creates a LocalNotifier with test overrides.
func NewLocalNotifierForTest(osName string, factory func(ctx context.Context, name string, args ...string) *exec.Cmd) *LocalNotifier {
	return &LocalNotifier{forceOS: osName, cmdFactory: factory}
}

// NewCmdNotifierForTest creates a CmdNotifier with a test command factory.
func NewCmdNotifierForTest(cmdTemplate string, factory func(ctx context.Context, name string, args ...string) *exec.Cmd) *CmdNotifier {
	return &CmdNotifier{cmdTemplate: cmdTemplate, cmdFactory: factory}
}

// NewCmdApproverForTest creates a CmdApprover with a test command factory.
func NewCmdApproverForTest(cmdTemplate string, factory func(ctx context.Context, name string, args ...string) *exec.Cmd) *CmdApprover {
	return &CmdApprover{cmdTemplate: cmdTemplate, cmdFactory: factory}
}

// DBForTest returns the underlying database connection for testing.
// Only available in test builds.
func (s *SQLiteOutboxStore) DBForTest() *sql.DB { return s.db }

// ReceiveDMailIfNewForTest exposes receiveDMailIfNew for external test packages.
func ReceiveDMailIfNewForTest(baseDir, filename string, logger domain.Logger) *DMail {
	return receiveDMailIfNew(baseDir, filename, logger)
}

// SetDMailUUID overrides the UUID generator for deterministic test filenames.
// Returns a cleanup function to restore the original generator.
func SetDMailUUID(fn func() string) func() {
	old := uuidFunc
	uuidFunc = fn
	return func() { uuidFunc = old }
}

// ShortUUIDForTest exposes shortUUID for tests that need real UUIDs.
var ShortUUIDForTest = shortUUID
