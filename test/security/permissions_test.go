package security_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/godeps/agentkit/pkg/sandbox"
	"github.com/godeps/agentkit/pkg/tool"
	"github.com/stretchr/testify/require"
)

type integrationTool struct {
	name   string
	called bool
}

func (i *integrationTool) Name() string             { return i.name }
func (i *integrationTool) Description() string      { return "integration" }
func (i *integrationTool) Schema() *tool.JSONSchema { return nil }
func (i *integrationTool) Execute(ctx context.Context, params map[string]any) (*tool.ToolResult, error) {
	i.called = true
	return &tool.ToolResult{Success: true, Output: "ok"}, nil
}

// TestPermissionsIntegration exercises the permission gate through the public
// executor surface to ensure deny rules short-circuit execution.
func TestPermissionsIntegration(t *testing.T) {
	root := canonicalTempDir(t)
	claudeDir := filepath.Join(root, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	settings := map[string]any{
		"permissions": map[string]any{
			"deny":  []string{"Bash(ls:*)"},
			"allow": []string{"Bash(printf:*)"},
		},
	}
	data, err := json.Marshal(settings)
	require.NoError(t, err)
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), data, 0o600); err != nil {
		t.Fatalf("write settings: %v", err)
	}

	fs := sandbox.NewFileSystemAllowList(root)
	mgr := sandbox.NewManager(fs, nil, nil)

	reg := tool.NewRegistry()
	bash := &integrationTool{name: "Bash"}
	if err := reg.Register(bash); err != nil {
		t.Fatalf("register: %v", err)
	}

	exec := tool.NewExecutor(reg, mgr)
	_, err = exec.Execute(context.Background(), tool.Call{Name: "Bash", Params: map[string]any{"command": "ls -la"}, Path: root})
	if err == nil {
		t.Fatalf("expected deny error")
	}
	if bash.called {
		t.Fatalf("tool should not run when denied")
	}

	_, err = exec.Execute(context.Background(), tool.Call{Name: "Bash", Params: map[string]any{"command": "printf hi"}, Path: root})
	if err != nil {
		t.Fatalf("unexpected error for allowed command: %v", err)
	}
	if !bash.called {
		t.Fatalf("tool should run when allowed")
	}
}

func canonicalTempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(dir); err == nil && resolved != "" {
		return resolved
	}
	return dir
}
