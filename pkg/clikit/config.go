package clikit

import (
	"fmt"
	"io"
	"strings"
)

type multiValue []string

func (m *multiValue) String() string {
	return strings.Join(*m, ",")
}

func (m *multiValue) Set(value string) error {
	*m = append(*m, value)
	return nil
}

func boolPtr(v bool) *bool { return &v }

func PrintEffectiveConfig(out io.Writer, repoRoot string, cfg EffectiveConfig, timeoutMs int) {
	if out == nil {
		return
	}
	fmt.Fprintf(out, "effective-config (pre-runtime)\n")
	fmt.Fprintf(out, "  repo_root: %s\n", repoRoot)
	fmt.Fprintf(out, "  model: %s\n", cfg.ModelName)
	fmt.Fprintf(out, "  config_root: %s\n", strings.TrimSpace(cfg.ConfigRoot))
	fmt.Fprintf(out, "  timeout_ms: %d\n", timeoutMs)
	if cfg.SkillsRecursive == nil {
		fmt.Fprintf(out, "  skills_recursive: true (default)\n")
	} else {
		fmt.Fprintf(out, "  skills_recursive: %v\n", *cfg.SkillsRecursive)
	}
	if len(cfg.SkillsDirs) == 0 {
		fmt.Fprintf(out, "  skills_dirs: (auto)\n")
	} else {
		fmt.Fprintf(out, "  skills_dirs:\n")
		for _, d := range cfg.SkillsDirs {
			fmt.Fprintf(out, "    - %s\n", d)
		}
	}
}

func PrintRuntimeEffectiveConfig(out io.Writer, eng RuntimeInfo, timeoutMs int) {
	if out == nil {
		return
	}
	if eng == nil {
		fmt.Fprintf(out, "effective-config (runtime)\n")
		fmt.Fprintf(out, "  runtime: unavailable\n")
		return
	}
	fmt.Fprintf(out, "effective-config (runtime)\n")
	fmt.Fprintf(out, "  model: %s\n", eng.ModelName())
	fmt.Fprintf(out, "  config_root: %s\n", eng.SettingsRoot())
	fmt.Fprintf(out, "  timeout_ms: %d\n", timeoutMs)
	fmt.Fprintf(out, "  skills_recursive: %v\n", eng.SkillsRecursive())
	dirs := eng.SkillsDirs()
	if len(dirs) == 0 {
		fmt.Fprintf(out, "  skills_dirs: (none)\n")
	} else {
		fmt.Fprintf(out, "  skills_dirs:\n")
		for _, d := range dirs {
			fmt.Fprintf(out, "    - %s\n", d)
		}
	}
}
