package writerequestcancellation_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"buoy-calibration-gate/internal/calibration"
	"buoy-calibration-gate/internal/httpapi"
	"buoy-calibration-gate/internal/repository"
)

func TestCanceledSensorWriteDoesNotCommit(t *testing.T) {
	store, err := repository.Open(context.Background(), "file:write-request-cancellation?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	service := calibration.NewService(store)
	dossier, _, err := service.CreateDossier(context.Background(), calibration.CreateDossierCommand{
		BuoyCode:            "CANCEL-WRITE",
		TargetArea:          "东海",
		PlannedDeploymentAt: time.Now().UTC().Add(24 * time.Hour),
		Owner:               "取消测试工程师",
		IdempotencyKey:      "cancel-write-setup",
		Actor:               "engineer-cancel",
	})
	if err != nil {
		t.Fatal(err)
	}

	body, err := json.Marshal(map[string]any{
		"sensorType":            "temperature",
		"serialNumber":          "CANCEL-TEMP-1",
		"unit":                  "°C",
		"rangeMin":              -5,
		"rangeMax":              45,
		"tolerance":             0.1,
		"configurationRevision": "cancel-v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/calibration-dossiers/"+dossier.ID+"/sensors", bytes.NewReader(body)).WithContext(ctx)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Actor", "engineer-cancel")
	request.Header.Set("X-Role", "engineer")
	request.Header.Set("X-Expected-Version", strconv.FormatInt(dossier.Version, 10))
	response := httptest.NewRecorder()
	httpapi.New(service).Handler().ServeHTTP(response, request)

	snapshot, err := service.GetDossier(context.Background(), dossier.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Sensors) != 0 {
		t.Fatalf("已取消的写请求仍提交了 %d 个传感器，HTTP 状态码为 %d", len(snapshot.Sensors), response.Code)
	}
}
