package permit_audit_atomicity_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"buoy-calibration-gate/internal/calibration"
	"buoy-calibration-gate/internal/domain"
	"buoy-calibration-gate/internal/repository"
	_ "modernc.org/sqlite"
)

func TestPermitIssuanceRollsBackWhenAuditAppendFails(t *testing.T) {
	ctx := context.Background()
	databasePath := filepath.Join(t.TempDir(), "permit-audit.db")
	dsn := "file:" + databasePath
	store, err := repository.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	service := calibration.NewService(store)
	dossier := prepareFrozenDossier(t, ctx, service)

	triggerDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	_, err = triggerDB.ExecContext(ctx, `CREATE TRIGGER reject_permit_audit
		BEFORE INSERT ON audit_events
		WHEN NEW.event_type = 'permit.issued'
		BEGIN
			SELECT RAISE(ABORT, 'forced permit audit failure');
		END`)
	if err != nil {
		triggerDB.Close()
		t.Fatal(err)
	}
	if err := triggerDB.Close(); err != nil {
		t.Fatal(err)
	}

	_, _, issueErr := service.IssuePermit(ctx, calibration.IssuePermitCommand{
		DossierID: dossier.ID, ExpectedVersion: dossier.Version, Actor: "deployer-atomicity",
	})
	if issueErr == nil || !strings.Contains(issueErr.Error(), "forced permit audit failure") {
		t.Fatalf("expected injected audit failure, got %v", issueErr)
	}

	snapshot, err := service.GetDossier(ctx, dossier.ID)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Dossier.Status != domain.StatusFrozen || snapshot.Dossier.Version != dossier.Version || snapshot.Permit != nil {
		t.Fatalf("permit issuance escaped failed audit transaction: status=%s version=%d permit=%+v", snapshot.Dossier.Status, snapshot.Dossier.Version, snapshot.Permit)
	}
}

func prepareFrozenDossier(t *testing.T, ctx context.Context, service *calibration.Service) domain.CalibrationDossier {
	t.Helper()
	dossier, _, err := service.CreateDossier(ctx, calibration.CreateDossierCommand{
		BuoyCode: "ATOMIC-01", TargetArea: "东海", Owner: "工程师",
		PlannedDeploymentAt: time.Now().UTC().Add(48 * time.Hour),
		IdempotencyKey:      "atomic-create", Actor: "engineer-atomicity",
	})
	if err != nil {
		t.Fatal(err)
	}

	type baseline struct {
		typ, serial, unit string
		minimum, maximum  float64
	}
	baselines := []baseline{
		{typ: "temperature", serial: "AT-01", unit: "°C", minimum: -5, maximum: 45},
		{typ: "salinity", serial: "AS-01", unit: "PSU", minimum: 0, maximum: 50},
		{typ: "dissolved_oxygen", serial: "AO-01", unit: "mg/L", minimum: 0, maximum: 20},
	}
	sensors := make([]domain.SensorBaseline, 0, len(baselines))
	for index, item := range baselines {
		sensor, updated, addErr := service.AddSensor(ctx, calibration.AddSensorCommand{
			DossierID: dossier.ID, SensorType: item.typ, SerialNumber: item.serial, Unit: item.unit,
			RangeMin: item.minimum, RangeMax: item.maximum, Tolerance: 0.1, ConfigurationRevision: "atomic-v1",
			Complete: index == len(baselines)-1, ExpectedVersion: dossier.Version, Actor: "engineer-atomicity",
		})
		if addErr != nil {
			t.Fatal(addErr)
		}
		sensors = append(sensors, sensor)
		dossier = updated
	}

	for index, sensor := range sensors {
		_, _, updated, _, runErr := service.SubmitRun(ctx, calibration.SubmitRunCommand{
			DossierID: dossier.ID, SensorID: sensor.ID, ReferenceValue: 10, MeasuredValue: 10.01,
			AmbientTemperature: 20, EvidenceDigest: "sha256:" + strings.Repeat(string(rune('a'+index)), 64),
			IdempotencyKey: "atomic-run-" + string(rune('1'+index)), ExpectedVersion: dossier.Version, Actor: "engineer-atomicity",
		})
		if runErr != nil {
			t.Fatal(runErr)
		}
		dossier = updated
	}
	if dossier.Status != domain.StatusReviewPending {
		t.Fatalf("setup did not reach review_pending: %s", dossier.Status)
	}

	_, _, dossier, err = service.Approve(ctx, calibration.ReviewCommand{
		DossierID: dossier.ID, Note: "原子性复现", ExpectedVersion: dossier.Version, Actor: "reviewer-atomicity",
	})
	if err != nil {
		t.Fatal(err)
	}
	return dossier
}
