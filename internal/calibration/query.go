package calibration

import (
	"context"

	"buoy-calibration-gate/internal/audit"
	"buoy-calibration-gate/internal/repository"
)

func (s *Service) GetDossier(ctx context.Context, id string) (repository.DossierSnapshot, error) {
	var result repository.DossierSnapshot
	err := s.store.View(ctx, func(tx *repository.Tx) error {
		var err error
		result, err = tx.Snapshot(id)
		return err
	})
	return result, err
}

func (s *Service) Timeline(ctx context.Context, id string) (audit.Timeline, error) {
	return audit.LoadTimeline(ctx, s.store, id)
}
