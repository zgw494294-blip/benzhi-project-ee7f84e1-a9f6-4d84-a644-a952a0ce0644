package submit_run_idempotency_scope_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"buoy-calibration-gate/internal/calibration"
	"buoy-calibration-gate/internal/domain"
	"buoy-calibration-gate/internal/repository"
)

func TestSubmitRunIdempotencyIsScopedToDossierAndSensor(t *testing.T) {
	ctx := context.Background()
	store, err := repository.Open(ctx, filepath.Join(t.TempDir(), "scope.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	service := calibration.NewService(store)

	firstDossier, firstSensor := readyDossier(t, service, "BUOY-SCOPE-A", "create-scope-a", "A")
	secondDossier, secondSensor := readyDossier(t, service, "BUOY-SCOPE-B", "create-scope-b", "B")
	sharedKey := "station-upload-42"

	_, _, firstDossier, replay, err := service.SubmitRun(ctx, calibration.SubmitRunCommand{
		DossierID:          firstDossier.ID,
		SensorID:           firstSensor.ID,
		ReferenceValue:     20,
		MeasuredValue:      20.02,
		AmbientTemperature: 21,
		EvidenceDigest:     digest("a"),
		IdempotencyKey:     sharedKey,
		ExpectedVersion:    firstDossier.Version,
		Actor:              "engineer-a",
	})
	if err != nil || replay {
		t.Fatalf("首个档案预热运行重放缓存失败: replay=%v err=%v", replay, err)
	}

	secondRun, _, secondDossier, replay, err := service.SubmitRun(ctx, calibration.SubmitRunCommand{
		DossierID:          secondDossier.ID,
		SensorID:           secondSensor.ID,
		ReferenceValue:     19,
		MeasuredValue:      19.01,
		AmbientTemperature: 20,
		EvidenceDigest:     digest("b"),
		IdempotencyKey:     sharedKey,
		ExpectedVersion:    secondDossier.Version,
		Actor:              "engineer-b",
	})
	if err != nil {
		t.Fatalf("不同档案与传感器应拥有独立幂等作用域: %v", err)
	}
	if replay {
		t.Fatal("第二个档案的首次运行被错误标记为重放")
	}
	if secondRun.DossierID != secondDossier.ID || secondRun.SensorID != secondSensor.ID {
		t.Fatalf("第二个档案收到其他作用域的运行: %+v", secondRun)
	}
	snapshot, err := service.GetDossier(ctx, secondDossier.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Runs) != 1 || snapshot.Runs[0].ID != secondRun.ID {
		t.Fatalf("第二个档案的运行未独立持久化: %+v", snapshot.Runs)
	}
}

func readyDossier(t *testing.T, service *calibration.Service, buoyCode, createKey, serialPrefix string) (domain.CalibrationDossier, domain.SensorBaseline) {
	t.Helper()
	ctx := context.Background()
	dossier, _, err := service.CreateDossier(ctx, calibration.CreateDossierCommand{
		BuoyCode:            buoyCode,
		TargetArea:          "南海试验区",
		PlannedDeploymentAt: time.Now().UTC().Add(24 * time.Hour),
		Owner:               "校准工程师",
		IdempotencyKey:      createKey,
		Actor:               "engineer-setup",
	})
	if err != nil {
		t.Fatal(err)
	}
	temperature, dossier, err := service.AddSensor(ctx, calibration.AddSensorCommand{
		DossierID: dossier.ID, SensorType: "temperature", SerialNumber: serialPrefix + "-T", Unit: "°C",
		RangeMin: -5, RangeMax: 45, Tolerance: 0.1, ConfigurationRevision: "cfg-1",
		ExpectedVersion: dossier.Version, Actor: "engineer-setup",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, dossier, err = service.AddSensor(ctx, calibration.AddSensorCommand{
		DossierID: dossier.ID, SensorType: "salinity", SerialNumber: serialPrefix + "-S", Unit: "PSU",
		RangeMin: 0, RangeMax: 50, Tolerance: 0.1, ConfigurationRevision: "cfg-1",
		ExpectedVersion: dossier.Version, Actor: "engineer-setup",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, dossier, err = service.AddSensor(ctx, calibration.AddSensorCommand{
		DossierID: dossier.ID, SensorType: "dissolved_oxygen", SerialNumber: serialPrefix + "-O", Unit: "mg/L",
		RangeMin: 0, RangeMax: 20, Tolerance: 0.1, ConfigurationRevision: "cfg-1", Complete: true,
		ExpectedVersion: dossier.Version, Actor: "engineer-setup",
	})
	if err != nil {
		t.Fatal(err)
	}
	return dossier, temperature
}

func digest(ch string) string {
	return "sha256:" + strings.Repeat(ch, 64)
}
