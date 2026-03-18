# Go Agent 竞品分析包

日期：2026-03-11

## 文档组成

- 技术决策版：[2026-03-11-go-agent-competitor-pack-decision.zh-CN.md](./2026-03-11-go-agent-competitor-pack-decision.zh-CN.md)
- 源码证据附录：[2026-03-11-go-agent-competitor-pack-evidence.zh-CN.md](./2026-03-11-go-agent-competitor-pack-evidence.zh-CN.md)
- PR 摘要版：[2026-03-11-go-agent-competitor-pack-pr-summary.zh-CN.md](./2026-03-11-go-agent-competitor-pack-pr-summary.zh-CN.md)

## 适用场景

- 如果你要做路线判断或资源投入判断，先看“技术决策版”。
- 如果你要追溯结论从哪里来，或准备进一步深挖某个项目，直接看“源码证据附录”。
- 如果你要把这次调研结果同步给团队、写到变更说明里、或者放到 PR 描述中，使用“PR 摘要版”。

## 配套原始文档

- 英文完整版：[2026-03-11-go-agent-projects-comparison.md](./2026-03-11-go-agent-projects-comparison.md)
- 中文完整版：[2026-03-11-go-agent-projects-comparison.zh-CN.md](./2026-03-11-go-agent-projects-comparison.zh-CN.md)
- 中文简版：[2026-03-11-go-agent-projects-comparison-summary.zh-CN.md](./2026-03-11-go-agent-projects-comparison-summary.zh-CN.md)

## 一句话结论

如果把 `agentkit` 定义为“Go 版、可嵌入的 Claude Code-style runtime”，那么最接近的开源对照物是 `agent-sdk-go`，其次是 `AgenticGoKit`；其余项目更适合拿来做局部能力或架构思路对照，而不是完整运行时对照。
