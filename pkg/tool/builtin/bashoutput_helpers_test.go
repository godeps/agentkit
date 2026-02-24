//go:build helperstub

package toolbuiltin

import (
	"path/filepath"
	"runtime"
	"testing"
)

func skipIfWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("requires POSIX environment")
	}
}

func cleanTempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return dir
	}
	return resolved
}
