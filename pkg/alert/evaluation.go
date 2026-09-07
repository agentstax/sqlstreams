package alert

import (
	"errors"
	"fmt"
	"time"
)

// AlertEvaluationState describes evidence, not a recorded alert's lifecycle.
type AlertEvaluationState string

const (
	AlertEvaluationStateHealthy              AlertEvaluationState = "healthy"
	AlertEvaluationStatePending              AlertEvaluationState = "pending"
	AlertEvaluationStateActive               AlertEvaluationState = "active"
	AlertEvaluationStateInsufficientEvidence AlertEvaluationState = "insufficient_evidence"
)

// AlertEvaluationSnapshot describes retained evidence under one resolved policy.
// It is calculated on demand, not a recorded alert or the last scheduled check.
type AlertEvaluationSnapshot struct {
	// State describes current evidence, not the last recorded alert.
	State AlertEvaluationState `json:"state"`
	// Finding is present for pending and active conditions; nil otherwise.
	Finding *Alert `json:"finding"`
	// EvaluatedAt is the database time used to evaluate evidence.
	EvaluatedAt time.Time `json:"evaluated_at"`
	// ObservedAt is the newest sample's storage time, or the collector completion time; zero if unavailable.
	ObservedAt time.Time `json:"observed_at"`
	// UnhealthySince starts the established span; zero when no span was calculated.
	UnhealthySince time.Time `json:"unhealthy_since"`
	// ObservedDuration is the established unhealthy span, bounded by the evidence window.
	ObservedDuration time.Duration `json:"observed_duration"`
	// PendingDuration is the resolved required unhealthy duration.
	PendingDuration time.Duration `json:"pending_duration"`
	// MaximumGap is the resolved sample-gap limit; zero for collector progress.
	MaximumGap time.Duration `json:"maximum_gap"`
	// MaximumAge is the resolved completion-age limit; zero for topic alerts.
	MaximumAge time.Duration `json:"maximum_age"`
	// DisablePending reports whether the evaluation permits immediate activation.
	DisablePending bool `json:"disable_pending"`
	// Reason explains insufficient evidence for display; empty otherwise. Branch on State, not this text.
	Reason string `json:"reason"`
}

func NewAlertEvaluationSnapshot(state AlertEvaluationState, finding *Alert, cfg *AlertEvaluationSnapshotConfig) (*AlertEvaluationSnapshot, error) {
	if cfg == nil {
		cfg = &AlertEvaluationSnapshotConfig{}
	}
	cfg.WithDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	result := &AlertEvaluationSnapshot{
		State:            state,
		Finding:          finding,
		EvaluatedAt:      cfg.EvaluatedAt,
		ObservedAt:       cfg.ObservedAt,
		UnhealthySince:   cfg.UnhealthySince,
		ObservedDuration: cfg.ObservedDuration,
		PendingDuration:  cfg.PendingDuration,
		MaximumGap:       cfg.MaximumGap,
		MaximumAge:       cfg.MaximumAge,
		DisablePending:   cfg.DisablePending,
		Reason:           cfg.Reason,
	}
	if err := result.Validate(); err != nil {
		return nil, err
	}
	return result, nil
}

// Validate checks that State and Finding describe a consistent condition.
func (s *AlertEvaluationSnapshot) Validate() error {
	switch s.State {
	case AlertEvaluationStateHealthy, AlertEvaluationStateInsufficientEvidence:
		if s.Finding != nil {
			return fmt.Errorf("Finding must be nil for state %q", s.State)
		}
	case AlertEvaluationStatePending, AlertEvaluationStateActive:
		if s.Finding == nil {
			return errors.New("Finding must not be nil")
		}
		if s.Finding.Status != AlertStatusActive {
			return fmt.Errorf("Finding.Status must be %q, got %q", AlertStatusActive, s.Finding.Status)
		}
	default:
		return fmt.Errorf("unrecognized alert evaluation state: %q", s.State)
	}
	return nil
}

// AlertEvaluationSnapshotConfig carries optional diagnostic facts already calculated by an evaluator.
// Zero values mean absent or not applicable; no diagnostic defaults are supplied.
type AlertEvaluationSnapshotConfig struct {
	EvaluatedAt      time.Time
	ObservedAt       time.Time
	UnhealthySince   time.Time
	ObservedDuration time.Duration
	PendingDuration  time.Duration
	MaximumGap       time.Duration
	MaximumAge       time.Duration
	DisablePending   bool
	Reason           string
}

func (c *AlertEvaluationSnapshotConfig) WithDefaults() *AlertEvaluationSnapshotConfig {
	return c
}

func (c *AlertEvaluationSnapshotConfig) Validate() error {
	return nil
}
