package api

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/godeps/agentkit/pkg/middleware"
	"github.com/godeps/agentkit/pkg/model"
)

type AvailableSkill struct {
	Name        string
	Description string
}

type ModelTurnStat struct {
	Iteration    int       `json:"iteration"`
	InputTokens  int       `json:"input_tokens"`
	OutputTokens int       `json:"output_tokens"`
	TotalTokens  int       `json:"total_tokens"`
	StopReason   string    `json:"stop_reason"`
	Preview      string    `json:"preview"`
	Timestamp    time.Time `json:"timestamp"`
}

type ModelTurnRecorder struct {
	mu        sync.RWMutex
	bySession map[string][]ModelTurnStat
}

func NewModelTurnRecorder() *ModelTurnRecorder {
	return &ModelTurnRecorder{bySession: make(map[string][]ModelTurnStat)}
}

func (r *ModelTurnRecorder) Record(sessionID string, stat ModelTurnStat) {
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

func (r *ModelTurnRecorder) Count(sessionID string) int {
	if r == nil {
		return 0
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.bySession[strings.TrimSpace(sessionID)])
}

func (r *ModelTurnRecorder) Since(sessionID string, offset int) []ModelTurnStat {
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

func ModelTurnRecorderMiddleware(recorder *ModelTurnRecorder) middleware.Middleware {
	return middleware.Funcs{
		Identifier: "api-model-turn-recorder",
		OnAfterModel: func(_ context.Context, st *middleware.State) error {
			if st == nil || recorder == nil {
				return nil
			}
			values := st.Values
			sessionID, _ := values["session_id"].(string)
			usage, _ := values["model.usage"].(model.Usage)
			stopReason, _ := values["model.stop_reason"].(string)
			recorder.Record(sessionID, ModelTurnStat{
				Iteration:    st.Iteration,
				InputTokens:  usage.InputTokens,
				OutputTokens: usage.OutputTokens,
				TotalTokens:  usage.TotalTokens,
				StopReason:   strings.TrimSpace(stopReason),
				Preview:      modelTurnPreview(st),
				Timestamp:    time.Now().UTC(),
			})
			return nil
		},
	}
}

func modelTurnPreview(st *middleware.State) string {
	if st == nil {
		return ""
	}
	if st.Values != nil {
		if resp, ok := st.Values["model.response"].(*model.Response); ok && resp != nil {
			return strings.TrimSpace(resp.Message.TextContent())
		}
	}
	return strings.TrimSpace(modelOutputPreview(st.ModelOutput))
}

func modelOutputPreview(v any) string {
	switch typed := v.(type) {
	case nil:
		return ""
	case *model.Response:
		if typed == nil {
			return ""
		}
		return strings.TrimSpace(typed.Message.TextContent())
	case model.Response:
		return strings.TrimSpace(typed.Message.TextContent())
	case interface{ TextContent() string }:
		return strings.TrimSpace(typed.TextContent())
	case string:
		return strings.TrimSpace(typed)
	case []byte:
		return strings.TrimSpace(string(typed))
	default:
		return strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(toString(v)), "\n", " "), "\r", " "))
	}
}

func toString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(interface{ String() string }); ok {
		return s.String()
	}
	return ""
}

func (rt *Runtime) AvailableSkills() []AvailableSkill {
	if rt == nil || rt.skReg == nil {
		return nil
	}
	defs := rt.skReg.List()
	if len(defs) == 0 {
		return nil
	}
	out := make([]AvailableSkill, 0, len(defs))
	for _, def := range defs {
		name := strings.TrimSpace(def.Name)
		if name == "" {
			continue
		}
		out = append(out, AvailableSkill{
			Name:        name,
			Description: strings.TrimSpace(def.Description),
		})
	}
	return out
}
