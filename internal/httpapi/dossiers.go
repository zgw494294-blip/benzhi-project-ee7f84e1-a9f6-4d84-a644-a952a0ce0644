package httpapi

import (
	"net/http"
	"strings"
	"time"

	"buoy-calibration-gate/internal/calibration"
	"buoy-calibration-gate/internal/domain"
)

func (a *API) CreateDossierHandler(w http.ResponseWriter, r *http.Request) {
	actor, err := a.actorAndRole(domain.RoleEngineer)
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		BuoyCode            string `json:"buoyCode"`
		TargetArea          string `json:"targetArea"`
		PlannedDeploymentAt string `json:"plannedDeploymentAt"`
		Owner               string `json:"owner"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	planned, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(body.PlannedDeploymentAt))
	if err != nil || !strings.ContainsAny(body.PlannedDeploymentAt, "Zz+-") {
		writeError(w, &domain.RuleError{Field: "plannedDeploymentAt", Message: "必须包含明确时区并使用 RFC3339 格式"})
		return
	}
	cmd := calibration.CreateDossierCommand{BuoyCode: body.BuoyCode, TargetArea: body.TargetArea, PlannedDeploymentAt: planned.UTC(), Owner: body.Owner}
	cmd.Actor = actor
	cmd.IdempotencyKey = strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	dossier, replay, err := a.service.CreateDossier(r.Context(), cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	status := http.StatusCreated
	if replay {
		status = http.StatusOK
		w.Header().Set("Idempotent-Replayed", "true")
	}
	w.Header().Set("Location", "/api/v1/calibration-dossiers/"+dossier.ID)
	writeJSON(w, status, map[string]any{"dossier": dossier})
}

func (a *API) GetDossierHandler(w http.ResponseWriter, r *http.Request) {
	result, err := a.service.GetDossier(r.Context(), r.PathValue("dossierID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
