package calibration

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"buoy-calibration-gate/internal/audit"
	"buoy-calibration-gate/internal/domain"
	"buoy-calibration-gate/internal/repository"
)

func (s *Service) Remediate(ctx context.Context, cmd RemediateCommand) (domain.DeviationCase, domain.CalibrationRun, domain.RemediationAttempt, domain.CalibrationDossier, error) {
	var deviation domain.DeviationCase
	var run domain.CalibrationRun
	var savedAttempt domain.RemediationAttempt
	var dossier domain.CalibrationDossier
	if err := requireActor(cmd.Actor); err != nil {
		return deviation, run, savedAttempt, dossier, err
	}
	err := s.store.Transaction(ctx, func(tx *repository.Tx) error {
		var err error
		dossier, err = tx.GetDossier(cmd.DossierID)
		if err != nil {
			return err
		}
		if err := requireExpected(dossier.Version, cmd.ExpectedVersion); err != nil {
			return err
		}
		if dossier.Status != domain.StatusRemediationRequired {
			return domain.ErrInvalidState
		}
		deviation, err = tx.GetDeviation(cmd.DeviationID)
		if err != nil {
			return err
		}
		if deviation.DossierID != dossier.ID {
			return &domain.RuleError{Field: "deviationId", Message: "不属于当前档案"}
		}
		sensor, err := tx.GetSensor(deviation.SensorID)
		if err != nil {
			return err
		}
		now := s.now().UTC()
		run, err = domain.NewRun(s.id(), sensor, domain.RunRetest, cmd.ReferenceValue, cmd.MeasuredValue, cmd.AmbientTemperature, cmd.EvidenceDigest, cmd.Actor, now)
		if err != nil {
			return err
		}
		if occupied, found, err := tx.FindRunByEvidence(dossier.ID, run.EvidenceDigest); err != nil {
			return err
		} else if found {
			return &domain.ResourceConflictError{Kind: domain.ErrEvidenceConflict, Field: "evidenceDigest", Message: "证据摘要已被其他运行占用", ResourceID: occupied.ID}
		}
		attempt, err := domain.NewRemediationAttempt(s.id(), deviation, sensor, run, cmd.Cause, cmd.Adjustment, now)
		if err != nil {
			return err
		}
		savedAttempt = attempt
		if run.Passed {
			if err := deviation.Close(strings.TrimSpace(cmd.Cause), strings.TrimSpace(cmd.Adjustment), run, now); err != nil {
				return err
			}
		}
		if err := tx.InsertRun(run); err != nil {
			return err
		}
		if err := tx.InsertRemediationAttempt(attempt); err != nil {
			return err
		}
		if run.Passed {
			if err := tx.UpdateDeviation(deviation); err != nil {
				return err
			}
		}
		open, err := tx.OpenDeviationCount(dossier.ID)
		if err != nil {
			return err
		}
		if open == 0 {
			if err := dossier.Transition(domain.StatusReviewPending, now); err != nil {
				return err
			}
		} else {
			dossier.UpdatedAt = now
		}
		if err := tx.UpdateDossier(&dossier, cmd.ExpectedVersion); err != nil {
			return err
		}
		_, err = audit.Append(tx, dossier.ID, "deviation.remediation_attempted", cmd.Actor, map[string]any{"deviation": deviation, "attempt": attempt, "retest": run, "passed": run.Passed, "version": dossier.Version}, now)
		return err
	})
	return deviation, run, savedAttempt, dossier, err
}

type BatchRemediationResult struct {
	Deviation domain.DeviationCase      `json:"deviation"`
	RetestRun domain.CalibrationRun     `json:"retestRun"`
	Attempt   domain.RemediationAttempt `json:"attempt"`
	Passed    bool                      `json:"passed"`
}

func (s *Service) BatchRemediate(ctx context.Context, cmd BatchRemediationCommand) ([]BatchRemediationResult, domain.CalibrationDossier, error) {
	var results []BatchRemediationResult
	var dossier domain.CalibrationDossier
	if err := requireActor(cmd.Actor); err != nil {
		return nil, dossier, err
	}
	if len(cmd.Items) == 0 || len(cmd.Items) > 20 {
		return nil, dossier, &domain.RuleError{Field: "items", Message: "批次必须包含 1 至 20 个整改项"}
	}
	err := s.store.Transaction(ctx, func(tx *repository.Tx) error {
		var err error
		dossier, err = tx.GetDossier(cmd.DossierID)
		if err != nil {
			return err
		}
		if err := requireExpected(dossier.Version, cmd.ExpectedVersion); err != nil {
			return err
		}
		if dossier.Status != domain.StatusRemediationRequired {
			return domain.ErrInvalidState
		}
		now := s.now().UTC()
		seenDeviations := make(map[string]bool, len(cmd.Items))
		seenEvidence := make(map[string]bool, len(cmd.Items))
		results = make([]BatchRemediationResult, 0, len(cmd.Items))
		for index, item := range cmd.Items {
			if seenDeviations[item.DeviationID] {
				return indexedValidation(index, "deviationId", "批次内偏差项不能重复")
			}
			seenDeviations[item.DeviationID] = true
			deviation, err := tx.GetDeviation(item.DeviationID)
			if err != nil {
				return indexedValidation(index, "deviationId", "偏差项不存在")
			}
			if deviation.DossierID != dossier.ID {
				return indexedValidation(index, "deviationId", "不属于当前档案")
			}
			if deviation.Status != domain.DeviationOpen && deviation.Status != domain.DeviationReturned {
				return indexedValidation(index, "deviationId", "当前偏差状态不可整改")
			}
			sensor, err := tx.GetSensor(deviation.SensorID)
			if err != nil || sensor.DossierID != dossier.ID {
				return indexedValidation(index, "sensorId", "传感器引用无效")
			}
			run, err := domain.NewRun(s.id(), sensor, domain.RunRetest, item.ReferenceValue, item.MeasuredValue, item.AmbientTemperature, item.EvidenceDigest, cmd.Actor, now)
			if err != nil {
				return indexedCause(index, err)
			}
			if seenEvidence[run.EvidenceDigest] {
				return indexedValidation(index, "evidenceDigest", "批次内证据摘要不能重复")
			}
			seenEvidence[run.EvidenceDigest] = true
			if _, found, err := tx.FindRunByEvidence(dossier.ID, run.EvidenceDigest); err != nil {
				return err
			} else if found {
				return indexedValidation(index, "evidenceDigest", "证据摘要已被其他运行占用")
			}
			attempt, err := domain.NewRemediationAttempt(s.id(), deviation, sensor, run, item.Cause, item.Adjustment, now)
			if err != nil {
				return indexedCause(index, err)
			}
			if run.Passed {
				if err := deviation.Close(strings.TrimSpace(item.Cause), strings.TrimSpace(item.Adjustment), run, now); err != nil {
					return indexedCause(index, err)
				}
			}
			results = append(results, BatchRemediationResult{Deviation: deviation, RetestRun: run, Attempt: attempt, Passed: run.Passed})
		}
		for _, result := range results {
			if err := tx.InsertRun(result.RetestRun); err != nil {
				return err
			}
			if err := tx.InsertRemediationAttempt(result.Attempt); err != nil {
				return err
			}
			if result.Passed {
				if err := tx.UpdateDeviation(result.Deviation); err != nil {
					return err
				}
			}
		}
		open, err := tx.OpenDeviationCount(dossier.ID)
		if err != nil {
			return err
		}
		if open == 0 {
			if err := dossier.Transition(domain.StatusReviewPending, now); err != nil {
				return err
			}
		} else {
			dossier.UpdatedAt = now
		}
		if err := tx.UpdateDossier(&dossier, cmd.ExpectedVersion); err != nil {
			return err
		}
		summary := make([]map[string]any, len(results))
		for index, result := range results {
			summary[index] = map[string]any{"deviationId": result.Deviation.ID, "retestRunId": result.RetestRun.ID, "attemptId": result.Attempt.ID, "passed": result.Passed}
		}
		_, err = audit.Append(tx, dossier.ID, "deviation.remediation_batch_attempted", cmd.Actor, map[string]any{"items": summary, "version": dossier.Version}, now)
		return err
	})
	return results, dossier, err
}

func indexedValidation(index int, field, message string) error {
	return &domain.ValidationError{Field: fmt.Sprintf("items[%d].%s", index, field), Message: message, ItemIndex: &index}
}

func indexedCause(index int, err error) error {
	var rule *domain.RuleError
	if ok := errors.As(err, &rule); ok {
		return indexedValidation(index, rule.Field, rule.Message)
	}
	return err
}
