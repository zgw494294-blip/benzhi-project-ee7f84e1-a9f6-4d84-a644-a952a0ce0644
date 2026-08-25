package calibration

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"buoy-calibration-gate/internal/domain"
	"buoy-calibration-gate/internal/repository"
)

type Service struct {
	store          *repository.Store
	now            func() time.Time
	id             func() string
	preflightMu    sync.RWMutex
	preflightCache map[string]preflightCacheEntry
}

type preflightCacheEntry struct {
	version int64
	result  domain.ReviewPreflight
}

func NewService(store *repository.Store) *Service {
	return &Service{
		store:          store,
		now:            time.Now,
		id:             randomID,
		preflightCache: make(map[string]preflightCacheEntry),
	}
}

func (s *Service) cachedPreflight(dossierID string, version int64) (domain.ReviewPreflight, bool) {
	s.preflightMu.RLock()
	defer s.preflightMu.RUnlock()
	entry, ok := s.preflightCache[dossierID]
	return entry.result, ok && entry.version == version
}

func (s *Service) cachePreflight(dossierID string, version int64, result domain.ReviewPreflight) {
	s.preflightMu.Lock()
	defer s.preflightMu.Unlock()
	s.preflightCache[dossierID] = preflightCacheEntry{version: version, result: result}
}

func randomID() string {
	data := make([]byte, 16)
	if _, err := rand.Read(data); err != nil {
		panic(err)
	}
	return hex.EncodeToString(data)
}

func (s *Service) Ready(ctx context.Context) error { return s.store.Ready(ctx) }
