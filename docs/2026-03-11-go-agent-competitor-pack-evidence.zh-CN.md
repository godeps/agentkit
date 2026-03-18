# Go Agent 竞品分析包：源码证据附录

日期：2026-03-11

## 说明

这份文档只记录支撑判断的源码证据与目录证据，便于后续复查。

## 本地源码目录

- `other/eino`
- `other/AgenticGoKit`
- `other/go-agent`
- `other/go-agent-framework`
- `other/langchaingo-src`
- `other/agent-sdk-go-src`

## `agentkit`

关键包：

- `pkg/mcp`
- `pkg/sandbox`
- `pkg/security`
- `pkg/core/hooks`
- `pkg/runtime/subagents`
- `pkg/runtime/tasks`
- `pkg/runtime/skills`
- `pkg/tool`
- `pkg/api`

关键证据：

- MCP：`pkg/mcp/mcp.go`
- Hooks：`pkg/core/hooks/executor.go`
- Sandbox：`pkg/sandbox/interface.go`
- FS policy：`pkg/sandbox/fs_policy.go`
- Net policy：`pkg/sandbox/net_policy.go`
- Subagents：`pkg/runtime/subagents/manager.go`
- Tasks：`pkg/runtime/tasks/task.go`
- Structured output：`pkg/model/interface.go`
- OTEL：`pkg/api/otel.go`

## `agent-sdk-go`

关键目录：

- `pkg/agent`
- `pkg/mcp`
- `pkg/memory`
- `pkg/orchestration`
- `pkg/task`
- `pkg/workflow`
- `pkg/structuredoutput`
- `pkg/guardrails`
- `pkg/tracing`

关键证据：

- Module：`other/agent-sdk-go-src/go.mod`
- MCP：`other/agent-sdk-go-src/pkg/mcp`
- Memory：`other/agent-sdk-go-src/pkg/memory`
- Orchestration：`other/agent-sdk-go-src/pkg/orchestration`
- Task：`other/agent-sdk-go-src/pkg/task`
- Workflow：`other/agent-sdk-go-src/pkg/workflow`
- Structured output：`other/agent-sdk-go-src/pkg/structuredoutput`
- Guardrails：`other/agent-sdk-go-src/pkg/guardrails`
- Tracing：`other/agent-sdk-go-src/pkg/tracing`
- Subagents 示例：`other/agent-sdk-go-src/examples/subagents`

结论支撑：

- 它是当前最接近 `agentkit` 的宽表面 agent runtime SDK。
- 但未发现与 `agentkit/pkg/sandbox` 对等的独立隔离包。

## `AgenticGoKit`

关键目录：

- `internal/agents`
- `internal/mcp`
- `internal/memory`
- `internal/orchestrator`
- `internal/observability`
- `plugins/mcp`
- `plugins/memory`
- `plugins/orchestrator`
- `v1beta`

关键证据：

- MCP plugin：`other/AgenticGoKit/plugins/mcp/default/default.go`
- MCP core：`other/AgenticGoKit/core/mcp.go`
- Workflow：`other/AgenticGoKit/v1beta/workflow.go`
- Tool discovery：`other/AgenticGoKit/v1beta/tool_discovery.go`
- Observability exports：`other/AgenticGoKit/observability/exports.go`
- OTEL tests：`other/AgenticGoKit/test/internal/llm/openai_instrumentation_test.go`

结论支撑：

- 强多智能体编排
- 强 observability
- MCP 是一等能力
- 未见与 `agentkit` 同等级的独立 sandbox 隔离层

## `go-agent`

关键目录：

- `src/memory`
- `src/subagents`
- `src/swarm`

关键证据：

- Tool catalog：`other/go-agent/catalog.go`
- Tool schema：`other/go-agent/types.go`
- SubAgent tool：`other/go-agent/agent_tool.go`
- Orchestrator：`other/go-agent/agent_orchestrators.go`
- Memory checkpoint tests：`other/go-agent/agent_checkpoint_test.go`

结论支撑：

- subagents / swarm / memory 是强项
- 当前源码树中未见 dedicated MCP subsystem
- 当前源码树中未见 hooks / sandbox / task runtime 对等实现

## `eino`

关键目录：

- `adk`
- `compose`
- `flow`
- `callbacks`
- `components`

关键证据：

- Project doc：`other/eino/doc.go`
- Callback framework：`other/eino/callbacks/doc.go`
- Callback template：`other/eino/utils/callbacks/template.go`
- Multi-agent host：`other/eino/flow/agent/multiagent/host/types.go`
- Multi-agent compose：`other/eino/flow/agent/multiagent/host/compose.go`
- Graph state/checkpoint tests：`other/eino/compose/resume_test.go`

结论支撑：

- graph / compose / ADK 是核心
- callbacks 很强
- multi-agent 有，但运行时中心不是 coding-agent runtime

## `go-agent-framework`

关键目录：

- `mcp`
- `nodes`
- `store`

关键证据：

- Workflow core：`other/go-agent-framework/workflow.go`
- Node execution：`other/go-agent-framework/node_execution.go`
- Core execution context：`other/go-agent-framework/core.go`
- MCP adapter：`other/go-agent-framework/mcp/adapter.go`
- Structured output middleware：`other/go-agent-framework/nodes/openai/middleware/structuredoutput.go`

结论支撑：

- workflow execution 是主产品面
- MCP 是 adapter，而不是完整 runtime 子系统
- 适合作为 typed workflow 参考，不适合作为 `agentkit` 的完整竞品

## `langchaingo`

关键目录：

- `agents`
- `chains`
- `callbacks`
- `memory`
- `tools`

关键证据：

- Project overview：`other/langchaingo-src/doc.go`
- Callback interfaces：`other/langchaingo-src/callbacks/callbacks.go`
- Agent docs：`other/langchaingo-src/agents/doc.go`

结论支撑：

- 它是成熟的通用 LLM toolkit
- 当前快照中未见 dedicated MCP、sandbox、subagent runtime、task runtime
- 更适合当库层参考，而不是运行时对照

## 粗粒度数量信号

| 项目 | Go 文件数 | 测试文件数 | 示例目录数 |
|---|---:|---:|---:|
| `agentkit` | 2630 | 834 | 12 |
| `langchaingo` | 671 | 226 | 77 |
| `eino` | 285 | 103 | 0 |
| `AgenticGoKit` | 308 | 74 | 27 |
| `agent-sdk-go` | 338 | 67 | 38 |
| `go-agent` | 101 | 28 | 0 |
| `go-agent-framework` | 30 | 8 | 4 |

## 证据使用建议

- 要追源码深挖，优先先看 `agent-sdk-go` 和 `AgenticGoKit`。
- 要验证 `agentkit` 的差异点，优先比 `sandbox`、`hooks`、`subagents/tasks`、`MCP runtime integration`。
- 要避免误判，别把“有 agent 包”直接等价成“有完整 runtime”。
