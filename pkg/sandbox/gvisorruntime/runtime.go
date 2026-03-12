package gvisorruntime

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	runscmain "gvisor.dev/gvisor/runsc/cli/maincli"
	"gvisor.dev/gvisor/runsc/specutils"
)

// Run delegates the current process to the imported runsc entrypoint.
func Run() error {
	if exe := resolveExecutablePath(); exe != "" {
		specutils.ExePath = exe
	}
	runscmain.Main()
	return nil
}

func resolveExecutablePath() string {
	if len(os.Args) > 0 && strings.TrimSpace(os.Args[0]) != "" {
		if filepath.IsAbs(os.Args[0]) {
			return filepath.Clean(os.Args[0])
		}
		if lp, err := execLookPath(os.Args[0]); err == nil {
			return filepath.Clean(lp)
		}
	}
	if exe, err := os.Executable(); err == nil && strings.TrimSpace(exe) != "" {
		return filepath.Clean(exe)
	}
	return ""
}

var execLookPath = func(file string) (string, error) {
	return exec.LookPath(file)
}
