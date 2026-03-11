# Go Agent Projects Comparison

Date: 2026-03-11

## Scope

This document compares `agentkit` with six similar open-source Go projects based on their current source trees fetched into `other/`.

Compared projects:

- `agentkit` (`.`)
- `langchaingo` (`other/langchaingo-src`)
- `eino` (`other/eino`)
- `AgenticGoKit` (`other/AgenticGoKit`)
- `agent-sdk-go` (`other/agent-sdk-go-src`)
- `go-agent` (`other/go-agent`)
- `go-agent-framework` (`other/go-agent-framework`)

Notes:

- `langchaingo` and `agent-sdk-go` were fetched as current GitHub tarball snapshots into `other/langchaingo-src` and `other/agent-sdk-go-src` because direct `git clone` stalled on network transfer.
- Findings below are source-based. When a capability is marked "not found", that means no dedicated package or code evidence was found in the fetched snapshot; it is an inference from current source layout, not a claim about the whole ecosystem forever.

## Method

The comparison is based on:

- `go.mod` module and Go version
- top-level package layout
- direct code evidence for `MCP`, `sandbox/security`, `hooks/callbacks`, `subagents/multi-agent`, `memory`, `workflow/task`, `observability/tracing`, and `structured output`
- simple maturity signals: number of Go files, test files, and example directories

## Quick Ranking

If the question is "which project is most similar to `agentkit` as a production-oriented Go agent runtime", the ordering from closest to farthest is:

1. `agent-sdk-go`
2. `AgenticGoKit`
3. `go-agent`
4. `eino`
5. `go-agent-framework`
6. `langchaingo`

Reason:

- `agent-sdk-go` overlaps the most on runtime concerns: MCP, memory, orchestration, tasks, structured output, tracing, guardrails, subagents.
- `AgenticGoKit` overlaps on multi-agent orchestration, MCP, memory, streaming, observability, but is less Claude Code-style and less sandbox-centric.
- `go-agent` overlaps on subagents, swarm, memory, tool catalogs, and orchestration, but does not expose the same broad runtime surface.
- `eino` is strong and mature, but architecturally closer to an ADK/graph framework than a Claude Code-style runtime.
- `go-agent-framework` is mainly a typed workflow kernel with MCP adapters, not a full agent runtime.
- `langchaingo` is the most general LLM app toolkit of the set; it has agents, tools, memory, and callbacks, but not the same runtime envelope.

## Comparison Matrix

| Project | Module | Go | Source shape | MCP | Sandbox / security isolation | Hooks / callbacks | Multi-agent / subagents | Memory | Workflow / task system | Observability | Structured output | Closest reading vs `agentkit` |
|---|---|---:|---|---|---|---|---|---|---|---|---|---|
| `agentkit` | `github.com/godeps/agentkit` | 1.24.0 | Full runtime SDK with `pkg/api`, `pkg/agent`, `pkg/mcp`, `pkg/sandbox`, `pkg/runtime/*`, `pkg/tool`, `pkg/security` | Yes. `pkg/mcp/mcp.go` | Yes. `pkg/sandbox/*`, `pkg/security/*` | Yes. `pkg/core/hooks/*`, `pkg/middleware/*` | Yes. `pkg/runtime/subagents/*` | Session/message oriented. `pkg/message/*` | Yes. `pkg/runtime/tasks/*`, commands, async bash | Yes. `pkg/api/otel.go` with build tags | Yes. `pkg/model/interface.go`, `pkg/api/agent_test.go` | Baseline |
| `agent-sdk-go` | `github.com/Ingenimax/agent-sdk-go` | 1.24.4 | Broad SDK with `pkg/agent`, `pkg/mcp`, `pkg/memory`, `pkg/orchestration`, `pkg/task`, `pkg/workflow`, `pkg/structuredoutput`, `pkg/guardrails`, `pkg/tracing` | Yes. `pkg/mcp/*` | No dedicated sandbox package found; stronger on guardrails than isolation | No runtime hook layer found comparable to `agentkit`; mostly docs/examples and app-side callbacks | Yes. `examples/subagents`, docs around sub-agent streaming, orchestration | Yes. `pkg/memory/*` | Yes. `pkg/task/*`, `pkg/workflow/*`, `pkg/executionplan/*` | Yes. `pkg/tracing/*`, OTEL in `go.mod` | Yes. `pkg/structuredoutput/*` | Closest overall alternative |
| `AgenticGoKit` | `github.com/agenticgokit/agenticgokit` | 1.24.1 | Plugin-heavy framework with `internal/agents`, `internal/mcp`, `internal/memory`, `internal/orchestrator`, `internal/observability`, `plugins/*`, `v1beta/*` | Yes. `plugins/mcp/default/default.go`, `core/mcp.go` | Claimed in docs, but no dedicated isolation package like `pkg/sandbox` found in current tree | Internal callbacks and streaming handlers exist; not as explicit as `agentkit` lifecycle hooks | Yes. `v1beta/workflow.go`, builder subworkflow tests, orchestrator packages | Yes. `internal/memory/*`, memory plugins | Yes. Strong workflow/orchestrator emphasis in `v1beta/*` and `internal/orchestrator/*` | Yes. `internal/observability/*`, OTEL tests | Partial to moderate. Tool schemas and structured result mentions exist, but less cleanly centralized than `agentkit`/`agent-sdk-go` | Strong multi-agent orchestration framework |
| `go-agent` | `github.com/Protocol-Lattice/go-agent` | 1.25.0 | Leaner runtime with root package plus `src/memory`, `src/subagents`, `src/swarm`, model adapters | Indirect dependency only in current `go.mod`; no first-class MCP package found | No dedicated sandbox/isolation package found | No explicit hook framework found | Yes. `src/subagents`, `src/swarm`, `agent_tool.go`, orchestrator files | Yes. `src/memory/*`, checkpoint tests | Moderate. Has orchestration and tool chains, but not a broad task framework | Indirect OTEL deps only; observability is not a first-class surface in current code layout | Tool input/output schemas exist in tool layer | Good lightweight multi-agent runtime, narrower surface |
| `eino` | `github.com/cloudwego/eino` | 1.18 | ADK/graph framework with `adk`, `compose`, `flow`, `callbacks`, `components/*` | No dedicated MCP package found in current snapshot | Some sandbox-oriented prompt/middleware content in `adk/middlewares/filesystem`, but not a general runtime isolation subsystem | Yes. `callbacks/*`, callback handler templates | Yes. `flow/agent/multiagent/host/*`, supervisor/sub-agent patterns | Checkpoint/state store present in graph runtime; less conversation-memory-centric than agent SDKs | Yes. This is one of its strongest areas: `compose`, `flow`, graph compilation, checkpointing | Callback-based tracing/observability hooks appear, but no dedicated OTEL package surfaced | Schema types are pervasive, but structured output is less of a dedicated product surface than in `agentkit` | Strong graph/ADK framework, not Claude Code-style runtime |
| `go-agent-framework` | `github.com/stephanoumenos/go-agent-framework` | 1.23.4 | Small typed workflow engine with `workflow.go`, `nodes/*`, `store/*`, `mcp/*` | Yes. `mcp/adapter.go` | No sandbox/isolation subsystem found | No lifecycle hook framework; only incidental test cleanup hooks | No dedicated multi-agent system found | Workflow-state persistence via store, not agent memory in the conversational sense | Yes. This is the core product: workflows, node execution, persistence | Minimal; storage/traces mainly for execution introspection | Yes. `nodes/openai/middleware/structuredoutput.go` | Workflow kernel with MCP adapters |
| `langchaingo` | `github.com/tmc/langchaingo` | 1.24.4 | Broad LLM toolkit with `agents`, `chains`, `callbacks`, `memory`, `tools`, vectorstores, provider adapters | No dedicated MCP package found in current snapshot | No sandbox/isolation subsystem found | Yes. `callbacks/*` | Only limited agent orchestration evidence in current tree; no dedicated subagent runtime found | Yes. `memory/*` | Chains and composable workflows exist, but no task runtime analogous to `agentkit` | No first-class OTEL surface found in current tree | Provider/tool schemas exist, but no centralized structured-output runtime surface like `agentkit` | General-purpose LLM toolkit, not a full agent runtime |

## Code Evidence By Project

### `agentkit`

Key evidence:

- Runtime surface is explicit in package layout: `pkg/mcp`, `pkg/sandbox`, `pkg/runtime/skills`, `pkg/runtime/subagents`, `pkg/runtime/tasks`, `pkg/security`.
- Hooks are first-class: `pkg/core/hooks/executor.go`, `pkg/core/hooks/lifecycle.go`.
- MCP is first-class: `pkg/mcp/mcp.go`.
- Sandbox and policy isolation are first-class: `pkg/sandbox/interface.go`, `pkg/sandbox/fs_policy.go`, `pkg/sandbox/net_policy.go`.
- Structured output is explicit in model API: `pkg/model/interface.go`.
- OpenTelemetry is implemented behind build tags: `pkg/api/otel.go`, `pkg/api/otel_noop.go`.

Interpretation:

- `agentkit` is unusual in Go because it combines agent loop, tools, MCP, sandboxing, subagents, commands/tasks, and middleware in one runtime.
- This makes it look less like a plain "AI app framework" and more like an embeddable Claude Code-style runtime kernel.

### `agent-sdk-go`

Key evidence:

- Breadth of runtime packages is the closest match in this set: `pkg/agent`, `pkg/mcp`, `pkg/memory`, `pkg/orchestration`, `pkg/task`, `pkg/workflow`, `pkg/structuredoutput`, `pkg/guardrails`, `pkg/tracing`.
- The module depends directly on `github.com/modelcontextprotocol/go-sdk` and OTEL packages in `other/agent-sdk-go-src/go.mod`.
- Examples are broad and production-oriented: `examples/mcp`, `examples/memory`, `examples/subagents`, `examples/guardrails`, `examples/structured_output`, `examples/tracing`.

Interpretation:

- This is the nearest peer to `agentkit`.
- Main difference: it appears stronger on guardrails, enterprise integrations, and broad platform features, while `agentkit` is stronger on Claude Code-style runtime semantics such as sandbox, hooks, rules, skills, and command/task ergonomics.
- I did not find a dedicated sandbox subsystem comparable to `pkg/sandbox` in `agentkit`.

### `AgenticGoKit`

Key evidence:

- Internal architecture is centered on agents, MCP, memory, orchestration, observability, and plugins: `internal/agents`, `internal/mcp`, `internal/memory`, `internal/orchestrator`, `internal/observability`, `plugins/*`.
- Multi-agent workflow orchestration is explicit in `v1beta/workflow.go`.
- MCP is implemented through plugin and core layers: `plugins/mcp/default/default.go`, `core/mcp.go`.
- OTEL and tracing are actively tested: `test/internal/llm/openai_instrumentation_test.go`, `test/internal/agents/builder_instrumentation_test.go`.

Interpretation:

- This project overlaps heavily with `agentkit` on orchestration, memory, streaming, and observability.
- It is less "runtime-kernel-like" than `agentkit`; its plugin and builder orientation pushes it toward app framework territory.
- I did not find a source-level sandbox/isolation module comparable to `pkg/sandbox`.

### `go-agent`

Key evidence:

- Dedicated subagent and swarm packages exist: `src/subagents`, `src/swarm`.
- The root package exposes a catalog and directory model for tools and subagents: `catalog.go`, `agent_tool.go`, `agent_orchestrators.go`.
- Memory is first-class: `src/memory/*`, plus checkpoint tests in `agent_checkpoint_test.go`.
- Tool schemas are explicit in code: `types.go`, `agent_tool.go`.

Interpretation:

- `go-agent` is closer to a lean multi-agent runtime than to a full agent platform.
- It overlaps with `agentkit` on subagents, orchestration, and memory, but I did not find comparable first-class implementations for hooks, sandboxing, commands/tasks, or MCP runtime integration.
- MCP appears only as an indirect dependency in the current module graph.

### `eino`

Key evidence:

- The strongest surface is graph/compose/ADK: `compose/*`, `flow/*`, `adk/*`, `components/*`.
- Callbacks are first-class and fine-grained: `callbacks/*`, `utils/callbacks/template.go`.
- Multi-agent host pattern exists explicitly: `flow/agent/multiagent/host/*`.
- State/checkpoint behavior appears in graph runtime tests such as `compose/resume_test.go`.

Interpretation:

- `eino` is a serious Go framework, but it solves a somewhat different problem.
- It is strongest when you want graph composition, agents as graph nodes, callback instrumentation, and checkpointable execution.
- Compared with `agentkit`, it is less centered on "embedded coding-agent runtime" concerns like sandbox, slash-command routing, skills, and session/task semantics.

### `go-agent-framework`

Key evidence:

- Core abstraction is workflows and nodes: `workflow.go`, `node_execution.go`, `core.go`.
- Persistence is part of workflow execution: `store/*`, `nodestate.go`.
- MCP integration exists as an adapter that turns workflows into MCP tools: `mcp/adapter.go`.
- Structured output exists in OpenAI middleware: `nodes/openai/middleware/structuredoutput.go`.

Interpretation:

- This is a compact workflow execution framework, not a broad agent runtime.
- It is useful as a typed workflow kernel or as a building block if someone wants to expose workflow definitions as MCP tools.
- It is significantly narrower than `agentkit`.

### `langchaingo`

Key evidence:

- Large general toolkit layout: `agents`, `chains`, `callbacks`, `memory`, `tools`.
- Callbacks are explicit: `callbacks/callbacks.go`.
- Agents and tools are explicit: `agents/*`, `tools/*`.
- Memory is explicit: `memory/*`.

Interpretation:

- `langchaingo` is the broadest LLM application toolkit in the set, but not the closest runtime match.
- It overlaps on agent/tool/memory primitives, but I did not find a dedicated MCP layer, sandbox subsystem, subagent runtime, or built-in command/task runtime comparable to `agentkit`.
- It is best viewed as a general library layer, not a Claude Code-style runtime.

## Maturity Signals

These are rough source-tree signals only.

| Project | Go files | Test files | Example dirs |
|---|---:|---:|---:|
| `agentkit` | 2630 | 834 | 12 |
| `langchaingo` | 671 | 226 | 77 |
| `eino` | 285 | 103 | 0 |
| `AgenticGoKit` | 308 | 74 | 27 |
| `agent-sdk-go` | 338 | 67 | 38 |
| `go-agent` | 101 | 28 | 0 |
| `go-agent-framework` | 30 | 8 | 4 |

Interpretation:

- `langchaingo` has the broadest example surface.
- `agentkit` has the heaviest local source and test footprint among the compared trees in this workspace.
- `eino`, `AgenticGoKit`, and `agent-sdk-go` sit in the middle: substantial enough to be serious, but each optimized for a different abstraction level.
- `go-agent-framework` is clearly the smallest and most specialized.

## Bottom-Line Recommendations

### If you want the nearest benchmark for `agentkit`

Use `agent-sdk-go` first, then `AgenticGoKit`.

Why:

- Both cover a large part of the same runtime problem space.
- `agent-sdk-go` is the best direct comparison for SDK surface and production features.
- `AgenticGoKit` is the best comparison for multi-agent orchestration and observability.

### If you want the strongest architectural contrast

Use `eino` and `langchaingo`.

Why:

- `eino` shows how the problem looks when graph composition is the center.
- `langchaingo` shows how the problem looks when a general LLM toolkit is the center.

### If you want smaller focused references

Use `go-agent` and `go-agent-framework`.

Why:

- `go-agent` is useful for studying lean subagent/swarm and memory patterns.
- `go-agent-framework` is useful for studying typed workflow and MCP adapter design with less runtime surface area.

## API Design-Level Delta Table

This section compares the projects at the API and integration-surface level rather than only by package topology.

| Surface | `agentkit` | `agent-sdk-go` | `AgenticGoKit` | `go-agent` | `eino` | `go-agent-framework` | `langchaingo` |
|---|---|---|---|---|---|---|---|
| Primary entrypoint shape | Runtime-first SDK: `api.New(...)`, then `Run` / `RunStream` | SDK-first with multiple feature packages and YAML/config-driven composition | Builder/plugin-oriented APIs plus `v1beta` workflow surface | Root agent type plus supporting tool/subagent abstractions | ADK + graph/compose APIs | Workflow definition and execution functions | Library-style package imports by capability |
| Session model | Explicit `SessionID`, per-session concurrency rules, history management | Memory and task oriented; less obviously centered on a single runtime session primitive | Workflow/run oriented, streaming-first | Agent plus memory session patterns | Graph execution and checkpoint store | Workflow execution context and persisted node state | Memory buffers and chain/agent context objects |
| Tool abstraction | Runtime tool registry plus built-in tools and MCP tools in `pkg/tool` | `pkg/tools`, MCP, subagents, structured task execution | Internal tool registry plus MCP tool discovery in `v1beta/tool_discovery.go` | Tool catalog and typed tool schemas in `types.go` | Tool nodes/components inside graph/ADK | Workflow-to-MCP adapter surface | Tool packages and agent executors |
| MCP integration style | First-class runtime subsystem that bridges external tool servers into agent runtime | First-class SDK subsystem in `pkg/mcp` | Plugin/core based MCP manager and discovery model | No first-class MCP subsystem found in current tree | No dedicated MCP subsystem found in current tree | Adapter that exposes workflows as MCP tools | No dedicated MCP subsystem found in current tree |
| Hook / callback model | Explicit lifecycle hooks plus middleware interception points | No `agentkit`-style lifecycle hook framework found | Internal callbacks and streaming handlers | Minimal callback use, not a lifecycle framework | Rich callback handler APIs across components and graphs | No lifecycle hook layer | Callback handler package used across chains/agents/tools |
| Multi-agent API shape | Runtime subagents with dispatch context and task integration | Subagent examples plus orchestration packages | Multi-agent workflows are a central feature | Subagents and swarm are core concepts | Multi-agent host/supervisor patterns inside graph system | Not a primary abstraction | Agent executors exist, but no dedicated subagent runtime found |
| Task / workflow abstraction | Separate runtime task subsystem and command/task semantics | Task, workflow, execution-plan packages | Workflow/orchestrator centric | Orchestration and chain decisions, but lighter task model | Graph compilation and checkpointable flows | Core product is workflow execution | Chains and agents, not a task runtime |
| Structured output API | Explicit provider-agnostic `ResponseFormat` / `OutputSchema` surface | Dedicated `pkg/structuredoutput` package | Present, but less centralized | Tool schemas present; agent-wide response-format surface is weaker | Schema-heavy internals, not an obvious dedicated response-format API | OpenAI structured-output middleware | Provider/tool schema support, less centralized as a runtime feature |
| Security / isolation boundary | Strongest in set: dedicated sandbox and security packages | Guardrails present; sandbox isolation not found | Permissions/sandboxing referenced in docs, but no dedicated isolation package found | No dedicated isolation package found | Filesystem middleware hints, not a generalized runtime isolation subsystem | No dedicated isolation package found | No dedicated isolation package found |
| Observability API shape | Built-in OTEL tracer with build-tag split, request/tool/model spans | Dedicated tracing package and OTEL dependencies | Internal observability/tracing packages and instrumentation tests | Indirect OTEL deps, not a primary API surface | Callback/tracing mentions inside runtime, but less centralized | Persistence/traces mainly for workflow introspection | No clear first-class observability package found |
| Best mental model | Embedded coding-agent runtime | Production agent SDK/platform | Multi-agent orchestration framework | Lean multi-agent runtime | Graph/ADK framework | Typed workflow kernel | General LLM app toolkit |

### API-Level Conclusions

- `agentkit` is the most opinionated runtime in the set. Its public surface is organized around a long-lived runtime with session identity, lifecycle interception, sandbox boundaries, and runtime-managed MCP/tool integration.
- `agent-sdk-go` is the closest alternative if the evaluation criterion is "how much adjacent runtime capability is already packaged for application builders".
- `eino` has one of the strongest abstractions in the group, but the center of gravity is graph composition rather than runtime/session semantics.
- `langchaingo` is a useful library benchmark for breadth, but it does not currently present the same "single runtime kernel" shape.
- `go-agent-framework` is the cleanest narrow reference if the goal is to study typed workflow execution and MCP exposure without broad runtime concerns.
