package httpapi

import (
	"net/http"
	"strconv"
	"strings"

	"buoy-calibration-gate/internal/domain"
)

func (a *API) actorAndRole(allowed ...domain.Role) (string, error) {
	a.identityMu.RLock()
	identity := a.identity
	a.identityMu.RUnlock()
	actor := identity.actor
	if actor == "" {
		return "", &domain.RuleError{Field: "X-Actor", Message: "请求头不能为空"}
	}
	if err := domain.RequireRole(identity.role, allowed...); err != nil {
		return "", err
	}
	return actor, nil
}

func expectedVersion(r *http.Request) (int64, error) {
	raw := strings.TrimSpace(r.Header.Get("X-Expected-Version"))
	if raw == "" {
		return 0, &domain.RuleError{Field: "X-Expected-Version", Message: "请求头不能为空"}
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return 0, &domain.RuleError{Field: "X-Expected-Version", Message: "必须为正整数"}
	}
	return value, nil
}
