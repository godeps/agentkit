package toolbuiltin

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/godeps/agentkit/pkg/runtime/commands"
)

func TestSlashCommandExecutes(t *testing.T) {
	exec := commands.NewExecutor()
	if err := exec.Register(commands.Definition{Name: "review-pr"}, commands.HandlerFunc(func(ctx context.Context, inv commands.Invocation) (commands.Result, error) {
		if len(inv.Args) != 1 || inv.Args[0] != "42" {
			t.Fatalf("unexpected args: %#v", inv.Args)
		}
		return commands.Result{Command: inv.Name, Output: "ok"}, nil
	})); err != nil {
		t.Fatalf("register: %v", err)
	}
	tool := NewSlashCommandTool(exec)
	res, err := tool.Execute(context.Background(), map[string]interface{}{"command": "/review-pr 42"})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(res.Output, "ok") {
		t.Fatalf("unexpected output %q", res.Output)
	}
}

func TestSlashCommandRejectsInvalidInput(t *testing.T) {
	tool := NewSlashCommandTool(commands.NewExecutor())
	if _, err := tool.Execute(context.Background(), map[string]interface{}{"command": "review"}); err == nil {
		t.Fatalf("expected error for missing /")
	}
	if _, err := tool.Execute(context.Background(), map[string]interface{}{}); err == nil {
		t.Fatalf("expected error for missing command")
	}
	if _, err := tool.Execute(nil, map[string]interface{}{"command": "/x"}); err == nil {
		t.Fatalf("expected context error")
	}
	tool = &SlashCommandTool{}
	if _, err := tool.Execute(context.Background(), map[string]interface{}{"command": "/x"}); err == nil {
		t.Fatalf("expected executor error")
	}
}

func TestSlashCommandMultiple(t *testing.T) {
	exec := commands.NewExecutor()
	register := func(name string) {
		if err := exec.Register(commands.Definition{Name: name}, commands.HandlerFunc(func(ctx context.Context, inv commands.Invocation) (commands.Result, error) {
			return commands.Result{Command: inv.Name, Output: strings.Join(inv.Args, " ")}, nil
		})); err != nil {
			t.Fatalf("register %s: %v", name, err)
		}
	}
	register("first")
	register("second")
	cmd := "/first a b\n/second c"
	tool := NewSlashCommandTool(exec)
	res, err := tool.Execute(context.Background(), map[string]interface{}{"command": cmd})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(res.Output, "first") || !strings.Contains(res.Output, "second") {
		t.Fatalf("expected both command outputs, got %s", res.Output)
	}
}

type cmdStringer struct{}

func (cmdStringer) String() string { return "pretty" }

func TestSlashCommandMetadataAndFormatting(t *testing.T) {
	tool := NewSlashCommandTool(commands.NewExecutor())
	if tool.Name() != "SlashCommand" {
		t.Fatalf("unexpected name %q", tool.Name())
	}
	if tool.Description() == "" || tool.Schema() == nil {
		t.Fatalf("missing metadata")
	}

	results := []commands.Result{
		{Command: "a", Output: cmdStringer{}},
		{Command: "b", Error: "boom"},
		{Command: "c", Output: nil},
		{Command: "", Output: fmt.Sprintf("%d", 42)},
	}
	if out := formatCommandOutput(results); !strings.Contains(out, "boom") || !strings.Contains(out, "pretty") {
		t.Fatalf("unexpected combined output %q", out)
	}
}

func TestSlashCommandExecutorErrors(t *testing.T) {
	exec := commands.NewExecutor()
	tool := NewSlashCommandTool(exec)
	_, err := tool.Execute(context.Background(), map[string]interface{}{"command": "/missing"})
	if err == nil {
		t.Fatalf("expected executor error")
	}

	custom := &SlashCommandTool{
		executor: exec,
		parser: func(string) ([]commands.Invocation, error) {
			return nil, fmt.Errorf("parser boom")
		},
	}
	if _, err := custom.Execute(context.Background(), map[string]interface{}{"command": "/x"}); err == nil {
		t.Fatalf("expected parser error")
	}
}
