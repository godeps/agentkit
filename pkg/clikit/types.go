package clikit

import (
	"context"
	"time"

	"github.com/godeps/agentkit/pkg/api"
)

type SkillMeta struct {
	Name string
}

type ModelTurnStat struct {
	Iteration    int
	InputTokens  int
	OutputTokens int
	TotalTokens  int
	StopReason   string
	Preview      string
	Timestamp    time.Time
}

type EffectiveConfig struct {
	ModelName       string
	ConfigRoot      string
	SkillsDirs      []string
	SkillsRecursive *bool
}

type RuntimeInfo interface {
	ModelName() string
	SettingsRoot() string
	SkillsRecursive() bool
	SkillsDirs() []string
}

type StreamEngine interface {
	RunStream(ctx context.Context, sessionID, prompt string) (<-chan api.StreamEvent, error)
	ModelTurnCount(sessionID string) int
	ModelTurnsSince(sessionID string, offset int) []ModelTurnStat
	RepoRoot() string
}

type ReplEngine interface {
	StreamEngine
	ModelName() string
	Skills() []SkillMeta
}
