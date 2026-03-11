# Go Agent 项目源码对比

日期：2026-03-11

## 范围

本文基于本地抓取到的当前源码快照，对 `agentkit` 与 6 个相近的开源 Go 项目做代码级对比，而不是只看 README 宣传语。

本次对比的目录：

- `agentkit`：`.`
- `langchaingo`：`other/langchaingo-src`
- `eino`：`other/eino`
- `AgenticGoKit`：`other/AgenticGoKit`
- `agent-sdk-go`：`other/agent-sdk-go-src`
- `go-agent`：`other/go-agent`
- `go-agent-framework`：`other/go-agent-framework`

说明：

- `langchaingo` 与 `agent-sdk-go` 的直接 `git clone` 在网络传输阶段卡住，因此改为抓取 GitHub 当前默认分支 tarball 快照，分别落在 `other/langchaingo-src` 与 `other/agent-sdk-go-src`。
- 文中“未发现”表示在当前快照中没有找到专门包或明确代码证据，这是基于当前源码树的推断，不代表项目永久没有该能力。

## 方法

对比依据包括：

- `go.mod` 中的 module path 与 Go 版本
- 顶层目录与包布局
- 代码中对以下能力的直接证据：`MCP`、`sandbox/security`、`hooks/callbacks`、`subagents/multi-agent`、`memory`、`workflow/task`、`observability/tracing`、`structured output`
- 粗粒度成熟度信号：Go 文件数、测试文件数、示例目录数

## 快速结论

如果问题是“谁最像 `agentkit` 这种生产级 Go agent runtime”，我给出的相似度排序是：

1. `agent-sdk-go`
2. `AgenticGoKit`
3. `go-agent`
4. `eino`
5. `go-agent-framework`
6. `langchaingo`

原因：

- `agent-sdk-go` 与 `agentkit` 的重叠面最大，覆盖 MCP、memory、orchestration、tasks、structured output、tracing、guardrails、subagents。
- `AgenticGoKit` 在多智能体编排、MCP、memory、streaming、observability 上重叠很大，但没有 `agentkit` 那么强的 Claude Code-style runtime 气质。
- `go-agent` 覆盖了 subagents、swarm、memory、tool catalog 和 orchestration，但运行时表面不如 `agentkit` 宽。
- `eino` 很强，但更像 ADK/graph framework，不像嵌入式 coding-agent runtime。
- `go-agent-framework` 是一个 typed workflow kernel，带 MCP adapter，但不是完整 runtime。
- `langchaingo` 是通用 LLM toolkit，能力面宽，但运行时边界最不像 `agentkit`。

## 总体对比表

| 项目 | Module | Go | 源码形态 | MCP | Sandbox / 隔离 | Hook / Callback | 多智能体 / Subagent | Memory | Workflow / Task | 可观测性 | 结构化输出 | 与 `agentkit` 的关系 |
|---|---|---:|---|---|---|---|---|---|---|---|---|---|
| `agentkit` | `github.com/godeps/agentkit` | 1.24.0 | 完整 runtime SDK，含 `pkg/api`、`pkg/mcp`、`pkg/sandbox`、`pkg/runtime/*`、`pkg/security` | 有，`pkg/mcp/mcp.go` | 有，`pkg/sandbox/*`、`pkg/security/*` | 有，`pkg/core/hooks/*`、`pkg/middleware/*` | 有，`pkg/runtime/subagents/*` | 以 session/message 为中心，`pkg/message/*` | 有，`pkg/runtime/tasks/*`、commands、async bash | 有，`pkg/api/otel.go` | 有，`pkg/model/interface.go` | 基线 |
| `agent-sdk-go` | `github.com/Ingenimax/agent-sdk-go` | 1.24.4 | 宽表面 SDK，含 `pkg/agent`、`pkg/mcp`、`pkg/memory`、`pkg/orchestration`、`pkg/task`、`pkg/workflow`、`pkg/structuredoutput`、`pkg/guardrails`、`pkg/tracing` | 有 | 当前快照未发现类似 `pkg/sandbox` 的独立隔离子系统，更偏 guardrails | 未发现与 `agentkit` 同级别的生命周期 hook 层 | 有，`examples/subagents` 和 orchestration 相关代码 | 有，`pkg/memory/*` | 有，`pkg/task/*`、`pkg/workflow/*`、`pkg/executionplan/*` | 有，`pkg/tracing/*` 与 OTEL 依赖 | 有，`pkg/structuredoutput/*` | 最接近的替代项 |
| `AgenticGoKit` | `github.com/agenticgokit/agenticgokit` | 1.24.1 | 插件化 framework，含 `internal/agents`、`internal/mcp`、`internal/memory`、`internal/orchestrator`、`internal/observability`、`plugins/*`、`v1beta/*` | 有，`plugins/mcp/default/default.go`、`core/mcp.go` | 文档提到权限/沙箱，但源码中未发现与 `agentkit` 同类的独立隔离包 | 有 internal callbacks 和 streaming handler，但没有 `agentkit` 那种清晰生命周期 hook 面 | 有，`v1beta/workflow.go`、subworkflow tests、orchestrator 包 | 有，`internal/memory/*` | 很强，`v1beta/*` 与 `internal/orchestrator/*` 是核心 | 有，`internal/observability/*` 与 OTEL 测试 | 有一定支持，但不如 `agentkit` / `agent-sdk-go` 集中 | 强多智能体编排框架 |
| `go-agent` | `github.com/Protocol-Lattice/go-agent` | 1.25.0 | 较轻量 runtime，核心在根包、`src/memory`、`src/subagents`、`src/swarm` | 当前只看到 `go.mod` 中间接依赖，未发现一等 MCP 子系统 | 未发现独立 sandbox / isolation 子系统 | 未发现明确的 hook framework | 有，`src/subagents`、`src/swarm`、`agent_tool.go`、`agent_orchestrators.go` | 有，`src/memory/*` 与 checkpoint tests | 中等，有 orchestration 与 tool chains，但 task 体系不宽 | OTEL 主要是间接依赖，不是一等 API 面 | tool 层有 schema，整体验证/输出约束面较弱 | 轻量多智能体 runtime |
| `eino` | `github.com/cloudwego/eino` | 1.18 | ADK/graph framework，核心在 `adk`、`compose`、`flow`、`callbacks`、`components/*` | 当前快照未发现独立 MCP 子系统 | `adk/middlewares/filesystem` 中有 sandbox 风格内容，但不像通用 runtime 隔离层 | 很强，`callbacks/*` 与 handler templates 很完整 | 有，`flow/agent/multiagent/host/*`、supervisor/sub-agent 模式 | 有 state/checkpoint store，但不如 agent SDK 那样 conversation-memory-centric | 很强，graph compile、checkpoint、compose 是核心 | 有 callback/tracing 能力，但不是单独 OTEL 子系统 | schema 能力重，但不是以“结构化输出 runtime”作为独立产品面 | 强 ADK / graph framework |
| `go-agent-framework` | `github.com/stephanoumenos/go-agent-framework` | 1.23.4 | 小而专的 typed workflow engine，核心在 `workflow.go`、`nodes/*`、`store/*`、`mcp/*` | 有，`mcp/adapter.go` | 未发现独立隔离子系统 | 未发现生命周期 hook 层 | 未发现 dedicated multi-agent 系统 | 有 workflow-state persistence，但不是 conversational memory | 很强，这是主产品面 | 以 workflow introspection 为主，非完整 observability 套件 | 有，`nodes/openai/middleware/structuredoutput.go` | 窄而清晰的 workflow kernel |
| `langchaingo` | `github.com/tmc/langchaingo` | 1.24.4 | 通用 LLM toolkit，核心在 `agents`、`chains`、`callbacks`、`memory`、`tools` | 当前快照未发现独立 MCP 子系统 | 未发现 sandbox / isolation 子系统 | 有，`callbacks/*` | 当前快照只看到有限 agent orchestration 证据，没有 dedicated subagent runtime | 有，`memory/*` | 有 chains/workflows，但没有 `agentkit` 式 task runtime | 当前源码中未发现一等 observability 子系统 | 有 provider/tool schema，但没有集中式 structured-output runtime 面 | 更像通用库层，不像 runtime |

## API 设计级差异表

这一节不是看“有哪些包”，而是看“对外 API 长什么样”。

| 设计面 | `agentkit` | `agent-sdk-go` | `AgenticGoKit` | `go-agent` | `eino` | `go-agent-framework` | `langchaingo` |
|---|---|---|---|---|---|---|---|
| 入口形态 | runtime-first：`api.New(...)` 后 `Run` / `RunStream` | SDK-first，多个功能包 + YAML/config 组合 | builder/plugin 导向，外加 `v1beta` workflow API | root agent 类型 + tool/subagent 抽象 | ADK + graph/compose API | workflow 定义 + execute 函数 | 按能力导入的 library-style package |
| Session 模型 | 明确 `SessionID`，有同 session 并发规则与 history 管理 | 更偏 memory/task，不是单一 runtime session 中心 | 更偏 run/workflow 中心，streaming-first | agent + memory session 模式 | graph execution + checkpoint store | workflow execution context + persisted node state | memory buffer / chain context |
| Tool 抽象 | runtime tool registry + builtin tools + MCP tools | `pkg/tools` + MCP + subagents + structured task | internal tool registry + `v1beta/tool_discovery.go` | tool catalog + typed tool schema | graph/ADK 中的 tool nodes | workflow 转 MCP tool adapter | tools 包 + executor |
| MCP 风格 | 作为 runtime 一等子系统接入外部 tool servers | 作为 SDK 一等子系统 | plugin/core 式 MCP manager 与 discovery | 当前未发现一等 MCP 子系统 | 当前未发现一等 MCP 子系统 | 以 adapter 方式把 workflow 暴露为 MCP tools | 当前未发现一等 MCP 子系统 |
| Hook / Callback | 显式 lifecycle hooks + middleware interception points | 未发现与 `agentkit` 同级别的生命周期 hook 框架 | 有 internal callbacks 与 stream handlers | 只有少量 callback 用法，不成体系 | callbacks 很强，贯穿 components/graphs | 没有 lifecycle hook 层 | callbacks 包较成熟 |
| 多智能体 API | runtime subagents + dispatch context + task integration | subagent 示例 + orchestration packages | multi-agent workflows 是核心卖点 | subagents 与 swarm 是核心概念 | graph 内 multi-agent host/supervisor | 不是主要抽象 | 有 agent executor，但未见 dedicated subagent runtime |
| Task / Workflow | 独立 runtime tasks + commands/tasks 语义 | task/workflow/execution-plan 都有 | workflow/orchestrator 非常强 | orchestration 有，但 task 模型较轻 | graph compile / checkpoint flow 是核心 | 核心就是 workflow | chains/agents 为主，不是 task runtime |
| Structured Output | `ResponseFormat` / `OutputSchema` 明确而统一 | `pkg/structuredoutput` 独立清晰 | 有，但不够集中 | tool schema 有，agent-wide 响应约束弱 | schema 重，但不以结构化输出为产品面 | OpenAI structured-output middleware | schema 有，但不够集中 |
| 安全边界 | 本组里最强：独立 sandbox + security | guardrails 强，隔离弱 | 文档提权限/沙箱，源码未见独立隔离层 | 未见独立隔离层 | 有 filesystem middleware，但不是完整隔离系统 | 未见独立隔离层 | 未见独立隔离层 |
| Observability | 内建 OTEL tracer，区分 build-tag/noop | dedicated tracing package + OTEL 依赖 | internal observability/tracing + instrumentation tests | OTEL 主要是依赖层面 | callback/tracing 有，但不够集中 | 主要用于 workflow introspection | 未见一等 observability package |
| 心智模型 | 嵌入式 coding-agent runtime | 生产级 agent SDK / platform | 多智能体编排框架 | 轻量多智能体 runtime | Graph / ADK framework | Typed workflow kernel | 通用 LLM 应用工具箱 |

## 每个项目的代码证据与判断

### `agentkit`

关键证据：

- 运行时表面明确：`pkg/mcp`、`pkg/sandbox`、`pkg/runtime/skills`、`pkg/runtime/subagents`、`pkg/runtime/tasks`、`pkg/security`
- Hooks 是一等能力：`pkg/core/hooks/executor.go`、`pkg/core/hooks/lifecycle.go`
- MCP 是一等能力：`pkg/mcp/mcp.go`
- Sandbox 与策略隔离是一等能力：`pkg/sandbox/interface.go`、`pkg/sandbox/fs_policy.go`、`pkg/sandbox/net_policy.go`
- 结构化输出接口明确：`pkg/model/interface.go`
- OpenTelemetry 通过 build tag 提供：`pkg/api/otel.go`、`pkg/api/otel_noop.go`

判断：

- 在 Go 生态里，`agentkit` 的特征不是“有 agent”，而是把 agent loop、MCP、sandbox、hooks、subagents、tasks、security 放在一个统一 runtime 里。
- 这使它比一般 AI framework 更像一个可嵌入的 Claude Code-style runtime kernel。

### `agent-sdk-go`

关键证据：

- 最接近 `agentkit` 的宽表面结构：`pkg/agent`、`pkg/mcp`、`pkg/memory`、`pkg/orchestration`、`pkg/task`、`pkg/workflow`、`pkg/structuredoutput`、`pkg/guardrails`、`pkg/tracing`
- `other/agent-sdk-go-src/go.mod` 直接依赖 `github.com/modelcontextprotocol/go-sdk` 和 OTEL 相关包
- 示例覆盖面大：`examples/mcp`、`examples/memory`、`examples/subagents`、`examples/guardrails`、`examples/structured_output`、`examples/tracing`

判断：

- 这是当前最适合作为 `agentkit` 直接 benchmark 的项目。
- 和 `agentkit` 的差异主要在于：它更偏 enterprise/platform 能力、guardrails、外部服务集成；`agentkit` 则更偏 runtime 语义、sandbox、hooks、skills、commands/tasks。
- 当前快照中没有找到与 `pkg/sandbox` 对等的独立隔离子系统。

### `AgenticGoKit`

关键证据：

- 核心架构围绕 agents、MCP、memory、orchestrator、observability、plugins：`internal/agents`、`internal/mcp`、`internal/memory`、`internal/orchestrator`、`internal/observability`、`plugins/*`
- 多智能体 workflow 明确存在：`v1beta/workflow.go`
- MCP 通过 plugin/core 两层暴露：`plugins/mcp/default/default.go`、`core/mcp.go`
- OTEL instrumentation 测试比较积极：`test/internal/llm/openai_instrumentation_test.go`、`test/internal/agents/builder_instrumentation_test.go`

判断：

- 它和 `agentkit` 在 orchestration、memory、streaming、observability 上很接近。
- 但它的 builder/plugin 取向更强，因此更像一个 app framework，而不是“嵌入式 runtime kernel”。
- 没找到 `agentkit` 级别的独立 sandbox / isolation 子系统。

### `go-agent`

关键证据：

- 有明确的 subagent 与 swarm 包：`src/subagents`、`src/swarm`
- 根包暴露 tool/subagent catalog 与 orchestration：`catalog.go`、`agent_tool.go`、`agent_orchestrators.go`
- memory 是一等能力：`src/memory/*` 与 `agent_checkpoint_test.go`
- tool schema 明确：`types.go`、`agent_tool.go`

判断：

- `go-agent` 更像一个精简版多智能体 runtime。
- 它与 `agentkit` 在 subagents、orchestration、memory 上有交集，但没有发现可比的 hooks、sandbox、commands/tasks、MCP runtime integration。

### `eino`

关键证据：

- 最强的产品面是 graph/compose/ADK：`compose/*`、`flow/*`、`adk/*`、`components/*`
- callbacks 很成熟：`callbacks/*`、`utils/callbacks/template.go`
- multi-agent host 模式是显式能力：`flow/agent/multiagent/host/*`
- graph runtime 的 state/checkpoint 能力可见：`compose/resume_test.go`

判断：

- `eino` 很强，但解决的问题和 `agentkit` 不完全相同。
- 它更擅长 graph composition、agent-as-node、checkpointable execution，而不是 coding-agent runtime 的 session/sandbox/command/skills 语义。

### `go-agent-framework`

关键证据：

- 核心抽象是 workflow 与 node：`workflow.go`、`node_execution.go`、`core.go`
- persistence 是 workflow 执行的一部分：`store/*`、`nodestate.go`
- MCP 通过 adapter 存在：`mcp/adapter.go`
- OpenAI structured output 在 middleware 中：`nodes/openai/middleware/structuredoutput.go`

判断：

- 它是一个小而清晰的 workflow execution framework，不是宽表面 agent runtime。
- 如果目标是研究 typed workflow 与 MCP exposure，它很值得看；如果目标是研究完整 runtime，则不够接近 `agentkit`。

### `langchaingo`

关键证据：

- 通用工具箱布局明显：`agents`、`chains`、`callbacks`、`memory`、`tools`
- callbacks 是明确的一等能力：`callbacks/callbacks.go`
- agents/tools/memory 都是成熟产品面：`agents/*`、`tools/*`、`memory/*`

判断：

- `langchaingo` 最适合当“通用 Go LLM toolkit”的参考，而不是 `agentkit` 的直接 runtime 对照物。
- 它与 `agentkit` 在 agent/tool/memory 原语上有重叠，但没有找到 dedicated MCP layer、sandbox、subagent runtime 或 command/task runtime。

## 粗粒度成熟度信号

| 项目 | Go 文件数 | 测试文件数 | 示例目录数 |
|---|---:|---:|---:|
| `agentkit` | 2630 | 834 | 12 |
| `langchaingo` | 671 | 226 | 77 |
| `eino` | 285 | 103 | 0 |
| `AgenticGoKit` | 308 | 74 | 27 |
| `agent-sdk-go` | 338 | 67 | 38 |
| `go-agent` | 101 | 28 | 0 |
| `go-agent-framework` | 30 | 8 | 4 |

解释：

- `langchaingo` 的示例面最广。
- 当前工作区里，`agentkit` 的源码与测试体量最大。
- `eino`、`AgenticGoKit`、`agent-sdk-go` 处在中间区间，说明都已经超过 demo 级。
- `go-agent-framework` 很明显是更小、更窄、更专的项目。

## 最终建议

### 如果你要找最接近 `agentkit` 的 benchmark

优先看：

1. `agent-sdk-go`
2. `AgenticGoKit`

### 如果你要找架构对照

优先看：

1. `eino`
2. `langchaingo`

### 如果你要找更小、更聚焦的参考实现

优先看：

1. `go-agent`
2. `go-agent-framework`
