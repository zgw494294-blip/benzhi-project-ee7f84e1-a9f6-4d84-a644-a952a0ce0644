package calibration

import (
	"context"
	"strings"

	"buoy-calibration-gate/internal/audit"
	"buoy-calibration-gate/internal/domain"
	"buoy-calibration-gate/internal/repository"
)

type runReplay struct {
	requestHash string
	run         domain.CalibrationRun
	deviation   *domain.DeviationCase
	dossier     domain.CalibrationDossier
}

func (s *Service) SubmitRun(ctx context.Context, cmd SubmitRunCommand) (domain.CalibrationRun, *domain.DeviationCase, domain.CalibrationDossier, bool, error) {
	var run domain.CalibrationRun
	var deviation *domain.DeviationCase
	var dossier domain.CalibrationDossier
	var replay bool
	if err := requireActor(cmd.Actor); err != nil {
		return run, deviation, dossier, false, err
	}
	if strings.TrimSpace(cmd.IdempotencyKey) == "" {
		return run, deviation, dossier, false, &domain.RuleError{Field: "Idempotency-Key", Message: "不能为空"}
	}
	hash, err := requestHash(struct {
		DossierID, SensorID, EvidenceDigest               string
		ReferenceValue, MeasuredValue, AmbientTemperature float64
	}{cmd.DossierID, cmd.SensorID, strings.ToLower(strings.TrimSpace(cmd.EvidenceDigest)), cmd.ReferenceValue, cmd.MeasuredValue, cmd.AmbientTemperature})
	if err != nil {
		return run, deviation, dossier, false, err
	}
	s.runReplayMu.Lock()
	cached, cachedFound := s.runReplays[cmd.IdempotencyKey]
	s.runReplayMu.Unlock()
	if cachedFound {
		if cached.requestHash != hash {
			return run, deviation, dossier, false, domain.ErrIdempotencyKey
		}
		return cached.run, cached.deviation, cached.dossier, true, nil
	}
	err = s.store.Transaction(ctx, func(tx *repository.Tx) error {
		scope := "submit-run:" + cmd.DossierID + ":" + cmd.SensorID
		existing, found, err := tx.FindIdempotency(scope, cmd.IdempotencyKey)
		if err != nil {
			return err
		}
		if found {
			if existing.RequestHash != hash {
				return domain.ErrIdempotencyKey
			}
			if run, err = tx.GetRun(existing.ResourceID); err != nil {
				return err
			}
			if d, ok, err := tx.GetDeviationBySourceRun(run.ID); err != nil {
				return err
			} else if ok {
				deviation = &d
			}
			dossier, err = tx.GetDossier(cmd.DossierID)
			replay = err == nil
			return err
		}
		dossier, err = tx.GetDossier(cmd.DossierID)
		if err != nil {
			return err
		}
		if err := requireExpected(dossier.Version, cmd.ExpectedVersion); err != nil {
			return err
		}
		if dossier.Status != domain.StatusCalibrating {
			return domain.ErrInvalidState
		}
		sensor, err := tx.GetSensor(cmd.SensorID)
		if err != nil {
			return err
		}
		if sensor.DossierID != dossier.ID {
			return &domain.RuleError{Field: "sensorId", Message: "不属于当前档案"}
		}
		runs, err := tx.ListRuns(dossier.ID)
		if err != nil {
			return err
		}
		for _, old := range runs {
			if old.SensorID == sensor.ID && old.RunKind == domain.RunInitial {
				return &domain.RuleError{Field: "sensorId", Message: "该传感器已提交初次校准运行"}
			}
		}
		now := s.now().UTC()
		run, err = domain.NewRun(s.id(), sensor, domain.RunInitial, cmd.ReferenceValue, cmd.MeasuredValue, cmd.AmbientTemperature, cmd.EvidenceDigest, cmd.Actor, now)
		if err != nil {
			return err
		}
		if occupied, found, err := tx.FindRunByEvidence(dossier.ID, run.EvidenceDigest); err != nil {
			return err
		} else if found {
			return &domain.ResourceConflictError{Kind: domain.ErrEvidenceConflict, Field: "evidenceDigest", Message: "证据摘要已被其他校准读数组合占用", ResourceID: occupied.ID}
		}
		if err := tx.InsertRun(run); err != nil {
			return err
		}
		if !run.Passed {
			d, err := domain.NewDeviation(s.id(), run, now)
			if err != nil {
				return err
			}
			if err := tx.InsertDeviation(d); err != nil {
				return err
			}
			deviation = &d
		}
		sensors, err := tx.ListSensors(dossier.ID)
		if err != nil {
			return err
		}
		completed, err := tx.InitialRunSensorCount(dossier.ID)
		if err != nil {
			return err
		}
		if completed == len(sensors) {
			open, err := tx.OpenDeviationCount(dossier.ID)
			if err != nil {
				return err
			}
			next := domain.StatusReviewPending
			if open > 0 {
				next = domain.StatusRemediationRequired
			}
			if err := dossier.Transition(next, now); err != nil {
				return err
			}
		} else {
			dossier.UpdatedAt = now
		}
		if err := tx.UpdateDossier(&dossier, cmd.ExpectedVersion); err != nil {
			return err
		}
		if _, err = audit.Append(tx, dossier.ID, "calibration.run_recorded", cmd.Actor, map[string]any{"run": run, "deviation": deviation, "version": dossier.Version}, now); err != nil {
			return err
		}
		return tx.InsertIdempotency(repository.IdempotencyRecord{Scope: scope, Key: cmd.IdempotencyKey, RequestHash: hash, ResourceID: run.ID, CreatedAt: now})
	})
	if err == nil {
		s.runReplayMu.Lock()
		s.runReplays[cmd.IdempotencyKey] = runReplay{requestHash: hash, run: run, deviation: deviation, dossier: dossier}
		s.runReplayMu.Unlock()
	}
	return run, deviation, dossier, replay, err
}
