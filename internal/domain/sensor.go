package domain

import (
	"strings"
	"time"
)

const (
	SensorTemperature     = "temperature"
	SensorSalinity        = "salinity"
	SensorDissolvedOxygen = "dissolved_oxygen"
)

var RequiredSensorTypes = []string{SensorTemperature, SensorSalinity, SensorDissolvedOxygen}

func NormalizeSensorType(value string) string {
	v := strings.ToLower(strings.TrimSpace(value))
	switch v {
	case "temp", "temperature", "温度":
		return SensorTemperature
	case "salinity", "盐度":
		return SensorSalinity
	case "oxygen", "dissolved-oxygen", "dissolved oxygen", "dissolved_oxygen", "溶解氧":
		return SensorDissolvedOxygen
	default:
		return v
	}
}

func NormalizeSensorUnit(sensorType, value string) (string, error) {
	v := strings.ToLower(strings.TrimSpace(value))
	switch NormalizeSensorType(sensorType) {
	case SensorTemperature:
		if v == "°c" || v == "c" || v == "celsius" {
			return "°C", nil
		}
	case SensorSalinity:
		if v == "psu" {
			return "PSU", nil
		}
	case SensorDissolvedOxygen:
		if v == "mg/l" || v == "mg·l-1" || v == "mg·l⁻¹" {
			return "mg/L", nil
		}
	default:
		return "", &RuleError{Field: "sensorType", Message: "仅支持 temperature、salinity、dissolved_oxygen"}
	}
	return "", &RuleError{Field: "unit", Message: "与 sensorType 不匹配"}
}

func NormalizeSerialNumber(value string) string {
	return strings.ToUpper(strings.Join(strings.Fields(value), ""))
}

type SensorBaseline struct {
	ID                    string    `json:"id"`
	DossierID             string    `json:"dossierId"`
	SensorType            string    `json:"sensorType"`
	SerialNumber          string    `json:"serialNumber"`
	Unit                  string    `json:"unit"`
	RangeMin              float64   `json:"rangeMin"`
	RangeMax              float64   `json:"rangeMax"`
	Tolerance             float64   `json:"tolerance"`
	ConfigurationRevision string    `json:"configurationRevision"`
	CreatedAt             time.Time `json:"createdAt"`
}

func (s SensorBaseline) Validate() error {
	if strings.TrimSpace(s.ID) == "" {
		return &RuleError{Field: "id", Message: "不能为空"}
	}
	if strings.TrimSpace(s.DossierID) == "" {
		return &RuleError{Field: "dossierId", Message: "不能为空"}
	}
	if strings.TrimSpace(s.SensorType) == "" {
		return &RuleError{Field: "sensorType", Message: "不能为空"}
	}
	if _, err := NormalizeSensorUnit(s.SensorType, s.Unit); err != nil {
		return err
	}
	if strings.TrimSpace(s.SerialNumber) == "" {
		return &RuleError{Field: "serialNumber", Message: "不能为空"}
	}
	if strings.TrimSpace(s.Unit) == "" {
		return &RuleError{Field: "unit", Message: "不能为空"}
	}
	if s.RangeMax <= s.RangeMin {
		return &RuleError{Field: "rangeMax", Message: "必须大于 rangeMin"}
	}
	if s.Tolerance < 0 {
		return &RuleError{Field: "tolerance", Message: "不得为负数"}
	}
	if strings.TrimSpace(s.ConfigurationRevision) == "" {
		return &RuleError{Field: "configurationRevision", Message: "不能为空"}
	}
	return nil
}

func ValidateBaselineSet(sensors []SensorBaseline) error {
	seenTypes := make(map[string]bool, len(sensors))
	seenSerials := make(map[string]bool, len(sensors))
	var conflicts []string
	for _, sensor := range sensors {
		if err := sensor.Validate(); err != nil {
			return err
		}
		typ := NormalizeSensorType(sensor.SensorType)
		serial := NormalizeSerialNumber(sensor.SerialNumber)
		if seenTypes[typ] {
			conflicts = append(conflicts, "sensorType")
		}
		if seenSerials[serial] {
			conflicts = append(conflicts, "serialNumber")
		}
		seenTypes[typ], seenSerials[serial] = true, true
	}
	var missing []string
	for _, required := range RequiredSensorTypes {
		if !seenTypes[required] {
			missing = append(missing, required)
		}
	}
	if len(missing) > 0 || len(conflicts) > 0 {
		return &ValidationError{Field: "sensors", Message: "传感器基线集合不完整或存在冲突", MissingTypes: missing, ConflictFields: conflicts}
	}
	return nil
}
