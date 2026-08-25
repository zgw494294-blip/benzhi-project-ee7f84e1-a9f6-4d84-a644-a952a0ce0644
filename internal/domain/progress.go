package domain

import "sort"

type MissingInitialRun struct {
	SensorID   string `json:"sensorId"`
	SensorType string `json:"sensorType"`
}

type OutOfToleranceItem struct {
	RunID               string   `json:"runId"`
	SensorID            string   `json:"sensorId"`
	SensorType          string   `json:"sensorType"`
	AbsoluteDeviation   float64  `json:"absoluteDeviation"`
	Tolerance           float64  `json:"tolerance"`
	ExcessOverTolerance float64  `json:"excessOverTolerance"`
	ToleranceMultiple   *float64 `json:"toleranceMultiple,omitempty"`
	ToleranceComparison string   `json:"toleranceComparison"`
}

type CalibrationProgress struct {
	BaselineCount      int                  `json:"baselineCount"`
	InitialRunCount    int                  `json:"initialRunCount"`
	PassedCount        int                  `json:"passedCount"`
	FailedCount        int                  `json:"failedCount"`
	OpenDeviationCount int                  `json:"openDeviationCount"`
	MissingInitialRuns []MissingInitialRun  `json:"missingInitialRuns"`
	OutOfTolerance     []OutOfToleranceItem `json:"outOfTolerance"`
}

type WorkflowGuidance struct {
	Allowed []string `json:"allowed"`
	Blocked []string `json:"blocked"`
}

func BuildProgress(d CalibrationDossier, sensors []SensorBaseline, runs []CalibrationRun, deviations []DeviationCase) (CalibrationProgress, WorkflowGuidance) {
	p := CalibrationProgress{BaselineCount: len(sensors), MissingInitialRuns: []MissingInitialRun{}, OutOfTolerance: []OutOfToleranceItem{}}
	sensorByID := make(map[string]SensorBaseline, len(sensors))
	initialBySensor := make(map[string]bool, len(sensors))
	for _, sensor := range sensors {
		sensorByID[sensor.ID] = sensor
	}
	for _, run := range runs {
		if run.RunKind != RunInitial {
			continue
		}
		p.InitialRunCount++
		initialBySensor[run.SensorID] = true
		if run.Passed {
			p.PassedCount++
			continue
		}
		p.FailedCount++
		sensor := sensorByID[run.SensorID]
		excess := run.AbsoluteDeviation - sensor.Tolerance
		item := OutOfToleranceItem{RunID: run.ID, SensorID: run.SensorID, SensorType: sensor.SensorType, AbsoluteDeviation: run.AbsoluteDeviation, Tolerance: sensor.Tolerance, ExcessOverTolerance: excess, ToleranceComparison: "comparable"}
		if sensor.Tolerance == 0 {
			item.ToleranceComparison = "not_comparable_zero_tolerance"
		} else {
			multiple := run.AbsoluteDeviation / sensor.Tolerance
			item.ToleranceMultiple = &multiple
		}
		p.OutOfTolerance = append(p.OutOfTolerance, item)
	}
	for _, sensor := range sensors {
		if !initialBySensor[sensor.ID] {
			p.MissingInitialRuns = append(p.MissingInitialRuns, MissingInitialRun{SensorID: sensor.ID, SensorType: sensor.SensorType})
		}
	}
	for _, deviation := range deviations {
		if deviation.Status != DeviationClosed {
			p.OpenDeviationCount++
		}
	}
	sort.Slice(p.MissingInitialRuns, func(i, j int) bool { return p.MissingInitialRuns[i].SensorID < p.MissingInitialRuns[j].SensorID })
	sort.Slice(p.OutOfTolerance, func(i, j int) bool {
		a, b := p.OutOfTolerance[i], p.OutOfTolerance[j]
		if (a.ToleranceMultiple == nil) != (b.ToleranceMultiple == nil) {
			return a.ToleranceMultiple == nil
		}
		if a.ToleranceMultiple != nil && b.ToleranceMultiple != nil && *a.ToleranceMultiple != *b.ToleranceMultiple {
			return *a.ToleranceMultiple > *b.ToleranceMultiple
		}
		if a.ExcessOverTolerance != b.ExcessOverTolerance {
			return a.ExcessOverTolerance > b.ExcessOverTolerance
		}
		return a.SensorID < b.SensorID
	})
	g := WorkflowGuidance{Allowed: []string{}, Blocked: []string{}}
	switch d.Status {
	case StatusDraft:
		g.Allowed = append(g.Allowed, "register_sensor_baseline")
		g.Blocked = append(g.Blocked, "传感器基线成套完整后才能开始校准")
	case StatusCalibrating:
		if len(p.MissingInitialRuns) > 0 {
			g.Allowed = append(g.Allowed, "submit_missing_initial_runs")
			g.Blocked = append(g.Blocked, "仍有传感器缺少初次校准运行")
		}
	case StatusRemediationRequired:
		g.Allowed = append(g.Allowed, "remediate_open_deviations")
		g.Blocked = append(g.Blocked, "必须先关闭全部偏差项")
	case StatusReviewPending:
		g.Allowed = append(g.Allowed, "review_preflight", "approve_or_return")
	case StatusFrozen:
		g.Allowed = append(g.Allowed, "issue_deployment_permit")
	case StatusPermitted:
		g.Allowed = append(g.Allowed, "verify_deployment_permit")
	}
	return p, g
}
