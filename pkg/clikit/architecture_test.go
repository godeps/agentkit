package clikit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPackageDoesNotImportDownstreamModule(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		content := string(b)
		if strings.Contains(content, "internal/agent") || strings.Contains(content, "github.com/cinience/alicloud-skills") {
			t.Fatalf("%s should not import downstream module code", f)
		}
	}
}
