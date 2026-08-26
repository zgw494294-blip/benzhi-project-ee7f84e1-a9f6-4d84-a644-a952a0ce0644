package create_idempotency_restart_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"buoy-calibration-gate/internal/calibration"
	"buoy-calibration-gate/internal/repository"
)

func TestCreateIdempotencySurvivesServiceRestart(t *testing.T) {
	ctx := context.Background()
	databasePath := filepath.Join(t.TempDir(), "calibration.db")
	cmd := calibration.CreateDossierCommand{
		BuoyCode:            "BUOY-RESTART-17",
		TargetArea:          "东海重启恢复试验区",
		PlannedDeploymentAt: time.Date(2027, time.March, 18, 6, 30, 0, 0, time.UTC),
		Owner:               "重启测试工程师",
		IdempotencyKey:      "create-across-restart-17",
		Actor:               "engineer-restart",
	}

	firstStore, err := repository.Open(ctx, databasePath)
	if err != nil {
		t.Fatal(err)
	}
	firstService := calibration.NewService(firstStore)
	first, replayed, err := firstService.CreateDossier(ctx, cmd)
	if err != nil || replayed {
		t.Fatalf("首次创建失败: replayed=%v err=%v", replayed, err)
	}
	if err := firstStore.Close(); err != nil {
		t.Fatal(err)
	}

	reopenedStore, err := repository.Open(ctx, databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer reopenedStore.Close()
	restartedService := calibration.NewService(reopenedStore)
	second, replayed, err := restartedService.CreateDossier(ctx, cmd)
	if err != nil {
		t.Fatalf("服务重启后的幂等重放返回错误: %v", err)
	}
	if !replayed {
		t.Fatal("服务重启后相同 Idempotency-Key 未标记为重放")
	}
	if second.ID != first.ID {
		t.Fatalf("服务重启后创建了不同档案: first=%s second=%s", first.ID, second.ID)
	}
}
