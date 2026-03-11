package clikit

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	ansiReset  = "\033[0m"
	ansiBold   = "\033[1m"
	ansiDim    = "\033[2m"
	ansiGreen  = "\033[32m"
	ansiRed    = "\033[31m"
	ansiBlue   = "\033[34m"
	ansiCyan   = "\033[36m"
	ansiYellow = "\033[33m"

	WaterfallModeOff     = "off"
	WaterfallModeSummary = "summary"
	WaterfallModeFull    = "full"
)

func printBlockHeader(out io.Writer, title string) {
	if out == nil {
		return
	}
	useANSI := supportsANSI(out)
	trimmed := strings.TrimSpace(title)
	if trimmed == "LLM RESPONSE" {
		fmt.Fprintf(out, "\n%s\n", colorize("[LLM]", ansiCyan, useANSI))
		return
	}
	header := trimmed
	if useANSI {
		header = colorize(trimmed, blockHeaderColor(trimmed), true)
	}
	fmt.Fprintf(out, "\n=== %s ===\n", header)
}

func printBlockFooter(out io.Writer) {
	if out == nil {
		return
	}
	fmt.Fprintln(out)
}

func summarizeOutput(v any) string {
	if v == nil {
		return ""
	}
	switch typed := v.(type) {
	case string:
		return strings.TrimSpace(typed)
	case []byte:
		return strings.TrimSpace(string(typed))
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return strings.TrimSpace(string(raw))
}

func truncateSummary(s string, max int) string {
	s = normalizeSummaryText(s)
	if max <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	if max <= 3 {
		return string(runes[:max])
	}
	return string(runes[:max-3]) + "..."
}

func truncateSummaryHeadTail(s string, head, tail int) string {
	s = normalizeSummaryText(s)
	runes := []rune(s)
	if head <= 0 || tail <= 0 || len(runes) <= head+tail+5 {
		return s
	}
	return string(runes[:head]) + " ... " + string(runes[len(runes)-tail:])
}

func normalizeSummaryText(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	return strings.TrimSpace(s)
}

func runeCount(s string) int {
	return len([]rune(s))
}

func renderDurationBar(dur, max int64, width int) string {
	if width <= 0 {
		return ""
	}
	filled := 1
	if max > 0 {
		filled = int(float64(width) * float64(dur) / float64(max))
		if filled < 1 {
			filled = 1
		}
		if filled > width {
			filled = width
		}
	}
	return strings.Repeat("#", filled) + strings.Repeat(".", width-filled)
}

func formatDurationMs(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	seconds := float64(ms) / 1000.0
	if seconds < 10 {
		return fmt.Sprintf("%.2fs", seconds)
	}
	return fmt.Sprintf("%.1fs", seconds)
}

func decodeInputJSONChunk(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var chunk string
	if err := json.Unmarshal(raw, &chunk); err == nil {
		return chunk
	}
	return string(raw)
}

func summarizeToolInput(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return truncateSummary(raw, 120)
	}
	keys := []string{"description", "command", "file_path", "path", "url", "query", "glob_pattern", "pattern", "text", "prompt"}
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		v, ok := m[k]
		if !ok {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%q", k, truncateSummary(summarizeOutput(v), 48)))
		if len(parts) >= 3 {
			break
		}
	}
	if len(parts) == 0 {
		return truncateSummary(raw, 120)
	}
	return strings.Join(parts, " ")
}

func supportsANSI(out io.Writer) bool {
	if strings.TrimSpace(os.Getenv("NO_COLOR")) != "" {
		return false
	}
	if v := strings.TrimSpace(os.Getenv("CLICOLOR_FORCE")); v == "1" {
		return true
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("TERM")), "dumb") {
		return false
	}
	f, ok := out.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

func colorize(s, ansi string, enabled bool) string {
	if !enabled || s == "" || ansi == "" {
		return s
	}
	return ansi + s + ansiReset
}

func buildLLMToolHint(llmText, toolName, inputSummary string) string {
	if !isThinLLMText(llmText) {
		return ""
	}
	toolName = strings.TrimSpace(toolName)
	if toolName == "" {
		return ""
	}
	inputSummary = strings.TrimSpace(inputSummary)
	if inputSummary == "" {
		return fmt.Sprintf("note: switching to tool call: %s", toolName)
	}
	return fmt.Sprintf("note: switching to tool call: %s (%s)", toolName, truncateSummaryHeadTail(inputSummary, 72, 40))
}

func resolveToolResultName(evtName, fallback string) string {
	name := strings.TrimSpace(evtName)
	if name != "" {
		return name
	}
	return strings.TrimSpace(fallback)
}

func isThinLLMText(s string) bool {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return true
	}
	letters := 0
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z':
			letters++
		case r >= 'A' && r <= 'Z':
			letters++
		case r >= '0' && r <= '9':
			letters++
		case r >= 0x4e00 && r <= 0x9fff:
			letters++
		}
	}
	return letters <= 1
}

func blockHeaderColor(title string) string {
	switch strings.TrimSpace(title) {
	case "RESULT":
		return ansiGreen
	case "WATERFALL":
		return ansiYellow
	case "ERROR":
		return ansiRed
	case "TOOL START", "TOOL END":
		return ansiBlue
	default:
		return ansiCyan
	}
}

func NormalizeWaterfallMode(v string) string {
	normalized := strings.ToLower(strings.TrimSpace(v))
	switch normalized {
	case "", "true", "on", "1", WaterfallModeSummary:
		return WaterfallModeSummary
	case "false", "off", "0", "none":
		return WaterfallModeOff
	case WaterfallModeFull:
		return WaterfallModeFull
	default:
		return WaterfallModeSummary
	}
}

func statusBadge(status string, ansi bool) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "running":
		return colorize("[RUNNING]", ansiBlue, ansi)
	case "error":
		return colorize("[ERROR]", ansiRed, ansi)
	default:
		return colorize("[OK]", ansiGreen, ansi)
	}
}

func printToolProgressLine(out io.Writer, ansi bool, status, name, toolID string, durationMs int64, inputSummary, outputSummary string) {
	if out == nil {
		return
	}
	name = strings.TrimSpace(name)
	toolID = strings.TrimSpace(toolID)
	meta := name
	if toolID != "" {
		meta = fmt.Sprintf("%s id=%s", name, toolID)
	}
	line := fmt.Sprintf("%s %s", statusBadge(status, ansi), meta)
	if durationMs > 0 && status != "running" {
		line += " " + colorize(fmt.Sprintf("cost=%s", formatDurationMs(durationMs)), ansiDim, ansi)
	}
	if in := strings.TrimSpace(inputSummary); in != "" {
		line += " " + colorize("input="+truncateSummaryHeadTail(in, 80, 48), ansiDim, ansi)
	}
	if outSum := strings.TrimSpace(outputSummary); outSum != "" {
		line += " " + colorize("output="+truncateSummaryHeadTail(outSum, 80, 48), ansiDim, ansi)
	}
	fmt.Fprintln(out, line)
}
