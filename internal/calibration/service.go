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
	store             *repository.Store
	now               func() time.Time
	id                func() string
	verificationMu    sync.RWMutex
	verificationCache map[string]PermitVerification
}

func NewService(store *repository.Store) *Service {
	return &Service{
		store:             store,
		now:               time.Now,
		id:                randomID,
		verificationCache: make(map[string]PermitVerification),
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
