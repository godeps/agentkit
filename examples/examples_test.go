package examples

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExamplesExist(t *testing.T) {
	for _, dir := range []string{
		"14-artifact-pipeline",
		"15-resumable-review",
		"16-timeline",
	} {
		if _, err := os.Stat(filepath.Join(".", dir, "main.go")); err != nil {
			t.Fatalf("expected example %q to exist: %v", dir, err)
		}
		if _, err := os.Stat(filepath.Join(".", dir, "README.md")); err != nil {
			t.Fatalf("expected example readme %q to exist: %v", dir, err)
		}
	}
}
