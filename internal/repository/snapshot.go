package repository

import "buoy-calibration-gate/internal/domain"

type DossierSnapshot struct {
	Dossier             domain.CalibrationDossier   `json:"dossier"`
	Sensors             []domain.SensorBaseline     `json:"sensors"`
	Runs                []domain.CalibrationRun     `json:"runs"`
	Deviations          []domain.DeviationCase      `json:"deviations"`
	RemediationAttempts []domain.RemediationAttempt `json:"remediationAttempts"`
	Progress            domain.CalibrationProgress  `json:"progress"`
	NextActions         domain.WorkflowGuidance     `json:"nextActions"`
	Permit              *domain.DeploymentPermit    `json:"permit,omitempty"`
}

func (t *Tx) Snapshot(dossierID string) (DossierSnapshot, error) {
	var s DossierSnapshot
	var err error
	if s.Dossier, err = t.GetDossier(dossierID); err != nil {
		return s, err
	}
	if s.Sensors, err = t.ListSensors(dossierID); err != nil {
		return s, err
	}
	if s.Runs, err = t.ListRuns(dossierID); err != nil {
		return s, err
	}
	if s.Deviations, err = t.ListDeviations(dossierID); err != nil {
		return s, err
	}
	if s.RemediationAttempts, err = t.ListRemediationAttempts(dossierID); err != nil {
		return s, err
	}
	s.Progress, s.NextActions = domain.BuildProgress(s.Dossier, s.Sensors, s.Runs, s.Deviations)
	permit, err := t.GetPermitByDossier(dossierID)
	if err == nil {
		s.Permit = &permit
	} else if err != domain.ErrNotFound {
		return s, err
	}
	return s, nil
}
