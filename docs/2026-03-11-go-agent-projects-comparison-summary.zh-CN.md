# Go Agent 项目对比摘要

日期：2026-03-11

## 一句话结论

如果把 `agentkit` 定义为“Go 版、可嵌入的 Claude Code-style runtime”，那么当前最接近它的开源项目是 `agent-sdk-go`，其次是 `AgenticGoKit`；`eino` 和 `langchaingo` 更像通用 AI/Agent 应用框架；`go-agent-framework` 则更像 workflow kernel。

## 排序

按与 `agentkit` 的相似度排序：

1. `agent-sdk-go`
2. `AgenticGoKit`
3. `go-agent`
4. `eino`
5. `go-agent-framework`
6. `langchaingo`

## 为什么 `agentkit` 不太像普通 Go AI 框架

`agentkit` 的独特之处，不在于“支持 agent”，而在于它把这些能力统一放进一个 runtime：

- agent loop
- tool calling
- MCP
- sandbox
- hooks / middleware
- subagents
- commands / tasks
- security
- structured output
- OTEL tracing

在这组项目里，只有 `agent-sdk-go` 在“宽 runtime 表面”上真正接近；多数其他项目只覆盖其中一部分。

## 每个项目的定位

### `agent-sdk-go`

- 最像 `agentkit`
- 强在 MCP、memory、orchestration、task/workflow、guardrails、structured output、tracing
- 弱项是没有看到与 `agentkit` 同级的独立 sandbox 子系统

### `AgenticGoKit`

- 强多智能体编排、streaming、MCP、memory、observability
- 更像多智能体框架，不像 coding-agent runtime kernel

### `go-agent`

- 轻量多智能体 runtime
- 强在 subagents、swarm、memory、tool schema
- 运行时边界明显比 `agentkit` 窄

### `eino`

- 强 ADK / graph / compose
- 强 callbacks、multi-agent host、checkpointable graph execution
- 更适合当 graph framework 对照，而不是 runtime 对照

### `go-agent-framework`

- 强 typed workflow + MCP adapter
- 很适合作为小型架构参考
- 不适合作为完整 agent runtime benchmark

### `langchaingo`

- 强通用性、provider 覆盖、tool/memory/chains
- 更像 Go 版通用 LLM 应用工具箱
- 与 `agentkit` 的 runtime 心智模型差距最大

## 建议怎么用这些对照物

- 做直接竞争性 benchmark：看 `agent-sdk-go`
- 做多智能体编排能力 benchmark：看 `AgenticGoKit`
- 做 graph/ADK 设计对照：看 `eino`
- 做工具箱式 API 宽度对照：看 `langchaingo`
- 做小型精简实现参考：看 `go-agent`、`go-agent-framework`

## 本地文件

- 完整英文版：`docs/2026-03-11-go-agent-projects-comparison.md`
- 完整中文版：`docs/2026-03-11-go-agent-projects-comparison.zh-CN.md`
- 本摘要：`docs/2026-03-11-go-agent-projects-comparison-summary.zh-CN.md`
