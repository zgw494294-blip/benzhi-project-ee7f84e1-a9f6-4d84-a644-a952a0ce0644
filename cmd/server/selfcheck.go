package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type checkResponse struct {
	Dossier struct {
		ID      string `json:"id"`
		Version int64  `json:"version"`
		Status  string `json:"status"`
	} `json:"dossier"`
	Sensor struct {
		ID string `json:"id"`
	} `json:"sensor"`
	Permit struct {
		PermitNumber string `json:"permitNumber"`
	} `json:"permit"`
	Valid         bool   `json:"valid"`
	Verified      bool   `json:"verified"`
	PreviewDigest string `json:"previewDigest"`
}

func runSelfcheck(ctx context.Context, address string) error {
	baseURL := "http://" + address
	client := &http.Client{Timeout: 2 * time.Second}
	if _, err := checkRequest(ctx, client, http.MethodGet, baseURL+"/health/ready", nil, nil, http.StatusOK); err != nil {
		return err
	}
	planned := time.Now().UTC().Add(72 * time.Hour).Format(time.RFC3339)
	created, err := checkRequest(ctx, client, http.MethodPost, baseURL+"/api/v1/calibration-dossiers", map[string]any{"buoyCode": "SELF-BUOY-01", "targetArea": "自检海域", "plannedDeploymentAt": planned, "owner": "自检工程师"}, map[string]string{"X-Actor": "selfcheck-engineer", "X-Role": "engineer", "Idempotency-Key": "selfcheck-create"}, http.StatusCreated)
	if err != nil {
		return err
	}
	id, version := created.Dossier.ID, created.Dossier.Version
	sensorSpecs := []map[string]any{
		{"sensorType": "temperature", "serialNumber": "SELF-TEMP-01", "unit": "°C", "rangeMin": -5, "rangeMax": 45, "tolerance": 0.1, "configurationRevision": "self-v1"},
		{"sensorType": "salinity", "serialNumber": "SELF-SALT-01", "unit": "PSU", "rangeMin": 0, "rangeMax": 50, "tolerance": 0.1, "configurationRevision": "self-v1"},
		{"sensorType": "dissolved_oxygen", "serialNumber": "SELF-OXY-01", "unit": "mg/L", "rangeMin": 0, "rangeMax": 20, "tolerance": 0.1, "configurationRevision": "self-v1", "complete": true},
	}
	sensorIDs := make([]string, 0, len(sensorSpecs))
	for _, body := range sensorSpecs {
		added, err := checkRequest(ctx, client, http.MethodPost, baseURL+"/api/v1/calibration-dossiers/"+id+"/sensors", body, writeHeaders("selfcheck-engineer", "engineer", version), http.StatusCreated)
		if err != nil {
			return err
		}
		version = added.Dossier.Version
		sensorIDs = append(sensorIDs, added.Sensor.ID)
	}
	for index, sensorID := range sensorIDs {
		headers := writeHeaders("selfcheck-engineer", "engineer", version)
		headers["Idempotency-Key"] = fmt.Sprintf("selfcheck-run-%d", index)
		run, err := checkRequest(ctx, client, http.MethodPost, baseURL+"/api/v1/calibration-dossiers/"+id+"/runs", map[string]any{"sensorId": sensorID, "referenceValue": 10.0, "measuredValue": 10.05, "ambientTemperature": 22.0, "evidenceDigest": "sha256:" + strings.Repeat(fmt.Sprint(index+1), 64)}, headers, http.StatusCreated)
		if err != nil {
			return err
		}
		version = run.Dossier.Version
		if index == len(sensorIDs)-1 && run.Dossier.Status != "review_pending" {
			return fmt.Errorf("校准后状态异常: %s", run.Dossier.Status)
		}
	}
	preflight, err := checkRequest(ctx, client, http.MethodGet, baseURL+"/api/v1/calibration-dossiers/"+id+"/reviews/preflight", nil, map[string]string{"X-Actor": "selfcheck-reviewer", "X-Role": "reviewer"}, http.StatusOK)
	if err != nil {
		return err
	}
	if preflight.PreviewDigest == "" {
		return fmt.Errorf("冻结前预检未生成摘要")
	}
	approved, err := checkRequest(ctx, client, http.MethodPost, baseURL+"/api/v1/calibration-dossiers/"+id+"/reviews/approve", map[string]any{"note": "自检复核通过", "previewDigest": preflight.PreviewDigest}, writeHeaders("selfcheck-reviewer", "reviewer", version), http.StatusOK)
	if err != nil {
		return err
	}
	version = approved.Dossier.Version
	issued, err := checkRequest(ctx, client, http.MethodPost, baseURL+"/api/v1/calibration-dossiers/"+id+"/permit", map[string]any{}, writeHeaders("selfcheck-deployer", "deployer", version), http.StatusCreated)
	if err != nil {
		return err
	}
	verified, err := checkRequest(ctx, client, http.MethodGet, baseURL+"/api/v1/deployment-permits/"+issued.Permit.PermitNumber+"/verify", nil, nil, http.StatusOK)
	if err != nil {
		return err
	}
	if !verified.Valid {
		return fmt.Errorf("许可验真未通过")
	}
	timeline, err := checkRequest(ctx, client, http.MethodGet, baseURL+"/api/v1/calibration-dossiers/"+id+"/timeline", nil, nil, http.StatusOK)
	if err != nil {
		return err
	}
	if !timeline.Verified {
		return fmt.Errorf("审计链校验未通过")
	}
	return nil
}

func writeHeaders(actor, role string, version int64) map[string]string {
	return map[string]string{"X-Actor": actor, "X-Role": role, "X-Expected-Version": fmt.Sprint(version)}
}

func checkRequest(ctx context.Context, client *http.Client, method, url string, body any, headers map[string]string, expected int) (checkResponse, error) {
	var data io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return checkResponse{}, err
		}
		data = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, data)
	if err != nil {
		return checkResponse{}, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := client.Do(req)
	if err != nil {
		return checkResponse{}, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return checkResponse{}, err
	}
	if resp.StatusCode != expected {
		return checkResponse{}, fmt.Errorf("%s %s 返回 %d: %s", method, url, resp.StatusCode, string(raw))
	}
	var decoded checkResponse
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return checkResponse{}, err
	}
	return decoded, nil
}
