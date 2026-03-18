# Go Agent 竞品分析包：技术决策版

日期：2026-03-11

## 结论先行

从源码层面看，`agentkit` 在 Go 生态里的直接同类并不多。它不是单纯的“AI SDK”或“Agent library”，而是一个把 `agent loop`、`tool calling`、`MCP`、`sandbox`、`hooks/middleware`、`subagents`、`commands/tasks`、`security`、`structured output`、`OTEL tracing` 放在同一运行时中的系统。

因此，如果目标是寻找“最像本项目的开源 Go 项目”，应当按下面三类来理解，而不是把所有有 agent、tool、memory 的库都当成同类。

## 分类

### A 类：完整 runtime 的近似对照

- `agent-sdk-go`
- `AgenticGoKit`

这两者最适合当直接竞品或对标对象。

### B 类：局部 runtime 能力对照

- `go-agent`

适合对比 subagent、swarm、memory、tool schema 这些局部设计。

### C 类：架构思路对照，不是直接 runtime 竞品

- `eino`
- `go-agent-framework`
- `langchaingo`

它们都值得参考，但参考点不同：

- `eino`：graph / ADK / checkpointable execution
- `go-agent-framework`：typed workflow + MCP adapter
- `langchaingo`：通用 LLM app toolkit 的 API 广度

## 相似度排序

1. `agent-sdk-go`
2. `AgenticGoKit`
3. `go-agent`
4. `eino`
5. `go-agent-framework`
6. `langchaingo`

## 为什么 `agent-sdk-go` 最接近

从当前源码树看，它与 `agentkit` 的重叠面最大：

- 有 `pkg/mcp`
- 有 `pkg/memory`
- 有 `pkg/orchestration`
- 有 `pkg/task`
- 有 `pkg/workflow`
- 有 `pkg/structuredoutput`
- 有 `pkg/guardrails`
- 有 `pkg/tracing`
- 有 `examples/subagents`

这意味着它已经覆盖了大部分“生产级 agent SDK”需要的横向能力。

它和 `agentkit` 的主要差异是：

- `agentkit` 更强调 runtime 语义和 coding-agent 场景
- `agentkit` 有更明确的 sandbox / security / hooks / skills / commands/tasks 组合
- `agent-sdk-go` 更像企业级 agent platform SDK，guardrails 与外部平台集成味道更重

## 为什么 `AgenticGoKit` 排第二

它在以下方面非常强：

- 多智能体编排
- streaming-first
- MCP
- memory
- observability
- plugin 化扩展

但它和 `agentkit` 的“产品心智”并不完全一致。`AgenticGoKit` 更像一个多智能体应用框架，而 `agentkit` 更像一个可嵌入的运行时内核。

## 为什么 `eino` 不应被当成直接竞品

`eino` 很强，但强点不一样。

从代码结构看，它的中心是：

- `compose`
- `flow`
- `adk`
- `callbacks`

这说明它更偏 graph / ADK / composable execution。它适合做架构对照，但如果直接拿来和 `agentkit` 做 runtime 能力表面对比，会出现“都支持 agent，但中心问题不一样”的错位。

## 为什么 `langchaingo` 不应被误判为最像

`langchaingo` 很成熟，也很大，但它更像通用 LLM toolkit，而不是完整 runtime。

当前源码快照中明确可见的是：

- `agents`
- `chains`
- `callbacks`
- `memory`
- `tools`

这足够说明它适合做“通用库层”的对照，却不适合当 `agentkit` 这种 Claude Code-style runtime 的最近竞品。

## 对 `agentkit` 的战略判断

### 项目定位

`agentkit` 最合理的定位不是“另一个 Go AI SDK”，而是：

“面向 coding-agent / enterprise embedding 场景的 Go runtime kernel”

### 最值得放大的差异点

- `sandbox + security` 是显著差异点
- `hooks + middleware interception` 是显著差异点
- `subagents + tasks + commands` 的组合是显著差异点
- `MCP` 作为运行时一等能力，而不是附加工具接入，是显著差异点

### 最容易被市场误读的点

- 容易被误读成 `langchaingo` 一类通用库
- 容易被误读成 `eino` 一类 graph framework
- 容易被误读成 `agent-sdk-go` 一类泛平台 SDK

实际更准确的说法应当是：

`agentkit` 不是只提供“搭 agent 的组件”，而是提供“运行 agent 的内核”。

## 建议的对外竞品口径

### 如果对外只说一句话

`agentkit` 是一个偏 Claude Code-style 的 Go agent runtime，不只是 Go 版 agent library。

### 如果要点名对照物

- 直接竞品：`agent-sdk-go`
- 次级竞品：`AgenticGoKit`
- 架构参考：`eino`
- 库层参考：`langchaingo`

## 建议的下一步

- 如果目标是产品定位或官网文案，优先围绕 `runtime kernel`、`sandbox`、`hooks`、`MCP`、`subagents/tasks` 重写表述。
- 如果目标是技术路线评审，优先继续深挖 `agent-sdk-go` 与 `AgenticGoKit` 的 API 细节和运行时边界。
- 如果目标是开源竞争分析，下一轮不需要再泛搜项目，而应该只围绕前两名做深度源码对比。
