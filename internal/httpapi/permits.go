package httpapi

import (
	"context"
	"net/http"

	"buoy-calibration-gate/internal/calibration"
	"buoy-calibration-gate/internal/domain"
)

func (a *API) IssuePermitHandler(w http.ResponseWriter, r *http.Request) {
	actor, err := actorAndRole(r, domain.RoleDeployer)
	if err != nil {
		writeError(w, err)
		return
	}
	version, err := expectedVersion(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct{}
	if err := decodeJSON(w, r, &body); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	permit, dossier, err := a.service.IssuePermit(context.WithoutCancel(r.Context()), calibration.IssuePermitCommand{DossierID: r.PathValue("dossierID"), ExpectedVersion: version, Actor: actor})
	if err != nil {
		writeError(w, err)
		return
	}
	verificationURL := "/api/v1/deployment-permits/" + permit.PermitNumber + "/verify"
	writeJSON(w, http.StatusCreated, map[string]any{"permit": permit, "dossier": dossier, "verificationUrl": verificationURL})
}

func (a *API) VerifyPermitHandler(w http.ResponseWriter, r *http.Request) {
	result, err := a.service.VerifyPermit(r.Context(), r.PathValue("permitNumber"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
