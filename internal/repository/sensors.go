package repository

import (
	"database/sql"
	"errors"

	"buoy-calibration-gate/internal/domain"
)

func (t *Tx) InsertSensor(s domain.SensorBaseline) error {
	_, err := t.tx.Exec(`INSERT INTO sensors(id, dossier_id, sensor_type, serial_number, unit, range_min, range_max, tolerance, configuration_revision, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, s.ID, s.DossierID, s.SensorType, s.SerialNumber, s.Unit, s.RangeMin, s.RangeMax, s.Tolerance, s.ConfigurationRevision, formatTime(s.CreatedAt))
	return err
}

func (t *Tx) GetSensor(id string) (domain.SensorBaseline, error) {
	var s domain.SensorBaseline
	var created string
	err := t.tx.QueryRow(`SELECT id, dossier_id, sensor_type, serial_number, unit, range_min, range_max, tolerance, configuration_revision, created_at FROM sensors WHERE id=?`, id).
		Scan(&s.ID, &s.DossierID, &s.SensorType, &s.SerialNumber, &s.Unit, &s.RangeMin, &s.RangeMax, &s.Tolerance, &s.ConfigurationRevision, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return s, domain.ErrNotFound
	}
	if err != nil {
		return s, err
	}
	s.CreatedAt, err = parseTime(created)
	return s, err
}

func (t *Tx) ListSensors(dossierID string) ([]domain.SensorBaseline, error) {
	rows, err := t.tx.Query(`SELECT id, dossier_id, sensor_type, serial_number, unit, range_min, range_max, tolerance, configuration_revision, created_at FROM sensors WHERE dossier_id=? ORDER BY id`, dossierID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []domain.SensorBaseline
	for rows.Next() {
		var s domain.SensorBaseline
		var created string
		if err := rows.Scan(&s.ID, &s.DossierID, &s.SensorType, &s.SerialNumber, &s.Unit, &s.RangeMin, &s.RangeMax, &s.Tolerance, &s.ConfigurationRevision, &created); err != nil {
			return nil, err
		}
		if s.CreatedAt, err = parseTime(created); err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}
