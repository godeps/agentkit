package checkpoint

import (
	"encoding/json"
	"testing"
)

func TestDecodeRecordBackwardCompatibleEnvelope(t *testing.T) {
	t.Parallel()

	legacy := []byte(`{"id":"cp-legacy","session":"sess-1","kind":"plan","state":{"step":"beta"}}`)
	rec, err := DecodeRecord(legacy)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if rec.ID != "cp-legacy" || rec.Kind != KindPlan || rec.Session != "sess-1" {
		t.Fatalf("unexpected record: %+v", rec)
	}
	state, ok := rec.State.(map[string]any)
	if !ok || state["step"] != "beta" {
		t.Fatalf("unexpected state: %#v", rec.State)
	}
}

func TestEncodeRecordVersionedEnvelope(t *testing.T) {
	t.Parallel()

	data, err := EncodeRecord(Record{
		ID:      "cp-1",
		Session: "sess-1",
		Kind:    KindApproval,
		State:   map[string]any{"tool": "echo"},
	})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("json decode: %v", err)
	}
	if raw["version"] != float64(1) {
		t.Fatalf("expected versioned envelope, got %#v", raw)
	}
}
