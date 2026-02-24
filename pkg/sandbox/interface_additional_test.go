package sandbox

import "testing"

func TestManagerPermissionAudits(t *testing.T) {
	var nilMgr *Manager
	if audits := nilMgr.PermissionAudits(); audits != nil {
		t.Fatalf("expected nil audits for nil manager")
	}

	root := t.TempDir()
	fs := NewFileSystemAllowList(root)
	mgr := NewManager(fs, nil, nil)
	if audits := mgr.PermissionAudits(); audits == nil {
		t.Fatalf("expected non-nil audits slice")
	}
}
