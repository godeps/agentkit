package clikit

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/chzyer/readline"
	"github.com/google/uuid"
)

type InteractiveShellConfig struct {
	Engine            ReplEngine
	InitialSessionID  string
	TimeoutMs         int
	Verbose           bool
	WaterfallMode     string
	ShowStatusPerTurn bool
}

type InteractiveShell struct {
	cfg InteractiveShellConfig
}

func NewInteractiveShell(cfg InteractiveShellConfig) *InteractiveShell {
	return &InteractiveShell{cfg: cfg}
}

func PrintBanner(out io.Writer, modelName string, metas []SkillMeta) {
	if out == nil {
		return
	}
	fmt.Fprintf(out, "\nAgentkit CLI\n")
	fmt.Fprintf(out, "Model: %s\n", modelName)
	fmt.Fprintf(out, "Skills: %d loaded\n", len(metas))
	fmt.Fprintf(out, "Commands: /skills /new /session /model /help /quit\n\n")
}

func RunREPL(ctx context.Context, in io.ReadCloser, out, errOut io.Writer, eng ReplEngine, timeoutMs int, verbose bool, waterfallMode string, initialSessionID string) {
	_ = RunInteractiveShell(ctx, in, out, errOut, eng, timeoutMs, verbose, waterfallMode, initialSessionID)
}

func RunInteractiveShell(ctx context.Context, in io.ReadCloser, out, errOut io.Writer, eng ReplEngine, timeoutMs int, verbose bool, waterfallMode string, initialSessionID string) error {
	shell := NewInteractiveShell(InteractiveShellConfig{
		Engine:            eng,
		InitialSessionID:  initialSessionID,
		TimeoutMs:         timeoutMs,
		Verbose:           verbose,
		WaterfallMode:     waterfallMode,
		ShowStatusPerTurn: true,
	})
	if err := shell.Run(ctx, in, out, errOut); err != nil && errOut != nil {
		fmt.Fprintf(errOut, "interactive shell failed: %v\n", err)
	}
	return nil
}

func (s *InteractiveShell) Run(ctx context.Context, in io.ReadCloser, out, errOut io.Writer) error {
	if out == nil {
		out = io.Discard
	}
	if errOut == nil {
		errOut = io.Discard
	}
	if in == nil {
		in = io.NopCloser(strings.NewReader(""))
	}
	rl, err := readline.NewEx(&readline.Config{
		Prompt:          "> ",
		Stdin:           in,
		Stdout:          nopWriteCloser{Writer: out},
		Stderr:          nopWriteCloser{Writer: errOut},
		HistoryLimit:    1000,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		fmt.Fprintf(errOut, "init repl failed: %v\n", err)
		return err
	}
	defer rl.Close()

	sessionID := strings.TrimSpace(s.cfg.InitialSessionID)
	if sessionID == "" {
		sessionID = uuid.NewString()
	}

	for {
		if s.cfg.ShowStatusPerTurn {
			printShellStatus(out, s.cfg.Engine, sessionID)
		}
		line, err := rl.Readline()
		if isReadTermination(err) {
			break
		}
		if err != nil {
			return fmt.Errorf("read failed: %w", err)
		}

		input := strings.TrimSpace(line)
		if input == "" {
			continue
		}

		if strings.HasPrefix(input, "/") {
			if handleCommand(input, s.cfg.Engine, &sessionID, out) {
				return nil
			}
			continue
		}

		if err := RunStream(ctx, out, errOut, s.cfg.Engine, sessionID, input, s.cfg.TimeoutMs, s.cfg.Verbose, s.cfg.WaterfallMode); err != nil {
			fmt.Fprintf(errOut, "run failed: %v\n", err)
		}
	}
	fmt.Fprintln(out, "bye")
	return nil
}

type nopWriteCloser struct {
	io.Writer
}

func (n nopWriteCloser) Close() error { return nil }

func isReadTermination(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, io.EOF) || errors.Is(err, readline.ErrInterrupt)
}

func handleCommand(input string, eng ReplEngine, sessionID *string, out io.Writer) (quit bool) {
	if out == nil {
		out = io.Discard
	}
	cmd := strings.ToLower(strings.Fields(input)[0])
	switch cmd {
	case "/quit", "/exit", "/q":
		fmt.Fprintln(out, "bye")
		return true
	case "/new":
		*sessionID = uuid.NewString()
		fmt.Fprintln(out, "new conversation")
	case "/model":
		fmt.Fprintf(out, "model: %s\n", eng.ModelName())
	case "/session":
		fmt.Fprintf(out, "session: %s\n", *sessionID)
	case "/help":
		fmt.Fprintln(out, "/skills /new /session /model /help /quit")
	case "/skills":
		metas := eng.Skills()
		sort.Slice(metas, func(i, j int) bool { return metas[i].Name < metas[j].Name })
		for _, m := range metas {
			fmt.Fprintf(out, "- %s\n", m.Name)
		}
	default:
		fmt.Fprintf(out, "unknown command: %s\n", cmd)
	}
	return false
}

func printShellStatus(out io.Writer, eng ReplEngine, sessionID string) {
	if out == nil || eng == nil {
		return
	}
	fmt.Fprintf(out, "Session: %s | Model: %s | Repo: %s | Sandbox: %s | Skills: %d\n",
		sessionID, eng.ModelName(), eng.RepoRoot(), displayValue(eng.SandboxBackend(), "host"), len(eng.Skills()))
}

func displayValue(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
