package permitverification_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"buoy-calibration-gate/internal/audit"
	"buoy-calibration-gate/internal/calibration"
	"buoy-calibration-gate/internal/domain"
	"buoy-calibration-gate/internal/repository"
)

func TestPermitVerificationReloadsChangedMaterial(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "verification.db")
	store, err := repository.Open(context.Background(), databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	issuedAt := time.Date(2026, 8, 25, 4, 0, 0, 0, time.UTC)
	dossier := domain.CalibrationDossier{
		ID:                  "dossier-cache-repro",
		BuoyCode:            "BUOY-CACHE-01",
		TargetArea:          "东海测试海域",
		PlannedDeploymentAt: issuedAt.Add(72 * time.Hour),
		Owner:               "校准工程师",
		Status:              domain.StatusPermitted,
		Version:             2,
		CreatedAt:           issuedAt.Add(-time.Hour),
		UpdatedAt:           issuedAt,
	}
	permit := domain.DeploymentPermit{
		ID:             "permit-cache-repro",
		PermitNumber:   "BCG-20260825-CACHE00001",
		DossierID:      dossier.ID,
		FrozenVersion:  1,
		ManifestDigest: "manifest-cache-repro",
		ApprovedBy:     "reviewer-cache-repro",
		IssuedAt:       issuedAt,
	}
	permit.VerificationHash = audit.PermitVerificationHash(permit)
	if err := store.Transaction(context.Background(), func(tx *repository.Tx) error {
		if err := tx.InsertDossier(dossier); err != nil {
			return err
		}
		return tx.InsertPermit(permit)
	}); err != nil {
		t.Fatal(err)
	}

	service := calibration.NewService(store)
	first, err := service.VerifyPermit(context.Background(), permit.PermitNumber)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Checks["permitHash"].Valid {
		t.Fatal("测试前置条件失败：原始许可哈希应有效")
	}

	external, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := external.ExecContext(context.Background(),
		"UPDATE deployment_permits SET verification_hash=? WHERE permit_number=?",
		"tampered-verification-hash", permit.PermitNumber); err != nil {
		external.Close()
		t.Fatal(err)
	}
	if err := external.Close(); err != nil {
		t.Fatal(err)
	}

	second, err := service.VerifyPermit(context.Background(), permit.PermitNumber)
	if err != nil {
		t.Fatal(err)
	}
	if second.Checks["permitHash"].Valid {
		t.Fatalf("TestPermitVerificationReloadsChangedMaterial: 底层许可哈希已变化，验真仍复用了旧结果")
	}
}
