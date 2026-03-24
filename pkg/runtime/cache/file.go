package cache

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/godeps/agentkit/pkg/artifact"
	"github.com/godeps/agentkit/pkg/tool"
)

// FileStore persists cache entries as JSON on disk.
type FileStore struct {
	mu      sync.RWMutex
	path    string
	entries map[artifact.CacheKey]*tool.ToolResult
}

func NewFileStore(path string) (*FileStore, error) {
	store := &FileStore{
		path:    filepath.Clean(path),
		entries: map[artifact.CacheKey]*tool.ToolResult{},
	}
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

func (f *FileStore) Load(_ context.Context, key artifact.CacheKey) (*tool.ToolResult, bool, error) {
	if f == nil {
		return nil, false, nil
	}
	f.mu.RLock()
	defer f.mu.RUnlock()
	result, ok := f.entries[key]
	if !ok {
		return nil, false, nil
	}
	return cloneToolResult(result), true, nil
}

func (f *FileStore) Save(_ context.Context, key artifact.CacheKey, result *tool.ToolResult) error {
	if f == nil {
		return nil
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.entries[key] = cloneToolResult(result)
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
