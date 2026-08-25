package repository

import (
	"database/sql"
	"errors"

	"buoy-calibration-gate/internal/domain"
)

func (t *Tx) PutReview(dossierID string, review domain.ReviewDecision) error {
	_, err := t.tx.Exec(`INSERT INTO review_decisions(dossier_id, reviewer, conclusion, note, reviewed_at) VALUES (?, ?, ?, ?, ?)
ON CONFLICT(dossier_id) DO UPDATE SET reviewer=excluded.reviewer, conclusion=excluded.conclusion, note=excluded.note, reviewed_at=excluded.reviewed_at`, dossierID, review.Reviewer, review.Conclusion, review.Note, formatTime(review.ReviewedAt))
	return err
}

func (t *Tx) GetReview(dossierID string) (domain.ReviewDecision, error) {
	var r domain.ReviewDecision
	var reviewed string
	err := t.tx.QueryRow(`SELECT reviewer, conclusion, note, reviewed_at FROM review_decisions WHERE dossier_id=?`, dossierID).Scan(&r.Reviewer, &r.Conclusion, &r.Note, &reviewed)
	if errors.Is(err, sql.ErrNoRows) {
		return r, domain.ErrNotFound
	}
	if err != nil {
		return r, err
	}
	r.ReviewedAt, err = parseTime(reviewed)
	return r, err
}
