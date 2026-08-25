package domain

import (
	"strings"
	"time"
)

type RemediationAttempt struct {
	ID                    string    `json:"id"`
	DossierID             string    `json:"dossierId"`
	DeviationID           string    `json:"deviationId"`
	RetestRunID           string    `json:"retestRunId"`
	Cause                 string    `json:"cause"`
	Adjustment            string    `json:"adjustment"`
	Passed                bool      `json:"passed"`
	AbsoluteDeviation     float64   `json:"absoluteDeviation"`
	RemainingToleranceGap float64   `json:"remainingToleranceGap"`
	CreatedAt             time.Time `json:"createdAt"`
}

func NewRemediationAttempt(id string, deviation DeviationCase, sensor SensorBaseline, retest CalibrationRun, cause, adjustment string, now time.Time) (RemediationAttempt, error) {
	cause, adjustment = strings.TrimSpace(cause), strings.TrimSpace(adjustment)
	if deviation.Status != DeviationOpen && deviation.Status != DeviationReturned {
		return RemediationAttempt{}, ErrInvalidState
	}
	if retest.SensorID != deviation.SensorID || retest.DossierID != deviation.DossierID || retest.RunKind != RunRetest {
		return RemediationAttempt{}, &RuleError{Field: "retest", Message: "复测引用与偏差项不一致"}
	}
	if cause == "" || adjustment == "" {
		return RemediationAttempt{}, &RuleError{Field: "remediation", Message: "原因和调整参数不能为空"}
	}
	gap := retest.AbsoluteDeviation - sensor.Tolerance
	if gap < 0 {
		gap = 0
	}
	return RemediationAttempt{ID: id, DossierID: deviation.DossierID, DeviationID: deviation.ID, RetestRunID: retest.ID, Cause: cause, Adjustment: adjustment, Passed: retest.Passed, AbsoluteDeviation: retest.AbsoluteDeviation, RemainingToleranceGap: gap, CreatedAt: now.UTC()}, nil
}

type DeviationCase struct {
	ID          string          `json:"id"`
	DossierID   string          `json:"dossierId"`
	SensorID    string          `json:"sensorId"`
	SourceRunID string          `json:"sourceRunId"`
	Status      DeviationStatus `json:"status"`
	Cause       string          `json:"cause,omitempty"`
	Adjustment  string          `json:"adjustment,omitempty"`
	RetestRunID string          `json:"retestRunId,omitempty"`
	ReviewNote  string          `json:"reviewNote,omitempty"`
	ClosedAt    *time.Time      `json:"closedAt,omitempty"`
	CreatedAt   time.Time       `json:"createdAt"`
}

func NewDeviation(id string, run CalibrationRun, now time.Time) (DeviationCase, error) {
	if run.Passed {
		return DeviationCase{}, &RuleError{Field: "sourceRunId", Message: "通过的运行不能形成偏差项"}
	}
	return DeviationCase{ID: id, DossierID: run.DossierID, SensorID: run.SensorID, SourceRunID: run.ID, Status: DeviationOpen, CreatedAt: now.UTC()}, nil
}

func (d *DeviationCase) Close(cause, adjustment string, retest CalibrationRun, now time.Time) error {
	if d.Status != DeviationOpen && d.Status != DeviationReturned {
		return ErrInvalidState
	}
	if retest.SensorID != d.SensorID || retest.DossierID != d.DossierID || retest.RunKind != RunRetest {
		return &RuleError{Field: "retest", Message: "复测引用与偏差项不一致"}
	}
	if !retest.Passed {
		return &RuleError{Field: "retest", Message: "复测未达到容差"}
	}
	if cause == "" || adjustment == "" {
		return &RuleError{Field: "remediation", Message: "原因和调整参数不能为空"}
	}
	t := now.UTC()
	d.Cause, d.Adjustment, d.RetestRunID, d.Status, d.ClosedAt = cause, adjustment, retest.ID, DeviationClosed, &t
	return nil
}

func (d *DeviationCase) Return(note string) error {
	if d.Status != DeviationClosed {
		return ErrInvalidState
	}
	if note == "" {
		return &RuleError{Field: "reviewNote", Message: "不能为空"}
	}
	d.Status, d.ReviewNote, d.ClosedAt = DeviationReturned, note, nil
	return nil
}
