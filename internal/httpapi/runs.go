package httpapi

import (
	"net/http"
	"strings"

	"buoy-calibration-gate/internal/calibration"
	"buoy-calibration-gate/internal/domain"
)

func (a *API) SubmitRunHandler(w http.ResponseWriter, r *http.Request) {
	actor, err := actorAndRole(r, domain.RoleEngineer)
	if err != nil {
		writeError(w, err)
		return
	}
	version, err := expectedVersion(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var cmd calibration.SubmitRunCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	cmd.DossierID, cmd.ExpectedVersion, cmd.Actor = r.PathValue("dossierID"), version, actor
	cmd.IdempotencyKey = strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	run, deviation, dossier, replay, err := a.service.SubmitRun(r.Context(), cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	status := http.StatusCreated
	if replay {
		status = http.StatusOK
		w.Header().Set("Idempotent-Replayed", "true")
	}
	writeJSON(w, status, map[string]any{"run": run, "deviation": deviation, "dossier": dossier})
}
