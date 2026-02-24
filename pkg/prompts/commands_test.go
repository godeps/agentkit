package prompts

import "testing"

func TestBuildCommandMetadataMap(t *testing.T) {
	meta := commandMetadata{
		AllowedTools:           "Bash",
		ArgumentHint:           "hint",
		Model:                  "model",
		DisableModelInvocation: true,
	}
	out := buildCommandMetadataMap(meta, "/path/to/cmd.md")
	if out["allowed-tools"] != "Bash" || out["argument-hint"] != "hint" || out["model"] != "model" || out["disable-model-invocation"] != true || out["source"] != "/path/to/cmd.md" {
		t.Fatalf("unexpected metadata map %v", out)
	}
	if buildCommandMetadataMap(commandMetadata{}, "") != nil {
		t.Fatalf("expected nil map for empty metadata")
	}
}
