package domain

import (
	"fmt"
	"sort"
)

type PreflightBlocker struct {
	Code       string `json:"code"`
	Field      string `json:"field"`
	ResourceID string `json:"resourceId,omitempty"`
	Message    string `json:"message"`
}

type ReviewPreflight struct {
	DossierVersion int64              `json:"dossierVersion"`
	SensorCount    int                `json:"sensorCount"`
	RunCount       int                `json:"runCount"`
	DeviationCount int                `json:"deviationCount"`
	Blockers       []PreflightBlocker `json:"blockers"`
	PreviewDigest  string             `json:"previewDigest,omitempty"`
}

func ReviewBlockers(sensors []SensorBaseline, runs []CalibrationRun, deviations []DeviationCase) []PreflightBlocker {
	var blockers []PreflightBlocker
	if err := ValidateBaselineSet(sensors); err != nil {
		if detail, ok := err.(*ValidationError); ok {
			for _, typ := range detail.MissingTypes {
				blockers = append(blockers, PreflightBlocker{Code: "missing_required_sensor", Field: "sensors", ResourceID: typ, Message: "缺少必需传感器基线"})
			}
		} else {
			blockers = append(blockers, PreflightBlocker{Code: "invalid_sensor_baseline", Field: "sensors", Message: err.Error()})
		}
	}
	initial := make(map[string]bool, len(sensors))
	runByID := make(map[string]CalibrationRun, len(runs))
	for _, run := range runs {
		runByID[run.ID] = run
		if run.RunKind == RunInitial {
			initial[run.SensorID] = true
		}
		if _, err := NormalizeEvidenceDigest(run.EvidenceDigest); err != nil {
			blockers = append(blockers, PreflightBlocker{Code: "invalid_evidence_digest", Field: "runs.evidenceDigest", ResourceID: run.ID, Message: "运行缺少有效证据摘要"})
		}
	}
	for _, sensor := range sensors {
		if !initial[sensor.ID] {
			blockers = append(blockers, PreflightBlocker{Code: "missing_initial_run", Field: "runs", ResourceID: sensor.ID, Message: "传感器缺少初次校准运行"})
		}
	}
	for _, deviation := range deviations {
		if deviation.Status != DeviationClosed {
			blockers = append(blockers, PreflightBlocker{Code: "deviation_not_closed", Field: "deviations.status", ResourceID: deviation.ID, Message: "偏差项尚未关闭"})
			continue
		}
		retest, ok := runByID[deviation.RetestRunID]
		if deviation.RetestRunID == "" || !ok || retest.RunKind != RunRetest || !retest.Passed || retest.SensorID != deviation.SensorID {
			blockers = append(blockers, PreflightBlocker{Code: "invalid_passing_retest", Field: "deviations.retestRunId", ResourceID: deviation.ID, Message: "关闭偏差缺少有效的通过复测"})
		} else if _, err := NormalizeEvidenceDigest(retest.EvidenceDigest); err != nil {
			blockers = append(blockers, PreflightBlocker{Code: "invalid_retest_evidence", Field: "runs.evidenceDigest", ResourceID: retest.ID, Message: "通过复测缺少有效证据摘要"})
		}
	}
	sort.Slice(blockers, func(i, j int) bool {
		return fmt.Sprintf("%s\x00%s\x00%s", blockers[i].Code, blockers[i].ResourceID, blockers[i].Field) < fmt.Sprintf("%s\x00%s\x00%s", blockers[j].Code, blockers[j].ResourceID, blockers[j].Field)
	})
	return blockers
}
