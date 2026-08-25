package repository

import (
	"database/sql"
	"errors"

	"buoy-calibration-gate/internal/domain"
)

func (t *Tx) FindActiveDossierByBuoyCode(buoyCode string) (domain.CalibrationDossier, bool, error) {
	var id string
	err := t.tx.QueryRow(`SELECT id FROM dossiers WHERE buoy_code=? AND status IN (?, ?, ?, ?, ?) ORDER BY created_at, id LIMIT 1`,
		domain.NormalizeBuoyCode(buoyCode), domain.StatusDraft, domain.StatusCalibrating, domain.StatusRemediationRequired, domain.StatusReviewPending, domain.StatusFrozen).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.CalibrationDossier{}, false, nil
	}
	if err != nil {
		return domain.CalibrationDossier{}, false, err
	}
	d, err := t.GetDossier(id)
	return d, err == nil, err
}

func (t *Tx) InsertDossier(d domain.CalibrationDossier) error {
	_, err := t.tx.Exec(`INSERT INTO dossiers(id, buoy_code, target_area, planned_at, owner, status, version, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, d.ID, d.BuoyCode, d.TargetArea, formatTime(d.PlannedDeploymentAt), d.Owner, d.Status, d.Version, formatTime(d.CreatedAt), formatTime(d.UpdatedAt))
	return err
}

func (t *Tx) GetDossier(id string) (domain.CalibrationDossier, error) {
	var d domain.CalibrationDossier
	var planned, created, updated string
	err := t.tx.QueryRow(`SELECT id, buoy_code, target_area, planned_at, owner, status, version, created_at, updated_at FROM dossiers WHERE id=?`, id).
		Scan(&d.ID, &d.BuoyCode, &d.TargetArea, &planned, &d.Owner, &d.Status, &d.Version, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return d, domain.ErrNotFound
	}
	if err != nil {
		return d, err
	}
	if d.PlannedDeploymentAt, err = parseTime(planned); err != nil {
		return d, err
	}
	if d.CreatedAt, err = parseTime(created); err != nil {
		return d, err
	}
	d.UpdatedAt, err = parseTime(updated)
	return d, err
}

func (t *Tx) UpdateDossier(d *domain.CalibrationDossier, expected int64) error {
	result, err := t.tx.Exec(`UPDATE dossiers SET status=?, version=version+1, updated_at=? WHERE id=? AND version=?`, d.Status, formatTime(d.UpdatedAt), d.ID, expected)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return domain.ErrConflict
	}
	d.Version = expected + 1
	return nil
}
