package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	acpserver "github.com/godeps/agentkit/pkg/acp"
	"github.com/godeps/agentkit/pkg/api"
	"github.com/godeps/agentkit/pkg/clikit"
	modelpkg "github.com/godeps/agentkit/pkg/model"
	"github.com/godeps/agentkit/pkg/sandbox/gvisorhelper"
	govmclient "github.com/godeps/govm/pkg/client"
	"github.com/google/uuid"
)

var serveACPStdio = acpserver.ServeStdio
var runtimeFactory = func(ctx context.Context, opts api.Options) (runtimeClient, error) {
	return api.New(ctx, opts)
}
var clikitRunStream = clikit.RunStream
var clikitRunREPL = clikit.RunREPL
var clikitRunInteractiveShell = clikit.RunInteractiveShell
var runGVisorHelper = gvisorhelper.Run
var validateGovmPlatform = defaultValidateGovmPlatform
var validateGovmRuntime = defaultValidateGovmRuntime

type runtimeClient interface {
	Run(context.Context, api.Request) (*api.Response, error)
	RunStream(context.Context, api.Request) (<-chan api.StreamEvent, error)
	Resume(context.Context, string) (*api.Response, error)
	Close() error
}

type streamEngine = clikit.StreamEngine
type replEngine = clikit.ReplEngine

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(argv []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("agentsdk-cli", flag.ContinueOnError)
	flags.SetOutput(stderr)

	entry := flags.String("entry", "cli", "Entry point type (cli/ci/platform)")
	project := flags.String("project", ".", "Project root")
	claudeDir := flags.String("claude", "", "Optional path to .claude directory")
	configRoot := flags.String("config-root", "", "Optional config root directory (defaults to <project>/.claude)")
	modelName := flags.String("model", "claude-3-5-sonnet-20241022", "Anthropic model name")
	systemPrompt := flags.String("system-prompt", "", "System prompt override")
	sessionID := flags.String("session", "", "Session identifier override")
	timeoutMs := flags.Int("timeout-ms", 10*60*1000, "Run timeout in milliseconds")
	printConfig := flags.Bool("print-effective-config", false, "Print resolved runtime config before running")
	printTimeline := flags.Bool("print-timeline", false, "Print unified runtime timeline after completion")
	promptFile := flags.String("prompt-file", "", "Read prompt from file (defaults to stdin/args)")
	promptLiteral := flags.String("prompt", "", "Prompt literal (overrides stdin)")
	resumeCheckpoint := flags.String("resume", "", "Resume a previously interrupted run by checkpoint ID")
	stream := flags.Bool("stream", false, "Stream events instead of waiting for completion")
	streamFormat := flags.String("stream-format", "json", "Streaming output format: json|rendered")
	repl := flags.Bool("repl", false, "Run interactive REPL mode")
	gvisorHelper := flags.Bool("agentkit-gvisor-helper", false, "Run hidden gVisor helper mode")
	sandboxBackend := flags.String("sandbox-backend", "host", "Sandbox backend: host|gvisor|govm")
	sandboxProjectMount := flags.String("sandbox-project-mount", "ro", "Project mount mode for virtualized sandbox: ro|rw|off")
	sandboxImage := flags.String("sandbox-image", "", "Offline image override for govm sandbox")
	verbose := flags.Bool("verbose", false, "Verbose stream diagnostics")
	waterfall := flags.String("waterfall", clikit.WaterfallModeFull, "Waterfall output mode: off|summary|full")
	skillsRecursive := flags.Bool("skills-recursive", true, "Discover SKILL.md recursively")
	acpMode := flags.Bool("acp", false, "Run ACP server over stdio")

	var mcpServers multiValue
	flags.Var(&mcpServers, "mcp", "Register an MCP server (repeatable)")
	var skillsDirs multiValue
	flags.Var(&skillsDirs, "skills-dir", "Additional skills directory (repeatable)")

	var tagFlags multiValue
	flags.Var(&tagFlags, "tag", "Attach tag key=value pairs (repeatable)")

	if err := flags.Parse(argv); err != nil {
		return err
	}
	if *gvisorHelper {
		return runGVisorHelper(context.Background(), os.Stdin, stdout, stderr)
	}
	if v := strings.TrimSpace(os.Getenv("AGENTSDK_TIMEOUT_MS")); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			*timeoutMs = parsed
		}
	}

	provider := &modelpkg.AnthropicProvider{
		ModelName: *modelName,
		System:    *systemPrompt,
	}
	selectedBackend := strings.ToLower(strings.TrimSpace(*sandboxBackend))
	resolvedSessionID := strings.TrimSpace(*sessionID)
	if selectedBackend != "" && selectedBackend != "host" && resolvedSessionID == "" {
		resolvedSessionID = uuid.NewString()
	}
	settingsPath := ""
	if strings.TrimSpace(*claudeDir) != "" {
		settingsPath = filepath.Join(*claudeDir, "settings.json")
	}
	finalConfigRoot := strings.TrimSpace(*configRoot)
	if finalConfigRoot == "" && strings.TrimSpace(*claudeDir) != "" {
		finalConfigRoot = *claudeDir
	}
	options := api.Options{
		EntryPoint:   api.EntryPoint(strings.ToLower(strings.TrimSpace(*entry))),
		ProjectRoot:  *project,
		ConfigRoot:   finalConfigRoot,
		SettingsPath: settingsPath,
		ModelFactory: provider,
		MCPServers:   mcpServers,
		SkillsDirs:   append([]string(nil), skillsDirs...),
		SkillsRecursive: func() *bool {
			v := *skillsRecursive
			return &v
		}(),
	}
	sandboxOpts, err := buildSandboxOptions(*project, selectedBackend, *sandboxProjectMount, *sandboxImage)
	if err != nil {
		return err
	}
	options.Sandbox = sandboxOpts
	if selectedBackend == "govm" {
		if err := validateGovmPlatform(); err != nil {
			return err
		}
		if options.Sandbox.Govm == nil {
			return errors.New("govm sandbox configuration is missing")
		}
		if err := validateGovmRuntime(*options.Sandbox.Govm); err != nil {
			if isGovmNativeUnavailable(err) {
				return fmt.Errorf("govm native runtime unavailable: build with -tags govm_native and ensure bundled native assets are present")
			}
			return fmt.Errorf("govm runtime preflight failed: %w", err)
		}
	}
	if *acpMode {
		return serveACPStdio(context.Background(), options, os.Stdin, stdout)
	}
	recorder := clikitTurnRecorder()
	options.Middleware = append(options.Middleware, clikit.TurnRecorderMiddleware(recorder))
	if *printConfig {
		clikit.PrintEffectiveConfig(stderr, options.ProjectRoot, clikit.EffectiveConfig{
			ModelName:       *modelName,
			ConfigRoot:      finalConfigRoot,
			SkillsDirs:      append([]string(nil), skillsDirs...),
			SkillsRecursive: options.SkillsRecursive,
		}, *timeoutMs)
	}

	runtime, err := runtimeFactory(context.Background(), options)
	if err != nil {
		return fmt.Errorf("create runtime: %w", err)
	}
	defer runtime.Close()
	adapter := clikit.NewRuntimeAdapter(runtime, clikit.RuntimeAdapterConfig{
		ProjectRoot:     options.ProjectRoot,
		ConfigRoot:      finalConfigRoot,
		ModelName:       *modelName,
		SandboxBackend:  selectedBackend,
		SkillsDirs:      append([]string(nil), skillsDirs...),
		SkillsRecursive: options.SkillsRecursive,
		TurnRecorder:    recorder,
	})
	if *printConfig {
		clikit.PrintRuntimeEffectiveConfig(stderr, adapter, *timeoutMs)
	}

	resumeID := strings.TrimSpace(*resumeCheckpoint)
	if *repl || shouldAutoEnterInteractive(*promptLiteral, *promptFile, flags.Args(), *stream, *acpMode, resumeID) {
		clikit.PrintBanner(stdout, adapter.ModelName(), adapter.Skills())
		return clikitRunInteractiveShell(context.Background(), os.Stdin, stdout, stderr, adapter, *timeoutMs, *verbose, *waterfall, resolvedSessionID)
	}

	ctx := context.Background()
	cancel := func() {}
	if *timeoutMs > 0 {
		ctxWithTimeout, c := context.WithTimeout(ctx, time.Duration(*timeoutMs)*time.Millisecond)
		ctx = ctxWithTimeout
		cancel = c
	}
	defer cancel()

	req := api.Request{
		SessionID: resolvedSessionID,
		Mode: api.ModeContext{
			EntryPoint: options.EntryPoint,
			CLI: &api.CLIContext{
				User:      os.Getenv("USER"),
				Workspace: *project,
				Args:      argv,
			},
		},
		Tags: parseTags(tagFlags),
	}
	if resumeID == "" {
		prompt, err := resolvePrompt(*promptLiteral, *promptFile, flags.Args())
		if err != nil {
			return err
		}
		if strings.TrimSpace(prompt) == "" {
			return errors.New("prompt is empty")
		}
		req.Prompt = prompt
	}
	if *stream {
		switch strings.ToLower(strings.TrimSpace(*streamFormat)) {
		case "", "json":
			return streamRunJSON(ctx, runtime, req, stdout, stderr, *verbose)
		case "rendered", "human", "pretty":
			return clikitRunStream(ctx, stdout, stderr, adapter, req, *timeoutMs, *verbose, *waterfall)
		default:
			return fmt.Errorf("unsupported stream format %q", *streamFormat)
		}
	}
	var resp *api.Response
	if resumeID != "" {
		resp, err = runtime.Resume(ctx, resumeID)
		if err != nil {
			return fmt.Errorf("resume failed for checkpoint %s: %w", resumeID, err)
		}
	} else {
		resp, err = runtime.Run(ctx, req)
		if err != nil {
			return err
		}
	}
	printResponse(resp, stdout, *printTimeline)
	return nil
}

func buildSandboxOptions(projectRoot, backend, projectMountMode, offlineImage string) (api.SandboxOptions, error) {
	projectRoot = strings.TrimSpace(projectRoot)
	if projectRoot == "" {
		projectRoot = "."
	}
	absProjectRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return api.SandboxOptions{}, fmt.Errorf("resolve project root: %w", err)
	}
	projectRoot = absProjectRoot
	backend = strings.ToLower(strings.TrimSpace(backend))
	if backend == "" {
		backend = "host"
	}
	projectMountMode = strings.ToLower(strings.TrimSpace(projectMountMode))
	if projectMountMode == "" {
		projectMountMode = "ro"
	}
	switch projectMountMode {
	case "ro", "rw", "off":
	default:
		return api.SandboxOptions{}, fmt.Errorf("invalid --sandbox-project-mount %q (expected ro|rw|off)", projectMountMode)
	}

	switch backend {
	case "host":
		return api.SandboxOptions{}, nil
	case "gvisor":
		opts := api.SandboxOptions{
			Type: "gvisor",
			GVisor: &api.GVisorOptions{
				Enabled:                    true,
				DefaultGuestCwd:            "/workspace",
				AutoCreateSessionWorkspace: true,
				SessionWorkspaceBase:       filepath.Join(projectRoot, "workspace"),
			},
		}
		if projectMountMode != "off" {
			opts.GVisor.Mounts = append(opts.GVisor.Mounts, api.MountSpec{
				HostPath:  projectRoot,
				GuestPath: "/project",
				ReadOnly:  projectMountMode != "rw",
			})
		}
		return opts, nil
	case "govm":
		if strings.TrimSpace(offlineImage) == "" {
			offlineImage = "py312-alpine"
		}
		opts := api.SandboxOptions{
			Type: "govm",
			Govm: &api.GovmOptions{
				Enabled:                    true,
				DefaultGuestCwd:            "/workspace",
				AutoCreateSessionWorkspace: true,
				SessionWorkspaceBase:       filepath.Join(projectRoot, "workspace"),
				RuntimeHome:                filepath.Join(projectRoot, ".govm"),
				OfflineImage:               offlineImage,
			},
		}
		if projectMountMode != "off" {
			opts.Govm.Mounts = append(opts.Govm.Mounts, api.MountSpec{
				HostPath:  projectRoot,
				GuestPath: "/project",
				ReadOnly:  projectMountMode != "rw",
			})
		}
		return opts, nil
	default:
		return api.SandboxOptions{}, fmt.Errorf("invalid --sandbox-backend %q (expected host|gvisor|govm)", backend)
	}
}

func defaultValidateGovmPlatform() error {
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "linux/amd64", "linux/arm64", "darwin/arm64":
		return nil
	default:
		return fmt.Errorf("govm requires linux/amd64, linux/arm64, or darwin/arm64; current platform is %s/%s", runtime.GOOS, runtime.GOARCH)
	}
}

func defaultValidateGovmRuntime(opts api.GovmOptions) error {
	rt, err := govmclient.NewRuntime(&govmclient.RuntimeOptions{HomeDir: opts.RuntimeHome})
	if err != nil {
		return err
	}
	rt.Close()
	return nil
}

func isGovmNativeUnavailable(err error) bool {
	return errors.Is(err, govmclient.ErrNativeUnavailable) || strings.Contains(strings.ToLower(err.Error()), "govm native bridge unavailable")
}

func resolvePrompt(literal, file string, tail []string) (string, error) {
	if strings.TrimSpace(literal) != "" {
		return literal, nil
	}
	if strings.TrimSpace(file) != "" {
		data, err := os.ReadFile(file)
		if err != nil {
			return "", fmt.Errorf("read prompt file: %w", err)
		}
		return string(data), nil
	}
	if len(tail) > 0 {
		return strings.Join(tail, " "), nil
	}
	info, err := os.Stdin.Stat()
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeCharDevice != 0 {
		return "", errors.New("no prompt provided")
	}
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 4096), 1024*1024)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return strings.Join(lines, "\n"), nil
}

func shouldAutoEnterInteractive(literal, file string, tail []string, stream, acp bool, resumeID string) bool {
	if acp || stream {
		return false
	}
	if strings.TrimSpace(resumeID) != "" {
		return false
	}
	if strings.TrimSpace(literal) != "" || strings.TrimSpace(file) != "" || len(tail) > 0 {
		return false
	}
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func printResponse(resp *api.Response, out io.Writer, printTimeline bool) {
	if resp == nil || out == nil {
		return
	}
	fmt.Fprintf(out, "# agentsdk run (%s)\n", resp.Mode.EntryPoint)
	if resp.Result != nil {
		fmt.Fprintf(out, "stop_reason: %s\n", resp.Result.StopReason)
		if resp.Result.Interrupted {
			fmt.Fprintln(out, "interrupted: true")
		}
		if checkpointID := strings.TrimSpace(resp.Result.CheckpointID); checkpointID != "" {
			fmt.Fprintf(out, "checkpoint_id: %s\n", checkpointID)
			if resp.Result.Interrupted {
				fmt.Fprintf(out, "next: agentkit --resume %s\n", checkpointID)
			}
		}
		fmt.Fprintf(out, "output:\n%s\n", resp.Result.Output)
	}
	if printTimeline {
		printTimelineEntries(out, resp.Timeline)
	}
}

func printTimelineEntries(out io.Writer, timeline []api.TimelineEntry) {
	if out == nil {
		return
	}
	fmt.Fprintln(out, "timeline:")
	if len(timeline) == 0 {
		fmt.Fprintln(out, "0. none")
		return
	}
	for i, entry := range timeline {
		if strings.TrimSpace(entry.Source) != "" {
			fmt.Fprintf(out, "%d. %s source=%s\n", i+1, entry.Kind, entry.Source)
			continue
		}
		fmt.Fprintf(out, "%d. %s\n", i+1, entry.Kind)
	}
}

func streamRunJSON(ctx context.Context, rt runtimeClient, req api.Request, out, errOut io.Writer, verbose bool) error {
	ch, err := rt.RunStream(ctx, req)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(out)
	var streamErr error
	for evt := range ch {
		if verbose && errOut != nil {
			switch evt.Type {
			case api.EventToolExecutionResult, api.EventMessageStop, api.EventError:
				_, _ = fmt.Fprintf(errOut, "[event] %s\n", evt.Type)
			}
		}
		if err := encoder.Encode(evt); err != nil {
			return err
		}
		if evt.Type == api.EventError && streamErr == nil {
			streamErr = fmt.Errorf("stream failed: %v", evt.Output)
		}
	}
	return streamErr
}

type multiValue []string

func (m *multiValue) String() string {
	return strings.Join(*m, ",")
}

func (m *multiValue) Set(value string) error {
	*m = append(*m, value)
	return nil
}

func parseTags(values multiValue) map[string]string {
	if len(values) == 0 {
		return nil
	}
	tags := map[string]string{}
	for _, value := range values {
		parts := strings.SplitN(value, "=", 2)
		key := strings.TrimSpace(parts[0])
		if key == "" {
			continue
		}
		val := "true"
		if len(parts) == 2 {
			val = strings.TrimSpace(parts[1])
		}
		tags[key] = val
	}
	return tags
}

func clikitTurnRecorder() *clikit.TurnRecorder {
	return clikit.NewTurnRecorder()
}
