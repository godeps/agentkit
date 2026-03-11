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
		return
	}
	defer rl.Close()

	sessionID := strings.TrimSpace(initialSessionID)
	if sessionID == "" {
		sessionID = uuid.NewString()
	}

	for {
		line, err := rl.Readline()
		if isReadTermination(err) {
			break
		}
		if err != nil {
			fmt.Fprintf(errOut, "read failed: %v\n", err)
			break
		}

		input := strings.TrimSpace(line)
		if input == "" {
			continue
		}

		if strings.HasPrefix(input, "/") {
			if handleCommand(input, eng, &sessionID, out) {
				return
			}
			continue
		}

		if err := RunStream(ctx, out, errOut, eng, sessionID, input, timeoutMs, verbose, waterfallMode); err != nil {
			fmt.Fprintf(errOut, "run failed: %v\n", err)
		}
	}
	fmt.Fprintln(out, "bye")
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
