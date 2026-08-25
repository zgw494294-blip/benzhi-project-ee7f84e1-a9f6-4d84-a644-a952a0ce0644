package repository

import (
	"database/sql"
	"errors"

	"buoy-calibration-gate/internal/domain"
)

func (t *Tx) InsertPermit(p domain.DeploymentPermit) error {
	_, err := t.tx.Exec(`INSERT INTO deployment_permits(id, permit_number, dossier_id, frozen_version, manifest_digest, verification_hash, approved_by, issued_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, p.ID, p.PermitNumber, p.DossierID, p.FrozenVersion, p.ManifestDigest, p.VerificationHash, p.ApprovedBy, formatTime(p.IssuedAt))
	return err
}

func scanPermit(row interface{ Scan(...any) error }) (domain.DeploymentPermit, error) {
	var p domain.DeploymentPermit
	var issued string
	err := row.Scan(&p.ID, &p.PermitNumber, &p.DossierID, &p.FrozenVersion, &p.ManifestDigest, &p.VerificationHash, &p.ApprovedBy, &issued)
	if errors.Is(err, sql.ErrNoRows) {
		return p, domain.ErrNotFound
	}
	if err != nil {
		return p, err
	}
	p.IssuedAt, err = parseTime(issued)
	return p, err
}

func (t *Tx) GetPermitByDossier(dossierID string) (domain.DeploymentPermit, error) {
	return scanPermit(t.tx.QueryRow(`SELECT id, permit_number, dossier_id, frozen_version, manifest_digest, verification_hash, approved_by, issued_at FROM deployment_permits WHERE dossier_id=?`, dossierID))
}

func (t *Tx) GetPermitByNumber(number string) (domain.DeploymentPermit, error) {
	return scanPermit(t.tx.QueryRow(`SELECT id, permit_number, dossier_id, frozen_version, manifest_digest, verification_hash, approved_by, issued_at FROM deployment_permits WHERE permit_number=?`, number))
}
