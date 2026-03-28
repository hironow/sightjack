//go:build e2e

package e2e

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestE2E_Gate_AutoApproveWithConvergence(t *testing.T) {
	// given: convergence d-mail in inbox
	dir := initDir(t)
	ensureMailDirs(t, dir)
	convergenceContent := fixtureBytes(t, "convergence_signal.md")
	writeDMailToDir(t, dir, "inbox", "convergence-arch-review.md", convergenceContent)

	// when: run with --auto-approve (fully non-interactive: gate, wave select, approval all auto)
	// --wait-timeout=-1s disables D-Mail waiting mode to prevent blocking after nextgen.
	cmd := exec.Command(sightjackBin(), "run", "--linear", "--auto-approve", "--wait-timeout=-1s", dir)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	// then: should succeed
	if err != nil {
		t.Fatalf("run --auto-approve failed: %v\nstderr: %s\nstdout: %s", err, stderr.String(), stdout.String())
	}

	// and: convergence d-mail consumed and archived
	inboxPath := filepath.Join(dir, ".siren", "inbox", "convergence-arch-review.md")
	archivePath := filepath.Join(dir, ".siren", "archive", "convergence-arch-review.md")
	assertFileNotExists(t, inboxPath)
	assertFileExists(t, archivePath)

	// and: session proceeded (events exist)
	assertEventsExist(t, dir)

	// and: archived convergence d-mail is parseable with correct kind
	dm := parseDMailFile(t, archivePath)
	if dm.Kind != "convergence" {
		t.Errorf("expected kind=convergence, got %q", dm.Kind)
	}
}

func TestE2E_Gate_ApproveCmdApproves(t *testing.T) {
	// given: convergence d-mail in inbox
	dir := initDir(t)
	ensureMailDirs(t, dir)
	convergenceContent := fixtureBytes(t, "convergence_signal.md")
	writeDMailToDir(t, dir, "inbox", "convergence-arch-review.md", convergenceContent)

	// when: run with --approve-cmd "exit 0" (external command approves)
	runFullSession(t, dir, withFlags("--approve-cmd", "exit 0"))

	// then: convergence d-mail consumed and archived
	assertFileNotExists(t, filepath.Join(dir, ".siren", "inbox", "convergence-arch-review.md"))
	assertFileExists(t, filepath.Join(dir, ".siren", "archive", "convergence-arch-review.md"))

	// and: session proceeded (events exist)
	assertEventsExist(t, dir)
}

func TestE2E_Gate_ApproveCmdDenies(t *testing.T) {
	// given: convergence d-mail in inbox
	dir := initDir(t)
	ensureMailDirs(t, dir)
	convergenceContent := fixtureBytes(t, "convergence_signal.md")
	writeDMailToDir(t, dir, "inbox", "convergence-arch-review.md", convergenceContent)

	// when: run with --approve-cmd "exit 1" (external command denies)
	cmd := exec.Command(sightjackBin(), "run", "--linear", "--approve-cmd", "exit 1", "--wait-timeout=-1s", dir)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	// then: process exits with code 0 (denial is not an error)
	if err != nil {
		t.Fatalf("run should exit 0 on gate denial: %v\nstderr: %s\nstdout: %s",
			err, stderr.String(), stdout.String())
	}

	// and: convergence d-mail was consumed (archived by MonitorInbox initial drain)
	assertFileNotExists(t, filepath.Join(dir, ".siren", "inbox", "convergence-arch-review.md"))
	assertFileExists(t, filepath.Join(dir, ".siren", "archive", "convergence-arch-review.md"))

	// and: session aborted (no events, no scan artifacts)
	assertNoEvents(t, dir)

	// and: no outbox d-mails generated (session never reached wave phase)
	outboxFiles := listMailDir(t, dir, "outbox")
	if len(outboxFiles) > 0 {
		t.Errorf("expected empty outbox after gate denial, got: %v", outboxFiles)
	}
}

func TestE2E_Gate_NotifyCmdInvoked(t *testing.T) {
	// given: convergence d-mail in inbox + notify command that writes to file
	dir := initDir(t)
	ensureMailDirs(t, dir)
	convergenceContent := fixtureBytes(t, "convergence_signal.md")
	writeDMailToDir(t, dir, "inbox", "convergence-arch-review.md", convergenceContent)

	notifyFile := filepath.Join(dir, "notify_output.txt")
	notifyCmd := "echo {title} {message} >> '" + notifyFile + "'"

	// when: run with --auto-approve + --notify-cmd (fully non-interactive)
	cmd := exec.Command(sightjackBin(), "run", "--linear", "--auto-approve", "--wait-timeout=-1s", "--notify-cmd", notifyCmd, dir)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	// then: should succeed
	if err != nil {
		t.Fatalf("run --auto-approve --notify-cmd failed: %v\nstderr: %s\nstdout: %s", err, stderr.String(), stdout.String())
	}

	// then: notification file was created (poll for fire-and-forget goroutine)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(notifyFile); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	assertFileExists(t, notifyFile)

	// and: notification content includes gate title and convergence name
	data, readErr := os.ReadFile(notifyFile)
	if readErr != nil {
		t.Fatalf("read notify output: %v", readErr)
	}
	content := string(data)
	if !strings.Contains(content, "Sightjack Convergence") {
		t.Errorf("notify output missing title, got: %s", content)
	}
	if !strings.Contains(content, "convergence-arch-review") {
		t.Errorf("notify output missing convergence name, got: %s", content)
	}
}
