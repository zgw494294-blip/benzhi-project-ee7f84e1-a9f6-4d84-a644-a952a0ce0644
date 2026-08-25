package domain

import (
	"fmt"
	"strings"
	"time"
)

const MaxDeploymentLeadTime = 365 * 24 * time.Hour

func NormalizeBuoyCode(value string) string {
	return strings.ToUpper(strings.Join(strings.Fields(value), ""))
}

type CalibrationDossier struct {
	ID                  string        `json:"id"`
	BuoyCode            string        `json:"buoyCode"`
	TargetArea          string        `json:"targetArea"`
	PlannedDeploymentAt time.Time     `json:"plannedDeploymentAt"`
	Owner               string        `json:"owner"`
	Status              DossierStatus `json:"status"`
	Version             int64         `json:"version"`
	CreatedAt           time.Time     `json:"createdAt"`
	UpdatedAt           time.Time     `json:"updatedAt"`
}

func NewDossier(id, buoyCode, targetArea, owner string, planned, now time.Time) (CalibrationDossier, error) {
	d := CalibrationDossier{ID: id, BuoyCode: NormalizeBuoyCode(buoyCode), TargetArea: strings.TrimSpace(targetArea), Owner: strings.TrimSpace(owner), PlannedDeploymentAt: planned.UTC(), Status: StatusDraft, Version: 1, CreatedAt: now.UTC(), UpdatedAt: now.UTC()}
	if err := d.Validate(); err != nil {
		return CalibrationDossier{}, err
	}
	return d, nil
}

func (d CalibrationDossier) Validate() error {
	if strings.TrimSpace(d.ID) == "" {
		return &RuleError{Field: "id", Message: "不能为空"}
	}
	if strings.TrimSpace(d.BuoyCode) == "" {
		return &RuleError{Field: "buoyCode", Message: "不能为空"}
	}
	if strings.TrimSpace(d.TargetArea) == "" {
		return &RuleError{Field: "targetArea", Message: "不能为空"}
	}
	if strings.TrimSpace(d.Owner) == "" {
		return &RuleError{Field: "owner", Message: "不能为空"}
	}
	if d.PlannedDeploymentAt.IsZero() {
		return &RuleError{Field: "plannedDeploymentAt", Message: "不能为空"}
	}
	if !d.PlannedDeploymentAt.After(d.CreatedAt) {
		return &RuleError{Field: "plannedDeploymentAt", Message: "必须晚于档案创建时间"}
	}
	if d.PlannedDeploymentAt.After(d.CreatedAt.Add(MaxDeploymentLeadTime)) {
		return &RuleError{Field: "plannedDeploymentAt", Message: "超出允许的 365 天计划窗口"}
	}
	if !d.Status.Valid() {
		return &RuleError{Field: "status", Message: "无效"}
	}
	return nil
}

func (d *CalibrationDossier) Transition(to DossierStatus, now time.Time) error {
	allowed := map[DossierStatus]map[DossierStatus]bool{
		StatusDraft:               {StatusCalibrating: true},
		StatusCalibrating:         {StatusRemediationRequired: true, StatusReviewPending: true},
		StatusRemediationRequired: {StatusReviewPending: true},
		StatusReviewPending:       {StatusRemediationRequired: true, StatusFrozen: true},
		StatusFrozen:              {StatusPermitted: true},
	}
	if !allowed[d.Status][to] {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidState, d.Status, to)
	}
	d.Status, d.UpdatedAt = to, now.UTC()
	return nil
}

func (d CalibrationDossier) Mutable() error {
	if d.Status == StatusFrozen || d.Status == StatusPermitted {
		return ErrFrozen
	}
	return nil
}
