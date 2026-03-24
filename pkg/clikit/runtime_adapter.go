package clikit

import (
	"context"
	"strings"
	"sync"

	"github.com/godeps/agentkit/pkg/api"
	"github.com/godeps/agentkit/pkg/middleware"
	runtimeskills "github.com/godeps/agentkit/pkg/runtime/skills"
)

type streamRuntime interface {
	RunStream(context.Context, api.Request) (<-chan api.StreamEvent, error)
	Run(context.Context, api.Request) (*api.Response, error)
	Resume(context.Context, string) (*api.Response, error)
}

type RuntimeAdapterConfig struct {
	ProjectRoot     string
	ConfigRoot      string
	ModelName       string
	SandboxBackend  string
	SkillsDirs      []string
	SkillsRecursive *bool
	TurnRecorder    *api.ModelTurnRecorder
}

type RuntimeAdapter struct {
	runtime         streamRuntime
	projectRoot     string
	configRoot      string
	modelName       string
	sandboxBackend  string
	skillsDirs      []string
	skillsRecursive bool
	turnRecorder    *api.ModelTurnRecorder
	skillsOnce      sync.Once
	skillsCached    []SkillMeta
}

type TurnRecorder = api.ModelTurnRecorder

func NewTurnRecorder() *TurnRecorder {
	return api.NewModelTurnRecorder()
}

func newTurnRecorder() *TurnRecorder {
	return api.NewModelTurnRecorder()
}

func TurnRecorderMiddleware(recorder *TurnRecorder) middleware.Middleware {
	return api.ModelTurnRecorderMiddleware(recorder)
}

func NewRuntimeAdapter(rt streamRuntime, cfg RuntimeAdapterConfig) *RuntimeAdapter {
	recorder := cfg.TurnRecorder
	if recorder == nil {
		recorder = api.NewModelTurnRecorder()
	}
	return &RuntimeAdapter{
		runtime:         rt,
		projectRoot:     strings.TrimSpace(cfg.ProjectRoot),
		configRoot:      strings.TrimSpace(cfg.ConfigRoot),
		modelName:       strings.TrimSpace(cfg.ModelName),
		sandboxBackend:  strings.TrimSpace(cfg.SandboxBackend),
		skillsDirs:      append([]string(nil), cfg.SkillsDirs...),
		skillsRecursive: cfg.SkillsRecursive == nil || *cfg.SkillsRecursive,
		turnRecorder:    recorder,
	}
}

func (a *RuntimeAdapter) ModelName() string {
	if a == nil {
		return ""
	}
	return a.modelName
}

func (a *RuntimeAdapter) SettingsRoot() string {
	if a == nil {
		return ""
	}
	return a.configRoot
}

func (a *RuntimeAdapter) SkillsRecursive() bool {
	if a == nil {
		return true
	}
	return a.skillsRecursive
}

func (a *RuntimeAdapter) SkillsDirs() []string {
	if a == nil {
		return nil
	}
	return append([]string(nil), a.skillsDirs...)
}

func (a *RuntimeAdapter) RepoRoot() string {
	if a == nil {
		return ""
	}
	return a.projectRoot
}

func (a *RuntimeAdapter) RunStream(ctx context.Context, req api.Request) (<-chan api.StreamEvent, error) {
	return a.runtime.RunStream(ctx, req)
}

func (a *RuntimeAdapter) Run(ctx context.Context, req api.Request) (*api.Response, error) {
	return a.runtime.Run(ctx, req)
}

func (a *RuntimeAdapter) Resume(ctx context.Context, checkpointID string) (*api.Response, error) {
	return a.runtime.Resume(ctx, checkpointID)
}

func (a *RuntimeAdapter) Timeline(resp *api.Response) []api.TimelineEntry {
	return api.BuildTimeline(resp)
}

func (a *RuntimeAdapter) ModelTurnCount(sessionID string) int {
	if a == nil {
		return 0
	}
	return a.turnRecorder.Count(sessionID)
}

func (a *RuntimeAdapter) ModelTurnsSince(sessionID string, offset int) []ModelTurnStat {
	if a == nil {
		return nil
	}
	stats := a.turnRecorder.Since(sessionID, offset)
	out := make([]ModelTurnStat, 0, len(stats))
	for _, st := range stats {
		out = append(out, ModelTurnStat{
			Iteration:    st.Iteration,
			InputTokens:  st.InputTokens,
			OutputTokens: st.OutputTokens,
			TotalTokens:  st.TotalTokens,
			StopReason:   st.StopReason,
			Preview:      st.Preview,
			Timestamp:    st.Timestamp,
		})
	}
	return out
}

func (a *RuntimeAdapter) Skills() []SkillMeta {
	if a == nil {
		return nil
	}
	a.skillsOnce.Do(func() {
		if withSkills, ok := a.runtime.(interface{ AvailableSkills() []api.AvailableSkill }); ok {
			src := withSkills.AvailableSkills()
			out := make([]SkillMeta, 0, len(src))
			for _, skill := range src {
				name := strings.TrimSpace(skill.Name)
				if name == "" {
					continue
				}
				out = append(out, SkillMeta{Name: name})
			}
			a.skillsCached = out
			return
		}
		var recursive *bool
		if a.skillsRecursive {
			recursive = boolPtr(true)
		} else {
			recursive = boolPtr(false)
		}
		regs, _ := runtimeskills.LoadFromFS(runtimeskills.LoaderOptions{
			ProjectRoot: a.projectRoot,
			ConfigRoot:  a.configRoot,
			Directories: a.skillsDirs,
			Recursive:   recursive,
		})
		out := make([]SkillMeta, 0, len(regs))
		for _, reg := range regs {
			name := strings.TrimSpace(reg.Definition.Name)
			if name == "" {
				continue
			}
			out = append(out, SkillMeta{Name: name})
		}
		a.skillsCached = out
	})
	return append([]SkillMeta(nil), a.skillsCached...)
}

func (a *RuntimeAdapter) SandboxBackend() string {
	if a == nil {
		return ""
	}
	return a.sandboxBackend
}
