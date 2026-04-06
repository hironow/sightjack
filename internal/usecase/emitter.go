package usecase

import (
	"time"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/usecase/port"
)

// sessionEventEmitter wraps a SessionAggregate and a Recorder to produce and
// persist domain events. Record errors are best-effort (logged, not propagated)
// to preserve session continuity — matching the existing LoggingRecorder
// semantics. Aggregate errors remain critical and are returned.
type sessionEventEmitter struct {
	agg      *domain.SessionAggregate
	recorder port.Recorder
	logger   domain.Logger
}

// NewSessionEventEmitter creates a SessionEventEmitter that wraps aggregate
// event production and recording. Record errors are logged as warnings via
// logger and do not abort the session.
func NewSessionEventEmitter(agg *domain.SessionAggregate, recorder port.Recorder, logger domain.Logger) port.SessionEventEmitter {
	return &sessionEventEmitter{agg: agg, recorder: recorder, logger: logger}
}

// record persists an event best-effort: errors are warned, not returned.
func (e *sessionEventEmitter) record(evt domain.Event) {
	if err := e.recorder.Record(evt); err != nil {
		e.logger.Warn("record event %s: %v", evt.Type, err)
	}
}

func (e *sessionEventEmitter) EmitStart(project, strictness string, now time.Time) error {
	evt, err := e.agg.Start(project, strictness, now) // nosemgrep: adr0003-otel-span-without-defer-end — not OTel; SessionAggregate.Start() [permanent]
	if err != nil {
		return err
	}
	e.record(evt)
	return nil
}

func (e *sessionEventEmitter) EmitRecordScan(payload domain.ScanCompletedPayload, now time.Time) error {
	evt, err := e.agg.RecordScan(payload, now)
	if err != nil {
		return err
	}
	e.record(evt)
	return nil
}

func (e *sessionEventEmitter) EmitResume(originalSessionID string, now time.Time) error {
	evt, err := e.agg.Resume(originalSessionID, now)
	if err != nil {
		return err
	}
	e.record(evt)
	return nil
}

func (e *sessionEventEmitter) EmitRescan(originalSessionID string, now time.Time) error {
	evt, err := e.agg.Rescan(originalSessionID, now)
	if err != nil {
		return err
	}
	e.record(evt)
	return nil
}

func (e *sessionEventEmitter) EmitRecordWavesGenerated(payload domain.WavesGeneratedPayload, now time.Time) error {
	evt, err := e.agg.RecordWavesGenerated(payload, now)
	if err != nil {
		return err
	}
	e.record(evt)
	return nil
}

func (e *sessionEventEmitter) EmitApproveWave(waveID, clusterName string, now time.Time) error {
	evt, err := e.agg.ApproveWave(waveID, clusterName, now)
	if err != nil {
		return err
	}
	e.record(evt)
	return nil
}

func (e *sessionEventEmitter) EmitRejectWave(waveID, clusterName string, now time.Time) error {
	evt, err := e.agg.RejectWave(waveID, clusterName, now)
	if err != nil {
		return err
	}
	e.record(evt)
	return nil
}

func (e *sessionEventEmitter) EmitModifyWave(payload domain.WaveModifiedPayload, now time.Time) error {
	evt, err := e.agg.ModifyWave(payload, now)
	if err != nil {
		return err
	}
	e.record(evt)
	return nil
}

func (e *sessionEventEmitter) EmitApplyWave(payload domain.WaveAppliedPayload, now time.Time) error {
	evt, err := e.agg.ApplyWave(payload, now)
	if err != nil {
		return err
	}
	e.record(evt)
	return nil
}

func (e *sessionEventEmitter) EmitCompleteWave(payload domain.WaveCompletedPayload, now time.Time) error {
	evt, err := e.agg.CompleteWave(payload, now)
	if err != nil {
		return err
	}
	e.record(evt)
	return nil
}

func (e *sessionEventEmitter) EmitUpdateCompleteness(clusterName string, clusterC, overallC float64, now time.Time) error {
	evt, err := e.agg.UpdateCompleteness(clusterName, clusterC, overallC, now)
	if err != nil {
		return err
	}
	e.record(evt)
	return nil
}

func (e *sessionEventEmitter) EmitUnlockWaves(unlockedIDs []string, now time.Time) error {
	evt, err := e.agg.UnlockWaves(unlockedIDs, now)
	if err != nil {
		return err
	}
	e.record(evt)
	return nil
}

func (e *sessionEventEmitter) EmitAddNextGenWaves(payload domain.NextGenWavesAddedPayload, now time.Time) error {
	evt, err := e.agg.AddNextGenWaves(payload, now)
	if err != nil {
		return err
	}
	e.record(evt)
	return nil
}

func (e *sessionEventEmitter) EmitApplyReadyLabels(payload domain.ReadyLabelsAppliedPayload, now time.Time) error {
	evt, err := e.agg.ApplyReadyLabels(payload, now)
	if err != nil {
		return err
	}
	e.record(evt)
	return nil
}

func (e *sessionEventEmitter) EmitSendSpecification(waveID, clusterName string, now time.Time) error {
	evt, err := e.agg.SendSpecification(waveID, clusterName, now)
	if err != nil {
		return err
	}
	e.record(evt)
	return nil
}

func (e *sessionEventEmitter) EmitSendReport(waveID, clusterName string, now time.Time) error {
	evt, err := e.agg.SendReport(waveID, clusterName, now)
	if err != nil {
		return err
	}
	e.record(evt)
	return nil
}

func (e *sessionEventEmitter) EmitSendFeedback(waveID, clusterName string, now time.Time) error {
	evt, err := e.agg.SendFeedback(waveID, clusterName, now)
	if err != nil {
		return err
	}
	e.record(evt)
	return nil
}

func (e *sessionEventEmitter) EmitReceiveFeedback(payload domain.FeedbackReceivedPayload, now time.Time) error {
	evt, err := e.agg.ReceiveFeedback(payload, now)
	if err != nil {
		return err
	}
	e.record(evt)
	return nil
}

func (e *sessionEventEmitter) EmitGenerateADR(payload domain.ADRGeneratedPayload, now time.Time) error {
	evt, err := e.agg.GenerateADR(payload, now)
	if err != nil {
		return err
	}
	e.record(evt)
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
	e.record(evt)
	return nil
}
