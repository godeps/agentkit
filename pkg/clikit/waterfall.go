package clikit

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/godeps/agentkit/pkg/api"
)

type waterfallStep struct {
	Kind         string
	Name         string
	ToolUseID    string
	Start        time.Time
	End          time.Time
	DurationMs   int64
	Summary      string
	InputTokens  int
	OutputTokens int
	TotalTokens  int
}

type waterfallTracer struct {
	eng            StreamEngine
	sessionID      string
	runStart       time.Time
	modelTurnIndex int
	llmRound       int
	toolOpen       map[string]*waterfallStep
	toolOrder      []string
	toolInputByID  map[string]string
	toolInputParts map[int]*toolInputPart
	currentLLM     *waterfallStep
	steps          []waterfallStep
}

type toolInputPart struct {
	toolUseID string
	name      string
	inputRaw  strings.Builder
}

func newWaterfallTracer(eng StreamEngine, sessionID string) *waterfallTracer {
	return &waterfallTracer{
		eng:            eng,
		sessionID:      sessionID,
		runStart:       time.Now(),
		modelTurnIndex: eng.ModelTurnCount(sessionID),
		toolOpen:       make(map[string]*waterfallStep),
		toolInputByID:  make(map[string]string),
		toolInputParts: make(map[int]*toolInputPart),
	}
}

func (w *waterfallTracer) OnEvent(evt api.StreamEvent) {
	if w == nil {
		return
	}
	now := time.Now()
	switch evt.Type {
	case api.EventMessageStart:
		if w.currentLLM == nil {
			w.llmRound++
			w.currentLLM = &waterfallStep{
				Kind:  "llm",
				Name:  fmt.Sprintf("llm_round_%d", w.llmRound),
				Start: now,
			}
		}
	case api.EventContentBlockDelta:
		w.appendLLMDelta(evt)
		w.appendToolInputDelta(evt)
	case api.EventContentBlockStart:
		w.startToolInput(evt)
	case api.EventContentBlockStop:
		w.finishToolInput(evt)
	case api.EventMessageStop:
		w.finishLLMStep(now)
	case api.EventToolExecutionStart:
		w.startToolStep(now, evt)
	case api.EventToolExecutionResult:
		w.finishToolStep(now, evt)
	case api.EventError:
		w.finishLLMStep(now)
	}
}

func (w *waterfallTracer) appendLLMDelta(evt api.StreamEvent) {
	if w.currentLLM == nil || evt.Delta == nil || evt.Delta.Type != "text_delta" {
		return
	}
	w.currentLLM.Summary += evt.Delta.Text
	if runeCount(w.currentLLM.Summary) > 200 {
		w.currentLLM.Summary = truncateSummary(w.currentLLM.Summary, 200)
	}
}

func (w *waterfallTracer) startToolStep(now time.Time, evt api.StreamEvent) {
	key := strings.TrimSpace(evt.ToolUseID)
	inputSummary := truncateSummary(strings.TrimSpace(w.toolInputByID[key]), 120)
	step := &waterfallStep{
		Kind:      "tool",
		Name:      strings.TrimSpace(evt.Name),
		ToolUseID: key,
		Start:     now,
		Summary:   inputSummary,
	}
	if key == "" {
		key = fmt.Sprintf("%s#%d", step.Name, len(w.toolOrder)+1)
	}
	w.toolOpen[key] = step
	w.toolOrder = append(w.toolOrder, key)
}

func (w *waterfallTracer) finishToolStep(now time.Time, evt api.StreamEvent) {
	key := strings.TrimSpace(evt.ToolUseID)
	step, ok := w.toolOpen[key]
	if !ok {
		for i := len(w.toolOrder) - 1; i >= 0; i-- {
			candidate := w.toolOrder[i]
			s := w.toolOpen[candidate]
			if s != nil && s.Name == strings.TrimSpace(evt.Name) {
				key = candidate
				step = s
				ok = true
				break
			}
		}
	}
	if !ok || step == nil {
		return
	}
	step.End = now
	step.DurationMs = durationMs(step.Start, step.End)
	outputSummary := truncateSummary(summarizeOutput(evt.Output), 120)
	if step.Summary != "" && outputSummary != "" {
		step.Summary = truncateSummary(step.Summary+" -> "+outputSummary, 120)
	} else if outputSummary != "" {
		step.Summary = outputSummary
	}
	if evt.IsError != nil && *evt.IsError {
		if step.Summary != "" {
			step.Summary = truncateSummary(step.Summary+"; status=error", 120)
		} else {
			step.Summary = "status=error"
		}
	}
	w.steps = append(w.steps, *step)
	delete(w.toolOpen, key)
}

func (w *waterfallTracer) finishLLMStep(now time.Time) {
	if w.currentLLM == nil {
		return
	}
	step := w.currentLLM
	step.End = now
	step.DurationMs = durationMs(step.Start, step.End)
	turns := w.eng.ModelTurnsSince(w.sessionID, w.modelTurnIndex)
	if len(turns) > 0 {
		turn := turns[0]
		w.modelTurnIndex++
		step.InputTokens = turn.InputTokens
		step.OutputTokens = turn.OutputTokens
		step.TotalTokens = turn.TotalTokens
		summary := strings.TrimSpace(turn.Preview)
		if summary == "" {
			summary = strings.TrimSpace(step.Summary)
		}
		if strings.TrimSpace(turn.StopReason) != "" {
			if summary != "" {
				summary += "; "
			}
			summary += "stop=" + strings.TrimSpace(turn.StopReason)
		}
		step.Summary = truncateSummary(summary, 120)
	}
	w.steps = append(w.steps, *step)
	w.currentLLM = nil
}

func (w *waterfallTracer) Print(out io.Writer, mode string) {
	if w == nil || out == nil {
		return
	}
	mode = NormalizeWaterfallMode(mode)
	if mode == WaterfallModeOff {
		return
	}
	if w.currentLLM != nil {
		w.finishLLMStep(time.Now())
	}
	now := time.Now()
	for _, key := range w.toolOrder {
		step := w.toolOpen[key]
		if step == nil {
			continue
		}
		step.End = now
		step.DurationMs = durationMs(step.Start, step.End)
		if step.Summary == "" {
			step.Summary = "unfinished"
		}
		w.steps = append(w.steps, *step)
		delete(w.toolOpen, key)
	}
	if len(w.steps) == 0 {
		printBlockHeader(out, "WATERFALL")
		fmt.Fprintln(out, "no llm/tool steps captured")
		printBlockFooter(out)
		return
	}

	total := durationMs(w.runStart, now)
	var totalIn, totalOut, totalTokens int
	var llmCount, toolCount int
	var maxDuration int64
	for _, step := range w.steps {
		totalIn += step.InputTokens
		totalOut += step.OutputTokens
		totalTokens += step.TotalTokens
		if step.Kind == "llm" {
			llmCount++
		} else if step.Kind == "tool" {
			toolCount++
		}
		if step.DurationMs > maxDuration {
			maxDuration = step.DurationMs
		}
	}

	printBlockHeader(out, "WATERFALL")
	fmt.Fprintf(out, "summary: total_ms=%d steps=%d llm=%d tool=%d llm_tokens=%d/%d/%d session=%s\n",
		total, len(w.steps), llmCount, toolCount, totalIn, totalOut, totalTokens, w.sessionID)
	if mode == WaterfallModeSummary {
		fmt.Fprintln(out, "top_steps:")
		for _, line := range topStepLines(w.steps, total) {
			fmt.Fprintf(out, "  %s\n", line)
		}
		fmt.Fprintf(out, "total: total_ms=%d llm_tokens=%d/%d/%d session=%s\n", total, totalIn, totalOut, totalTokens, w.sessionID)
		printBlockFooter(out)
		return
	}
	fmt.Fprintln(out, "timeline:")
	const maxBarWidth = 24
	useANSI := supportsANSI(out)
	for i, step := range w.steps {
		startMs := durationMs(w.runStart, step.Start)
		share := 0.0
		if total > 0 {
			share = float64(step.DurationMs) * 100 / float64(total)
		}
		bar := renderDurationBar(step.DurationMs, maxDuration, maxBarWidth)
		label := truncateSummary(step.Name, 24)
		detail := truncateSummary(step.Summary, 90)
		barColor := ansiCyan
		if step.Kind == "llm" {
			label = fmt.Sprintf("LLM #%d", i+1)
			detail = fmt.Sprintf("in=%d out=%d total=%d", step.InputTokens, step.OutputTokens, step.TotalTokens)
			if strings.TrimSpace(step.Summary) != "" {
				detail += " | " + truncateSummary(step.Summary, 64)
			}
			barColor = ansiYellow
		} else {
			label = "Tool-" + label
		}
		if useANSI {
			label = colorize(label, barColor, true)
			bar = colorize(bar, barColor, true)
			detail = colorize(detail, ansiDim, true)
		}
		fmt.Fprintf(out, "  %6.1fs | %-18s %s %6s %5.1f%% %s\n",
			float64(startMs)/1000.0,
			label,
			bar,
			formatDurationMs(step.DurationMs),
			share,
			detail,
		)
	}
	fmt.Fprintf(out, "  %6.1fs | done\n", float64(total)/1000.0)
	fmt.Fprintf(out, "total: total_ms=%d llm_tokens=%d/%d/%d session=%s\n", total, totalIn, totalOut, totalTokens, w.sessionID)
	printBlockFooter(out)
}

func topStepLines(steps []waterfallStep, total int64) []string {
	type ranked struct {
		idx  int
		step waterfallStep
	}
	rankedSteps := make([]ranked, 0, len(steps))
	for i, st := range steps {
		rankedSteps = append(rankedSteps, ranked{idx: i, step: st})
	}
	sort.SliceStable(rankedSteps, func(i, j int) bool {
		return rankedSteps[i].step.DurationMs > rankedSteps[j].step.DurationMs
	})
	if len(rankedSteps) > 3 {
		rankedSteps = rankedSteps[:3]
	}
	lines := make([]string, 0, len(rankedSteps))
	for i, r := range rankedSteps {
		label := "Tool-" + strings.TrimSpace(r.step.Name)
		if r.step.Kind == "llm" {
			label = fmt.Sprintf("LLM #%d", r.idx+1)
		}
		share := 0.0
		if total > 0 {
			share = float64(r.step.DurationMs) * 100 / float64(total)
		}
		lines = append(lines, fmt.Sprintf("%d) %s %s (%0.1f%%)", i+1, label, formatDurationMs(r.step.DurationMs), share))
	}
	return lines
}

func durationMs(start, end time.Time) int64 {
	if start.IsZero() || end.IsZero() {
		return 0
	}
	ms := end.Sub(start).Milliseconds()
	if ms < 0 {
		return 0
	}
	return ms
}

func (w *waterfallTracer) startToolInput(evt api.StreamEvent) {
	if w == nil || evt.ContentBlock == nil || evt.Index == nil {
		return
	}
	block := evt.ContentBlock
	if strings.TrimSpace(block.Type) != "tool_use" {
		return
	}
	idx := *evt.Index
	w.toolInputParts[idx] = &toolInputPart{
		toolUseID: strings.TrimSpace(block.ID),
		name:      strings.TrimSpace(block.Name),
	}
}

func (w *waterfallTracer) appendToolInputDelta(evt api.StreamEvent) {
	if w == nil || evt.Index == nil || evt.Delta == nil || evt.Delta.Type != "input_json_delta" {
		return
	}
	part := w.toolInputParts[*evt.Index]
	if part == nil {
		return
	}
	part.inputRaw.WriteString(decodeInputJSONChunk(evt.Delta.PartialJSON))
}

func (w *waterfallTracer) finishToolInput(evt api.StreamEvent) {
	if w == nil || evt.Index == nil {
		return
	}
	idx := *evt.Index
	part := w.toolInputParts[idx]
	if part == nil {
		return
	}
	raw := strings.TrimSpace(part.inputRaw.String())
	if raw != "" && part.toolUseID != "" {
		w.toolInputByID[part.toolUseID] = summarizeToolInput(raw)
	}
	delete(w.toolInputParts, idx)
}
