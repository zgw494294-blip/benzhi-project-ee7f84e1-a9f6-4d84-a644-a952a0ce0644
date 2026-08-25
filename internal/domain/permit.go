package domain

import "time"

type DeploymentPermit struct {
	ID               string    `json:"id"`
	PermitNumber     string    `json:"permitNumber"`
	DossierID        string    `json:"dossierId"`
	FrozenVersion    int64     `json:"frozenVersion"`
	ManifestDigest   string    `json:"manifestDigest"`
	VerificationHash string    `json:"verificationHash"`
	ApprovedBy       string    `json:"approvedBy"`
	IssuedAt         time.Time `json:"issuedAt"`
}

type FrozenManifest struct {
	Dossier    CalibrationDossier `json:"dossier"`
	Sensors    []SensorBaseline   `json:"sensors"`
	Runs       []CalibrationRun   `json:"runs"`
	Deviations []DeviationCase    `json:"deviations"`
	Review     ReviewDecision     `json:"review"`
}

type ReviewDecision struct {
	Reviewer   string    `json:"reviewer"`
	Conclusion string    `json:"conclusion"`
	Note       string    `json:"note"`
	ReviewedAt time.Time `json:"reviewedAt"`
}
