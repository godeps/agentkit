package prompts

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/godeps/agentkit/pkg/runtime/skills"
)

func TestParseSkills(t *testing.T) {
	t.Parallel()

	fs := fstest.MapFS{
		"skills/demo/SKILL.md": {Data: []byte("---\nname: demo\ndescription: test\nallowed-tools:\n - Bash\n---\nbody")},
	}
	regs, errs := parseSkills(fs, "skills", true)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors %v", errs)
	}
	if len(regs) != 1 || regs[0].Definition.Name != "demo" {
		t.Fatalf("unexpected registrations %v", regs)
	}
	res, err := regs[0].Handler.Execute(context.Background(), skills.ActivationContext{})
	if err != nil || res.Skill != "demo" {
		t.Fatalf("unexpected handler result %v err=%v", res, err)
	}
}

func TestParseSkillsErrors(t *testing.T) {
	t.Parallel()

	fs := fstest.MapFS{
		"skills/demo/SKILL.md": {Data: []byte("---\nname: other\n---\nbody")},
	}
	_, errs := parseSkills(fs, "skills", true)
	if len(errs) == 0 || !strings.Contains(errs[0].Error(), "does not match") {
		t.Fatalf("expected name mismatch error")
	}
}
