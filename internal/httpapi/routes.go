package httpapi

func (a *API) routes() {
	a.mux.HandleFunc("GET /health/ready", a.ReadinessHandler)
	a.mux.HandleFunc("POST /api/v1/calibration-dossiers", a.CreateDossierHandler)
	a.mux.HandleFunc("GET /api/v1/calibration-dossiers/{dossierID}", a.GetDossierHandler)
	a.mux.HandleFunc("POST /api/v1/calibration-dossiers/{dossierID}/sensors", a.AddSensorHandler)
	a.mux.HandleFunc("POST /api/v1/calibration-dossiers/{dossierID}/runs", a.SubmitRunHandler)
	a.mux.HandleFunc("POST /api/v1/calibration-dossiers/{dossierID}/deviations/{deviationID}/remediation", a.RemediateHandler)
	a.mux.HandleFunc("POST /api/v1/calibration-dossiers/{dossierID}/deviations/remediations:batch", a.BatchRemediateHandler)
	a.mux.HandleFunc("GET /api/v1/calibration-dossiers/{dossierID}/reviews/preflight", a.ReviewPreflightHandler)
	a.mux.HandleFunc("POST /api/v1/calibration-dossiers/{dossierID}/reviews/return", a.ReturnDeviationHandler)
	a.mux.HandleFunc("POST /api/v1/calibration-dossiers/{dossierID}/reviews/approve", a.ApproveHandler)
	a.mux.HandleFunc("POST /api/v1/calibration-dossiers/{dossierID}/permit", a.IssuePermitHandler)
	a.mux.HandleFunc("GET /api/v1/calibration-dossiers/{dossierID}/timeline", a.TimelineHandler)
	a.mux.HandleFunc("GET /api/v1/deployment-permits/{permitNumber}/verify", a.VerifyPermitHandler)
}
