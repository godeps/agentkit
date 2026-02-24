package middleware

import (
	"testing"
)

func TestStateSetModelIOHandlesNil(t *testing.T) {
	var nilState *State
	nilState.SetModelInput("noop")
	nilState.SetModelOutput("noop")

	st := &State{}
	st.SetModelInput("input")
	st.SetModelOutput("output")
	if st.ModelInput != "input" || st.ModelOutput != "output" {
		t.Fatalf("model IO not stored: %#v", st)
	}
}

func TestStateSetValueInitializesAndTrims(t *testing.T) {
	st := &State{}
	st.SetValue("  key  ", "value")
	if st.Values == nil {
		t.Fatalf("values map not initialized")
	}
	if got := st.Values["  key  "]; got != "value" {
		t.Fatalf("unexpected value: %v", got)
	}
	st.SetValue("   ", "ignored")
	if len(st.Values) != 1 {
		t.Fatalf("blank key should be ignored, map: %#v", st.Values)
	}

	var nilState *State
	nilState.SetValue("safe", "value")
}

func TestStateCloneIsolationRequiresMapCopy(t *testing.T) {
	original := &State{Iteration: 1, Values: map[string]any{"trace": "alpha"}}
	clone := *original
	clone.Values = map[string]any{}
	for k, v := range original.Values {
		clone.Values[k] = v
	}
	clone.SetValue("extra", "beta")

	if _, ok := original.Values["extra"]; ok {
		t.Fatalf("original mutated through clone: %#v", original.Values)
	}
	if clone.Values["extra"] != "beta" {
		t.Fatalf("clone missing value: %#v", clone.Values)
	}
}
