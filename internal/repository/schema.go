package repository

import (
	"context"
	"fmt"
)

const schemaVersion = 1

var schemaStatements = []string{
	`PRAGMA foreign_keys = ON`,
	`CREATE TABLE IF NOT EXISTS schema_meta (version INTEGER NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS dossiers (
id TEXT PRIMARY KEY, buoy_code TEXT NOT NULL, target_area TEXT NOT NULL, planned_at TEXT NOT NULL,
owner TEXT NOT NULL, status TEXT NOT NULL, version INTEGER NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS sensors (
id TEXT PRIMARY KEY, dossier_id TEXT NOT NULL REFERENCES dossiers(id), sensor_type TEXT NOT NULL,
serial_number TEXT NOT NULL, unit TEXT NOT NULL, range_min REAL NOT NULL, range_max REAL NOT NULL,
tolerance REAL NOT NULL, configuration_revision TEXT NOT NULL, created_at TEXT NOT NULL,
UNIQUE(dossier_id, serial_number))`,
	`CREATE TABLE IF NOT EXISTS calibration_runs (
id TEXT PRIMARY KEY, dossier_id TEXT NOT NULL REFERENCES dossiers(id), sensor_id TEXT NOT NULL REFERENCES sensors(id),
run_kind TEXT NOT NULL, reference_value REAL NOT NULL, measured_value REAL NOT NULL, absolute_deviation REAL NOT NULL,
ambient_temperature REAL NOT NULL, evidence_digest TEXT NOT NULL, recorded_by TEXT NOT NULL, recorded_at TEXT NOT NULL, passed INTEGER NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS deviation_cases (
id TEXT PRIMARY KEY, dossier_id TEXT NOT NULL REFERENCES dossiers(id), sensor_id TEXT NOT NULL REFERENCES sensors(id),
source_run_id TEXT NOT NULL REFERENCES calibration_runs(id), status TEXT NOT NULL, cause TEXT NOT NULL DEFAULT '',
adjustment TEXT NOT NULL DEFAULT '', retest_run_id TEXT, review_note TEXT NOT NULL DEFAULT '', closed_at TEXT, created_at TEXT NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS remediation_attempts (
id TEXT PRIMARY KEY, dossier_id TEXT NOT NULL REFERENCES dossiers(id), deviation_id TEXT NOT NULL REFERENCES deviation_cases(id),
retest_run_id TEXT NOT NULL UNIQUE REFERENCES calibration_runs(id), cause TEXT NOT NULL, adjustment TEXT NOT NULL,
passed INTEGER NOT NULL, absolute_deviation REAL NOT NULL, remaining_tolerance_gap REAL NOT NULL, created_at TEXT NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS review_decisions (
dossier_id TEXT PRIMARY KEY REFERENCES dossiers(id), reviewer TEXT NOT NULL, conclusion TEXT NOT NULL,
note TEXT NOT NULL, reviewed_at TEXT NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS frozen_manifests (
dossier_id TEXT PRIMARY KEY REFERENCES dossiers(id), frozen_version INTEGER NOT NULL, digest TEXT NOT NULL,
manifest_json BLOB NOT NULL, created_at TEXT NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS deployment_permits (
id TEXT PRIMARY KEY, permit_number TEXT NOT NULL UNIQUE, dossier_id TEXT NOT NULL UNIQUE REFERENCES dossiers(id),
frozen_version INTEGER NOT NULL, manifest_digest TEXT NOT NULL, verification_hash TEXT NOT NULL,
approved_by TEXT NOT NULL, issued_at TEXT NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS idempotency_records (
scope TEXT NOT NULL, key TEXT NOT NULL, request_hash TEXT NOT NULL, resource_id TEXT NOT NULL,
created_at TEXT NOT NULL, PRIMARY KEY(scope, key))`,
	`CREATE TABLE IF NOT EXISTS audit_events (
dossier_id TEXT NOT NULL REFERENCES dossiers(id), sequence INTEGER NOT NULL, event_type TEXT NOT NULL,
actor TEXT NOT NULL, payload_json BLOB NOT NULL, occurred_at TEXT NOT NULL, previous_hash TEXT NOT NULL,
event_hash TEXT NOT NULL, PRIMARY KEY(dossier_id, sequence))`,
	`CREATE INDEX IF NOT EXISTS idx_sensors_dossier ON sensors(dossier_id)`,
	`CREATE INDEX IF NOT EXISTS idx_runs_dossier ON calibration_runs(dossier_id)`,
	`CREATE INDEX IF NOT EXISTS idx_deviations_dossier ON deviation_cases(dossier_id)`,
	`CREATE INDEX IF NOT EXISTS idx_remediation_attempts_dossier ON remediation_attempts(dossier_id, created_at, id)`,
	`CREATE INDEX IF NOT EXISTS idx_dossiers_active_buoy ON dossiers(buoy_code, status)`,
	`CREATE INDEX IF NOT EXISTS idx_runs_evidence ON calibration_runs(dossier_id, evidence_digest)`,
}

func (s *Store) initialize(ctx context.Context) error {
	for _, stmt := range schemaStatements {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("initialize schema: %w", err)
		}
	}
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_meta`).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		if _, err := s.db.ExecContext(ctx, `INSERT INTO schema_meta(version) VALUES (?)`, schemaVersion); err != nil {
			return err
		}
	}
	var version int
	if err := s.db.QueryRowContext(ctx, `SELECT version FROM schema_meta LIMIT 1`).Scan(&version); err != nil {
		return err
	}
	if version != schemaVersion {
		return fmt.Errorf("unsupported schema version: %d", version)
	}
	return s.Ready(ctx)
}
