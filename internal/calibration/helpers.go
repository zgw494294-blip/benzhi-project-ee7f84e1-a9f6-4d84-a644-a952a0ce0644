package calibration

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"buoy-calibration-gate/internal/domain"
)

func requestHash(v any) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func requireActor(actor string) error {
	if strings.TrimSpace(actor) == "" {
		return &domain.RuleError{Field: "X-Actor", Message: "不能为空"}
	}
	return nil
}

func requireExpected(actual, expected int64) error {
	if expected <= 0 {
		return &domain.RuleError{Field: "expectedVersion", Message: "必须为正整数"}
	}
	if actual != expected {
		return fmt.Errorf("%w: expected %d, current %d", domain.ErrConflict, expected, actual)
	}
	return nil
}
