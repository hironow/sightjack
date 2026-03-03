package session

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/hironow/sightjack"
	"github.com/hironow/sightjack/internal/domain"
)

// TestRace_OutboxStore_ConcurrentStageAndRead verifies that concurrent
// Stage and query operations do not trigger the race detector.
func TestRace_OutboxStore_ConcurrentStageAndRead(t *testing.T) {
	dir := t.TempDir()
	EnsureMailDirs(dir)
	dbPath := filepath.Join(dir, sightjack.StateDir, ".run", "outbox.db")
	os.MkdirAll(filepath.Dir(dbPath), 0o755)
	archiveDir := sightjack.MailDir(dir, sightjack.ArchiveDir)
	outboxDir := sightjack.MailDir(dir, sightjack.OutboxDir)

	store, err := NewSQLiteOutboxStore(dbPath, archiveDir, outboxDir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer store.Close()

	var wg sync.WaitGroup
	const workers = 10

	for i := range workers {
		wg.Add(2)
		go func(id int) {
			defer wg.Done()
			name := fmt.Sprintf("race-%03d.md", id)
			store.Stage(name, []byte("data"))
		}(i)
		go func() {
			defer wg.Done()
			store.Flush()
		}()
	}
	wg.Wait()
}

// TestRace_FeedbackCollector_ConcurrentAccess verifies that the
// FeedbackCollector mutex protects concurrent field access.
func TestRace_FeedbackCollector_ConcurrentAccess(t *testing.T) {
	ch := make(chan *DMail, 10)
	fc := CollectFeedback(nil, ch, nil, domain.NewLogger(nil, false))

	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			if id%2 == 0 {
				_ = fc.All()
			} else {
				_ = fc.ReportsOnly()
			}
		}(i)
	}

	// Send items concurrently
	for i := range 5 {
		dm := &DMail{Name: fmt.Sprintf("dm-%d", i), Kind: "report"}
		ch <- dm
	}
	close(ch)
	wg.Wait()
}

// TestRace_Logger_ConcurrentWrite verifies that Logger's mutex protects
// concurrent log writes.
func TestRace_Logger_ConcurrentWrite(t *testing.T) {
	logger := domain.NewLogger(nil, false)

	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			logger.Info("concurrent log %d", id)
		}(i)
	}
	wg.Wait()
}
