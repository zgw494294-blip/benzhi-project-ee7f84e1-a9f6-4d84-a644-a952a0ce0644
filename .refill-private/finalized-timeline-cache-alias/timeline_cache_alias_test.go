package finalizedtimelinecachealias_test

import (
	"context"
	"testing"
	"time"

	"buoy-calibration-gate/internal/audit"
	"buoy-calibration-gate/internal/calibration"
	"buoy-calibration-gate/internal/domain"
	"buoy-calibration-gate/internal/repository"
)

func TestFinalizedTimelineCacheReturnsIndependentCopies(t *testing.T) {
	ctx := context.Background()
	store, err := repository.Open(ctx, "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	now := time.Date(2026, 8, 25, 4, 0, 0, 0, time.UTC)
	dossier := domain.CalibrationDossier{
		ID:                  "dossier-finalized",
		BuoyCode:            "BUOY-FINALIZED",
		TargetArea:          "南海",
		PlannedDeploymentAt: now.Add(48 * time.Hour),
		Owner:               "工程师",
		Status:              domain.StatusPermitted,
		Version:             7,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	if err := store.Transaction(ctx, func(tx *repository.Tx) error {
		if err := tx.InsertDossier(dossier); err != nil {
			return err
		}
		_, err := audit.Append(tx, dossier.ID, "permit.issued", "deployer-1", map[string]string{"permitNumber": "BCG-20260825-FINALIZED"}, now)
		return err
	}); err != nil {
		t.Fatal(err)
	}

	service := calibration.NewService(store)
	first, err := service.Timeline(ctx, dossier.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Events) != 1 || len(first.Events[0].Payload) == 0 {
		t.Fatalf("unexpected timeline fixture: %+v", first)
	}
	first.Events[0].Payload[0] = '!'

	second, err := service.Timeline(ctx, dossier.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := audit.VerifyEventChain(second.Events); err != nil {
		t.Fatalf("cached finalized timeline leaked caller mutation: %v", err)
	}
}
