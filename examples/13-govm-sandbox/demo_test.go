package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunDemo(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "testdata", "readonly"), 0o755); err != nil {
		t.Fatalf("mkdir readonly: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "testdata", "shared"), 0o755); err != nil {
		t.Fatalf("mkdir shared: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "testdata", "readonly", "policy.txt"), []byte("sandbox policy"), 0o644); err != nil {
		t.Fatalf("write readonly policy: %v", err)
	}

	report, err := runDemo(context.Background(), demoConfig{
		ProjectRoot: root,
		SessionID:   "demo-session",
	})
	if err != nil {
		t.Fatalf("run demo: %v", err)
	}

	for _, want := range []string{
		"STEP 1 READONLY_READ: OK",
		"STEP 2 READONLY_WRITE: EXPECTED_DENIED",
		"STEP 3 SHARED_WRITE: OK",
		"STEP 4 WORKSPACE_WRITE: OK",
		"STEP 5 HOST_VERIFY: OK",
		"/inputs",
		"/shared",
		"/workspace",
	} {
		if !strings.Contains(report.Render(), want) {
			t.Fatalf("report missing %q\n%s", want, report.Render())
		}
	}

	sharedData, err := os.ReadFile(filepath.Join(root, "testdata", "shared", "result.txt"))
	if err != nil {
		t.Fatalf("read shared result: %v", err)
	}
	if !strings.Contains(string(sharedData), "demo-session") {
		t.Fatalf("shared result missing session id: %q", string(sharedData))
	}

	workspaceData, err := os.ReadFile(filepath.Join(root, "workspace", "demo-session", "session-note.txt"))
	if err != nil {
		t.Fatalf("read workspace note: %v", err)
	}
	if !strings.Contains(string(workspaceData), "demo-session") {
		t.Fatalf("workspace note missing session id: %q", string(workspaceData))
	}
}
