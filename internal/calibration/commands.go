package calibration

import "time"

type CreateDossierCommand struct {
	BuoyCode            string    `json:"buoyCode"`
	TargetArea          string    `json:"targetArea"`
	PlannedDeploymentAt time.Time `json:"plannedDeploymentAt"`
	Owner               string    `json:"owner"`
	IdempotencyKey      string    `json:"-"`
	Actor               string    `json:"-"`
}

type AddSensorCommand struct {
	DossierID             string  `json:"-"`
	SensorType            string  `json:"sensorType"`
	SerialNumber          string  `json:"serialNumber"`
	Unit                  string  `json:"unit"`
	RangeMin              float64 `json:"rangeMin"`
	RangeMax              float64 `json:"rangeMax"`
	Tolerance             float64 `json:"tolerance"`
	ConfigurationRevision string  `json:"configurationRevision"`
	Complete              bool    `json:"complete"`
	ExpectedVersion       int64   `json:"-"`
	Actor                 string  `json:"-"`
}

type SubmitRunCommand struct {
	DossierID          string  `json:"-"`
	SensorID           string  `json:"sensorId"`
	ReferenceValue     float64 `json:"referenceValue"`
	MeasuredValue      float64 `json:"measuredValue"`
	AmbientTemperature float64 `json:"ambientTemperature"`
	EvidenceDigest     string  `json:"evidenceDigest"`
	IdempotencyKey     string  `json:"-"`
	ExpectedVersion    int64   `json:"-"`
	Actor              string  `json:"-"`
}

type RemediateCommand struct {
	DossierID          string  `json:"-"`
	DeviationID        string  `json:"-"`
	Cause              string  `json:"cause"`
	Adjustment         string  `json:"adjustment"`
	ReferenceValue     float64 `json:"referenceValue"`
	MeasuredValue      float64 `json:"measuredValue"`
	AmbientTemperature float64 `json:"ambientTemperature"`
	EvidenceDigest     string  `json:"evidenceDigest"`
	ExpectedVersion    int64   `json:"-"`
	Actor              string  `json:"-"`
}

type ReviewCommand struct {
	DossierID       string `json:"-"`
	DeviationID     string `json:"deviationId,omitempty"`
	Note            string `json:"note"`
	ExpectedVersion int64  `json:"-"`
	Actor           string `json:"-"`
	PreviewDigest   string `json:"previewDigest,omitempty"`
}

type BatchRemediationCommand struct {
	DossierID       string                 `json:"-"`
	Items           []BatchRemediationItem `json:"items"`
	ExpectedVersion int64                  `json:"-"`
	Actor           string                 `json:"-"`
}

type BatchRemediationItem struct {
	DeviationID        string  `json:"deviationId"`
	Cause              string  `json:"cause"`
	Adjustment         string  `json:"adjustment"`
	ReferenceValue     float64 `json:"referenceValue"`
	MeasuredValue      float64 `json:"measuredValue"`
	AmbientTemperature float64 `json:"ambientTemperature"`
	EvidenceDigest     string  `json:"evidenceDigest"`
}

type IssuePermitCommand struct {
	DossierID       string `json:"-"`
	ExpectedVersion int64  `json:"-"`
	Actor           string `json:"-"`
}
