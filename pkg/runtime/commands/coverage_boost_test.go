package commands

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadCommandDirMissingPath(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "missing")
	results, errs := loadCommandDir(dir, resolveFileOps(LoaderOptions{}), resolveWalkDirFunc(LoaderOptions{}))
	if len(results) != 0 {
		t.Fatalf("expected no results for missing dir")
	}
	if len(errs) != 0 {
		t.Fatalf("expected no errors for missing dir: %v", errs)
	}
}

func TestReadFrontMatterMetadataMissingClosing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "broken.md")
	mustWrite(t, path, "---\ntitle: ok")

	_, err := readFrontMatterMetadata(path, resolveFileOps(LoaderOptions{}))
	if err == nil || !strings.Contains(err.Error(), "missing closing frontmatter") {
		t.Fatalf("expected missing closing error, got %v", err)
	}
}

func TestExecutorExecuteEmpty(t *testing.T) {
	exec := NewExecutor()
	res, err := exec.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if res != nil {
		t.Fatalf("expected nil results, got %v", res)
	}
}

func TestExecutorRunParseError(t *testing.T) {
	exec := NewExecutor()
	_, err := exec.Run(context.Background(), "/bad^")
	if err == nil || !strings.Contains(err.Error(), "invalid") {
		t.Fatalf("expected parse error, got %v", err)
	}
}

func TestValidNameRejectsChars(t *testing.T) {
	if validName("UPPER") {
		t.Fatalf("expected uppercase to be invalid")
	}
	if validName("name!") {
		t.Fatalf("expected punctuation to be invalid")
	}
}
