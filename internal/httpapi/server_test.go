package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"buoy-calibration-gate/internal/calibration"
	"buoy-calibration-gate/internal/repository"
)

func TestCreateDossierHTTPContract(t *testing.T) {
	store, err := repository.Open(context.Background(), "file:http-contract?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	server := httptest.NewServer(New(calibration.NewService(store)).Handler())
	defer server.Close()
	body, _ := json.Marshal(map[string]any{"buoyCode": "B-HTTP", "targetArea": "东海", "plannedDeploymentAt": time.Now().UTC().Add(24 * time.Hour), "owner": "HTTP 工程师"})
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/calibration-dossiers", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Actor", "engineer-http")
	req.Header.Set("X-Role", "engineer")
	req.Header.Set("Idempotency-Key", "http-create")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("unexpected status %d", resp.StatusCode)
	}
	if resp.Header.Get("Location") == "" {
		t.Fatal("missing Location header")
	}
}

func TestRoleAndUnknownFieldValidation(t *testing.T) {
	store, err := repository.Open(context.Background(), "file:http-validation?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	handler := New(calibration.NewService(store)).Handler()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/calibration-dossiers", bytes.NewBufferString(`{"unknown":true}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Actor", "reviewer")
	request.Header.Set("X-Role", "reviewer")
	request.Header.Set("Idempotency-Key", "invalid")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden, got %d", response.Code)
	}
}
