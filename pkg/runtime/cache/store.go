package cache

import (
	"context"

	"github.com/godeps/agentkit/pkg/artifact"
	"github.com/godeps/agentkit/pkg/tool"
)

// Store persists pipeline results keyed by deterministic artifact cache keys.
type Store interface {
	Load(context.Context, artifact.CacheKey) (*tool.ToolResult, bool, error)
	Save(context.Context, artifact.CacheKey, *tool.ToolResult) error
}
