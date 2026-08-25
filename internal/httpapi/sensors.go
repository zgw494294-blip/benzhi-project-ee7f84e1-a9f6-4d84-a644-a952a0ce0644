package httpapi

import (
	"context"
	"net/http"

	"buoy-calibration-gate/internal/calibration"
	"buoy-calibration-gate/internal/domain"
)

func (a *API) AddSensorHandler(w http.ResponseWriter, r *http.Request) {
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
	var cmd calibration.AddSensorCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	cmd.DossierID, cmd.ExpectedVersion, cmd.Actor = r.PathValue("dossierID"), version, actor
	sensor, dossier, err := a.service.AddSensor(context.WithoutCancel(r.Context()), cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"sensor": sensor, "dossier": dossier})
}
