package clikit

import (
	"bytes"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestMultiValueSet(t *testing.T) {
	var m multiValue
	if err := m.Set("/a"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := m.Set("/b"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if got := m.String(); got != "/a,/b" {
		t.Fatalf("unexpected value: %s", got)
	}
}

func TestPrintEffectiveConfig(t *testing.T) {
	var buf bytes.Buffer
	cfg := EffectiveConfig{
		ModelName:       "m",
		ConfigRoot:      "/cfg",
		SkillsDirs:      []string{"/s1", "/s2"},
		SkillsRecursive: boolPtr(true),
	}
	PrintEffectiveConfig(&buf, "/repo", cfg, 1000)
	out := buf.String()
	for _, sub := range []string{"repo_root: /repo", "model: m", "config_root: /cfg", "skills_dirs:"} {
		if !strings.Contains(out, sub) {
			t.Fatalf("missing %q in output: %s", sub, out)
		}
	}
}

func TestPrintBanner(t *testing.T) {
	var buf bytes.Buffer
	PrintBanner(&buf, "model-x", []SkillMeta{{Name: "a"}, {Name: "b"}})
	out := buf.String()
	for _, sub := range []string{"Agentkit CLI", "Model: model-x", "Skills: 2 loaded"} {
		if !strings.Contains(out, sub) {
			t.Fatalf("missing %q in output: %s", sub, out)
		}
	}
}

func TestTruncateSummary(t *testing.T) {
	got := truncateSummary("  a    b   c  ", 5)
	if got != "a ..." {
		t.Fatalf("unexpected truncate result: %q", got)
	}
	if got := truncateSummary("abcdef", 5); got != "ab..." {
		t.Fatalf("unexpected shortened result: %q", got)
	}
	if got := truncateSummary("abc", 0); got != "abc" {
		t.Fatalf("unexpected no-limit result: %q", got)
	}
	if got := truncateSummary("你好，世界，欢迎使用瀑布流", 8); strings.ContainsRune(got, '�') || !utf8.ValidString(got) {
		t.Fatalf("unexpected utf8 corruption: %q", got)
	}
}

func TestWaterfallPrintIncludesLLMTokens(t *testing.T) {
	tracer := &waterfallTracer{
		sessionID: "s-1",
		runStart:  time.Now().Add(-2 * time.Second),
		steps: []waterfallStep{
			{
				Kind:         "llm",
				Name:         "llm_round_1",
				DurationMs:   120,
				InputTokens:  11,
				OutputTokens: 7,
				TotalTokens:  18,
				Summary:      "hello world",
			},
			{
				Kind:       "tool",
				Name:       "file_read",
				DurationMs: 40,
				Summary:    "{\"ok\":true}",
			},
		},
	}
	var buf bytes.Buffer
	tracer.Print(&buf, WaterfallModeFull)
	out := buf.String()
	for _, sub := range []string{
		"\n=== WATERFALL ===\n",
		"summary: total_ms=",
		"timeline:",
		"steps=2 llm=1 tool=1",
		"in=11 out=7 total=18",
		"6.0%",
		"LLM #1",
		"Tool-file_read",
		"total: total_ms=",
	} {
		if !strings.Contains(out, sub) {
			t.Fatalf("missing %q in output: %s", sub, out)
		}
	}
}

func TestWaterfallPrintSummaryModeCondensesTimeline(t *testing.T) {
	tracer := &waterfallTracer{
		sessionID: "s-2",
		runStart:  time.Now().Add(-3 * time.Second),
		steps: []waterfallStep{
			{Kind: "tool", Name: "A", DurationMs: 1800, Summary: "slowest"},
			{Kind: "llm", Name: "llm_round_1", DurationMs: 600, InputTokens: 3, OutputTokens: 2, TotalTokens: 5},
			{Kind: "tool", Name: "B", DurationMs: 400, Summary: "mid"},
			{Kind: "tool", Name: "C", DurationMs: 200, Summary: "fast"},
		},
	}
	var buf bytes.Buffer
	tracer.Print(&buf, WaterfallModeSummary)
	out := buf.String()
	for _, sub := range []string{
		"\n=== WATERFALL ===\n",
		"summary: total_ms=",
		"top_steps:",
		"1) Tool-A",
		"2) LLM #2",
		"3) Tool-B",
	} {
		if !strings.Contains(out, sub) {
			t.Fatalf("missing %q in output: %s", sub, out)
		}
	}
	if strings.Contains(out, "timeline:") {
		t.Fatalf("summary mode should not include full timeline: %s", out)
	}
}

func TestSummarizeToolInput(t *testing.T) {
	raw := `{"description":"list files","command":"ls -la","path":"/tmp/x","extra":"ignored"}`
	got := summarizeToolInput(raw)
	for _, sub := range []string{`description="list files"`, `command="ls -la"`, `path="/tmp/x"`} {
		if !strings.Contains(got, sub) {
			t.Fatalf("missing %q in %q", sub, got)
		}
	}
}

func TestDecodeInputJSONChunk(t *testing.T) {
	got := decodeInputJSONChunk([]byte(`"abc"`))
	if got != "abc" {
		t.Fatalf("unexpected decoded chunk: %q", got)
	}
}

func TestPrintBlockFormatting(t *testing.T) {
	var buf bytes.Buffer
	printBlockHeader(&buf, "TOOL START")
	buf.WriteString("name: bash\n")
	printBlockFooter(&buf)
	out := buf.String()
	for _, sub := range []string{
		"=== TOOL START ===",
		"name: bash",
	} {
		if !strings.Contains(out, sub) {
			t.Fatalf("missing %q in output: %s", sub, out)
		}
	}
}

func TestPrintBlockHeaderLLMResponseCompact(t *testing.T) {
	var buf bytes.Buffer
	printBlockHeader(&buf, "LLM RESPONSE")
	out := buf.String()
	if !strings.Contains(out, "[LLM]") {
		t.Fatalf("missing compact llm header: %s", out)
	}
	if strings.Contains(out, "=== LLM RESPONSE ===") {
		t.Fatalf("legacy llm header should not appear: %s", out)
	}
}

func TestTruncateSummaryHeadTail(t *testing.T) {
	got := truncateSummaryHeadTail("abcdefghijklmnopqrstuvwxyz0123456789", 12, 8)
	want := "abcdefghijkl ... 23456789"
	if got != want {
		t.Fatalf("unexpected head/tail summary: got %q want %q", got, want)
	}
	if got := truncateSummaryHeadTail("short", 12, 8); got != "short" {
		t.Fatalf("short string should stay unchanged: %q", got)
	}
}

func TestDetectArtifactInfoPicksLatestPath(t *testing.T) {
	raw := map[string]any{
		"output": `old output/ai-image-qwen-image/images/cat.png then new output/ai-image-qwen-image/images/futuristic-city-night-poster.png 1024x1024 PNG`,
	}
	info, ok := detectArtifactInfo(raw)
	if !ok {
		t.Fatalf("expected artifact info to be detected")
	}
	if info.Path != "output/ai-image-qwen-image/images/futuristic-city-night-poster.png" {
		t.Fatalf("unexpected path: %s", info.Path)
	}
	if info.Dimensions != "1024 x 1024" {
		t.Fatalf("unexpected dimensions: %s", info.Dimensions)
	}
	if info.Format != "PNG" {
		t.Fatalf("unexpected format: %s", info.Format)
	}
}

func TestIsThinLLMText(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{in: ".", want: true},
		{in: " ... ", want: true},
		{in: "", want: true},
		{in: "正在生成图片", want: false},
		{in: "Generating image now", want: false},
	}
	for _, tc := range cases {
		if got := isThinLLMText(tc.in); got != tc.want {
			t.Fatalf("isThinLLMText(%q)=%v want=%v", tc.in, got, tc.want)
		}
	}
}

func TestBuildLLMToolHint(t *testing.T) {
	hint := buildLLMToolHint(".", "Bash", `description="Generate image" command="python run.py"`)
	if !strings.Contains(hint, "switching to tool call: Bash") {
		t.Fatalf("unexpected hint: %q", hint)
	}
	if !strings.Contains(hint, "description=") {
		t.Fatalf("hint should include input summary: %q", hint)
	}
	if got := buildLLMToolHint("我先说明步骤", "Bash", "x"); got != "" {
		t.Fatalf("non-thin llm text should not emit hint: %q", got)
	}
}

func TestResolveToolResultName(t *testing.T) {
	cases := []struct {
		evtName  string
		fallback string
		want     string
	}{
		{evtName: "tool_a", fallback: "tool_b", want: "tool_a"},
		{evtName: "", fallback: "tool_b", want: "tool_b"},
		{evtName: " ", fallback: " tool_b ", want: "tool_b"},
	}
	for _, tc := range cases {
		if got := resolveToolResultName(tc.evtName, tc.fallback); got != tc.want {
			t.Fatalf("resolveToolResultName(%q,%q)=%q want=%q", tc.evtName, tc.fallback, got, tc.want)
		}
	}
}
