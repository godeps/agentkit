package checkpoint

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/google/uuid"
)

// FileStore persists checkpoint entries as JSON on disk.
type FileStore struct {
	mu      sync.RWMutex
	path    string
	entries map[string]Entry
}

func NewFileStore(path string) (*FileStore, error) {
	store := &FileStore{
		path:    filepath.Clean(path),
		entries: map[string]Entry{},
	}
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

func (f *FileStore) Save(_ context.Context, entry Entry) (string, error) {
	if f == nil {
		return "", ErrNotFound
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if entry.ID == "" {
		entry.ID = uuid.NewString()
	}
	f.entries[entry.ID] = cloneEntry(entry)
	return entry.ID, f.flushLocked()
}

func (f *FileStore) Load(_ context.Context, id string) (Entry, error) {
	if f == nil {
		return Entry{}, ErrNotFound
	}
	f.mu.RLock()
	defer f.mu.RUnlock()
	entry, ok := f.entries[id]
	if !ok {
		return Entry{}, ErrNotFound
	}
	return cloneEntry(entry), nil
}

func (f *FileStore) Delete(_ context.Context, id string) error {
	if f == nil {
		return ErrNotFound
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.entries[id]; !ok {
		return ErrNotFound
	}
	delete(f.entries, id)
	return f.flushLocked()
}

func (f *FileStore) load() error {
	if err := os.MkdirAll(filepath.Dir(f.path), 0o755); err != nil {
		return err
	}
	data, err := os.ReadFile(f.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, &f.entries)
}

func (f *FileStore) flushLocked() error {
	data, err := json.MarshalIndent(f.entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(f.path, data, 0o600)
}
