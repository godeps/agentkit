package prompts

import "testing"

func TestValidateSubagentMetadata(t *testing.T) {
	if err := validateSubagentMetadata(subagentMetadata{}); err == nil {
		t.Fatalf("expected name required error")
	}
	if err := validateSubagentMetadata(subagentMetadata{Name: "Bad!", Description: "x"}); err == nil {
		t.Fatalf("expected invalid name error")
	}
	if err := validateSubagentMetadata(subagentMetadata{Name: "agent", Description: " "}); err == nil {
		t.Fatalf("expected description error")
	}
	if err := validateSubagentMetadata(subagentMetadata{Name: "agent", Description: "ok", Model: "bad"}); err == nil {
		t.Fatalf("expected invalid model error")
	}
	if err := validateSubagentMetadata(subagentMetadata{Name: "agent", Description: "ok", PermissionMode: "bad"}); err == nil {
		t.Fatalf("expected invalid permission mode error")
	}
	if err := validateSubagentMetadata(subagentMetadata{Name: "agent", Description: "ok", Model: "sonnet"}); err != nil {
		t.Fatalf("unexpected valid metadata error: %v", err)
	}
}

func TestNormalizeSubagentModel(t *testing.T) {
	if got, err := normalizeSubagentModel("inherit"); err != nil || got != "" {
		t.Fatalf("expected inherit to normalize empty, got %q err=%v", got, err)
	}
	if _, err := normalizeSubagentModel("bad"); err == nil {
		t.Fatalf("expected invalid model error")
	}
}

func TestParseSubagentList(t *testing.T) {
	list := parseSubagentList("a, B, a,,")
	if len(list) != 2 || list[0] != "a" || list[1] != "b" {
		t.Fatalf("unexpected list %v", list)
	}
}
