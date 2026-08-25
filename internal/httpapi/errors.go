package httpapi

import (
	"errors"
	"net/http"

	"buoy-calibration-gate/internal/domain"
)

type errorBody struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code               string   `json:"code"`
	Message            string   `json:"message"`
	Field              string   `json:"field,omitempty"`
	MissingTypes       []string `json:"missingTypes,omitempty"`
	ConflictFields     []string `json:"conflictFields,omitempty"`
	ItemIndex          *int     `json:"itemIndex,omitempty"`
	ConflictResourceID string   `json:"conflictResourceId,omitempty"`
}

func writeError(w http.ResponseWriter, err error) {
	status, code, message := http.StatusInternalServerError, "internal_error", "服务内部错误"
	var rule *domain.RuleError
	var validation *domain.ValidationError
	var conflict *domain.ResourceConflictError
	switch {
	case errors.As(err, &rule):
		status, code, message = http.StatusBadRequest, "validation_error", rule.Message
	case errors.As(err, &validation):
		status, code, message = http.StatusBadRequest, "validation_error", validation.Message
	case errors.Is(err, domain.ErrNotFound):
		status, code, message = http.StatusNotFound, "not_found", "资源不存在"
	case errors.Is(err, domain.ErrConflict):
		status, code, message = http.StatusConflict, "version_conflict", "档案版本冲突"
	case errors.Is(err, domain.ErrInvalidState):
		status, code, message = http.StatusConflict, "invalid_state", "当前状态不允许此操作"
	case errors.Is(err, domain.ErrFrozen):
		status, code, message = http.StatusConflict, "dossier_frozen", "冻结档案不可修改"
	case errors.Is(err, domain.ErrAlreadyIssued):
		status, code, message = http.StatusConflict, "permit_already_issued", "部署许可已签发"
	case errors.Is(err, domain.ErrUnauthorized):
		status, code, message = http.StatusForbidden, "forbidden", "当前角色无权执行此操作"
	case errors.Is(err, domain.ErrIdempotencyKey):
		status, code, message = http.StatusConflict, "idempotency_conflict", "幂等键已用于不同请求"
	case errors.As(err, &conflict):
		status, message = http.StatusConflict, conflict.Message
		switch {
		case errors.Is(err, domain.ErrActiveDossier):
			code = "active_dossier_conflict"
		case errors.Is(err, domain.ErrEvidenceConflict):
			code = "evidence_conflict"
		case errors.Is(err, domain.ErrSensorConflict):
			code = "sensor_identity_conflict"
		case errors.Is(err, domain.ErrPreviewConflict):
			code = "preview_conflict"
		default:
			code = "resource_conflict"
		}
	}
	body := errorBody{Error: apiError{Code: code, Message: message}}
	if rule != nil {
		body.Error.Field = rule.Field
	}
	if validation != nil {
		body.Error.Field = validation.Field
		body.Error.MissingTypes = validation.MissingTypes
		body.Error.ConflictFields = validation.ConflictFields
		body.Error.ItemIndex = validation.ItemIndex
	}
	if conflict != nil {
		body.Error.Field = conflict.Field
		body.Error.ConflictResourceID = conflict.ResourceID
	}
	writeJSON(w, status, body)
}

func writeBadRequest(w http.ResponseWriter, message string) {
	writeJSON(w, http.StatusBadRequest, errorBody{Error: apiError{Code: "bad_request", Message: message}})
}
