package gvisorhelper

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestRunExecutesRequestFromJSON(t *testing.T) {
	var stdin bytes.Buffer
	if err := json.NewEncoder(&stdin).Encode(Request{
		Version:  "v1",
		Command:  "printf 'hello'",
		GuestCwd: t.TempDir(),
	}); err != nil {
		t.Fatalf("encode request: %v", err)
	}

	var stdout bytes.Buffer
	if err := Run(context.Background(), &stdin, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("run helper: %v", err)
	}

	var resp Response
	if err := json.NewDecoder(&stdout).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success response: %#v", resp)
	}
	if strings.TrimSpace(resp.Stdout) != "hello" {
		t.Fatalf("unexpected stdout %q", resp.Stdout)
	}
}

func TestInvokeFallsBackInTests(t *testing.T) {
	resp, err := Invoke(context.Background(), Request{
		Version:  "v1",
		Command:  "printf 'fallback'",
		GuestCwd: t.TempDir(),
	}, "")
	if err != nil {
		t.Fatalf("invoke helper: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success response: %#v", resp)
	}
	if strings.TrimSpace(resp.Stdout) != "fallback" {
		t.Fatalf("unexpected stdout %q", resp.Stdout)
	}
}
