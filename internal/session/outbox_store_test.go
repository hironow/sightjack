package session_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	sightjack "github.com/hironow/sightjack"
	"github.com/hironow/sightjack/internal/session"
)

func TestSQLiteOutboxStore_PragmaSynchronousNormal(t *testing.T) {
	// given
	dir := t.TempDir()
	session.EnsureMailDirs(dir)
	store, err := session.NewOutboxStoreForBase(dir)
	if err != nil {
		t.Fatalf("create outbox store: %v", err)
	}
	defer store.Close()

	// when: query PRAGMA on the store's own connection
	var synchronous string
	if err := store.DBForTest().QueryRow("PRAGMA synchronous").Scan(&synchronous); err != nil {
		t.Fatalf("query PRAGMA synchronous: %v", err)
	}

	// then: synchronous = 1 (NORMAL)
	if synchronous != "1" {
		t.Errorf("PRAGMA synchronous: got %q, want %q (NORMAL)", synchronous, "1")
	}
}

func TestNewSQLiteOutboxStore_CreatesAllDirectories(t *testing.T) {
	// given: non-existent archive and outbox directories
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "db", "outbox.db")
	archiveDir := filepath.Join(dir, "nonexistent", "archive")
	outboxDir := filepath.Join(dir, "nonexistent", "outbox")

	// when: construct store
	store, err := session.NewSQLiteOutboxStore(dbPath, archiveDir, outboxDir)
	if err != nil {
		t.Fatalf("NewSQLiteOutboxStore: %v", err)
	}
	defer store.Close()

	// then: all directories exist
	for _, d := range []string{filepath.Dir(dbPath), archiveDir, outboxDir} {
		info, statErr := os.Stat(d)
		if statErr != nil {
			t.Errorf("directory %q not created: %v", d, statErr)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%q is not a directory", d)
		}
	}
}

func TestSQLiteOutboxStore_StageAndFlush(t *testing.T) {
	// given
	dir := t.TempDir()
	session.EnsureMailDirs(dir)
	store := testOutboxStore(t, dir)

	// when: stage a d-mail
	err := store.Stage("test-mail.md", []byte("hello"))
	if err != nil {
		t.Fatalf("Stage: %v", err)
	}

	// when: flush
	n, err := store.Flush()
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}

	// then: one item flushed
	if n != 1 {
		t.Errorf("flushed count: got %d, want 1", n)
	}

	// then: archive file exists with correct content
	archivePath := filepath.Join(sightjack.MailDir(dir, sightjack.ArchiveDir), "test-mail.md")
	data, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("read archive: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("archive content: got %q, want %q", string(data), "hello")
	}

	// then: outbox file exists with correct content
	outboxPath := filepath.Join(sightjack.MailDir(dir, sightjack.OutboxDir), "test-mail.md")
	data, err = os.ReadFile(outboxPath)
	if err != nil {
		t.Fatalf("read outbox: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("outbox content: got %q, want %q", string(data), "hello")
	}
}

func TestSQLiteOutboxStore_StageIdempotent(t *testing.T) {
	// given
	dir := t.TempDir()
	session.EnsureMailDirs(dir)
	store := testOutboxStore(t, dir)

	// when: stage the same name twice with different data
	if err := store.Stage("dup.md", []byte("first")); err != nil {
		t.Fatalf("Stage 1: %v", err)
	}
	if err := store.Stage("dup.md", []byte("second")); err != nil {
		t.Fatalf("Stage 2: %v", err)
	}

	// when: flush
	n, err := store.Flush()
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}

	// then: only one item flushed (INSERT OR IGNORE keeps first)
	if n != 1 {
		t.Errorf("flushed count: got %d, want 1", n)
	}

	// then: content is the first version (INSERT OR IGNORE)
	outboxPath := filepath.Join(sightjack.MailDir(dir, sightjack.OutboxDir), "dup.md")
	data, err := os.ReadFile(outboxPath)
	if err != nil {
		t.Fatalf("read outbox: %v", err)
	}
	if string(data) != "first" {
		t.Errorf("content: got %q, want %q", string(data), "first")
	}
}

func TestSQLiteOutboxStore_FlushEmpty(t *testing.T) {
	// given: empty store
	dir := t.TempDir()
	session.EnsureMailDirs(dir)
	store := testOutboxStore(t, dir)

	// when
	n, err := store.Flush()

	// then: no error, zero items
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if n != 0 {
		t.Errorf("flushed count: got %d, want 0", n)
	}
}

func TestSQLiteOutboxStore_FlushOnlyUnflushed(t *testing.T) {
	// given: stage and flush one item, then stage another
	dir := t.TempDir()
	session.EnsureMailDirs(dir)
	store := testOutboxStore(t, dir)

	store.Stage("first.md", []byte("one"))
	store.Flush()

	store.Stage("second.md", []byte("two"))

	// when: flush again
	n, err := store.Flush()

	// then: only the new item is flushed
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if n != 1 {
		t.Errorf("flushed count: got %d, want 1", n)
	}

	// then: both files exist
	for _, name := range []string{"first.md", "second.md"} {
		outboxPath := filepath.Join(sightjack.MailDir(dir, sightjack.OutboxDir), name)
		if _, err := os.Stat(outboxPath); err != nil {
			t.Errorf("outbox %s missing: %v", name, err)
		}
	}
}

// TestSQLiteOutboxStore_ConcurrentStageAndFlush simulates multiple CLI
// instances (separate SQLiteOutboxStore connections to the same DB) performing
// Stage+Flush concurrently. Validates that:
//   - No errors from SQLite locking (WAL + busy_timeout handles contention)
//   - All staged items eventually appear in archive/ and outbox/ (at-least-once)
//   - No data corruption from concurrent atomic writes
func TestSQLiteOutboxStore_ConcurrentStageAndFlush(t *testing.T) {
	// given: shared directory with two independent store connections (simulating 2 CLI processes)
	dir := t.TempDir()
	session.EnsureMailDirs(dir)

	dbPath := filepath.Join(dir, sightjack.StateDir, ".run", "outbox.db")
	archiveDir := sightjack.MailDir(dir, sightjack.ArchiveDir)
	outboxDir := sightjack.MailDir(dir, sightjack.OutboxDir)

	storeA, err := session.NewSQLiteOutboxStore(dbPath, archiveDir, outboxDir)
	if err != nil {
		t.Fatalf("create store A: %v", err)
	}
	defer storeA.Close()

	storeB, err := session.NewSQLiteOutboxStore(dbPath, archiveDir, outboxDir)
	if err != nil {
		t.Fatalf("create store B: %v", err)
	}
	defer storeB.Close()

	const itemsPerStore = 10

	// when: both stores Stage + Flush concurrently
	var wg sync.WaitGroup
	errA := make(chan error, 1)
	errB := make(chan error, 1)

	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := range itemsPerStore {
			name := fmt.Sprintf("a-%03d.md", i)
			if err := storeA.Stage(name, []byte("from-A-"+name)); err != nil {
				errA <- err
				return
			}
			if _, err := storeA.Flush(); err != nil {
				errA <- err
				return
			}
		}
		errA <- nil
	}()
	go func() {
		defer wg.Done()
		for i := range itemsPerStore {
			name := fmt.Sprintf("b-%03d.md", i)
			if err := storeB.Stage(name, []byte("from-B-"+name)); err != nil {
				errB <- err
				return
			}
			if _, err := storeB.Flush(); err != nil {
				errB <- err
				return
			}
		}
		errB <- nil
	}()
	wg.Wait()

	// then: no errors from either store
	if e := <-errA; e != nil {
		t.Fatalf("store A error: %v", e)
	}
	if e := <-errB; e != nil {
		t.Fatalf("store B error: %v", e)
	}

	// then: all 20 files exist in both archive/ and outbox/
	for _, prefix := range []string{"a", "b"} {
		for i := range itemsPerStore {
			name := fmt.Sprintf("%s-%03d.md", prefix, i)
			for _, sub := range []string{sightjack.ArchiveDir, sightjack.OutboxDir} {
				p := filepath.Join(sightjack.MailDir(dir, sub), name)
				data, readErr := os.ReadFile(p)
				if readErr != nil {
					t.Errorf("%s/%s missing: %v", sub, name, readErr)
					continue
				}
				// verify content is not corrupted
				expected := fmt.Sprintf("from-%s-%s", strings.ToUpper(prefix), name)
				if string(data) != expected {
					t.Errorf("%s/%s content: got %q, want %q", sub, name, string(data), expected)
				}
			}
		}
	}
}

// TestSQLiteOutboxStore_ConcurrentFlushSameItem verifies that two stores
// flushing the same unflushed item concurrently results in at-least-once
// delivery with no errors or data corruption.
func TestSQLiteOutboxStore_ConcurrentFlushSameItem(t *testing.T) {
	// given: one item staged, two stores ready to flush
	dir := t.TempDir()
	session.EnsureMailDirs(dir)

	dbPath := filepath.Join(dir, sightjack.StateDir, ".run", "outbox.db")
	archiveDir := sightjack.MailDir(dir, sightjack.ArchiveDir)
	outboxDir := sightjack.MailDir(dir, sightjack.OutboxDir)

	storeSetup, err := session.NewSQLiteOutboxStore(dbPath, archiveDir, outboxDir)
	if err != nil {
		t.Fatalf("create setup store: %v", err)
	}
	if err := storeSetup.Stage("shared.md", []byte("shared-content")); err != nil {
		t.Fatalf("stage: %v", err)
	}
	storeSetup.Close()

	storeA, err := session.NewSQLiteOutboxStore(dbPath, archiveDir, outboxDir)
	if err != nil {
		t.Fatalf("create store A: %v", err)
	}
	defer storeA.Close()

	storeB, err := session.NewSQLiteOutboxStore(dbPath, archiveDir, outboxDir)
	if err != nil {
		t.Fatalf("create store B: %v", err)
	}
	defer storeB.Close()

	// when: both flush concurrently
	var wg sync.WaitGroup
	var nA, nB int
	var eA, eB error

	wg.Add(2)
	go func() {
		defer wg.Done()
		nA, eA = storeA.Flush()
	}()
	go func() {
		defer wg.Done()
		nB, eB = storeB.Flush()
	}()
	wg.Wait()

	// then: no errors
	if eA != nil {
		t.Fatalf("store A flush error: %v", eA)
	}
	if eB != nil {
		t.Fatalf("store B flush error: %v", eB)
	}

	// then: at-least-once — total flushed is 1 or 2 (both may see the item as unflushed)
	total := nA + nB
	if total < 1 || total > 2 {
		t.Errorf("total flushed: got %d (A=%d, B=%d), want 1 or 2", total, nA, nB)
	}

	// then: file exists with correct content
	outboxPath := filepath.Join(sightjack.MailDir(dir, sightjack.OutboxDir), "shared.md")
	data, err := os.ReadFile(outboxPath)
	if err != nil {
		t.Fatalf("read outbox: %v", err)
	}
	if string(data) != "shared-content" {
		t.Errorf("content: got %q, want %q", string(data), "shared-content")
	}
}

func TestSQLiteOutboxStore_MultipleStageThenFlush(t *testing.T) {
	// given: stage multiple items before flush
	dir := t.TempDir()
	session.EnsureMailDirs(dir)
	store := testOutboxStore(t, dir)

	store.Stage("a.md", []byte("aaa"))
	store.Stage("b.md", []byte("bbb"))
	store.Stage("c.md", []byte("ccc"))

	// when
	n, err := store.Flush()

	// then: all three flushed
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if n != 3 {
		t.Errorf("flushed count: got %d, want 3", n)
	}

	// then: all files exist in both dirs
	for _, name := range []string{"a.md", "b.md", "c.md"} {
		for _, sub := range []string{sightjack.ArchiveDir, sightjack.OutboxDir} {
			p := filepath.Join(sightjack.MailDir(dir, sub), name)
			if _, err := os.Stat(p); err != nil {
				t.Errorf("%s/%s missing: %v", sub, name, err)
			}
		}
	}
}
