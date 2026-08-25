package calibration

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"buoy-calibration-gate/internal/repository"
)

type Service struct {
	store *repository.Store
	now   func() time.Time
	id    func() string
}

func NewService(store *repository.Store) *Service {
	return &Service{store: store, now: time.Now, id: randomID}
}

func randomID() string {
	data := make([]byte, 16)
	if _, err := rand.Read(data); err != nil {
		panic(err)
	}
	return hex.EncodeToString(data)
}

func (s *Service) Ready(ctx context.Context) error {
	if err := s.store.Ready(ctx); err != nil {
		return fmt.Errorf("calibration readiness: %v", err)
	}
	return nil
}
