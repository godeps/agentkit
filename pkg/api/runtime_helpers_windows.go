//go:build windows

package api

import (
	"os"
	"path/filepath"
)

func bashOutputBaseDir() string {
	return filepath.Join(os.TempDir(), "agentsdk", "bash-output")
}

func toolOutputBaseDir() string {
	return filepath.Join(os.TempDir(), "agentsdk", "tool-output")
}
