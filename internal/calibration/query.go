package calibration

import (
	"context"
	"fmt"

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
	if err != nil {
		err = fmt.Errorf("get dossier: %v", err)
	}
	return result, err
}

func (s *Service) Timeline(ctx context.Context, id string) (audit.Timeline, error) {
	result, err := audit.LoadTimeline(ctx, s.store, id)
	if err != nil {
		err = fmt.Errorf("load timeline: %v", err)
	}
	return result, err
}
