package domain

import (
	"encoding/hex"
	"math"
	"strings"
	"time"
)

func NormalizeEvidenceDigest(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	parts := strings.SplitN(value, ":", 2)
	if len(parts) != 2 || parts[0] != "sha256" || len(parts[1]) != 64 {
		return "", &RuleError{Field: "evidenceDigest", Message: "必须为 sha256:<64 位十六进制摘要>"}
	}
	if _, err := hex.DecodeString(parts[1]); err != nil {
		return "", &RuleError{Field: "evidenceDigest", Message: "必须为 sha256:<64 位十六进制摘要>"}
	}
	return value, nil
}

type RunKind string

const (
	RunInitial RunKind = "initial"
	RunRetest  RunKind = "retest"
)

type CalibrationRun struct {
	ID                 string    `json:"id"`
	DossierID          string    `json:"dossierId"`
	SensorID           string    `json:"sensorId"`
	RunKind            RunKind   `json:"runKind"`
	ReferenceValue     float64   `json:"referenceValue"`
	MeasuredValue      float64   `json:"measuredValue"`
	AbsoluteDeviation  float64   `json:"absoluteDeviation"`
	AmbientTemperature float64   `json:"ambientTemperature"`
	EvidenceDigest     string    `json:"evidenceDigest"`
	RecordedBy         string    `json:"recordedBy"`
	RecordedAt         time.Time `json:"recordedAt"`
	Passed             bool      `json:"passed"`
}

func NewRun(id string, sensor SensorBaseline, kind RunKind, reference, measured, ambient float64, evidence, actor string, now time.Time) (CalibrationRun, error) {
	digest, err := NormalizeEvidenceDigest(evidence)
	if err != nil {
		return CalibrationRun{}, err
	}
	r := CalibrationRun{ID: id, DossierID: sensor.DossierID, SensorID: sensor.ID, RunKind: kind, ReferenceValue: reference, MeasuredValue: measured, AbsoluteDeviation: math.Abs(measured - reference), AmbientTemperature: ambient, EvidenceDigest: digest, RecordedBy: strings.TrimSpace(actor), RecordedAt: now.UTC()}
	r.Passed = r.AbsoluteDeviation <= sensor.Tolerance
	if err := r.Validate(sensor); err != nil {
		return CalibrationRun{}, err
	}
	return r, nil
}

func (r CalibrationRun) Validate(sensor SensorBaseline) error {
	if r.ReferenceValue < sensor.RangeMin || r.ReferenceValue > sensor.RangeMax {
		return &RuleError{Field: "referenceValue", Message: "超出传感器量程"}
	}
	if r.MeasuredValue < sensor.RangeMin || r.MeasuredValue > sensor.RangeMax {
		return &RuleError{Field: "measuredValue", Message: "超出传感器量程"}
	}
	if math.IsNaN(r.ReferenceValue) || math.IsInf(r.ReferenceValue, 0) || math.IsNaN(r.MeasuredValue) || math.IsInf(r.MeasuredValue, 0) {
		return &RuleError{Field: "value", Message: "必须为有限数值"}
	}
	if strings.TrimSpace(r.EvidenceDigest) == "" {
		return &RuleError{Field: "evidenceDigest", Message: "不能为空"}
	}
	if strings.TrimSpace(r.RecordedBy) == "" {
		return &RuleError{Field: "recordedBy", Message: "不能为空"}
	}
	if r.RunKind != RunInitial && r.RunKind != RunRetest {
		return &RuleError{Field: "runKind", Message: "无效"}
	}
	return nil
}
