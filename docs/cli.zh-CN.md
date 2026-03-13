[English](cli.md) | 中文

# CLI 使用指南

`cmd/cli` 是 `agentkit` 自带的终端入口，支持：

- 交互式 shell 模式
- 单次 prompt 执行
- JSON 或渲染式流输出
- 基于 stdio 的 ACP 服务模式
- 可选的 `host`、`gvisor`、`govm` sandbox backend

本文档说明当前 [cmd/cli/main.go](/home/vipas/workspace/saker-ai/godeps/agentkit/cmd/cli/main.go) 已实现并验证过的 CLI 行为。

## 快速开始

启动默认交互模式：

```bash
go run ./cmd/cli
```

执行一次 prompt 后退出：

```bash
go run ./cmd/cli --prompt "总结这个仓库"
```

通过 stdin 传入输入：

```bash
echo "总结这个仓库" | go run ./cmd/cli
```

使用面向人的渲染流输出：

```bash
go run ./cmd/cli --prompt "检查这个仓库" --stream --stream-format rendered
```

启动 ACP 服务：

```bash
go run ./cmd/cli --acp
```

使用 `govm` sandbox 启动 CLI：

```bash
go run -tags govm_native ./cmd/cli --sandbox-backend=govm --repl
```

## 输入解析顺序

CLI 按下面的顺序解析用户输入：

1. `--prompt`
2. `--prompt-file`
3. 位置参数
4. stdin，如果 stdin 是 pipe
5. 交互模式，如果没有任何 prompt 来源且 stdin 是 TTY

示例：

```bash
go run ./cmd/cli --prompt "hello"
go run ./cmd/cli --prompt-file prompt.txt
go run ./cmd/cli hello world
echo "hello" | go run ./cmd/cli
go run ./cmd/cli
```

关键行为：

- 在真实终端中直接执行 `go run ./cmd/cli`，现在会默认进入交互模式。
- `echo "hello" | go run ./cmd/cli` 不会进入交互模式，而是把 stdin 作为 prompt。
- `--stream` 和 `--acp` 不会自动进入交互模式。

## 交互模式

显式启动交互模式：

```bash
go run ./cmd/cli --repl
```

或者直接执行：

```bash
go run ./cmd/cli
```

当 stdin 是终端且没有提供 prompt 来源时，CLI 会自动进入交互 shell。

每轮状态头会显示：

- 当前 session id
- 当前 model
- repo 根目录
- sandbox backend
- 已加载 skills 数量

内置 slash commands：

- `/skills`
- `/new`
- `/session`
- `/model`
- `/help`
- `/quit`

说明：

- `/new` 会生成一个新的 session id
- `/quit`、`/exit`、`/q` 都会退出 shell
- 单轮执行失败不会直接结束整个 shell

## 非交互执行

执行一次 prompt 并等待最终结果：

```bash
go run ./cmd/cli --prompt "列出核心包"
```

从文件读取 prompt：

```bash
go run ./cmd/cli --prompt-file ./prompt.txt
```

向请求元数据传入 tags：

```bash
go run ./cmd/cli --prompt "分析这个仓库" --tag source=manual --tag dryrun
```

`--tag key=value` 会设置字符串值。  
`--tag key` 会被视为 `key=true`。

## 流式输出

启用流式输出：

```bash
go run ./cmd/cli --prompt "检查仓库" --stream
```

支持的格式：

- `json`，默认
- `rendered`

示例：

```bash
go run ./cmd/cli --prompt "检查仓库" --stream --stream-format json
go run ./cmd/cli --prompt "检查仓库" --stream --stream-format rendered
```

`--verbose` 会输出额外的流式事件诊断信息。

`--waterfall off|summary|full` 用于控制渲染流的 waterfall 输出级别。

## ACP 模式

通过 stdio 启动 ACP 服务：

```bash
go run ./cmd/cli --acp
```

开启 `--acp` 后：

- CLI 不再解析 prompt 输入
- 不会进入交互模式
- 运行时仍然使用相同的 project/config/model 构造参数

另见 [docs/acp-integration.md](/home/vipas/workspace/saker-ai/godeps/agentkit/docs/acp-integration.md)。

## Project 与配置目录

主要路径参数：

- `--project`
- `--config-root`
- `--claude`

默认值：

- `--project` 默认为 `.`
- `--config-root` 默认为 `<project>/.claude`
- 如果传了 `--claude /path/to/.claude` 且没有传 `--config-root`，则 config root 会取该 `--claude` 路径

典型目录结构：

```text
<project>/
  .claude/
    settings.json
    settings.local.json
    skills/
```

示例：

```bash
go run ./cmd/cli --project /path/to/repo
go run ./cmd/cli --project /path/to/repo --config-root /path/to/config
go run ./cmd/cli --project /path/to/repo --claude /path/to/repo/.claude
```

可以用 `--print-effective-config` 在执行前打印解析后的运行时配置。

## Skills

当没有传 `--skills-dir` 时，CLI 默认会从下面两个位置发现 skills：

- `<config-root>/skills`
- `~/.agents/skills`

你也可以追加额外目录：

```bash
go run ./cmd/cli --skills-dir ./team-skills --skills-dir /opt/shared-skills
```

如果传了 `--skills-dir`，就会使用这些显式目录，而不是默认目录。

递归发现默认开启：

```bash
go run ./cmd/cli --skills-recursive=true
```

如果只想扫描顶层 skill 目录：

```bash
go run ./cmd/cli --skills-recursive=false
```

## MCP Servers

通过可重复的 `--mcp` 参数注册 MCP server：

```bash
go run ./cmd/cli --mcp server-a --mcp server-b
```

具体 server 字符串格式取决于你在 `agentkit` 中使用的 MCP 集成方式。

## Sandbox Backends

可用 backend：

- `host`
- `gvisor`
- `govm`

选择方式：

```bash
go run ./cmd/cli --sandbox-backend=host
go run ./cmd/cli --sandbox-backend=gvisor
go run -tags govm_native ./cmd/cli --sandbox-backend=govm
```

### host

`host` 是默认 backend，不增加虚拟化隔离层。

### gVisor

启用方式：

```bash
go run ./cmd/cli --sandbox-backend=gvisor
```

默认行为：

- 会在 `<project>/workspace/<session-id>` 下自动创建 session workspace
- guest 工作目录是 `/workspace`
- 项目根目录可以挂载到 `/project`

project mount 控制：

```bash
go run ./cmd/cli --sandbox-backend=gvisor --sandbox-project-mount=ro
go run ./cmd/cli --sandbox-backend=gvisor --sandbox-project-mount=rw
go run ./cmd/cli --sandbox-backend=gvisor --sandbox-project-mount=off
```

### govm

启用方式：

```bash
go run -tags govm_native ./cmd/cli --sandbox-backend=govm
```

`govm` 需要：

- 支持的平台：`linux/amd64`、`linux/arm64`、`darwin/arm64`
- native 构建：`-tags govm_native`
- 可用的 govm native runtime assets

默认 `govm` 行为：

- `RuntimeHome = <project>/.govm`
- `OfflineImage = py312-alpine`，除非通过 `--sandbox-image` 覆盖
- session workspace 自动创建在 `<project>/workspace/<session-id>`
- guest cwd 是 `/workspace`
- 除非显式关闭，项目根目录会挂载到 `/project`

示例：

```bash
go run -tags govm_native ./cmd/cli --sandbox-backend=govm --sandbox-project-mount=ro --repl
go run -tags govm_native ./cmd/cli --sandbox-backend=govm --sandbox-project-mount=rw --prompt "审查这个仓库"
go run -tags govm_native ./cmd/cli --sandbox-backend=govm --sandbox-project-mount=off --sandbox-image py312-alpine --repl
```

project mount 模式：

- `ro`：把项目根目录只读挂载到 `/project`
- `rw`：把项目根目录读写挂载到 `/project`
- `off`：不挂载项目根目录

## 环境变量与运行说明

相关环境行为：

- `AGENTSDK_TIMEOUT_MS` 如果是正整数，会覆盖默认超时
- `USER` 会传入 CLI 请求元数据
- 模型 provider 的认证信息由对应 provider 实现自己解析

当前 CLI 默认构造的是 `AnthropicProvider`，主要使用：

- `--model`
- `--system-prompt`

## Flags 参考

### 通用

- `--entry`：入口类型，`cli|ci|platform`
- `--project`：项目根目录，默认 `.`
- `--session`：session id 覆盖
- `--timeout-ms`：请求超时，单位毫秒
- `--print-effective-config`：执行前打印解析后的配置

### Model

- `--model`：模型名
- `--system-prompt`：system prompt 覆盖

### 输入与执行模式

- `--prompt`：直接传入 prompt 文本
- `--prompt-file`：从文件读取 prompt
- `--repl`：强制进入交互 shell
- `--stream`：启用流式输出
- `--stream-format`：`json|rendered`
- `--verbose`：额外流式诊断输出
- `--waterfall`：`off|summary|full`
- `--acp`：通过 stdio 运行 ACP 服务

### 配置与运行时文件

- `--claude`：可选 `.claude` 目录路径
- `--config-root`：可选 config root，默认 `<project>/.claude`

### Skills 与 MCP

- `--skills-dir`：额外或显式 skills 目录，可重复
- `--skills-recursive`：递归发现 `SKILL.md`
- `--mcp`：注册 MCP server，可重复

### Sandbox

- `--sandbox-backend`：`host|gvisor|govm`
- `--sandbox-project-mount`：`ro|rw|off`
- `--sandbox-image`：`govm` 的 offline image 覆盖

### Tags

- `--tag`：附加 `key=value` 或布尔风格 `key`

### 内部参数

- `--agentkit-gvisor-helper`：隐藏的 gVisor helper 模式

## 常见问题

### `no prompt provided`

只有在下面这些条件同时成立时才会出现：

- 当前不是交互模式
- stdin 不是 TTY
- 也没有提供任何 prompt 来源

可用这些方式启动：

```bash
go run ./cmd/cli
go run ./cmd/cli --repl
go run ./cmd/cli --prompt "hello"
echo "hello" | go run ./cmd/cli
```

### `govm native runtime unavailable`

这表示 CLI 选择了 `govm`，但 native runtime 不可用。

检查：

- 是否使用了 `-tags govm_native`
- 当前平台是否受支持
- govm native assets 是否可用

示例：

```bash
go run -tags govm_native ./cmd/cli --sandbox-backend=govm --repl
```

### `invalid --sandbox-project-mount`

允许的值只有：

- `ro`
- `rw`
- `off`

### sandbox 已经初始化，但执行仍然失败

如果 `govm` 或 `gvisor` 已经成功初始化，而失败发生在后续模型调用阶段，问题更可能在：

- provider credentials
- provider base URL / gateway
- 上游模型服务可用性

这和 sandbox 初始化本身是两回事。

## 相关文档

- [README_zh.md](/home/vipas/workspace/saker-ai/godeps/agentkit/README_zh.md)
- [docs/getting-started.md](/home/vipas/workspace/saker-ai/godeps/agentkit/docs/getting-started.md)
- [docs/acp-integration.md](/home/vipas/workspace/saker-ai/godeps/agentkit/docs/acp-integration.md)
- [examples/02-cli/README.md](/home/vipas/workspace/saker-ai/godeps/agentkit/examples/02-cli/README.md)
- [examples/13-govm-sandbox/README.md](/home/vipas/workspace/saker-ai/godeps/agentkit/examples/13-govm-sandbox/README.md)
