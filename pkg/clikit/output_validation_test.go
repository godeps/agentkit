package clikit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDetectOutputPathsFromText(t *testing.T) {
	text := `done: output/a.png and "./output/b.json", repeat output/a.png and output/c.wav\n`
	paths := detectOutputPathsFromText(text)
	got := strings.Join(paths, ",")
	if !strings.Contains(got, "output/a.png") {
		t.Fatalf("missing output/a.png in %v", paths)
	}
	if !strings.Contains(got, "output/b.json") {
		t.Fatalf("missing output/b.json in %v", paths)
	}
	if !strings.Contains(got, "output/c.wav") {
		t.Fatalf("missing output/c.wav in %v", paths)
	}
}

func TestValidateGeneratedOutputs(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir tmp: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	if err := os.MkdirAll("output/demo", 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Clean("output/demo/ok.json"), []byte(`{"ok":true}`), 0o600); err != nil {
		t.Fatalf("write ok.json: %v", err)
	}
	now := time.Now()
	if err := validateGeneratedOutputs(tmp, []string{"output/demo/ok.json"}, now); err != nil {
		t.Fatalf("valid json should pass: %v", err)
	}

	if err := os.WriteFile(filepath.Clean("output/demo/bad.json"), []byte(`{bad}`), 0o600); err != nil {
		t.Fatalf("write bad.json: %v", err)
	}
	if err := validateGeneratedOutputs(tmp, []string{"output/demo/bad.json"}, now); err == nil {
		t.Fatalf("invalid json should fail")
	}

	if err := validateGeneratedOutputs(tmp, []string{"output/demo/missing.png"}, now); err == nil {
		t.Fatalf("missing file should fail")
	}
}

func TestValidateGeneratedOutputsRejectsStaleFile(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir tmp: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	if err := os.MkdirAll("output/demo", 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	p := filepath.Clean("output/demo/old.json")
	if err := os.WriteFile(p, []byte(`{"ok":true}`), 0o600); err != nil {
		t.Fatalf("write old.json: %v", err)
	}
	startedAt := time.Now().Add(3 * time.Second)
	if err := validateGeneratedOutputs(tmp, []string{p}, startedAt); err == nil {
		t.Fatalf("stale file should fail")
	}
}

func TestValidateGeneratedOutputsIgnoresDirectoryPath(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir tmp: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	if err := os.MkdirAll("output/demo", 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := validateGeneratedOutputs(tmp, []string{"output/demo"}, time.Now()); err != nil {
		t.Fatalf("directory candidate should be ignored, got: %v", err)
	}
}

func TestChooseValidationPathsPrefersFinalLLMResponse(t *testing.T) {
	artifact := &artifactInfo{Path: "output/from-tool.png"}
	paths := chooseValidationPaths("最终文件 output/final.json", artifact)
	if len(paths) != 1 || paths[0] != "output/final.json" {
		t.Fatalf("should prefer final llm path, got=%v", paths)
	}
}

func TestChooseValidationPathsFallbackToArtifact(t *testing.T) {
	artifact := &artifactInfo{Path: "output/from-tool.png"}
	paths := chooseValidationPaths("没有路径", artifact)
	if len(paths) != 1 || paths[0] != "output/from-tool.png" {
		t.Fatalf("should fallback artifact path, got=%v", paths)
	}
}
