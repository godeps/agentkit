# Go Agent 竞品分析包：PR 摘要版

日期：2026-03-11

## 本次补充了什么

这次在现有对比基础上，整理出一套更适合团队协作和评审使用的竞品分析包，包含：

- 技术决策版
- 源码证据附录
- PR 摘要版
- 总览索引

## 关键信息

- `agentkit` 最接近的开源 Go 对照物是 `agent-sdk-go`
- 第二接近的是 `AgenticGoKit`
- `eino`、`langchaingo` 更适合作为架构或库层对照，不适合作为完整 runtime 对照
- `go-agent-framework` 是 workflow kernel 参考，不是直接竞品

## 这套文档适合谁看

- 技术负责人：看“技术决策版”
- 核心开发：看“源码证据附录”
- PR reviewer / 协作者：看本摘要版

## 这次输出的价值

- 把“类似项目”从泛泛而谈，收敛成可执行的源码级比较
- 区分了“完整 runtime 竞品”和“局部能力参考”
- 明确了 `agentkit` 的差异化核心不只是 agent，而是 runtime kernel

## 后续建议

- 如果要继续深入，下一轮只需要聚焦 `agent-sdk-go` 和 `AgenticGoKit`
- 如果要转成对外叙事，建议围绕 `runtime kernel`、`sandbox`、`hooks`、`MCP`、`subagents/tasks` 来表达
