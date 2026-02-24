package security

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestValidatorBlocksBannedCommands(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
		want string
	}{
		{name: "rm -rf / blocked by fragment", cmd: "rm -rf /", want: "fragment"},
		{name: "mkfs command", cmd: "mkfs /dev/sda", want: "mkfs"},
		{name: "dd command", cmd: "dd if=/dev/zero of=/dev/null", want: "dd"},
		{name: "format command", cmd: "format disk", want: "format"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewValidator()
			err := v.Validate(tt.cmd)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected error containing %q got %v", tt.want, err)
			}
		})
	}
}

func TestValidatorRejectsInjectionPatterns(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
		want string
	}{
		{name: "pipe metacharacter", cmd: "ls | rm -rf /", want: "metacharacters"},
		{name: "redirection metacharacter", cmd: "cat secret > /tmp/out", want: "metacharacters"},
		{name: "command chaining", cmd: "echo ok && rm -rf /", want: "metacharacters"},
		{name: "semicolon attack", cmd: "echo ok; rm -rf /", want: "metacharacters"},
		{name: "subshell expansion", cmd: "echo $(rm -rf /)", want: "metacharacters"},
		{name: "banned fragment", cmd: "touch --no-preserve-root", want: "fragment"},
		{name: "parent traversal argument", cmd: "cat ../etc/passwd", want: "argument"},
		{name: "/dev argument", cmd: "cp file /dev/sda", want: "argument"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewValidator()
			err := v.Validate(tt.cmd)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected error containing %q got %v", tt.want, err)
			}
		})
	}
}

func TestValidatorEdgeCasesAndLimits(t *testing.T) {
	tests := []struct {
		name  string
		cmd   string
		tweak func(v *Validator)
		want  string
	}{
		{name: "empty command", cmd: "   ", want: ErrEmptyCommand.Error()},
		{
			name: "control characters",
			cmd:  "echo hi" + string(rune(0)),
			want: "control characters",
		},
		{
			name: "unterminated quote",
			cmd:  "echo \"unterminated",
			want: "parse failed",
		},
		{
			name: "too many args",
			cmd:  "printf one two",
			tweak: func(v *Validator) {
				v.mu.Lock()
				defer v.mu.Unlock()
				v.maxArgs = 1
			},
			want: "too many arguments",
		},
		{
			name: "command too long",
			cmd:  strings.Repeat("a", 10),
			tweak: func(v *Validator) {
				v.mu.Lock()
				defer v.mu.Unlock()
				v.maxCommandBytes = 5
			},
			want: "command too long",
		},
		{name: "safe command allowed", cmd: "printf \"hello world\"", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewValidator()
			if tt.tweak != nil {
				tt.tweak(v)
			}
			err := v.Validate(tt.cmd)
			if tt.want == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected error containing %q got %v", tt.want, err)
			}
		})
	}
}

func TestSplitCommandHandlesQuotesAndEscapes(t *testing.T) {
	cmd := `echo "hello world" 'and more' arg\ with\ spaces`
	args, err := splitCommand(cmd)
	if err != nil {
		t.Fatalf("splitCommand: %v", err)
	}
	want := []string{"echo", "hello world", "and more", "arg with spaces"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("unexpected args: %#v", args)
	}
}

func TestSplitCommandDetectsEdgeErrors(t *testing.T) {
	if _, err := splitCommand(`printf unfinished\`); err == nil || !strings.Contains(err.Error(), "unfinished escape") {
		t.Fatalf("expected unfinished escape error got %v", err)
	}
	if _, err := splitCommand(`echo "missing end`); err == nil || !strings.Contains(err.Error(), "unterminated quote") {
		t.Fatalf("expected quote error got %v", err)
	}
}

func TestValidatorAllowsMetacharsWhenEnabled(t *testing.T) {
	v := NewValidator()
	cmd := "echo ok | grep ok"

	if err := v.Validate(cmd); err == nil || !strings.Contains(err.Error(), "metacharacters") {
		t.Fatalf("expected metacharacters to be blocked, got %v", err)
	}

	v.AllowShellMetachars(true)
	if err := v.Validate(cmd); err != nil {
		t.Fatalf("metacharacters should be allowed after toggle: %v", err)
	}
}

func TestSandboxForwardsAllowShellMetachars(t *testing.T) {
	sb := NewSandbox(t.TempDir())
	cmd := "echo ok | grep ok"

	if err := sb.ValidateCommand(cmd); err == nil || !strings.Contains(err.Error(), "metacharacters") {
		t.Fatalf("sandbox should block metacharacters by default, got %v", err)
	}

	sb.AllowShellMetachars(true)
	if err := sb.ValidateCommand(cmd); err != nil {
		t.Fatalf("sandbox should allow metacharacters when enabled: %v", err)
	}

	if err := sb.ValidateCommand("rm -rf /"); err == nil || !strings.Contains(err.Error(), "fragment") {
		t.Fatalf("banned fragments must still be enforced, got %v", err)
	}
}

func TestValidatorBannedFragmentsExhaustive(t *testing.T) {
	t.Parallel()
	v := NewValidator()
	cases := []string{
		"rm -rf /",
		"echo --no-preserve-root",
		"echo --preserve-root=false",
		"rm -fr /tmp",
		"rm -r /tmp",
		"rm --recursive /tmp",
		"rmdir -p /tmp",
		"rm *",
		"rm /",
	}

	for _, cmd := range cases {
		cmd := cmd
		t.Run(cmd, func(t *testing.T) {
			t.Helper()
			if err := v.Validate(cmd); err == nil || !strings.Contains(err.Error(), "fragment") {
				t.Fatalf("expected fragment error for %q, got %v", cmd, err)
			}
		})
	}
}

func TestValidatorBannedCommandsExhaustive(t *testing.T) {
	t.Parallel()
	v := NewValidator()
	commands := []string{
		"dd if=/dev/zero of=/dev/null",
		"mkfs /dev/sda",
		"fdisk -l",
		"parted --list",
		"format disk",
		"mkfs.ext4 /dev/sdb1",
		"shutdown now",
		"reboot",
		"halt",
		"poweroff",
		"mount /dev/sda1 /mnt",
		"sudo ls",
	}

	for _, cmd := range commands {
		cmd := cmd
		t.Run(cmd, func(t *testing.T) {
			t.Helper()
			if err := v.Validate(cmd); err == nil || !strings.Contains(err.Error(), strings.Fields(cmd)[0]) {
				t.Fatalf("expected banned command error for %q, got %v", cmd, err)
			}
		})
	}
}

func TestValidatorTreatsEmptyQuotedCommandAsEmpty(t *testing.T) {
	v := NewValidator()

	err := v.Validate(`""`)
	if !errors.Is(err, ErrEmptyCommand) {
		t.Fatalf("expected empty command error, got %v", err)
	}
}

func TestSplitCommandBackslashInsideSingleQuotes(t *testing.T) {
	cmd := "echo 'path\\to'"
	args, err := splitCommand(cmd)
	if err != nil {
		t.Fatalf("splitCommand: %v", err)
	}
	want := []string{"echo", "path\\to"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("unexpected args: %#v", args)
	}
}
