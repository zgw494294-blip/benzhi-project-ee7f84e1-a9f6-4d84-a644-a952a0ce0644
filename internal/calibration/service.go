package calibration

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"buoy-calibration-gate/internal/repository"
)

type Service struct {
	store       *repository.Store
	now         func() time.Time
	id          func() string
	runReplayMu sync.Mutex
	runReplays  map[string]runReplay
}

func NewService(store *repository.Store) *Service {
	return &Service{store: store, now: time.Now, id: randomID, runReplays: make(map[string]runReplay)}
}

func randomID() string {
	data := make([]byte, 16)
	if _, err := rand.Read(data); err != nil {
		panic(err)
	}
	return hex.EncodeToString(data)
}

func (s *Service) Ready(ctx context.Context) error { return s.store.Ready(ctx) }
