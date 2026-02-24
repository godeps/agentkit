package toolbuiltin

import (
	"context"
	"strings"
	"testing"

	"github.com/godeps/agentkit/pkg/runtime/tasks"
)

func TestSortIDsByTaskKey(t *testing.T) {
	t.Parallel()

	items := map[string]tasks.Task{
		"a": {ID: "a", Status: tasks.TaskInProgress, Owner: "bob"},
		"b": {ID: "b", Status: tasks.TaskPending, Owner: "alice"},
		"c": {ID: "c", Status: tasks.TaskCompleted, Owner: "alice"},
	}
	ids := []string{"b", "c", "a"}
	sortIDsByTaskKey(ids, items)
	if got := strings.Join(ids, ","); got != "a,b,c" {
		t.Fatalf("unexpected order %s", got)
	}
}

func TestTaskStatusPriority(t *testing.T) {
	t.Parallel()

	if taskStatusPriority(TaskStatusInProgress) >= taskStatusPriority(TaskStatusPending) {
		t.Fatalf("in_progress should rank above pending")
	}
	if taskStatusPriority("unknown") <= taskStatusPriority(TaskStatusCompleted) {
		t.Fatalf("unknown should be lowest priority")
	}
}

func TestFormatTaskListOutput(t *testing.T) {
	t.Parallel()

	taskList := []tasks.Task{
		{ID: "a", Status: tasks.TaskPending, Subject: "Alpha", Owner: "Bob"},
		{ID: "b", Status: tasks.TaskInProgress, Subject: "Beta"},
	}
	order := []string{"b", "a"}
	depth := map[string]int{"b": 0, "a": 1}
	blocks := map[string][]string{"b": {"a"}}
	counts := map[string]int{
		TaskStatusPending:    1,
		TaskStatusInProgress: 1,
		TaskStatusCompleted:  0,
		TaskStatusBlocked:    0,
	}

	out := formatTaskListOutput(taskList, order, depth, blocks, counts)
	if !strings.Contains(out, "2 tasks") || !strings.Contains(out, "[in_progress]") {
		t.Fatalf("unexpected output %q", out)
	}
	if out == "no tasks" {
		t.Fatalf("unexpected empty output")
	}
}

func TestFormatTaskListOutputEmpty(t *testing.T) {
	t.Parallel()

	if out := formatTaskListOutput(nil, nil, nil, nil, nil); out != "no tasks" {
		t.Fatalf("expected no tasks output, got %q", out)
	}
}

func TestTaskListToolMetadataAndParams(t *testing.T) {
	t.Parallel()

	store := tasks.NewTaskStore()
	tool := NewTaskListTool(store)
	if tool.Name() == "" || tool.Description() == "" || tool.Schema() == nil {
		t.Fatalf("expected metadata to be set")
	}

	if _, _, err := parseTaskListParams(map[string]interface{}{"status": 1}); err == nil {
		t.Fatalf("expected status type error")
	}
	if _, _, err := parseTaskListParams(map[string]interface{}{"status": "bad"}); err == nil {
		t.Fatalf("expected status validation error")
	}
	status, owner, err := parseTaskListParams(map[string]interface{}{"status": "pending", "owner": "Bob"})
	if err != nil || status != TaskStatusPending || owner != "Bob" {
		t.Fatalf("unexpected params status=%q owner=%q err=%v", status, owner, err)
	}
}

func TestTaskListToolExecute(t *testing.T) {
	t.Parallel()

	store := tasks.NewTaskStore()
	task, _ := store.Create("alpha", "", "")
	tool := NewTaskListTool(store)
	res, err := tool.Execute(context.Background(), map[string]interface{}{
		"status": "pending",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if !res.Success || !strings.Contains(res.Output, task.ID) {
		t.Fatalf("unexpected output %q", res.Output)
	}
}

func TestTaskListFilterAndOrder(t *testing.T) {
	taskList := []tasks.Task{
		{ID: "a", Status: tasks.TaskPending, Owner: "Bob"},
		{ID: "b", Status: tasks.TaskCompleted, Owner: "Alice", BlockedBy: []string{"a"}},
	}
	filtered := filterTasks(taskList, TaskStatusPending, "bob")
	if len(filtered) != 1 || filtered[0].ID != "a" {
		t.Fatalf("unexpected filtered tasks %v", filtered)
	}

	order, depth, blocks := taskDisplayOrder(taskList)
	if len(order) != 2 || order[0] != "a" || order[1] != "b" {
		t.Fatalf("unexpected order %v", order)
	}
	if depth["a"] != 0 || depth["b"] != 1 {
		t.Fatalf("unexpected depth %v", depth)
	}
	if len(blocks["a"]) != 1 || blocks["a"][0] != "b" {
		t.Fatalf("unexpected blocks %v", blocks)
	}
}

func TestSortIDsByTaskKeyMissingEntries(t *testing.T) {
	items := map[string]tasks.Task{
		"a": {ID: "a", Status: tasks.TaskPending, Owner: "bob"},
	}
	ids := []string{"missing", "a"}
	sortIDsByTaskKey(ids, items)
	if len(ids) != 2 || ids[0] != "a" {
		t.Fatalf("unexpected order with missing entry %v", ids)
	}
}

func TestTaskDisplayOrderCycle(t *testing.T) {
	taskList := []tasks.Task{
		{ID: "a", Status: tasks.TaskPending, BlockedBy: []string{"b"}},
		{ID: "b", Status: tasks.TaskPending, BlockedBy: []string{"a"}},
	}
	order, depth, blocks := taskDisplayOrder(taskList)
	if len(order) != 2 {
		t.Fatalf("expected order to include both tasks, got %v", order)
	}
	if depth["a"] != 0 || depth["b"] != 1 {
		t.Fatalf("unexpected depth for cycle, got %v", depth)
	}
	if len(blocks) == 0 {
		t.Fatalf("expected blocks map populated")
	}
}

func TestTaskListToolExecuteErrors(t *testing.T) {
	tool := NewTaskListTool(nil)
	if _, err := tool.Execute(context.Background(), map[string]interface{}{}); err == nil {
		t.Fatalf("expected missing store error")
	}

	store := tasks.NewTaskStore()
	tool = NewTaskListTool(store)
	if _, err := tool.Execute(nil, map[string]interface{}{}); err == nil {
		t.Fatalf("expected nil context error")
	}
	if _, err := tool.Execute(context.Background(), map[string]interface{}{"owner": 1}); err == nil {
		t.Fatalf("expected owner type error")
	}
}
