package subagents

import "testing"

func TestMemoryStoreCreateGetUpdate(t *testing.T) {
	store := NewMemoryStore()
	inst := Instance{
		ID:              "sub-1",
		Profile:         "plan",
		ParentSessionID: "parent-1",
		SessionID:       "child-1",
		Status:          StatusQueued,
	}
	if err := store.Create(inst); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := store.Update(inst.ID, func(current *Instance) error {
		current.Status = StatusRunning
		return nil
	}); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, ok := store.Get(inst.ID)
	if !ok {
		t.Fatal("expected stored instance")
	}
	if got.Status != StatusRunning {
		t.Fatalf("expected running status, got %+v", got)
	}
}
