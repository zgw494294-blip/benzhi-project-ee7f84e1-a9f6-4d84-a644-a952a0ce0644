package httpapi

import (
	"net/http"

	"buoy-calibration-gate/internal/calibration"
	"buoy-calibration-gate/internal/domain"
)

func (a *API) RemediateHandler(w http.ResponseWriter, r *http.Request) {
	actor, err := a.actorAndRole(domain.RoleEngineer)
	if err != nil {
		writeError(w, err)
		return
	}
	version, err := expectedVersion(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var cmd calibration.RemediateCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	cmd.DossierID, cmd.DeviationID, cmd.ExpectedVersion, cmd.Actor = r.PathValue("dossierID"), r.PathValue("deviationID"), version, actor
	deviation, retest, attempt, dossier, err := a.service.Remediate(r.Context(), cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deviation": deviation, "retestRun": retest, "attempt": attempt, "passed": retest.Passed, "dossier": dossier})
}

func (a *API) BatchRemediateHandler(w http.ResponseWriter, r *http.Request) {
	actor, err := a.actorAndRole(domain.RoleEngineer)
	if err != nil {
		writeError(w, err)
		return
	}
	version, err := expectedVersion(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var cmd calibration.BatchRemediationCommand
	if err := decodeJSONLimit(w, r, &cmd, 256<<10); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	cmd.DossierID, cmd.ExpectedVersion, cmd.Actor = r.PathValue("dossierID"), version, actor
	results, dossier, err := a.service.BatchRemediate(r.Context(), cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": results, "dossier": dossier})
}
