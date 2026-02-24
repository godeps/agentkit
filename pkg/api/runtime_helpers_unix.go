//go:build !windows

package api

import "path/filepath"

func bashOutputBaseDir() string {
	return filepath.Join(string(filepath.Separator), "tmp", "agentsdk", "bash-output")
}

func toolOutputBaseDir() string {
	return filepath.Join(string(filepath.Separator), "tmp", "agentsdk", "tool-output")
}
