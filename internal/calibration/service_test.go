package calibration

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"buoy-calibration-gate/internal/domain"
	"buoy-calibration-gate/internal/repository"
)

func testService(t *testing.T) (*Service, func()) {
	t.Helper()
	store, err := repository.Open(context.Background(), "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(store)
	clock := time.Date(2026, 8, 25, 3, 0, 0, 0, time.UTC)
	service.now = func() time.Time { clock = clock.Add(time.Second); return clock }
	return service, func() { _ = store.Close() }
}

func createForTest(t *testing.T, service *Service, key string) domain.CalibrationDossier {
	return createBuoyForTest(t, service, key, "B-001")
}

func createBuoyForTest(t *testing.T, service *Service, key, buoy string) domain.CalibrationDossier {
	t.Helper()
	d, _, err := service.CreateDossier(context.Background(), CreateDossierCommand{BuoyCode: buoy, TargetArea: "南海", PlannedDeploymentAt: time.Now().UTC().Add(48 * time.Hour), Owner: "工程师", IdempotencyKey: key, Actor: "engineer-1"})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func testDigest(ch string) string { return "sha256:" + strings.Repeat(ch, 64) }

func TestFullRemediationPermitFlow(t *testing.T) {
	service, closeStore := testService(t)
	defer closeStore()
	ctx := context.Background()
	d := createForTest(t, service, "flow-1")
	temperature, d, err := service.AddSensor(ctx, AddSensorCommand{DossierID: d.ID, SensorType: "temperature", SerialNumber: "T-001", Unit: "°C", RangeMin: -5, RangeMax: 45, Tolerance: .1, ConfigurationRevision: "cfg-1", ExpectedVersion: d.Version, Actor: "engineer-1"})
	if err != nil {
		t.Fatal(err)
	}
	salinity, d, err := service.AddSensor(ctx, AddSensorCommand{DossierID: d.ID, SensorType: "salinity", SerialNumber: "S-001", Unit: "PSU", RangeMin: 0, RangeMax: 50, Tolerance: .05, ConfigurationRevision: "cfg-1", ExpectedVersion: d.Version, Actor: "engineer-1"})
	if err != nil {
		t.Fatal(err)
	}
	oxygen, d, err := service.AddSensor(ctx, AddSensorCommand{DossierID: d.ID, SensorType: "dissolved_oxygen", SerialNumber: "O-001", Unit: "mg/L", RangeMin: 0, RangeMax: 20, Tolerance: .05, ConfigurationRevision: "cfg-1", Complete: true, ExpectedVersion: d.Version, Actor: "engineer-1"})
	if err != nil {
		t.Fatal(err)
	}
	_, _, d, _, err = service.SubmitRun(ctx, SubmitRunCommand{DossierID: d.ID, SensorID: temperature.ID, ReferenceValue: 20, MeasuredValue: 20.02, AmbientTemperature: 20, EvidenceDigest: testDigest("a"), IdempotencyKey: "run-temperature", ExpectedVersion: d.Version, Actor: "engineer-1"})
	if err != nil {
		t.Fatal(err)
	}
	_, _, d, _, err = service.SubmitRun(ctx, SubmitRunCommand{DossierID: d.ID, SensorID: salinity.ID, ReferenceValue: 35, MeasuredValue: 35.02, AmbientTemperature: 20, EvidenceDigest: testDigest("b"), IdempotencyKey: "run-salinity", ExpectedVersion: d.Version, Actor: "engineer-1"})
	if err != nil {
		t.Fatal(err)
	}
	_, deviation, d, _, err := service.SubmitRun(ctx, SubmitRunCommand{DossierID: d.ID, SensorID: oxygen.ID, ReferenceValue: 8, MeasuredValue: 8.1, AmbientTemperature: 20, EvidenceDigest: testDigest("c"), IdempotencyKey: "run-oxygen", ExpectedVersion: d.Version, Actor: "engineer-1"})
	if err != nil {
		t.Fatal(err)
	}
	if deviation == nil || d.Status != domain.StatusRemediationRequired {
		t.Fatalf("expected remediation: %+v %+v", deviation, d)
	}
	_, failedRetest, failedAttempt, d, err := service.Remediate(ctx, RemediateCommand{DossierID: d.ID, DeviationID: deviation.ID, Cause: "增益漂移", Adjustment: "gain=0.990", ReferenceValue: 8, MeasuredValue: 8.08, AmbientTemperature: 20, EvidenceDigest: testDigest("d"), ExpectedVersion: d.Version, Actor: "engineer-1"})
	if err != nil || failedRetest.Passed || failedAttempt.RemainingToleranceGap <= 0 || d.Status != domain.StatusRemediationRequired {
		t.Fatalf("failed retest was not retained: %+v %+v %+v %v", failedRetest, failedAttempt, d, err)
	}
	_, _, _, d, err = service.Remediate(ctx, RemediateCommand{DossierID: d.ID, DeviationID: deviation.ID, Cause: "增益漂移", Adjustment: "gain=0.997", ReferenceValue: 8, MeasuredValue: 8.02, AmbientTemperature: 20, EvidenceDigest: testDigest("e"), ExpectedVersion: d.Version, Actor: "engineer-1"})
	if err != nil {
		t.Fatal(err)
	}
	if d.Status != domain.StatusReviewPending {
		t.Fatalf("unexpected status %s", d.Status)
	}
	preflight, err := service.ReviewPreflight(ctx, d.ID, "reviewer-1")
	if err != nil || len(preflight.Blockers) != 0 || preflight.PreviewDigest == "" {
		t.Fatalf("preflight failed: %+v %v", preflight, err)
	}
	if _, _, _, err := service.Approve(ctx, ReviewCommand{DossierID: d.ID, Note: "证据完整", PreviewDigest: testDigest("0"), ExpectedVersion: d.Version, Actor: "reviewer-1"}); !errors.Is(err, domain.ErrPreviewConflict) {
		t.Fatalf("expected stale preview conflict, got %v", err)
	}
	_, digest, d, err := service.Approve(ctx, ReviewCommand{DossierID: d.ID, Note: "证据完整", PreviewDigest: preflight.PreviewDigest, ExpectedVersion: d.Version, Actor: "reviewer-1"})
	if err != nil {
		t.Fatal(err)
	}
	if digest == "" || d.Status != domain.StatusFrozen {
		t.Fatalf("freeze failed: %s %+v", digest, d)
	}
	if digest != preflight.PreviewDigest {
		t.Fatalf("frozen manifest digest %s differs from preview %s", digest, preflight.PreviewDigest)
	}
	permit, d, err := service.IssuePermit(ctx, IssuePermitCommand{DossierID: d.ID, ExpectedVersion: d.Version, Actor: "deployer-1"})
	if err != nil {
		t.Fatal(err)
	}
	if d.Status != domain.StatusPermitted {
		t.Fatalf("unexpected status %s", d.Status)
	}
	verification, err := service.VerifyPermit(ctx, permit.PermitNumber)
	if err != nil || !verification.Valid || !verification.Checks["auditChain"].Valid || verification.VerificationMaterial.VerificationHash != permit.VerificationHash {
		t.Fatalf("permit verification failed: %+v %v", verification, err)
	}
	timeline, err := service.Timeline(ctx, d.ID)
	if err != nil || !timeline.Verified || len(timeline.Events) != 11 {
		t.Fatalf("timeline failed: %+v %v", timeline, err)
	}
	snapshot, err := service.GetDossier(ctx, d.ID)
	if err != nil || len(snapshot.RemediationAttempts) != 2 {
		t.Fatalf("remediation attempts missing: %+v %v", snapshot.RemediationAttempts, err)
	}
}

func TestIdempotencyAndOptimisticConflict(t *testing.T) {
	service, closeStore := testService(t)
	defer closeStore()
	ctx := context.Background()
	cmd := CreateDossierCommand{BuoyCode: "B-002", TargetArea: "黄海", PlannedDeploymentAt: time.Now().UTC().Add(24 * time.Hour), Owner: "李工", IdempotencyKey: "same-key", Actor: "engineer-2"}
	first, replay, err := service.CreateDossier(ctx, cmd)
	if err != nil || replay {
		t.Fatalf("first create: %v %v", replay, err)
	}
	second, replay, err := service.CreateDossier(ctx, cmd)
	if err != nil || !replay || second.ID != first.ID {
		t.Fatalf("replay: %+v %v %v", second, replay, err)
	}
	cmd.TargetArea = "渤海"
	if _, _, err := service.CreateDossier(ctx, cmd); !errors.Is(err, domain.ErrIdempotencyKey) {
		t.Fatalf("expected idempotency conflict, got %v", err)
	}
	_, _, err = service.AddSensor(ctx, AddSensorCommand{DossierID: first.ID, SensorType: "oxygen", SerialNumber: "O-1", Unit: "mg/L", RangeMin: 0, RangeMax: 20, Tolerance: .1, ConfigurationRevision: "v1", ExpectedVersion: first.Version + 1, Actor: "engineer-2"})
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("expected version conflict, got %v", err)
	}
}

func TestConcurrentCreateAndActiveBuoyConflict(t *testing.T) {
	service, closeStore := testService(t)
	defer closeStore()
	ctx := context.Background()
	cmd := CreateDossierCommand{BuoyCode: " buoy-09 ", TargetArea: "东海", PlannedDeploymentAt: time.Now().UTC().Add(24 * time.Hour), Owner: "工程师", IdempotencyKey: "concurrent", Actor: "engineer"}
	type outcome struct {
		d      domain.CalibrationDossier
		replay bool
		err    error
	}
	results := make(chan outcome, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d, replay, err := service.CreateDossier(ctx, cmd)
			results <- outcome{d, replay, err}
		}()
	}
	wg.Wait()
	close(results)
	var firstID string
	replays := 0
	for result := range results {
		if result.err != nil {
			t.Fatal(result.err)
		}
		if firstID == "" {
			firstID = result.d.ID
		} else if result.d.ID != firstID {
			t.Fatalf("concurrent create generated different dossiers")
		}
		if result.replay {
			replays++
		}
	}
	if replays != 1 {
		t.Fatalf("expected one replay, got %d", replays)
	}
	_, _, err := service.CreateDossier(ctx, CreateDossierCommand{BuoyCode: "BUOY-09", TargetArea: "黄海", PlannedDeploymentAt: time.Now().UTC().Add(48 * time.Hour), Owner: "另一工程师", IdempotencyKey: "other-key", Actor: "engineer"})
	var conflict *domain.ResourceConflictError
	if !errors.As(err, &conflict) || conflict.ResourceID != firstID || !errors.Is(err, domain.ErrActiveDossier) {
		t.Fatalf("expected active dossier conflict, got %v", err)
	}
	timeline, err := service.Timeline(ctx, firstID)
	if err != nil || len(timeline.Events) != 1 {
		t.Fatalf("unexpected creation audit events: %+v %v", timeline, err)
	}
}

func TestSensorCompletenessAndRunIdempotencyProgress(t *testing.T) {
	service, closeStore := testService(t)
	defer closeStore()
	ctx := context.Background()
	d := createForTest(t, service, "sensor-rules")
	temperature, d, err := service.AddSensor(ctx, AddSensorCommand{DossierID: d.ID, SensorType: "温度", SerialNumber: " t-1 ", Unit: "celsius", RangeMin: -5, RangeMax: 45, Tolerance: .1, ConfigurationRevision: "r1", ExpectedVersion: d.Version, Actor: "engineer"})
	if err != nil {
		t.Fatal(err)
	}
	version := d.Version
	_, _, err = service.AddSensor(ctx, AddSensorCommand{DossierID: d.ID, SensorType: "salinity", SerialNumber: "s-1", Unit: "PSU", RangeMin: 0, RangeMax: 50, Tolerance: .1, ConfigurationRevision: "r1", Complete: true, ExpectedVersion: d.Version, Actor: "engineer"})
	var validation *domain.ValidationError
	if !errors.As(err, &validation) || len(validation.MissingTypes) != 1 || validation.MissingTypes[0] != domain.SensorDissolvedOxygen {
		t.Fatalf("expected missing oxygen detail, got %v", err)
	}
	snapshot, _ := service.GetDossier(ctx, d.ID)
	if snapshot.Dossier.Version != version || len(snapshot.Sensors) != 1 {
		t.Fatalf("failed complete request changed state: %+v", snapshot)
	}
	_, d, err = service.AddSensor(ctx, AddSensorCommand{DossierID: d.ID, SensorType: "salinity", SerialNumber: "s-1", Unit: "PSU", RangeMin: 0, RangeMax: 50, Tolerance: .1, ConfigurationRevision: "r1", ExpectedVersion: d.Version, Actor: "engineer"})
	if err != nil {
		t.Fatal(err)
	}
	oxygen, d, err := service.AddSensor(ctx, AddSensorCommand{DossierID: d.ID, SensorType: "oxygen", SerialNumber: "o-1", Unit: "mg/l", RangeMin: 0, RangeMax: 20, Tolerance: .1, ConfigurationRevision: "r1", Complete: true, ExpectedVersion: d.Version, Actor: "engineer"})
	if err != nil || d.Status != domain.StatusCalibrating {
		t.Fatalf("baseline completion failed: %+v %v", d, err)
	}
	_, _, err = service.AddSensor(ctx, AddSensorCommand{DossierID: d.ID, SensorType: "oxygen", SerialNumber: oxygen.SerialNumber, Unit: "mg/L", RangeMin: 0, RangeMax: 20, Tolerance: .1, ConfigurationRevision: "r2", ExpectedVersion: d.Version, Actor: "engineer"})
	if !errors.Is(err, domain.ErrSensorConflict) {
		t.Fatalf("expected duplicate sensor conflict, got %v", err)
	}
	runCmd := SubmitRunCommand{DossierID: d.ID, SensorID: temperature.ID, ReferenceValue: 20, MeasuredValue: 20.2, AmbientTemperature: 21, EvidenceDigest: testDigest("f"), IdempotencyKey: "run-idem", ExpectedVersion: d.Version, Actor: "engineer"}
	first, firstDeviation, afterRun, replay, err := service.SubmitRun(ctx, runCmd)
	if err != nil || replay || firstDeviation == nil {
		t.Fatalf("first run failed: %+v %v %v", first, replay, err)
	}
	second, secondDeviation, _, replay, err := service.SubmitRun(ctx, runCmd)
	if err != nil || !replay || second.ID != first.ID || secondDeviation == nil || secondDeviation.ID != firstDeviation.ID {
		t.Fatalf("run replay failed: %+v %+v %v %v", second, secondDeviation, replay, err)
	}
	_, _, _, _, err = service.SubmitRun(ctx, SubmitRunCommand{DossierID: d.ID, SensorID: oxygen.ID, ReferenceValue: 8, MeasuredValue: 8, AmbientTemperature: 21, EvidenceDigest: testDigest("f"), IdempotencyKey: "other-run", ExpectedVersion: afterRun.Version, Actor: "engineer"})
	if !errors.Is(err, domain.ErrEvidenceConflict) {
		t.Fatalf("expected evidence conflict, got %v", err)
	}
	snapshot, err = service.GetDossier(ctx, d.ID)
	if err != nil || snapshot.Progress.InitialRunCount != 1 || snapshot.Progress.FailedCount != 1 || len(snapshot.Progress.MissingInitialRuns) != 2 || snapshot.Dossier.Version != afterRun.Version {
		t.Fatalf("unexpected progress: %+v %v", snapshot.Progress, err)
	}
}

func TestBatchRemediationIsAtomicAndIncrementsOnce(t *testing.T) {
	service, closeStore := testService(t)
	defer closeStore()
	ctx := context.Background()
	d := createBuoyForTest(t, service, "batch", "B-BATCH")
	temperature, d, err := service.AddSensor(ctx, AddSensorCommand{DossierID: d.ID, SensorType: "temperature", SerialNumber: "BT-1", Unit: "°C", RangeMin: -5, RangeMax: 45, Tolerance: .1, ConfigurationRevision: "v1", ExpectedVersion: d.Version, Actor: "engineer"})
	if err != nil {
		t.Fatal(err)
	}
	salinity, d, err := service.AddSensor(ctx, AddSensorCommand{DossierID: d.ID, SensorType: "salinity", SerialNumber: "BS-1", Unit: "PSU", RangeMin: 0, RangeMax: 50, Tolerance: .1, ConfigurationRevision: "v1", ExpectedVersion: d.Version, Actor: "engineer"})
	if err != nil {
		t.Fatal(err)
	}
	oxygen, d, err := service.AddSensor(ctx, AddSensorCommand{DossierID: d.ID, SensorType: "dissolved_oxygen", SerialNumber: "BO-1", Unit: "mg/L", RangeMin: 0, RangeMax: 20, Tolerance: .1, ConfigurationRevision: "v1", Complete: true, ExpectedVersion: d.Version, Actor: "engineer"})
	if err != nil {
		t.Fatal(err)
	}
	_, firstDeviation, d, _, err := service.SubmitRun(ctx, SubmitRunCommand{DossierID: d.ID, SensorID: temperature.ID, ReferenceValue: 10, MeasuredValue: 10.2, AmbientTemperature: 20, EvidenceDigest: testDigest("1"), IdempotencyKey: "batch-run-1", ExpectedVersion: d.Version, Actor: "engineer"})
	if err != nil {
		t.Fatal(err)
	}
	_, secondDeviation, d, _, err := service.SubmitRun(ctx, SubmitRunCommand{DossierID: d.ID, SensorID: salinity.ID, ReferenceValue: 10, MeasuredValue: 10.2, AmbientTemperature: 20, EvidenceDigest: testDigest("2"), IdempotencyKey: "batch-run-2", ExpectedVersion: d.Version, Actor: "engineer"})
	if err != nil {
		t.Fatal(err)
	}
	_, _, d, _, err = service.SubmitRun(ctx, SubmitRunCommand{DossierID: d.ID, SensorID: oxygen.ID, ReferenceValue: 8, MeasuredValue: 8.02, AmbientTemperature: 20, EvidenceDigest: testDigest("3"), IdempotencyKey: "batch-run-3", ExpectedVersion: d.Version, Actor: "engineer"})
	if err != nil || d.Status != domain.StatusRemediationRequired {
		t.Fatalf("setup failed: %+v %v", d, err)
	}
	version := d.Version
	validFirst := BatchRemediationItem{DeviationID: firstDeviation.ID, Cause: "漂移", Adjustment: "offset=-0.2", ReferenceValue: 10, MeasuredValue: 10.02, AmbientTemperature: 20, EvidenceDigest: testDigest("4")}
	_, _, err = service.BatchRemediate(ctx, BatchRemediationCommand{DossierID: d.ID, ExpectedVersion: version, Actor: "engineer", Items: []BatchRemediationItem{validFirst, {DeviationID: "other-dossier-deviation", Cause: "漂移", Adjustment: "offset=-0.2", ReferenceValue: 10, MeasuredValue: 10.02, AmbientTemperature: 20, EvidenceDigest: testDigest("5")}}})
	var validation *domain.ValidationError
	if !errors.As(err, &validation) || validation.ItemIndex == nil || *validation.ItemIndex != 1 {
		t.Fatalf("expected indexed batch error, got %v", err)
	}
	snapshot, err := service.GetDossier(ctx, d.ID)
	if err != nil || snapshot.Dossier.Version != version || len(snapshot.Runs) != 3 || len(snapshot.RemediationAttempts) != 0 {
		t.Fatalf("invalid batch wrote partial data: %+v %v", snapshot, err)
	}
	results, d, err := service.BatchRemediate(ctx, BatchRemediationCommand{DossierID: d.ID, ExpectedVersion: version, Actor: "engineer", Items: []BatchRemediationItem{validFirst, {DeviationID: secondDeviation.ID, Cause: "漂移", Adjustment: "offset=-0.2", ReferenceValue: 10, MeasuredValue: 10.02, AmbientTemperature: 20, EvidenceDigest: testDigest("5")}}})
	if err != nil || len(results) != 2 || !results[0].Passed || !results[1].Passed || d.Version != version+1 || d.Status != domain.StatusReviewPending {
		t.Fatalf("valid batch failed: %+v %+v %v", results, d, err)
	}
	snapshot, err = service.GetDossier(ctx, d.ID)
	if err != nil || len(snapshot.Runs) != 5 || len(snapshot.RemediationAttempts) != 2 {
		t.Fatalf("batch records missing: %+v %v", snapshot, err)
	}
	timeline, err := service.Timeline(ctx, d.ID)
	if err != nil || len(timeline.Events) != 8 || timeline.Events[len(timeline.Events)-1].EventType != "deviation.remediation_batch_attempted" {
		t.Fatalf("batch audit mismatch: %+v %v", timeline, err)
	}
}
