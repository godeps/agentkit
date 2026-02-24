package toolbuiltin

import (
	"context"
	"strings"
	"testing"

	"github.com/godeps/agentkit/pkg/runtime/tasks"
)

func TestTaskUpdateToolDelete(t *testing.T) {
	t.Parallel()

	store := tasks.NewTaskStore()
	task, err := store.Create("todo", "", "")
	if err != nil {
		t.Fatalf("create task failed: %v", err)
	}

	tool := NewTaskUpdateTool(store)
	res, err := tool.Execute(context.Background(), map[string]interface{}{
		"taskId": task.ID,
		"delete": true,
	})
	if err != nil {
		t.Fatalf("execute delete failed: %v", err)
	}
	if !res.Success || !strings.Contains(res.Output, "deleted") {
		t.Fatalf("unexpected delete output %q", res.Output)
	}
}

func TestTaskUpdateToolUnblocks(t *testing.T) {
	t.Parallel()

	store := tasks.NewTaskStore()
	blocker, err := store.Create("blocker", "", "")
	if err != nil {
		t.Fatalf("create blocker: %v", err)
	}
	blocked, err := store.Create("blocked", "", "")
	if err != nil {
		t.Fatalf("create blocked: %v", err)
	}
	if err := store.AddDependency(blocked.ID, blocker.ID); err != nil {
		t.Fatalf("add dependency: %v", err)
	}

	tool := NewTaskUpdateTool(store)
	res, err := tool.Execute(context.Background(), map[string]interface{}{
		"taskId": blocker.ID,
		"status": "completed",
	})
	if err != nil {
		t.Fatalf("execute update failed: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success")
	}
	if !strings.Contains(res.Output, "unblocked") {
		t.Fatalf("expected unblocked output, got %q", res.Output)
	}
	data, ok := res.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected data type %T", res.Data)
	}
	if _, ok := data["unblocked"]; !ok {
		t.Fatalf("expected unblocked list in data")
	}
}

func TestTaskUpdateSnapshot(t *testing.T) {
	t.Parallel()

	if _, ok := (*TaskUpdateTool)(nil).Snapshot("x"); ok {
		t.Fatalf("expected nil snapshot")
	}

	store := tasks.NewTaskStore()
	task, err := store.Create("snap", "", "")
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	tool := NewTaskUpdateTool(store)
	got, ok := tool.Snapshot(task.ID)
	if !ok || got.ID != task.ID {
		t.Fatalf("unexpected snapshot %v ok=%v", got, ok)
	}
}

func TestParseTaskIDList(t *testing.T) {
	t.Parallel()

	if _, err := parseTaskIDList("bad", "blocks", "x"); err == nil {
		t.Fatalf("expected error for invalid list")
	}
	if _, err := parseTaskIDList([]interface{}{""}, "blocks", "x"); err == nil {
		t.Fatalf("expected error for empty id")
	}
	if _, err := parseTaskIDList([]interface{}{"x"}, "blocks", "x"); err == nil {
		t.Fatalf("expected error for self reference")
	}

	ids, err := parseTaskIDList([]interface{}{"b", "a", "a"}, "blocks", "x")
	if err != nil {
		t.Fatalf("parse list failed: %v", err)
	}
	if strings.Join(ids, ",") != "a,b" {
		t.Fatalf("unexpected ids %v", ids)
	}
}

func TestNormalizeUpdateStatus(t *testing.T) {
	t.Parallel()

	if _, ok := normalizeUpdateStatus(""); ok {
		t.Fatalf("expected empty status rejected")
	}
	if v, ok := normalizeUpdateStatus("done"); !ok || v != tasks.TaskCompleted {
		t.Fatalf("expected done -> completed, got %v ok=%v", v, ok)
	}
}

func TestTaskUpdateToolMetadata(t *testing.T) {
	t.Parallel()

	tool := NewTaskUpdateTool(tasks.NewTaskStore())
	if tool.Name() == "" || tool.Description() == "" || tool.Schema() == nil {
		t.Fatalf("expected metadata")
	}
}

func TestReplaceBlockedByAndBlocks(t *testing.T) {
	t.Parallel()

	store := tasks.NewTaskStore()
	a, _ := store.Create("A", "", "")
	b, _ := store.Create("B", "", "")
	c, _ := store.Create("C", "", "")

	affected := map[string]struct{}{}
	if err := replaceBlockedBy(store, b.ID, nil, []string{a.ID}, affected); err != nil {
		t.Fatalf("replaceBlockedBy failed: %v", err)
	}
	if err := replaceBlocks(store, a.ID, nil, []string{c.ID}, affected); err != nil {
		t.Fatalf("replaceBlocks failed: %v", err)
	}
	if len(affected) == 0 {
		t.Fatalf("expected affected tasks")
	}
}

func TestParseTaskUpdateParamsErrors(t *testing.T) {
	t.Parallel()

	if _, err := parseTaskUpdateParams(nil); err == nil {
		t.Fatalf("expected nil params error")
	}
	if _, err := parseTaskUpdateParams(map[string]interface{}{"taskId": "x", "status": 1}); err == nil {
		t.Fatalf("expected status type error")
	}
	if _, err := parseTaskUpdateParams(map[string]interface{}{"taskId": "x", "owner": 1}); err == nil {
		t.Fatalf("expected owner type error")
	}
	if _, err := parseTaskUpdateParams(map[string]interface{}{"taskId": "x", "delete": "no"}); err == nil {
		t.Fatalf("expected delete type error")
	}
}

func TestParseTaskUpdateParamsBlocksAndUnblock(t *testing.T) {
	req, err := parseTaskUpdateParams(map[string]interface{}{
		"taskId":    "a",
		"blocks":    []interface{}{"b"},
		"blockedBy": []interface{}{"c"},
	})
	if err != nil {
		t.Fatalf("parse params failed: %v", err)
	}
	if !req.HasBlocks || !req.HasBlockedBy || len(req.Blocks) != 1 || len(req.BlockedBy) != 1 {
		t.Fatalf("unexpected request %+v", req)
	}

	store := tasks.NewTaskStore()
	a, _ := store.Create("A", "", "")
	b, _ := store.Create("B", "", "")
	c, _ := store.Create("C", "", "")
	_ = store.AddDependency(b.ID, a.ID)
	affected := map[string]struct{}{}
	if err := replaceBlockedBy(store, b.ID, []string{a.ID}, []string{c.ID}, affected); err != nil {
		t.Fatalf("replaceBlockedBy failed: %v", err)
	}
	if err := replaceBlocks(store, a.ID, []string{b.ID}, nil, affected); err != nil {
		t.Fatalf("replaceBlocks failed: %v", err)
	}

	list := []*tasks.Task{{ID: "x", Status: tasks.TaskPending, BlockedBy: []string{a.ID}}}
	before := map[string]tasks.TaskStatus{"x": tasks.TaskBlocked}
	unblocked := newlyUnblocked(list, a.ID, before)
	if len(unblocked) != 1 || unblocked[0] != "x" {
		t.Fatalf("expected unblocked list, got %v", unblocked)
	}
}

func TestTaskUpdateToolUpdateOwnerAndBlocks(t *testing.T) {
	store := tasks.NewTaskStore()
	a, _ := store.Create("A", "", "")
	b, _ := store.Create("B", "", "")
	c, _ := store.Create("C", "", "")
	_ = store.AddDependency(b.ID, a.ID)

	tool := NewTaskUpdateTool(store)
	res, err := tool.Execute(context.Background(), map[string]interface{}{
		"taskId": b.ID,
		"owner":  "Bob",
		"blocks": []interface{}{c.ID},
	})
	if err != nil {
		t.Fatalf("execute update failed: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success")
	}
	snap, ok := tool.Snapshot(b.ID)
	if !ok || snap.Owner != "Bob" {
		t.Fatalf("expected owner update, got %+v", snap)
	}
	if len(snap.Blocks) != 1 || snap.Blocks[0] != c.ID {
		t.Fatalf("expected blocks updated, got %+v", snap.Blocks)
	}
}

func TestTaskUpdateToolExecuteNilContext(t *testing.T) {
	tool := NewTaskUpdateTool(tasks.NewTaskStore())
	if _, err := tool.Execute(nil, map[string]interface{}{"taskId": "x"}); err == nil {
		t.Fatalf("expected nil context error")
	}
}
