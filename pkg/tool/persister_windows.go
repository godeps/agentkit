//go:build windows

package tool

import (
	"os"
	"path/filepath"
)

func toolOutputBaseDir() string {
	return filepath.Join(os.TempDir(), "agentsdk", "tool-output")
}
