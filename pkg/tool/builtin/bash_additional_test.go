package toolbuiltin

import (
	"context"
	"strings"
	"testing"
)

func TestBashToolRejectsUnsafeCommand(t *testing.T) {
	skipIfWindows(t)
	dir := cleanTempDir(t)
	tool := NewBashToolWithRoot(dir)
	if _, err := tool.Execute(context.Background(), map[string]any{"command": "rm -rf /"}); err == nil || !strings.Contains(err.Error(), "security") {
		t.Fatalf("expected sandbox rejection, got %v", err)
	}
}
