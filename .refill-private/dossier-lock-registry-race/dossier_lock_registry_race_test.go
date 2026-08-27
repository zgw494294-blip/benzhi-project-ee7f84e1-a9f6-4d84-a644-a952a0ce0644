package dossier_lock_registry_race_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"buoy-calibration-gate/internal/calibration"
	"buoy-calibration-gate/internal/repository"
)

func TestConcurrentDossierLockRegistryIsSynchronized(t *testing.T) {
	ctx := context.Background()
	store, err := repository.Open(ctx, "file:dossier-lock-registry-race?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	service := calibration.NewService(store)

	dossiers := make([]string, 2)
	for index := range dossiers {
		dossier, _, err := service.CreateDossier(ctx, calibration.CreateDossierCommand{
			BuoyCode:            "RACE-BUOY-" + string(rune('A'+index)),
			TargetArea:          "南海测试区",
			PlannedDeploymentAt: time.Now().UTC().Add(48 * time.Hour),
			Owner:               "并发测试工程师",
			IdempotencyKey:      "race-dossier-" + string(rune('A'+index)),
			Actor:               "engineer-race",
		})
		if err != nil {
			t.Fatal(err)
		}
		dossiers[index] = dossier.ID
	}

	start := make(chan struct{})
	errors := make(chan error, len(dossiers))
	var ready sync.WaitGroup
	var done sync.WaitGroup
	ready.Add(len(dossiers))
	done.Add(len(dossiers))
	for index, dossierID := range dossiers {
		go func() {
			defer done.Done()
			ready.Done()
			<-start
			_, _, err := service.AddSensor(ctx, calibration.AddSensorCommand{
				DossierID:             dossierID,
				SensorType:            "temperature",
				SerialNumber:          "RACE-SENSOR-" + string(rune('A'+index)),
				Unit:                  "°C",
				RangeMin:              -5,
				RangeMax:              45,
				Tolerance:             0.1,
				ConfigurationRevision: "race-v1",
				ExpectedVersion:       1,
				Actor:                 "engineer-race",
			})
			errors <- err
		}()
	}
	ready.Wait()
	close(start)
	done.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatalf("并发登记传感器失败: %v", err)
		}
	}
}
