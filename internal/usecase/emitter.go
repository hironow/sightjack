package usecase

import (
	"context"
	"time"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/usecase/port"
)

// sessionEventEmitter wraps a SessionAggregate with EventStore and EventDispatcher
// to produce and persist domain events. Store and dispatch errors are best-effort
// (logged, not propagated) to preserve session continuity.
// Aggregate errors remain critical and are returned.
type sessionEventEmitter struct {
	agg        *domain.SessionAggregate
	store      port.EventStore
	dispatcher port.EventDispatcher
	logger     domain.Logger
	seqAlloc   port.SeqAllocator
	sessionID  string // enriches events with session metadata
	prevID     string // previous event ID for causation chain
	ctx        context.Context //nolint:containedctx // stored for trace propagation into emit chain
}

// NewSessionEventEmitter creates a SessionEventEmitter that wraps aggregate
// event production with EventStore persistence and EventDispatcher.
// Store/dispatch errors are logged as warnings and do not abort the session.
// sessionID enriches events with session metadata (SessionID, CorrelationID, CausationID).
func NewSessionEventEmitter(
	ctx context.Context,
	agg *domain.SessionAggregate,
	store port.EventStore,
	dispatcher port.EventDispatcher,
	logger domain.Logger,
	sessionID string,
) port.SessionEventEmitter {
	return &sessionEventEmitter{
		ctx:        ctx,
		agg:        agg,
		store:      store,
		dispatcher: dispatcher,
		logger:     logger,
		sessionID:  sessionID,
	}
}

// SetSeqAllocator injects a SeqAllocator for SeqNr allocation into emitted events.
func (e *sessionEventEmitter) SetSeqAllocator(alloc port.SeqAllocator) {
	e.seqAlloc = alloc
}

// emit enriches events with session metadata, persists, and dispatches best-effort.
func (e *sessionEventEmitter) emit(events ...domain.Event) {
	ctx := e.ctx
	for i := range events {
		events[i].SessionID = e.sessionID
		events[i].CorrelationID = e.sessionID
		if e.prevID != "" {
			events[i].CausationID = e.prevID
		}
		if e.seqAlloc != nil {
			seq, err := e.seqAlloc.AllocSeqNr(ctx)
			if err != nil {
				e.logger.Warn("alloc seq nr: %v", err)
			} else {
				events[i].SeqNr = seq
			}
		}
	}
	if e.store != nil {
		if _, err := e.store.Append(ctx, events...); err != nil {
			e.logger.Warn("append events: %v", err)
		}
	}
	// Update causation chain after successful store
	if len(events) > 0 {
		e.prevID = events[len(events)-1].ID
	}
	if e.dispatcher != nil {
		for _, ev := range events {
			if err := e.dispatcher.Dispatch(ctx, ev); err != nil {
				e.logger.Warn("policy dispatch %s: %v", ev.Type, err)
			}
		}
	}
}

func (e *sessionEventEmitter) EmitStart(project, strictness string, now time.Time) error {
	evt, err := e.agg.Start(project, strictness, now) // nosemgrep: adr0003-otel-span-without-defer-end — not OTel; SessionAggregate.Start() [permanent]
	if err != nil {
		return err
	}
	e.emit(evt)
	return nil
}

func (e *sessionEventEmitter) EmitRecordScan(payload domain.ScanCompletedPayload, now time.Time) error {
	evt, err := e.agg.RecordScan(payload, now)
	if err != nil {
		return err
	}
	e.emit(evt)
	return nil
}

func (e *sessionEventEmitter) EmitResume(originalSessionID string, now time.Time) error {
	evt, err := e.agg.Resume(originalSessionID, now)
	if err != nil {
		return err
	}
	e.emit(evt)
	return nil
}

func (e *sessionEventEmitter) EmitRescan(originalSessionID string, now time.Time) error {
	evt, err := e.agg.Rescan(originalSessionID, now)
	if err != nil {
		return err
	}
	e.emit(evt)
	return nil
}

func (e *sessionEventEmitter) EmitRecordWavesGenerated(payload domain.WavesGeneratedPayload, now time.Time) error {
	evt, err := e.agg.RecordWavesGenerated(payload, now)
	if err != nil {
		return err
	}
	e.emit(evt)
	return nil
}

func (e *sessionEventEmitter) EmitApproveWave(waveID, clusterName string, now time.Time) error {
	evt, err := e.agg.ApproveWave(waveID, clusterName, now)
	if err != nil {
		return err
	}
	e.emit(evt)
	return nil
}

func (e *sessionEventEmitter) EmitRejectWave(waveID, clusterName string, now time.Time) error {
	evt, err := e.agg.RejectWave(waveID, clusterName, now)
	if err != nil {
		return err
	}
	e.emit(evt)
	return nil
}

func (e *sessionEventEmitter) EmitModifyWave(payload domain.WaveModifiedPayload, now time.Time) error {
	evt, err := e.agg.ModifyWave(payload, now)
	if err != nil {
		return err
	}
	e.emit(evt)
	return nil
}

func (e *sessionEventEmitter) EmitApplyWave(payload domain.WaveAppliedPayload, now time.Time) error {
	evt, err := e.agg.ApplyWave(payload, now)
	if err != nil {
		return err
	}
	e.emit(evt)
	return nil
}

func (e *sessionEventEmitter) EmitCompleteWave(payload domain.WaveCompletedPayload, now time.Time) error {
	evt, err := e.agg.CompleteWave(payload, now)
	if err != nil {
		return err
	}
	e.emit(evt)
	return nil
}

func (e *sessionEventEmitter) EmitUpdateCompleteness(clusterName string, clusterC, overallC float64, now time.Time) error {
	evt, err := e.agg.UpdateCompleteness(clusterName, clusterC, overallC, now)
	if err != nil {
		return err
	}
	e.emit(evt)
	return nil
}

func (e *sessionEventEmitter) EmitUnlockWaves(unlockedIDs []string, now time.Time) error {
	evt, err := e.agg.UnlockWaves(unlockedIDs, now)
	if err != nil {
		return err
	}
	e.emit(evt)
	return nil
}

func (e *sessionEventEmitter) EmitAddNextGenWaves(payload domain.NextGenWavesAddedPayload, now time.Time) error {
	evt, err := e.agg.AddNextGenWaves(payload, now)
	if err != nil {
		return err
	}
	e.emit(evt)
	return nil
}

func (e *sessionEventEmitter) EmitApplyReadyLabels(payload domain.ReadyLabelsAppliedPayload, now time.Time) error {
	evt, err := e.agg.ApplyReadyLabels(payload, now)
	if err != nil {
		return err
	}
	e.emit(evt)
	return nil
}

func (e *sessionEventEmitter) EmitSendSpecification(waveID, clusterName string, now time.Time) error {
	evt, err := e.agg.SendSpecification(waveID, clusterName, now)
	if err != nil {
		return err
	}
	e.emit(evt)
	return nil
}

func (e *sessionEventEmitter) EmitSendReport(waveID, clusterName string, now time.Time) error {
	evt, err := e.agg.SendReport(waveID, clusterName, now)
	if err != nil {
		return err
	}
	e.emit(evt)
	return nil
}

func (e *sessionEventEmitter) EmitSendFeedback(waveID, clusterName string, now time.Time) error {
	evt, err := e.agg.SendFeedback(waveID, clusterName, now)
	if err != nil {
		return err
	}
	e.emit(evt)
	return nil
}

func (e *sessionEventEmitter) EmitReceiveFeedback(payload domain.FeedbackReceivedPayload, now time.Time) error {
	evt, err := e.agg.ReceiveFeedback(payload, now)
	if err != nil {
		return err
	}
	e.emit(evt)
	return nil
}

func (e *sessionEventEmitter) EmitGenerateADR(payload domain.ADRGeneratedPayload, now time.Time) error {
	evt, err := e.agg.GenerateADR(payload, now)
	if err != nil {
		return err
	}
	e.emit(evt)
	return nil
}

func (e *sessionEventEmitter) EmitWaveStalled(waveID, clusterName, fingerprint, reason string, now time.Time) error {
	evt, err := domain.NewEvent(domain.EventWaveStalled, domain.WaveStalledPayload{
		WaveID:      waveID,
		ClusterName: clusterName,
		Fingerprint: fingerprint,
		Reason:      reason,
	}, now)
	if err != nil {
		return err
	}
	e.emit(evt)
	return nil
}
