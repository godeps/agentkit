# SDK 调用指南（详细）

本文面向“外部系统接入 agentsdk-go SDK”的场景，给出可直接复制的初始化与调用方式。

## 1. 最小可运行示例

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/godeps/agentkit/pkg/api"
	"github.com/godeps/agentkit/pkg/model"
)

func main() {
	ctx := context.Background()

	rt, err := api.New(ctx, api.Options{
		ProjectRoot: ".",
		ModelFactory: &model.AnthropicProvider{
			ModelName: "claude-sonnet-4-5-20250929",
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer rt.Close()

	resp, err := rt.Run(ctx, api.Request{
		SessionID: "demo-session",
		Prompt:    "请简要总结当前项目用途",
	})
	if err != nil {
		log.Fatal(err)
	}

	if resp.Result != nil {
		fmt.Println(resp.Result.Output)
	}
}
```

## 2. 配置目录模型（推荐）

SDK 现在支持统一配置根目录，而不是固定写死 `.claude`。

- `ProjectRoot`：项目根路径
- `ConfigRoot`：配置根目录（默认 `<ProjectRoot>/.claude`）
- `SkillsDirs`：额外 skills 目录（可多个）
- `SkillsRecursive`：是否递归发现 `SKILL.md`（`nil` 默认递归）

```go
opts := api.Options{
	ProjectRoot: ".",
	ConfigRoot:  "./runtime-config",
	ModelFactory: &model.AnthropicProvider{
		ModelName: "claude-sonnet-4-5-20250929",
	},
	SkillsDirs: []string{
		"./runtime-config/skills",
		"/opt/company/skills",
	},
	// nil = 默认 true；也可以显式设置
	SkillsRecursive: boolPtr(true),
}
```

目录约定（默认）：

- `settings.json` / `settings.local.json`
- `skills/`
- `commands/`
- `agents/`
- `rules/`
- `history/`

如果设置了 `ConfigRoot`，上述目录都在该根目录下解析。

## 3. skills 多目录与冲突策略

### 3.1 自动发现规则

- 发现文件名为 `SKILL.md` 的文件
- 默认递归扫描目录树
- `SkillsDirs` 可传多个，按传入顺序处理

### 3.2 同名 skill 冲突

当前策略：按加载顺序“先到先得”，后续同名会被记录为告警并跳过。  
建议在外部约束 skill 名全局唯一，避免意外遮蔽。

## 4. 同步调用（Run）

```go
resp, err := rt.Run(ctx, api.Request{
	SessionID: "s-001",
	Prompt:    "生成发布说明草稿",
	Tags: map[string]string{
		"source": "api",
		"team":   "platform",
	},
	Metadata: map[string]any{
		"ticket": "PLAT-1234",
	},
})
```

常用字段：

- `SessionID`：同一会话上下文隔离键
- `Prompt`：输入文本
- `ContentBlocks`：多模态输入（文本+图片等）
- `Tags/Metadata`：业务侧透传信息
- `TargetSubagent`：指定子代理
- `ToolWhitelist`：请求级工具白名单
- `ForceSkills`：请求级强制激活 skills

## 5. 流式调用（RunStream）

```go
stream, err := rt.RunStream(ctx, api.Request{
	SessionID: "s-stream-1",
	Prompt:    "分析这个仓库的核心模块",
})
if err != nil {
	log.Fatal(err)
}

for evt := range stream {
	switch evt.Type {
	case "content_block_delta":
		if evt.Delta != nil {
			fmt.Print(evt.Delta.Text)
		}
	case "tool_execution_start":
		fmt.Printf("\n[tool:start] %s\n", evt.Name)
	case "tool_execution_result":
		fmt.Printf("\n[tool:done] %s\n", evt.Name)
	case "error":
		fmt.Printf("\n[error] %s\n", evt.Output)
	}
}
```

## 6. 外部调用常见初始化模板

```go
func NewRuntime(ctx context.Context, projectRoot, configRoot string, skillsDirs []string) (*api.Runtime, error) {
	return api.New(ctx, api.Options{
		ProjectRoot: projectRoot,
		ConfigRoot:  configRoot,
		ModelFactory: &model.AnthropicProvider{
			ModelName: "claude-sonnet-4-5-20250929",
		},
		SkillsDirs: skillsDirs,
	})
}
```

建议：

- Runtime 建议按“进程级单例”或“租户级缓存”复用，避免频繁初始化
- 请求级使用 `SessionID` 做并发隔离
- 服务退出时调用 `rt.Close()`

## 7. CLI 对应参数（便于联调）

```bash
agentctl \
  --project /workspace/repo \
  --config-root /workspace/repo/runtime-config \
  --skills-dir /workspace/repo/runtime-config/skills \
  --skills-dir /opt/company/skills \
  --prompt "请总结当前改动"
```

## 8. 常见问题

1. `api: load settings` 报错  
说明配置文件路径错误或 JSON 非法。先检查 `ConfigRoot/settings.json`。

2. skills 没加载到  
先检查目录是否在 `SkillsDirs` 中，文件名是否严格为 `SKILL.md`，以及 frontmatter `name/description` 是否合法。

3. 同一 `SessionID` 并发报错  
这是 SDK 的会话互斥保护（同一会话不允许并发 Run/RunStream），请改成排队或改用不同 `SessionID`。

4. history 不落盘  
需确认 `cleanupPeriodDays > 0` 且目录可写。history 存储路径在 `<ConfigRoot>/history/`。

## 9. 辅助函数

```go
func boolPtr(v bool) *bool { return &v }
```

