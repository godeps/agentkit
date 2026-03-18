package subagents

import (
	"sync"
)

type Store interface {
	Create(inst Instance) error
	Get(id string) (Instance, bool)
	Update(id string, fn func(*Instance) error) error
	ListBySession(sessionID string) []Instance
}

type MemoryStore struct {
	mu    sync.RWMutex
	items map[string]Instance
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{items: map[string]Instance{}}
}

func (s *MemoryStore) Create(inst Instance) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[inst.ID]; ok {
		return ErrInstanceExists
	}
	s.items[inst.ID] = inst.clone()
	return nil
}

func (s *MemoryStore) Get(id string) (Instance, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	inst, ok := s.items[id]
	return inst.clone(), ok
}

func (s *MemoryStore) Update(id string, fn func(*Instance) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	inst, ok := s.items[id]
	if !ok {
		return ErrUnknownInstance
	}
	inst = inst.clone()
	if err := fn(&inst); err != nil {
		return err
	}
	s.items[id] = inst.clone()
	return nil
}

func (s *MemoryStore) ListBySession(sessionID string) []Instance {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Instance
	for _, inst := range s.items {
		if inst.SessionID == sessionID || inst.ParentSessionID == sessionID {
			out = append(out, inst.clone())
		}
	}
	return out
}
