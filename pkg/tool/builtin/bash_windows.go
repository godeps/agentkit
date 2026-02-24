//go:build windows

package toolbuiltin

import (
	"os"
	"path/filepath"
)

func bashOutputBaseDir() string {
	return filepath.Join(os.TempDir(), "agentsdk", "bash-output")
}
