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
	s.timelineMu.RLock()
	cached, found := s.timelineCache[id]
	s.timelineMu.RUnlock()
	if found {
		return cached, nil
	}
	timeline, err := audit.LoadTimeline(ctx, s.store, id)
	if err != nil {
		return timeline, err
	}
	if len(timeline.Events) > 0 && timeline.Events[len(timeline.Events)-1].EventType == "permit.issued" {
		s.timelineMu.Lock()
		s.timelineCache[id] = timeline
		s.timelineMu.Unlock()
	}
	return timeline, nil
}
