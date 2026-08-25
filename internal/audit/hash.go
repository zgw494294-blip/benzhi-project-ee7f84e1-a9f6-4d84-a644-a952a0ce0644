package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"buoy-calibration-gate/internal/domain"
)

func ManifestDigest(manifest domain.FrozenManifest) (string, []byte, error) {
	data, err := json.Marshal(manifest)
	if err != nil {
		return "", nil, fmt.Errorf("marshal manifest: %w", err)
	}
	canonical := struct {
		Dossier    domain.CalibrationDossier `json:"dossier"`
		Sensors    []domain.SensorBaseline   `json:"sensors"`
		Runs       []domain.CalibrationRun   `json:"runs"`
		Deviations []domain.DeviationCase    `json:"deviations"`
		Reviewer   string                    `json:"reviewer"`
		Conclusion string                    `json:"reviewConclusion"`
	}{manifest.Dossier, manifest.Sensors, manifest.Runs, manifest.Deviations, manifest.Review.Reviewer, manifest.Review.Conclusion}
	canonicalData, err := json.Marshal(canonical)
	if err != nil {
		return "", nil, fmt.Errorf("marshal canonical manifest: %w", err)
	}
	sum := sha256.Sum256(canonicalData)
	return hex.EncodeToString(sum[:]), data, nil
}

type PermitMaterial struct {
	PermitNumber   string `json:"permitNumber"`
	DossierID      string `json:"dossierId"`
	FrozenVersion  int64  `json:"frozenVersion"`
	ManifestDigest string `json:"manifestDigest"`
	ApprovedBy     string `json:"approvedBy"`
	IssuedAt       string `json:"issuedAt"`
}

func PermitVerificationMaterial(p domain.DeploymentPermit) PermitMaterial {
	return PermitMaterial{PermitNumber: p.PermitNumber, DossierID: p.DossierID, FrozenVersion: p.FrozenVersion, ManifestDigest: p.ManifestDigest, ApprovedBy: p.ApprovedBy, IssuedAt: p.IssuedAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00")}
}

func PermitVerificationHash(p domain.DeploymentPermit) string {
	m := PermitVerificationMaterial(p)
	canonical := fmt.Sprintf("%s\n%s\n%d\n%s\n%s\n%s", m.PermitNumber, m.DossierID, m.FrozenVersion, m.ManifestDigest, m.ApprovedBy, m.IssuedAt)
	sum := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(sum[:])
}

func VerifyPermit(p domain.DeploymentPermit) bool {
	return p.VerificationHash != "" && p.VerificationHash == PermitVerificationHash(p)
}
