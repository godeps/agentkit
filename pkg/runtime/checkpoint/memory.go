package checkpoint

import (
	"context"
	"sync"
	"time"
)

type MemoryStore struct {
	mu   sync.RWMutex
	data map[string]Record
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{data: map[string]Record{}}
}

func (s *MemoryStore) Save(_ context.Context, rec Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = time.Now().UTC()
	}
	s.data[rec.ID] = rec
	return nil
}

func (s *MemoryStore) Load(_ context.Context, id string) (Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, ok := s.data[id]
	if !ok {
		return Record{}, ErrNotFound
	}
	return rec, nil
}

func (s *MemoryStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.data[id]; !ok {
		return ErrNotFound
	}
	delete(s.data, id)
	return nil
}
