package calibration

import (
	"context"
	"encoding/json"

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
		return cloneTimeline(cached), nil
	}
	timeline, err := audit.LoadTimeline(ctx, s.store, id)
	if err != nil {
		return timeline, err
	}
	if len(timeline.Events) > 0 && timeline.Events[len(timeline.Events)-1].EventType == "permit.issued" {
		cloned := cloneTimeline(timeline)
		s.timelineMu.Lock()
		s.timelineCache[id] = cloned
		s.timelineMu.Unlock()
	}
	return timeline, nil
}

// cloneTimeline returns a deep copy of a timeline so that callers cannot
// mutate the cached timeline (or its event payloads) through the returned
// value, and vice versa. Slice headers share underlying arrays by default,
// which previously let caller-side payload edits leak into the cache and
// poison subsequent queries even though SQLite was untouched.
func cloneTimeline(t audit.Timeline) audit.Timeline {
	out := t
	if t.Events == nil {
		return out
	}
	events := make([]repository.AuditEvent, len(t.Events))
	for i := range t.Events {
		events[i] = t.Events[i]
		if payload := t.Events[i].Payload; payload != nil {
			cp := make(json.RawMessage, len(payload))
			copy(cp, payload)
			events[i].Payload = cp
		}
	}
	out.Events = events
	return out
}
