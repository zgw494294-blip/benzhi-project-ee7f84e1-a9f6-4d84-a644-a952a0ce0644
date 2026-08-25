package audit

import (
	"context"

	"buoy-calibration-gate/internal/repository"
)

type Timeline struct {
	DossierID string                  `json:"dossierId"`
	Verified  bool                    `json:"verified"`
	HeadHash  string                  `json:"headHash"`
	Events    []repository.AuditEvent `json:"events"`
}

func LoadTimeline(ctx context.Context, store *repository.Store, dossierID string) (Timeline, error) {
	result := Timeline{DossierID: dossierID}
	err := store.View(ctx, func(tx *repository.Tx) error {
		if _, err := tx.GetDossier(dossierID); err != nil {
			return err
		}
		events, err := tx.ListAuditEvents(dossierID)
		if err != nil {
			return err
		}
		if err := VerifyEventChain(events); err != nil {
			return err
		}
		result.Events, result.Verified = events, true
		if len(events) > 0 {
			result.HeadHash = events[len(events)-1].EventHash
		}
		return nil
	})
	return result, err
}
