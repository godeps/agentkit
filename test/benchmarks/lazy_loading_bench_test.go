package benchmarks

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/godeps/agentkit/pkg/api"
	"github.com/godeps/agentkit/pkg/config"
	"github.com/godeps/agentkit/pkg/model"
	"github.com/godeps/agentkit/pkg/runtime/commands"
	"github.com/godeps/agentkit/pkg/runtime/skills"
)

const iterationsPerBenchmark = 100

type noopModel struct{}

func (noopModel) Complete(context.Context, model.Request) (*model.Response, error) {
	return &model.Response{
		Message: model.Message{Role: "assistant"},
	}, nil
}

func (noopModel) CompleteStream(ctx context.Context, req model.Request, cb model.StreamHandler) error {
	resp, err := noopModel{}.Complete(ctx, req)
	if err != nil {
		return err
	}
	if cb != nil {
		return cb(model.StreamResult{Final: true, Response: resp})
	}
	return nil
}

func BenchmarkSkillsLazyLoading(b *testing.B) {
	root := b.TempDir()
	writeSkillFile(b, root, "lazy", "lazy body")

	var reads atomic.Int64
	restore := skills.SetReadFileForTest(func(path string) ([]byte, error) {
		reads.Add(1)
		return os.ReadFile(path)
	})
	b.Cleanup(restore)

	opts := skills.LoaderOptions{
		ProjectRoot: root,
		EnableUser:  false,
	}

	var startupReads int64
	var execReads int64

	b.ReportAllocs()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		for i := 0; i < iterationsPerBenchmark; i++ {
			reads.Store(0)

			regs, errs := skills.LoadFromFS(opts)
			if len(errs) != 0 {
				b.Fatalf("skills load: %v", errs)
			}
			if len(regs) == 0 {
				b.Fatal("no skills loaded")
			}

			afterLoad := reads.Load()
			startupReads += afterLoad

			if _, err := regs[0].Handler.Execute(context.Background(), skills.ActivationContext{}); err != nil {
				b.Fatalf("skill execute: %v", err)
			}
			execReads += reads.Load() - afterLoad
		}
	}
	b.StopTimer()

	ops := float64(b.N * iterationsPerBenchmark)
	b.ReportMetric(float64(startupReads)/ops, "startup-read/op")
	b.ReportMetric(float64(execReads)/ops, "body-read/op")
}

func BenchmarkCommandsLazyLoading(b *testing.B) {
	root := b.TempDir()
	writeCommandFile(b, root, "hello", "hello $ARGUMENTS")

	var bodyReads atomic.Int64
	var metaReads atomic.Int64
	var statCalls atomic.Int64

	restore := commands.SetCommandFileOpsForTest(
		func(path string) ([]byte, error) {
			bodyReads.Add(1)
			return os.ReadFile(path)
		},
		func(path string) (*os.File, error) {
			metaReads.Add(1)
			return os.Open(path)
		},
		func(path string) (fs.FileInfo, error) {
			statCalls.Add(1)
			return os.Stat(path)
		},
	)
	b.Cleanup(restore)

	opts := commands.LoaderOptions{
		ProjectRoot: root,
		EnableUser:  false,
	}

	var startupBody int64
	var startupMeta int64
	var execBody int64
	var execStat int64

	b.ReportAllocs()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		for i := 0; i < iterationsPerBenchmark; i++ {
			bodyReads.Store(0)
			metaReads.Store(0)
			statCalls.Store(0)

			regs, errs := commands.LoadFromFS(opts)
			if len(errs) != 0 {
				b.Fatalf("commands load: %v", errs)
			}
			if len(regs) == 0 {
				b.Fatal("no commands loaded")
			}

			bodyAfterLoad := bodyReads.Load()
			metaAfterLoad := metaReads.Load()
			statAfterLoad := statCalls.Load()
			startupBody += bodyAfterLoad
			startupMeta += metaAfterLoad

			if _, err := regs[0].Handler.Handle(context.Background(), commands.Invocation{Args: []string{"world"}}); err != nil {
				b.Fatalf("command handle: %v", err)
			}

			execBody += bodyReads.Load() - bodyAfterLoad
			execStat += statCalls.Load() - statAfterLoad
		}
	}
	b.StopTimer()

	ops := float64(b.N * iterationsPerBenchmark)
	b.ReportMetric(float64(startupBody)/ops, "startup-body-read/op")
	b.ReportMetric(float64(startupMeta)/ops, "startup-meta-read/op")
	b.ReportMetric(float64(execBody)/ops, "body-read/op")
	b.ReportMetric(float64(execStat)/ops, "stat/op")
}

func BenchmarkRuntimeStartup(b *testing.B) {
	root := b.TempDir()
	b.Setenv("HOME", root)
	writeSettingsFile(b, root)
	writeSkillFile(b, root, "lazy", "lazy body")
	writeCommandFile(b, root, "hello", "hello $ARGUMENTS")

	opts := api.Options{
		EntryPoint:  api.EntryPointCLI,
		ProjectRoot: root,
		Model:       noopModel{},
		SettingsLoader: &config.SettingsLoader{
			ProjectRoot: root,
		},
		Sandbox: api.SandboxOptions{
			Root: root,
		},
	}

	b.ReportAllocs()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		for i := 0; i < iterationsPerBenchmark; i++ {
			rt, err := api.New(context.Background(), opts)
			if err != nil {
				b.Fatalf("runtime init: %v", err)
			}
			_ = rt.Close()
		}
	}
}

func writeSkillFile(b *testing.B, root, name, body string) {
	b.Helper()
	path := filepath.Join(root, ".claude", "skills", name, "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		b.Fatalf("mkdir skills: %v", err)
	}
	content := fmt.Sprintf("---\nname: %s\ndescription: %s\n---\n%s", name, name+" skill", body)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		b.Fatalf("write skill: %v", err)
	}
}

func writeCommandFile(b *testing.B, root, name, body string) {
	b.Helper()
	path := filepath.Join(root, ".claude", "commands", name+".md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		b.Fatalf("mkdir commands: %v", err)
	}
	content := fmt.Sprintf("---\ndescription: %s command\n---\n%s", name, body)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		b.Fatalf("write command: %v", err)
	}
}

func writeSettingsFile(b *testing.B, root string) {
	b.Helper()
	path := filepath.Join(root, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		b.Fatalf("mkdir settings: %v", err)
	}
	const settings = `{"sandbox":{"enabled":false}}`
	if err := os.WriteFile(path, []byte(settings), 0o600); err != nil {
		b.Fatalf("write settings: %v", err)
	}
}
