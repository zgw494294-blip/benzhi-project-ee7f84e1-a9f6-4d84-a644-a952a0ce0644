package repository

import (
	"database/sql"
	"errors"
	"time"

	"buoy-calibration-gate/internal/domain"
)

type ManifestRecord struct {
	DossierID     string
	FrozenVersion int64
	Digest        string
	JSON          []byte
	CreatedAt     time.Time
}

func (t *Tx) InsertManifest(m ManifestRecord) error {
	_, err := t.tx.Exec(`INSERT INTO frozen_manifests(dossier_id, frozen_version, digest, manifest_json, created_at) VALUES (?, ?, ?, ?, ?)`, m.DossierID, m.FrozenVersion, m.Digest, m.JSON, formatTime(m.CreatedAt))
	return err
}

func (t *Tx) GetManifest(dossierID string) (ManifestRecord, error) {
	var m ManifestRecord
	var created string
	err := t.tx.QueryRow(`SELECT dossier_id, frozen_version, digest, manifest_json, created_at FROM frozen_manifests WHERE dossier_id=?`, dossierID).Scan(&m.DossierID, &m.FrozenVersion, &m.Digest, &m.JSON, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return m, domain.ErrNotFound
	}
	if err != nil {
		return m, err
	}
	m.CreatedAt, err = parseTime(created)
	return m, err
}
