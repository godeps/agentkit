package clikit

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/chzyer/readline"
	"github.com/godeps/agentkit/pkg/api"
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

type shellState struct {
	lastResponse   *api.Response
	lastCheckpoint string
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
	if err := shell.Run(ctx, in, out, errOut); err != nil {
		if errOut != nil {
			fmt.Fprintf(errOut, "interactive shell failed: %v\n", err)
		}
		return err
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
	state := &shellState{}

	for {
		if s.cfg.ShowStatusPerTurn {
			printShellStatus(out, s.cfg.Engine, sessionID)
		}
		line, err := rl.Readline()
		if errors.Is(err, io.EOF) {
			break
		}
		if errors.Is(err, readline.ErrInterrupt) {
			fmt.Fprintln(out)
			continue
		}
		if err != nil {
			return fmt.Errorf("read failed: %w", err)
		}

		input := strings.TrimSpace(line)
		if input == "" {
			continue
		}

		if handled, quit := handleCommand(input, s.cfg.Engine, &sessionID, state, ctx, out, errOut); handled {
			if quit {
				return nil
			}
			continue
		}

		resp, err := s.cfg.Engine.Run(ctx, api.Request{
			Prompt:    input,
			SessionID: sessionID,
		})
		if err != nil {
			fmt.Fprintf(errOut, "run failed: %v\n", err)
			continue
		}
		updateShellState(state, resp)
		renderResponseSummary(out, resp)
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
	return errors.Is(err, io.EOF)
}

func handleCommand(input string, eng ReplEngine, sessionID *string, state *shellState, ctx context.Context, out, errOut io.Writer) (handled bool, quit bool) {
	if out == nil {
		out = io.Discard
	}
	fields := strings.Fields(input)
	if len(fields) == 0 {
		return false, false
	}
	cmd := strings.ToLower(fields[0])
	switch cmd {
	case "/quit", "/exit", "/q":
		fmt.Fprintln(out, "bye")
		return true, true
	case "/new":
		*sessionID = uuid.NewString()
		fmt.Fprintln(out, "new conversation")
		return true, false
	case "/model":
		fmt.Fprintf(out, "model: %s\n", eng.ModelName())
		return true, false
	case "/session":
		fmt.Fprintf(out, "session: %s\n", *sessionID)
		return true, false
	case "/help":
		fmt.Fprintln(out, "/skills /new /session /model /checkpoint /timeline /resume <id> /help /quit")
		return true, false
	case "/skills":
		metas := eng.Skills()
		sort.Slice(metas, func(i, j int) bool { return metas[i].Name < metas[j].Name })
		for _, m := range metas {
			fmt.Fprintf(out, "- %s\n", m.Name)
		}
		return true, false
	case "/checkpoint":
		checkpointID := ""
		if state != nil {
			checkpointID = strings.TrimSpace(state.lastCheckpoint)
		}
		if checkpointID == "" {
			fmt.Fprintln(out, "no checkpoint available")
			return true, false
		}
		fmt.Fprintf(out, "checkpoint: %s\n", checkpointID)
		return true, false
	case "/timeline":
		if state == nil || state.lastResponse == nil || len(eng.Timeline(state.lastResponse)) == 0 {
			fmt.Fprintln(out, "no timeline available")
			return true, false
		}
		printTimeline(out, eng.Timeline(state.lastResponse))
		return true, false
	case "/resume":
		if len(fields) < 2 || strings.TrimSpace(fields[1]) == "" {
			fmt.Fprintln(out, "usage: /resume <checkpoint-id>")
			return true, false
		}
		resp, err := eng.Resume(ctx, strings.TrimSpace(fields[1]))
		if err != nil {
			if errOut != nil {
				fmt.Fprintf(errOut, "resume failed: %v\n", err)
			}
			return true, false
		}
		updateShellState(state, resp)
		renderResponseSummary(out, resp)
		return true, false
	}
	return false, false
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

func updateShellState(state *shellState, resp *api.Response) {
	if state == nil || resp == nil {
		return
	}
	state.lastResponse = resp
	if resp.Result != nil && strings.TrimSpace(resp.Result.CheckpointID) != "" {
		state.lastCheckpoint = strings.TrimSpace(resp.Result.CheckpointID)
	}
}

func renderResponseSummary(out io.Writer, resp *api.Response) {
	if out == nil || resp == nil || resp.Result == nil {
		return
	}
	if text := strings.TrimSpace(resp.Result.Output); text != "" {
		fmt.Fprintln(out, text)
	}
	if checkpointID := strings.TrimSpace(resp.Result.CheckpointID); checkpointID != "" {
		fmt.Fprintf(out, "checkpoint: %s\n", checkpointID)
	}
}

func printTimeline(out io.Writer, timeline []api.TimelineEntry) {
	if out == nil {
		return
	}
	for _, entry := range timeline {
		if strings.TrimSpace(entry.Source) != "" {
			fmt.Fprintf(out, "- %s source=%s\n", entry.Kind, entry.Source)
			continue
		}
		fmt.Fprintf(out, "- %s\n", entry.Kind)
	}
}
