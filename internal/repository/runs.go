package repository

import (
	"database/sql"
	"errors"

	"buoy-calibration-gate/internal/domain"
)

func (t *Tx) InsertRun(r domain.CalibrationRun) error {
	_, err := t.tx.Exec(`INSERT INTO calibration_runs(id, dossier_id, sensor_id, run_kind, reference_value, measured_value, absolute_deviation, ambient_temperature, evidence_digest, recorded_by, recorded_at, passed)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, r.ID, r.DossierID, r.SensorID, r.RunKind, r.ReferenceValue, r.MeasuredValue, r.AbsoluteDeviation, r.AmbientTemperature, r.EvidenceDigest, r.RecordedBy, formatTime(r.RecordedAt), boolInt(r.Passed))
	return err
}

func (t *Tx) GetRun(id string) (domain.CalibrationRun, error) {
	var r domain.CalibrationRun
	var recorded string
	var passed int
	err := t.tx.QueryRow(`SELECT id, dossier_id, sensor_id, run_kind, reference_value, measured_value, absolute_deviation, ambient_temperature, evidence_digest, recorded_by, recorded_at, passed FROM calibration_runs WHERE id=?`, id).
		Scan(&r.ID, &r.DossierID, &r.SensorID, &r.RunKind, &r.ReferenceValue, &r.MeasuredValue, &r.AbsoluteDeviation, &r.AmbientTemperature, &r.EvidenceDigest, &r.RecordedBy, &recorded, &passed)
	if errors.Is(err, sql.ErrNoRows) {
		return r, domain.ErrNotFound
	}
	if err != nil {
		return r, err
	}
	if r.RecordedAt, err = parseTime(recorded); err != nil {
		return r, err
	}
	r.Passed = passed != 0
	return r, nil
}

func (t *Tx) FindRunByEvidence(dossierID, digest string) (domain.CalibrationRun, bool, error) {
	var id string
	err := t.tx.QueryRow(`SELECT id FROM calibration_runs WHERE dossier_id=? AND evidence_digest=? ORDER BY recorded_at, id LIMIT 1`, dossierID, digest).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.CalibrationRun{}, false, nil
	}
	if err != nil {
		return domain.CalibrationRun{}, false, err
	}
	r, err := t.GetRun(id)
	return r, err == nil, err
}

func (t *Tx) ListRuns(dossierID string) ([]domain.CalibrationRun, error) {
	rows, err := t.tx.Query(`SELECT id, dossier_id, sensor_id, run_kind, reference_value, measured_value, absolute_deviation, ambient_temperature, evidence_digest, recorded_by, recorded_at, passed FROM calibration_runs WHERE dossier_id=? ORDER BY recorded_at, id`, dossierID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []domain.CalibrationRun
	for rows.Next() {
		var r domain.CalibrationRun
		var recorded string
		var passed int
		if err := rows.Scan(&r.ID, &r.DossierID, &r.SensorID, &r.RunKind, &r.ReferenceValue, &r.MeasuredValue, &r.AbsoluteDeviation, &r.AmbientTemperature, &r.EvidenceDigest, &r.RecordedBy, &recorded, &passed); err != nil {
			return nil, err
		}
		if r.RecordedAt, err = parseTime(recorded); err != nil {
			return nil, err
		}
		r.Passed = passed != 0
		result = append(result, r)
	}
	return result, rows.Err()
}

func (t *Tx) InitialRunSensorCount(dossierID string) (int, error) {
	var n int
	err := t.tx.QueryRow(`SELECT COUNT(DISTINCT sensor_id) FROM calibration_runs WHERE dossier_id=? AND run_kind=?`, dossierID, domain.RunInitial).Scan(&n)
	return n, err
}
