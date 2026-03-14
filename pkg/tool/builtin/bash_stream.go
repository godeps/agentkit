package toolbuiltin

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	sandboxenv "github.com/godeps/agentkit/pkg/sandbox/env"
	"github.com/godeps/agentkit/pkg/tool"
)

// StreamExecute runs the bash command while emitting incremental output. It
// preserves backwards compatibility by sharing validation and metadata with
// Execute, and spools output to disk after crossing the configured threshold.
func (b *BashTool) StreamExecute(ctx context.Context, params map[string]interface{}, emit func(chunk string, isStderr bool)) (*tool.ToolResult, error) {
	if ctx == nil {
		return nil, errors.New("context is nil")
	}
	if b == nil || b.sandbox == nil {
		return nil, errors.New("bash tool is not initialised")
	}

	command, err := extractCommand(params)
	if err != nil {
		return nil, err
	}
	if err := b.sandbox.ValidateCommand(command); err != nil {
		return nil, err
	}
	ps, err := b.prepareSession(ctx)
	if err != nil {
		return nil, err
	}
	workdir, err := b.resolveWorkdir(params, ps)
	if err != nil {
		return nil, err
	}
	if isVirtualizedSandboxSession(ps) {
		streamEnv, ok := b.env.(sandboxenv.StreamingExecutionEnvironment)
		if !ok {
			return nil, errors.New("streaming bash is not supported in virtualized sandbox mode")
		}
		timeout, err := b.resolveTimeout(params)
		if err != nil {
			return nil, err
		}
		start := time.Now()
		spool := newBashOutputSpool(ctx, b.effectiveOutputThresholdBytes())
		res, err := streamEnv.RunCommandStream(ctx, ps, sandboxenv.CommandRequest{
			Command: command,
			Workdir: workdir,
			Timeout: timeout,
		}, sandboxenv.CommandStreamCallbacks{
			OnStdout: func(chunk string) {
				if emit != nil {
					emit(chunk, false)
				}
				if spool != nil {
					_ = spool.Append(chunk, false)
					_ = spool.Append("\n", false)
				}
			},
			OnStderr: func(chunk string) {
				if emit != nil {
					emit(chunk, true)
				}
				if spool != nil {
					_ = spool.Append(chunk, true)
					_ = spool.Append("\n", true)
				}
			},
		})
		if res == nil {
			res = &sandboxenv.CommandResult{}
		}
		output, outputFile, spoolErr := spool.Finalize()
		duration := res.Duration
		if duration <= 0 {
			duration = time.Since(start)
		}
		data := map[string]interface{}{
			"workdir":     workdir,
			"duration_ms": duration.Milliseconds(),
			"timeout_ms":  timeout.Milliseconds(),
		}
		if outputFile != "" {
			data["output_file"] = outputFile
		}
		if spoolErr != nil {
			data["spool_error"] = spoolErr.Error()
		}
		if output == "" {
			output = combineOutput(res.Stdout, res.Stderr)
		}
		result := &tool.ToolResult{
			Success: err == nil && res.ExitCode == 0,
			Output:  output,
			Data:    data,
		}
		if err != nil {
			return result, err
		}
		return result, nil
	}
	timeout, err := b.resolveTimeout(params)
	if err != nil {
		return nil, err
	}

	execCtx := ctx
	var cancel context.CancelFunc
	if timeout > 0 {
		execCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(execCtx, "bash", "-c", command)
	cmd.Env = os.Environ()
	cmd.Dir = workdir

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	spool := newBashOutputSpool(ctx, b.effectiveOutputThresholdBytes())
	start := time.Now()
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start command: %w", err)
	}

	readCtx, stopReads := context.WithCancel(execCtx)
	defer stopReads()
	defer stdoutPipe.Close()
	defer stderrPipe.Close()

	var closePipesOnce sync.Once
	closePipes := func() {
		closePipesOnce.Do(func() {
			_ = stdoutPipe.Close()
			_ = stderrPipe.Close()
		})
	}

	cancelWatcherDone := make(chan struct{})
	go func() {
		select {
		case <-execCtx.Done():
			stopReads()
			closePipes()
		case <-cancelWatcherDone:
		}
	}()

	var stdoutErr, stderrErr error
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		stdoutErr = consumeStream(readCtx, stdoutPipe, emit, spool, false)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		stderrErr = consumeStream(readCtx, stderrPipe, emit, spool, true)
	}()

	wg.Wait()
	close(cancelWatcherDone)
	waitErr := cmd.Wait()
	duration := time.Since(start)

	runErr := waitErr
	if stdoutErr != nil {
		runErr = errors.Join(runErr, fmt.Errorf("stdout read: %w", stdoutErr))
	}
	if stderrErr != nil {
		runErr = errors.Join(runErr, fmt.Errorf("stderr read: %w", stderrErr))
	}

	output, outputFile, spoolErr := spool.Finalize()

	data := map[string]interface{}{
		"workdir":     workdir,
		"duration_ms": duration.Milliseconds(),
		"timeout_ms":  timeout.Milliseconds(),
	}
	if outputFile != "" {
		data["output_file"] = outputFile
	}
	if spoolErr != nil {
		data["spool_error"] = spoolErr.Error()
	}

	result := &tool.ToolResult{
		Success: runErr == nil,
		Output:  output,
		Data:    data,
	}

	if runErr != nil {
		if errors.Is(execCtx.Err(), context.DeadlineExceeded) {
			return result, fmt.Errorf("command timeout after %s", timeout)
		}
		if errors.Is(execCtx.Err(), context.Canceled) {
			return result, execCtx.Err()
		}
		return result, fmt.Errorf("command failed: %w", runErr)
	}
	return result, nil
}

func consumeStream(ctx context.Context, r io.ReadCloser, emit func(chunk string, isStderr bool), spool *bashOutputSpool, isStderr bool) error {
	defer r.Close()
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if emit != nil {
			emit(line, isStderr)
		}
		if spool != nil {
			_ = spool.Append(line, isStderr) //nolint:errcheck // best-effort spool
			_ = spool.Append("\n", isStderr) //nolint:errcheck // best-effort spool
		}
		if ctx.Err() != nil {
			break
		}
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		if ctx.Err() != nil || errors.Is(err, os.ErrClosed) || errors.Is(err, io.ErrClosedPipe) {
			return nil
		}
		return err
	}
	return nil
}
