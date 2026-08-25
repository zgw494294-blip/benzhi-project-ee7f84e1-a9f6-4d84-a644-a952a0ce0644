package domain

import "sort"

func StableManifest(d CalibrationDossier, sensors []SensorBaseline, runs []CalibrationRun, deviations []DeviationCase, review ReviewDecision) FrozenManifest {
	sort.Slice(sensors, func(i, j int) bool { return sensors[i].ID < sensors[j].ID })
	sort.Slice(runs, func(i, j int) bool {
		if runs[i].SensorID == runs[j].SensorID {
			return runs[i].ID < runs[j].ID
		}
		return runs[i].SensorID < runs[j].SensorID
	})
	sort.Slice(deviations, func(i, j int) bool { return deviations[i].ID < deviations[j].ID })
	return FrozenManifest{Dossier: d, Sensors: sensors, Runs: runs, Deviations: deviations, Review: review}
}
