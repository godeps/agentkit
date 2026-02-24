package sandbox

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/godeps/agentkit/pkg/security"
)

func TestManagerCheckToolPermissionLoadsAndAudits(t *testing.T) {
	root := canonicalTempDir(t)
	claudeDir := filepath.Join(root, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	settings := `{"permissions":{"deny":["Bash(rm:*)"],"allow":["Bash(ls:*)"]}}`
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(settings), 0o600); err != nil {
		t.Fatalf("write settings: %v", err)
	}

	mgr := NewManager(NewFileSystemAllowList(root), nil, nil)

	deny, err := mgr.CheckToolPermission("Bash", map[string]any{"command": "rm -rf /"})
	if err != nil {
		t.Fatalf("check deny: %v", err)
	}
	if deny.Action != security.PermissionDeny || deny.Rule == "" || !strings.Contains(deny.Target, "rm") {
		t.Fatalf("unexpected deny decision: %+v", deny)
	}

	allow, err := mgr.CheckToolPermission("Bash", map[string]any{"command": "ls"})
	if err != nil {
		t.Fatalf("check allow: %v", err)
	}
	if allow.Action != security.PermissionAllow {
		t.Fatalf("expected allow, got %+v", allow)
	}

	audits := mgr.PermissionAudits()
	if len(audits) != 2 {
		t.Fatalf("expected 2 audits, got %d", len(audits))
	}
	audits[0].Action = security.PermissionAllow
	if mgr.PermissionAudits()[0].Action != security.PermissionDeny {
		t.Fatalf("audits slice should be a copy")
	}
}

func TestManagerCheckToolPermissionCachesErrors(t *testing.T) {
	root := canonicalTempDir(t)
	claudeDir := filepath.Join(root, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// malformed rule forces LoadPermissions failure
	bad := `{"permissions":{"deny":["Bash("]}}`
	settingsPath := filepath.Join(claudeDir, "settings.json")
	if err := os.WriteFile(settingsPath, []byte(bad), 0o600); err != nil {
		t.Fatalf("write bad settings: %v", err)
	}

	mgr := NewManager(NewFileSystemAllowList(root), nil, nil)
	if _, err := mgr.CheckToolPermission("Bash", map[string]any{"command": "ls"}); err == nil {
		t.Fatal("expected error from malformed permissions")
	}

	// Even after fixing the file, the cached error should be returned due to permOnce.
	good := `{"permissions":{"allow":["Bash(ls:*)"]}}`
	if err := os.WriteFile(settingsPath, []byte(good), 0o600); err != nil {
		t.Fatalf("write good settings: %v", err)
	}
	if _, err := mgr.CheckToolPermission("Bash", map[string]any{"command": "ls"}); err == nil || !strings.Contains(err.Error(), "permission") {
		t.Fatalf("expected cached error after fix, got %v", err)
	}
	if audits := mgr.PermissionAudits(); len(audits) != 0 {
		t.Fatalf("audit log should stay empty on load failure, got %v", audits)
	}
}

func TestManagerCheckToolPermissionNilManager(t *testing.T) {
	var mgr *Manager
	decision, err := mgr.CheckToolPermission("Any", nil)
	if err != nil {
		t.Fatalf("unexpected error for nil manager: %v", err)
	}
	if decision.Action != security.PermissionAllow {
		t.Fatalf("nil manager should allow by default, got %v", decision.Action)
	}
}
