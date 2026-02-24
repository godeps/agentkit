package commands_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/godeps/agentkit/pkg/runtime/commands"
)

func TestLazyLoadStartupOnlyFrontmatter(t *testing.T) {
	root := t.TempDir()
	body := strings.Join([]string{
		"---",
		"description: say hi",
		"argument-hint: '<name>'",
		"---",
		"hello $ARGUMENTS",
	}, "\n")
	path := writeCommandFile(t, root, "hello.md", body)

	var bodyReads atomic.Int32
	var metaReads atomic.Int32
	restore := commands.SetCommandFileOpsForTest(
		func(p string) ([]byte, error) {
			bodyReads.Add(1)
			return os.ReadFile(p)
		},
		func(p string) (*os.File, error) {
			metaReads.Add(1)
			return os.Open(p)
		},
		nil,
	)
	t.Cleanup(restore)

	regs, errs := commands.LoadFromFS(commands.LoaderOptions{ProjectRoot: root})
	if len(errs) != 0 {
		t.Fatalf("load errors: %v", errs)
	}
	if len(regs) != 1 {
		t.Fatalf("expected 1 command, got %d", len(regs))
	}
	if got := regs[0].Definition.Description; got != "say hi" {
		t.Fatalf("description missing from frontmatter, got %q", got)
	}
	if bodyReads.Load() != 0 {
		t.Fatalf("body should not be read during load, reads=%d", bodyReads.Load())
	}
	if metaReads.Load() == 0 {
		t.Fatalf("frontmatter should be read at startup")
	}

	res, err := regs[0].Handler.Handle(context.Background(), commands.Invocation{Args: []string{"world"}})
	if err != nil {
		t.Fatalf("handle failed: %v", err)
	}
	if res.Output != "hello world" {
		t.Fatalf("unexpected output: %v", res.Output)
	}
	if res.Metadata == nil || res.Metadata["argument-hint"] != "<name>" {
		t.Fatalf("metadata not propagated: %#v", res.Metadata)
	}
	if res.Metadata["source"] != path {
		t.Fatalf("source metadata missing: %#v", res.Metadata)
	}
	if bodyReads.Load() != 1 {
		t.Fatalf("body should be read on first handle, reads=%d", bodyReads.Load())
	}
}

func TestLazyLoadCacheAndReload(t *testing.T) {
	root := t.TempDir()
	path := writeCommandFile(t, root, "echo.md", "first $1")

	var bodyReads atomic.Int32
	restore := commands.SetCommandFileOpsForTest(
		func(p string) ([]byte, error) {
			bodyReads.Add(1)
			return os.ReadFile(p)
		},
		nil,
		nil,
	)
	t.Cleanup(restore)

	regs, errs := commands.LoadFromFS(commands.LoaderOptions{ProjectRoot: root})
	if len(errs) != 0 {
		t.Fatalf("load errors: %v", errs)
	}
	handler := regs[0].Handler

	res1, err := handler.Handle(context.Background(), commands.Invocation{Args: []string{"A"}})
	if err != nil {
		t.Fatalf("first handle failed: %v", err)
	}
	if res1.Output != "first A" {
		t.Fatalf("unexpected first output: %v", res1.Output)
	}
	if bodyReads.Load() != 1 {
		t.Fatalf("body should be read once, reads=%d", bodyReads.Load())
	}

	res2, err := handler.Handle(context.Background(), commands.Invocation{Args: []string{"B"}})
	if err != nil {
		t.Fatalf("second handle failed: %v", err)
	}
	if res2.Output != "first B" {
		t.Fatalf("unexpected cached output: %v", res2.Output)
	}
	if bodyReads.Load() != 1 {
		t.Fatalf("cache miss triggered extra IO, reads=%d", bodyReads.Load())
	}

	updated := "second $1"
	if err := os.WriteFile(path, []byte(updated), 0o600); err != nil {
		t.Fatalf("rewrite command: %v", err)
	}
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	res3, err := handler.Handle(context.Background(), commands.Invocation{Args: []string{"C"}})
	if err != nil {
		t.Fatalf("reload handle failed: %v", err)
	}
	if res3.Output != "second C" {
		t.Fatalf("expected reloaded body, got %v", res3.Output)
	}
	if bodyReads.Load() != 2 {
		t.Fatalf("reload should trigger one extra read, reads=%d", bodyReads.Load())
	}

	res4, err := handler.Handle(context.Background(), commands.Invocation{Args: []string{"D"}})
	if err != nil {
		t.Fatalf("post-reload handle failed: %v", err)
	}
	if res4.Output != "second D" {
		t.Fatalf("unexpected post-reload output: %v", res4.Output)
	}
	if bodyReads.Load() != 2 {
		t.Fatalf("cache should prevent duplicate reads after reload, reads=%d", bodyReads.Load())
	}
}

func TestLazyLoadConcurrentSingleRead(t *testing.T) {
	root := t.TempDir()
	writeCommandFile(t, root, "pong.md", "pong $1")

	var bodyReads atomic.Int32
	restore := commands.SetCommandFileOpsForTest(
		func(p string) ([]byte, error) {
			bodyReads.Add(1)
			time.Sleep(20 * time.Millisecond)
			return os.ReadFile(p)
		},
		nil,
		nil,
	)
	t.Cleanup(restore)

	regs, errs := commands.LoadFromFS(commands.LoaderOptions{ProjectRoot: root})
	if len(errs) != 0 {
		t.Fatalf("load errors: %v", errs)
	}
	handler := regs[0].Handler

	const workers = 8
	var wg sync.WaitGroup
	wg.Add(workers)
	errsCh := make(chan error, workers)

	for i := 0; i < workers; i++ {
		go func(i int) {
			defer wg.Done()
			res, err := handler.Handle(context.Background(), commands.Invocation{Args: []string{fmt.Sprintf("%d", i)}})
			if err != nil {
				errsCh <- fmt.Errorf("worker %d: %w", i, err)
				return
			}
			out, ok := res.Output.(string)
			if !ok || !strings.HasPrefix(out, "pong ") {
				errsCh <- fmt.Errorf("worker %d: bad output %v", i, res.Output)
			}
		}(i)
	}

	wg.Wait()
	close(errsCh)
	for err := range errsCh {
		t.Fatal(err)
	}

	if bodyReads.Load() != 1 {
		t.Fatalf("concurrent loads should read body once, reads=%d", bodyReads.Load())
	}
}

func writeCommandFile(t *testing.T, root, name, content string) string {
	t.Helper()
	dir := filepath.Join(root, ".claude", "commands")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write command: %v", err)
	}
	return path
}
