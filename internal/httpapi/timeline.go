package httpapi

import "net/http"

func (a *API) TimelineHandler(w http.ResponseWriter, r *http.Request) {
	timeline, err := a.service.Timeline(r.Context(), r.PathValue("dossierID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, timeline)
}
