package reviewpreflightactorcache

import (
	"context"
	"strings"
	"testing"
	"time"

	"buoy-calibration-gate/internal/calibration"
	"buoy-calibration-gate/internal/domain"
	"buoy-calibration-gate/internal/repository"
)

func TestReviewPreflightCacheIsActorScoped(t *testing.T) {
	ctx := context.Background()
	store, err := repository.Open(ctx, t.TempDir()+"/calibration.sqlite")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	service := calibration.NewService(store)

	dossier, _, err := service.CreateDossier(ctx, calibration.CreateDossierCommand{
		BuoyCode:            "CACHE-ACTOR-01",
		TargetArea:          "南海",
		PlannedDeploymentAt: time.Now().UTC().Add(24 * time.Hour),
		Owner:               "校准工程师",
		IdempotencyKey:      "create-cache-actor",
		Actor:               "engineer-1",
	})
	if err != nil {
		t.Fatal(err)
	}

	sensorCommands := []calibration.AddSensorCommand{
		{SensorType: "temperature", SerialNumber: "TEMP-CACHE-01", Unit: "°C", RangeMin: -5, RangeMax: 45, Tolerance: 0.1, ConfigurationRevision: "cfg-1"},
		{SensorType: "salinity", SerialNumber: "SALT-CACHE-01", Unit: "PSU", RangeMin: 0, RangeMax: 50, Tolerance: 0.1, ConfigurationRevision: "cfg-1"},
		{SensorType: "dissolved_oxygen", SerialNumber: "DO-CACHE-01", Unit: "mg/L", RangeMin: 0, RangeMax: 20, Tolerance: 0.1, ConfigurationRevision: "cfg-1", Complete: true},
	}
	sensors := make([]domain.SensorBaseline, 0, len(sensorCommands))
	for _, command := range sensorCommands {
		command.DossierID = dossier.ID
		command.ExpectedVersion = dossier.Version
		command.Actor = "engineer-1"
		sensor, updated, addErr := service.AddSensor(ctx, command)
		if addErr != nil {
			t.Fatal(addErr)
		}
		sensors = append(sensors, sensor)
		dossier = updated
	}

	for index, sensor := range sensors {
		_, _, updated, _, submitErr := service.SubmitRun(ctx, calibration.SubmitRunCommand{
			DossierID:          dossier.ID,
			SensorID:           sensor.ID,
			ReferenceValue:     10,
			MeasuredValue:      10.01,
			AmbientTemperature: 20,
			EvidenceDigest:     "sha256:" + strings.Repeat(string(rune('a'+index)), 64),
			IdempotencyKey:     "run-cache-actor-" + string(rune('1'+index)),
			ExpectedVersion:    dossier.Version,
			Actor:              "engineer-1",
		})
		if submitErr != nil {
			t.Fatal(submitErr)
		}
		dossier = updated
	}
	if dossier.Status != domain.StatusReviewPending {
		t.Fatalf("setup status = %s, want %s", dossier.Status, domain.StatusReviewPending)
	}

	first, err := service.ReviewPreflight(ctx, dossier.ID, "reviewer-a")
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.ReviewPreflight(ctx, dossier.ID, "reviewer-b")
	if err != nil {
		t.Fatal(err)
	}
	if first.PreviewDigest == "" || second.PreviewDigest == "" {
		t.Fatal("preflight preview digest must not be empty")
	}

	_, _, frozen, err := service.Approve(ctx, calibration.ReviewCommand{
		DossierID:       dossier.ID,
		Note:            "证据完整",
		ExpectedVersion: dossier.Version,
		Actor:           "reviewer-b",
		PreviewDigest:   second.PreviewDigest,
	})
	if err != nil {
		t.Fatalf("reviewer-b cannot approve with its own preflight digest: %v", err)
	}
	if frozen.Status != domain.StatusFrozen {
		t.Fatalf("approved status = %s, want %s", frozen.Status, domain.StatusFrozen)
	}
}
