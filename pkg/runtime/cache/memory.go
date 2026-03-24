package cache

import (
	"context"
	"sync"

	"github.com/godeps/agentkit/pkg/artifact"
	"github.com/godeps/agentkit/pkg/tool"
)

// MemoryStore keeps cache entries in memory for the lifetime of the process.
type MemoryStore struct {
	mu      sync.RWMutex
	entries map[artifact.CacheKey]*tool.ToolResult
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{entries: map[artifact.CacheKey]*tool.ToolResult{}}
}

func (m *MemoryStore) Load(_ context.Context, key artifact.CacheKey) (*tool.ToolResult, bool, error) {
	if m == nil {
		return nil, false, nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	result, ok := m.entries[key]
	if !ok {
		return nil, false, nil
	}
	return cloneToolResult(result), true, nil
}

func (m *MemoryStore) Save(_ context.Context, key artifact.CacheKey, result *tool.ToolResult) error {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries[key] = cloneToolResult(result)
	return nil
}

func cloneToolResult(result *tool.ToolResult) *tool.ToolResult {
	if result == nil {
		return nil
	}
	cloned := *result
	if len(result.Artifacts) > 0 {
		cloned.Artifacts = append([]artifact.ArtifactRef(nil), result.Artifacts...)
	}
	return &cloned
}
