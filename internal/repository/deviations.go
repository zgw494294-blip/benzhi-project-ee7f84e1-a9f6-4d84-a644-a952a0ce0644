package repository

import (
	"database/sql"
	"errors"

	"buoy-calibration-gate/internal/domain"
)

func (t *Tx) InsertDeviation(d domain.DeviationCase) error {
	_, err := t.tx.Exec(`INSERT INTO deviation_cases(id, dossier_id, sensor_id, source_run_id, status, cause, adjustment, retest_run_id, review_note, closed_at, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, NULL, ?, NULL, ?)`, d.ID, d.DossierID, d.SensorID, d.SourceRunID, d.Status, d.Cause, d.Adjustment, d.ReviewNote, formatTime(d.CreatedAt))
	return err
}

func (t *Tx) GetDeviation(id string) (domain.DeviationCase, error) {
	var d domain.DeviationCase
	var retest, closed sql.NullString
	var created string
	err := t.tx.QueryRow(`SELECT id, dossier_id, sensor_id, source_run_id, status, cause, adjustment, retest_run_id, review_note, closed_at, created_at FROM deviation_cases WHERE id=?`, id).
		Scan(&d.ID, &d.DossierID, &d.SensorID, &d.SourceRunID, &d.Status, &d.Cause, &d.Adjustment, &retest, &d.ReviewNote, &closed, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return d, domain.ErrNotFound
	}
	if err != nil {
		return d, err
	}
	d.RetestRunID = retest.String
	if d.ClosedAt, err = parseOptionalTime(closed); err != nil {
		return d, err
	}
	d.CreatedAt, err = parseTime(created)
	return d, err
}

func (t *Tx) GetDeviationBySourceRun(runID string) (domain.DeviationCase, bool, error) {
	var id string
	err := t.tx.QueryRow(`SELECT id FROM deviation_cases WHERE source_run_id=?`, runID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.DeviationCase{}, false, nil
	}
	if err != nil {
		return domain.DeviationCase{}, false, err
	}
	d, err := t.GetDeviation(id)
	return d, err == nil, err
}

func (t *Tx) UpdateDeviation(d domain.DeviationCase) error {
	var closed any
	if d.ClosedAt != nil {
		closed = formatTime(*d.ClosedAt)
	}
	_, err := t.tx.Exec(`UPDATE deviation_cases SET status=?, cause=?, adjustment=?, retest_run_id=?, review_note=?, closed_at=? WHERE id=?`, d.Status, d.Cause, d.Adjustment, nullable(d.RetestRunID), d.ReviewNote, closed, d.ID)
	return err
}

func nullable(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func (t *Tx) ListDeviations(dossierID string) ([]domain.DeviationCase, error) {
	rows, err := t.tx.Query(`SELECT id, dossier_id, sensor_id, source_run_id, status, cause, adjustment, retest_run_id, review_note, closed_at, created_at FROM deviation_cases WHERE dossier_id=? ORDER BY id`, dossierID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []domain.DeviationCase
	for rows.Next() {
		var d domain.DeviationCase
		var retest, closed sql.NullString
		var created string
		if err := rows.Scan(&d.ID, &d.DossierID, &d.SensorID, &d.SourceRunID, &d.Status, &d.Cause, &d.Adjustment, &retest, &d.ReviewNote, &closed, &created); err != nil {
			return nil, err
		}
		d.RetestRunID = retest.String
		if d.ClosedAt, err = parseOptionalTime(closed); err != nil {
			return nil, err
		}
		if d.CreatedAt, err = parseTime(created); err != nil {
			return nil, err
		}
		result = append(result, d)
	}
	return result, rows.Err()
}

func (t *Tx) OpenDeviationCount(dossierID string) (int, error) {
	var n int
	err := t.tx.QueryRow(`SELECT COUNT(*) FROM deviation_cases WHERE dossier_id=? AND status<>?`, dossierID, domain.DeviationClosed).Scan(&n)
	return n, err
}

func (t *Tx) InsertRemediationAttempt(a domain.RemediationAttempt) error {
	_, err := t.tx.Exec(`INSERT INTO remediation_attempts(id, dossier_id, deviation_id, retest_run_id, cause, adjustment, passed, absolute_deviation, remaining_tolerance_gap, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, a.ID, a.DossierID, a.DeviationID, a.RetestRunID, a.Cause, a.Adjustment, boolInt(a.Passed), a.AbsoluteDeviation, a.RemainingToleranceGap, formatTime(a.CreatedAt))
	return err
}

func (t *Tx) ListRemediationAttempts(dossierID string) ([]domain.RemediationAttempt, error) {
	rows, err := t.tx.Query(`SELECT id, dossier_id, deviation_id, retest_run_id, cause, adjustment, passed, absolute_deviation, remaining_tolerance_gap, created_at FROM remediation_attempts WHERE dossier_id=? ORDER BY created_at, id`, dossierID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []domain.RemediationAttempt
	for rows.Next() {
		var a domain.RemediationAttempt
		var passed int
		var created string
		if err := rows.Scan(&a.ID, &a.DossierID, &a.DeviationID, &a.RetestRunID, &a.Cause, &a.Adjustment, &passed, &a.AbsoluteDeviation, &a.RemainingToleranceGap, &created); err != nil {
			return nil, err
		}
		a.Passed = passed != 0
		if a.CreatedAt, err = parseTime(created); err != nil {
			return nil, err
		}
		result = append(result, a)
	}
	return result, rows.Err()
}
