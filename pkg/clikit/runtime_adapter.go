package clikit

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/godeps/agentkit/pkg/api"
	"github.com/godeps/agentkit/pkg/middleware"
	"github.com/godeps/agentkit/pkg/model"
	runtimeskills "github.com/godeps/agentkit/pkg/runtime/skills"
)

type streamRuntime interface {
	RunStream(context.Context, api.Request) (<-chan api.StreamEvent, error)
}

type RuntimeAdapterConfig struct {
	ProjectRoot     string
	ConfigRoot      string
	ModelName       string
	SkillsDirs      []string
	SkillsRecursive *bool
	TurnRecorder    *TurnRecorder
}

type RuntimeAdapter struct {
	runtime         streamRuntime
	projectRoot     string
	configRoot      string
	modelName       string
	skillsDirs      []string
	skillsRecursive bool
	turnRecorder    *TurnRecorder
}

type turnRecorder struct {
	mu        sync.RWMutex
	bySession map[string][]ModelTurnStat
}

type TurnRecorder = turnRecorder

func NewTurnRecorder() *TurnRecorder {
	return newTurnRecorder()
}

func newTurnRecorder() *turnRecorder {
	return &turnRecorder{bySession: make(map[string][]ModelTurnStat)}
}

func (r *turnRecorder) record(sessionID string, stat ModelTurnStat) {
	if r == nil {
		return
	}
	sessionID = strings.TrimSpace(sessionID)
	r.mu.Lock()
	defer r.mu.Unlock()
	items := append(r.bySession[sessionID], stat)
	if len(items) > 256 {
		items = items[len(items)-256:]
	}
	r.bySession[sessionID] = items
}

func (r *turnRecorder) count(sessionID string) int {
	if r == nil {
		return 0
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.bySession[strings.TrimSpace(sessionID)])
}

func (r *turnRecorder) since(sessionID string, offset int) []ModelTurnStat {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := r.bySession[strings.TrimSpace(sessionID)]
	if offset < 0 {
		offset = 0
	}
	if offset >= len(items) {
		return nil
	}
	out := make([]ModelTurnStat, len(items)-offset)
	copy(out, items[offset:])
	return out
}

func TurnRecorderMiddleware(recorder *TurnRecorder) middleware.Middleware {
	return middleware.Funcs{
		Identifier: "clikit-turn-recorder",
		OnAfterModel: func(_ context.Context, st *middleware.State) error {
			if st == nil || recorder == nil {
				return nil
			}
			values := st.Values
			sessionID, _ := values["session_id"].(string)
			usage, _ := values["model.usage"].(model.Usage)
			stopReason, _ := values["model.stop_reason"].(string)
			recorder.record(sessionID, ModelTurnStat{
				Iteration:    st.Iteration,
				InputTokens:  usage.InputTokens,
				OutputTokens: usage.OutputTokens,
				TotalTokens:  usage.TotalTokens,
				StopReason:   strings.TrimSpace(stopReason),
				Preview:      previewFromState(st),
				Timestamp:    time.Now().UTC(),
			})
			return nil
		},
	}
}

func previewFromState(st *middleware.State) string {
	if st == nil {
		return ""
	}
	if st.Values != nil {
		if resp, ok := st.Values["model.response"].(*model.Response); ok && resp != nil {
			return strings.TrimSpace(resp.Message.TextContent())
		}
	}
	switch typed := st.ModelOutput.(type) {
	case *model.Response:
		if typed != nil {
			return strings.TrimSpace(typed.Message.TextContent())
		}
	case model.Response:
		return strings.TrimSpace(typed.Message.TextContent())
	case interface{ GetContent() string }:
		return strings.TrimSpace(typed.GetContent())
	}
	return truncateSummary(strings.TrimSpace(summarizeOutput(st.ModelOutput)), 120)
}

func NewRuntimeAdapter(rt streamRuntime, cfg RuntimeAdapterConfig) *RuntimeAdapter {
	recorder := cfg.TurnRecorder
	if recorder == nil {
		recorder = newTurnRecorder()
	}
	return &RuntimeAdapter{
		runtime:         rt,
		projectRoot:     strings.TrimSpace(cfg.ProjectRoot),
		configRoot:      strings.TrimSpace(cfg.ConfigRoot),
		modelName:       strings.TrimSpace(cfg.ModelName),
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

func (a *RuntimeAdapter) RunStream(ctx context.Context, sessionID, prompt string) (<-chan api.StreamEvent, error) {
	return a.runtime.RunStream(ctx, api.Request{Prompt: prompt, SessionID: sessionID})
}

func (a *RuntimeAdapter) ModelTurnCount(sessionID string) int {
	if a == nil {
		return 0
	}
	return a.turnRecorder.count(sessionID)
}

func (a *RuntimeAdapter) ModelTurnsSince(sessionID string, offset int) []ModelTurnStat {
	if a == nil {
		return nil
	}
	return a.turnRecorder.since(sessionID, offset)
}

func (a *RuntimeAdapter) Skills() []SkillMeta {
	if a == nil {
		return nil
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
	return out
}
