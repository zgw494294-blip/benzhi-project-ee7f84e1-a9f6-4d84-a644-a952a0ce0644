package calibration

import (
	"context"
	"strings"

	"buoy-calibration-gate/internal/audit"
	"buoy-calibration-gate/internal/domain"
	"buoy-calibration-gate/internal/repository"
)

func (s *Service) AddSensor(ctx context.Context, cmd AddSensorCommand) (domain.SensorBaseline, domain.CalibrationDossier, error) {
	var sensor domain.SensorBaseline
	var dossier domain.CalibrationDossier
	if err := requireActor(cmd.Actor); err != nil {
		return sensor, dossier, err
	}
	unlock := s.lockDossier(cmd.DossierID)
	defer unlock()
	err := s.store.Transaction(ctx, func(tx *repository.Tx) error {
		var err error
		dossier, err = tx.GetDossier(cmd.DossierID)
		if err != nil {
			return err
		}
		if err := requireExpected(dossier.Version, cmd.ExpectedVersion); err != nil {
			return err
		}
		now := s.now().UTC()
		typ := domain.NormalizeSensorType(cmd.SensorType)
		unit, err := domain.NormalizeSensorUnit(typ, cmd.Unit)
		if err != nil {
			return err
		}
		sensor = domain.SensorBaseline{ID: s.id(), DossierID: dossier.ID, SensorType: typ, SerialNumber: domain.NormalizeSerialNumber(cmd.SerialNumber), Unit: unit, RangeMin: cmd.RangeMin, RangeMax: cmd.RangeMax, Tolerance: cmd.Tolerance, ConfigurationRevision: strings.TrimSpace(cmd.ConfigurationRevision), CreatedAt: now}
		if err := sensor.Validate(); err != nil {
			return err
		}
		existing, err := tx.ListSensors(dossier.ID)
		if err != nil {
			return err
		}
		for _, current := range existing {
			if current.SerialNumber == sensor.SerialNumber {
				return &domain.ResourceConflictError{Kind: domain.ErrSensorConflict, Field: "serialNumber", Message: "同一档案内传感器序列号不能重复", ResourceID: current.ID}
			}
			if current.SensorType == sensor.SensorType {
				return &domain.ResourceConflictError{Kind: domain.ErrSensorConflict, Field: "sensorType", Message: "同一档案内传感器类型不能重复", ResourceID: current.ID}
			}
		}
		if dossier.Status != domain.StatusDraft {
			return domain.ErrInvalidState
		}
		prospective := append(append([]domain.SensorBaseline(nil), existing...), sensor)
		if cmd.Complete {
			if err := domain.ValidateBaselineSet(prospective); err != nil {
				return err
			}
		}
		if err := tx.InsertSensor(sensor); err != nil {
			return err
		}
		if cmd.Complete {
			if err := dossier.Transition(domain.StatusCalibrating, now); err != nil {
				return err
			}
		} else {
			dossier.UpdatedAt = now
		}
		if err := tx.UpdateDossier(&dossier, cmd.ExpectedVersion); err != nil {
			return err
		}
		_, err = audit.Append(tx, dossier.ID, "sensor.registered", cmd.Actor, map[string]any{"sensor": sensor, "baselineComplete": cmd.Complete, "version": dossier.Version}, now)
		return err
	})
	return sensor, dossier, err
}
