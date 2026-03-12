package api

import (
	"context"
	"slices"
	"testing"

	"github.com/godeps/agentkit/pkg/model"
	sandboxenv "github.com/godeps/agentkit/pkg/sandbox/env"
	"github.com/godeps/agentkit/pkg/sandbox/hostenv"
	toolbuiltin "github.com/godeps/agentkit/pkg/tool/builtin"
)

func TestEnabledBuiltinToolKeys(t *testing.T) {
	t.Parallel()

	defaults := EnabledBuiltinToolKeys(Options{})
	for _, want := range []string{"bash", "file_read", "file_write", "image_read"} {
		if !slices.Contains(defaults, want) {
			t.Fatalf("default builtins missing %q in %v", want, defaults)
		}
	}

	filtered := EnabledBuiltinToolKeys(Options{EnabledBuiltinTools: []string{"FILE_WRITE", "bash"}})
	if len(filtered) != 2 || filtered[0] != "bash" || filtered[1] != "file_write" {
		t.Fatalf("filtered builtins=%v, want [bash file_write]", filtered)
	}

	disabled := EnabledBuiltinToolKeys(Options{EnabledBuiltinTools: []string{}})
	if len(disabled) != 0 {
		t.Fatalf("disabled builtins=%v, want empty", disabled)
	}
}

func TestRuntimeAvailableToolsFromRegistry(t *testing.T) {
	t.Parallel()

	root := newClaudeProject(t)
	mdl := &stubModel{responses: []*model.Response{{Message: model.Message{Role: "assistant", Content: "ok"}}}}
	rt, err := New(context.Background(), Options{
		ProjectRoot: root,
		Model:       mdl,
		EnabledBuiltinTools: []string{
			"task_create",
			"task_list",
			"task_get",
			"task_update",
			"bash",
		},
	})
	if err != nil {
		t.Fatalf("runtime: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	defs := rt.AvailableTools()
	if len(defs) == 0 {
		t.Fatalf("expected non-empty available tools")
	}

	seen := map[string]struct{}{}
	for _, def := range defs {
		seen[def.Name] = struct{}{}
	}
	for _, want := range []string{"TaskCreate", "TaskList", "TaskGet", "TaskUpdate", "Bash"} {
		if _, ok := seen[want]; !ok {
			t.Fatalf("missing tool %q in %+v", want, defs)
		}
	}
}

func TestRuntimeAvailableToolsForWhitelist(t *testing.T) {
	t.Parallel()

	root := newClaudeProject(t)
	mdl := &stubModel{responses: []*model.Response{{Message: model.Message{Role: "assistant", Content: "ok"}}}}
	rt, err := New(context.Background(), Options{
		ProjectRoot:         root,
		Model:               mdl,
		EnabledBuiltinTools: []string{"task_create", "task_list", "bash"},
	})
	if err != nil {
		t.Fatalf("runtime: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	defs := rt.AvailableToolsForWhitelist([]string{"TaskCreate"})
	if len(defs) != 1 || defs[0].Name != "TaskCreate" {
		t.Fatalf("unexpected whitelisted defs: %+v", defs)
	}
}

func TestBuiltinFactoriesInjectExecutionEnvironment(t *testing.T) {
	root := t.TempDir()
	execEnv := hostenv.New(root)
	factories := builtinToolFactories(root, false, EntryPointCLI, nil, nil, nil, nil, execEnv)

	for name, wantType := range map[string]any{
		"file_read":  &toolbuiltin.ReadTool{},
		"file_write": &toolbuiltin.WriteTool{},
		"file_edit":  &toolbuiltin.EditTool{},
	} {
		impl := factories[name]()
		switch wantType.(type) {
		case *toolbuiltin.ReadTool:
			read, ok := impl.(*toolbuiltin.ReadTool)
			if !ok || read == nil {
				t.Fatalf("%s: unexpected tool type %T", name, impl)
			}
			read.SetEnvironment(execEnv)
		case *toolbuiltin.WriteTool:
			write, ok := impl.(*toolbuiltin.WriteTool)
			if !ok || write == nil {
				t.Fatalf("%s: unexpected tool type %T", name, impl)
			}
			write.SetEnvironment(execEnv)
		case *toolbuiltin.EditTool:
			edit, ok := impl.(*toolbuiltin.EditTool)
			if !ok || edit == nil {
				t.Fatalf("%s: unexpected tool type %T", name, impl)
			}
			edit.SetEnvironment(execEnv)
		}
	}
}

func TestBuildExecutionEnvironmentSelectsConfiguredType(t *testing.T) {
	root := t.TempDir()
	env := buildExecutionEnvironment(Options{
		ProjectRoot: root,
		Sandbox: SandboxOptions{
			Type:   "gvisor",
			GVisor: &sandboxenv.GVisorOptions{Enabled: true},
		},
	})
	if env == nil {
		t.Fatal("expected non-nil execution environment")
	}
}
