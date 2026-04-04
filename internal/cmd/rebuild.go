package cmd

import (
	"fmt"
	"sort"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/session"
	"github.com/spf13/cobra"
)

func newRebuildCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rebuild [path]",
		Short: "Rebuild projections from event store",
		Long: `Replays all events from .siren/events/ to regenerate materialized projection state from scratch.
Saves a snapshot after successful replay for faster future recovery.

If path is omitted, the current working directory is used.`,
		Example: `  # Rebuild projections for the current directory
  sightjack rebuild

  # Rebuild projections for a specific project
  sightjack rebuild /path/to/repo`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			baseDir, err := resolveBaseDir(args)
			if err != nil {
				return err
			}
			logger := loggerFrom(cmd)

			// Load all events across sessions via session layer (not eventsource directly)
			events, failedCount, loadErr := session.LoadAllEventsWithStatus(cmd.Context(), baseDir)
			if loadErr != nil {
				return fmt.Errorf("load events: %w", loadErr)
			}
			if failedCount > 0 {
				return fmt.Errorf("rebuild aborted: %d session store(s) could not be read (would produce incomplete snapshot)", failedCount)
			}

			// Sort events chronologically — LoadAllEventsAcrossSessions
			// concatenates per-session stores in directory order, not
			// event timestamp order. Projection is order-sensitive.
			sort.SliceStable(events, func(i, j int) bool {
				return events[i].Timestamp.Before(events[j].Timestamp)
			})

			logger.Info("rebuilding projection from %d event(s)", len(events))

			projector := domain.NewProjector()
			if err := projector.Rebuild(events); err != nil {
				return fmt.Errorf("rebuild: %w", err)
			}

			// Snapshot SeqNr from global counter (not event scan)
			var latestSeqNr uint64
			seqCounter, scErr := session.NewSeqCounter(baseDir)
			if scErr == nil {
				defer seqCounter.Close()
				if seq, seqErr := seqCounter.LatestSeqNr(cmd.Context()); seqErr == nil {
					latestSeqNr = seq
				}
			}

			// Save snapshot via session layer
			snapshotStore := session.NewSnapshotStore(baseDir)
			state, serErr := projector.Serialize()
			if serErr != nil {
				return fmt.Errorf("serialize projection: %w", serErr)
			}
			if err := snapshotStore.Save(cmd.Context(), "sightjack.state", latestSeqNr, state); err != nil {
				return fmt.Errorf("save snapshot: %w", err)
			}

			s := projector.State()
			logger.OK("rebuild complete: %d waves, completeness=%.1f%%, adrs=%d, feedback=%d (snapshot at SeqNr=%d)",
				len(s.Waves), s.Completeness*100, s.ADRCount, s.FeedbackCount, latestSeqNr)
			return nil
		},
	}
}
