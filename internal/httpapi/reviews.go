package httpapi

import (
	"net/http"

	"buoy-calibration-gate/internal/calibration"
	"buoy-calibration-gate/internal/domain"
)

func (a *API) ReturnDeviationHandler(w http.ResponseWriter, r *http.Request) {
	actor, err := a.actorAndRole(r, domain.RoleReviewer)
	if err != nil {
		writeError(w, err)
		return
	}
	version, err := expectedVersion(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var cmd calibration.ReviewCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	cmd.DossierID, cmd.ExpectedVersion, cmd.Actor = r.PathValue("dossierID"), version, actor
	deviation, dossier, err := a.service.ReturnDeviation(r.Context(), cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deviation": deviation, "dossier": dossier})
}

func (a *API) ApproveHandler(w http.ResponseWriter, r *http.Request) {
	actor, err := a.actorAndRole(r, domain.RoleReviewer)
	if err != nil {
		writeError(w, err)
		return
	}
	version, err := expectedVersion(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var cmd calibration.ReviewCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	cmd.DossierID, cmd.ExpectedVersion, cmd.Actor = r.PathValue("dossierID"), version, actor
	_, digest, dossier, err := a.service.Approve(r.Context(), cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"dossier": dossier, "manifestDigest": digest, "frozenVersion": dossier.Version})
}

func (a *API) ReviewPreflightHandler(w http.ResponseWriter, r *http.Request) {
	actor, err := a.actorAndRole(r, domain.RoleReviewer)
	if err != nil {
		writeError(w, err)
		return
	}
	result, err := a.service.ReviewPreflight(r.Context(), r.PathValue("dossierID"), actor)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
