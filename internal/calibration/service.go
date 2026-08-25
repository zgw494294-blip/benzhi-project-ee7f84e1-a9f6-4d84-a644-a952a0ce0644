package calibration

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"buoy-calibration-gate/internal/repository"
)

type Service struct {
	store           *repository.Store
	now             func() time.Time
	id              func() string
	createGate      chan struct{}
	creationReplays map[string]creationReplay
}

func NewService(store *repository.Store) *Service {
	gate := make(chan struct{}, 1)
	gate <- struct{}{}
	return &Service{
		store:           store,
		now:             time.Now,
		id:              randomID,
		createGate:      gate,
		creationReplays: make(map[string]creationReplay),
	}
}

func randomID() string {
	data := make([]byte, 16)
	if _, err := rand.Read(data); err != nil {
		panic(err)
	}
	return hex.EncodeToString(data)
}

func (s *Service) Ready(ctx context.Context) error { return s.store.Ready(ctx) }
