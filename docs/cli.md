[中文](cli.zh-CN.md) | English

# CLI Guide

`cmd/cli` is the built-in terminal entrypoint for `agentkit`. It supports:

- interactive shell mode
- one-shot prompt execution
- JSON or rendered streaming output
- ACP server mode over stdio
- optional `host`, `gvisor`, or `govm` sandbox backends

This guide documents the current behavior implemented in [cmd/cli/main.go](/home/vipas/workspace/saker-ai/godeps/agentkit/cmd/cli/main.go).

## Quick Start

Start the default interactive shell:

```bash
go run ./cmd/cli
```

Run one prompt and exit:

```bash
go run ./cmd/cli --prompt "summarize this repository"
```

Pass input through stdin:

```bash
echo "summarize this repository" | go run ./cmd/cli
```

Render a human-readable stream:

```bash
go run ./cmd/cli --prompt "inspect the repo" --stream --stream-format rendered
```

Start the ACP server:

```bash
go run ./cmd/cli --acp
```

Start the CLI with `govm` sandbox:

```bash
go run -tags govm_native ./cmd/cli --sandbox-backend=govm --repl
```

## Input Resolution

The CLI resolves user input in this order:

1. `--prompt`
2. `--prompt-file`
3. trailing positional arguments
4. stdin, if stdin is a pipe
5. interactive shell, if no prompt source is provided and stdin is a TTY

Examples:

```bash
go run ./cmd/cli --prompt "hello"
go run ./cmd/cli --prompt-file prompt.txt
go run ./cmd/cli hello world
echo "hello" | go run ./cmd/cli
go run ./cmd/cli
```

Important behavior:

- `go run ./cmd/cli` now starts the interactive shell by default when run in a real terminal.
- `echo "hello" | go run ./cmd/cli` stays non-interactive and uses stdin as the prompt.
- `--stream` and `--acp` never auto-enter interactive mode.

## Interactive Mode

Start interactive mode explicitly:

```bash
go run ./cmd/cli --repl
```

Or just run:

```bash
go run ./cmd/cli
```

When stdin is a terminal and no prompt source is provided, the CLI enters the interactive shell automatically.

The shell shows:

- current session id
- current model
- repo root
- sandbox backend
- loaded skills count

Built-in slash commands:

- `/skills`
- `/new`
- `/session`
- `/model`
- `/help`
- `/quit`

Notes:

- `/new` starts a fresh session id
- `/quit`, `/exit`, or `/q` exits the shell
- command failures in one turn do not terminate the whole shell

## Non-Interactive Execution

Run one prompt and wait for the final response:

```bash
go run ./cmd/cli --prompt "list the key packages"
```

Read from a file:

```bash
go run ./cmd/cli --prompt-file ./prompt.txt
```

Pass tags into request metadata:

```bash
go run ./cmd/cli --prompt "analyze this" --tag source=manual --tag dryrun
```

`--tag key=value` sets a string value.  
`--tag key` is treated as `key=true`.

## Streaming Output

Enable streaming:

```bash
go run ./cmd/cli --prompt "inspect repo" --stream
```

Supported formats:

- `json` (default)
- `rendered`

Examples:

```bash
go run ./cmd/cli --prompt "inspect repo" --stream --stream-format json
go run ./cmd/cli --prompt "inspect repo" --stream --stream-format rendered
```

Use `--verbose` to print extra event diagnostics during streaming.

Use `--waterfall off|summary|full` to control rendered waterfall output.

## ACP Mode

Run the ACP server over stdio:

```bash
go run ./cmd/cli --acp
```

When `--acp` is enabled:

- the CLI does not resolve prompt input
- it does not start interactive mode
- runtime options are still built from the same project/config/model settings

See also [docs/acp-integration.md](/home/vipas/workspace/saker-ai/godeps/agentkit/docs/acp-integration.md).

## Project and Config Layout

Key path flags:

- `--project`
- `--config-root`
- `--claude`

Defaults:

- `--project` defaults to `.`
- `--config-root` defaults to `<project>/.claude`
- if `--claude /path/to/.claude` is provided and `--config-root` is not, config root becomes that `--claude` path

Typical layout:

```text
<project>/
  .claude/
    settings.json
    settings.local.json
    skills/
```

Examples:

```bash
go run ./cmd/cli --project /path/to/repo
go run ./cmd/cli --project /path/to/repo --config-root /path/to/config
go run ./cmd/cli --project /path/to/repo --claude /path/to/repo/.claude
```

Use `--print-effective-config` to print the resolved runtime config before execution.

## Skills

By default, when `--skills-dir` is not provided, the CLI discovers skills from:

- `<config-root>/skills`
- `~/.agents/skills`

You can add extra roots:

```bash
go run ./cmd/cli --skills-dir ./team-skills --skills-dir /opt/shared-skills
```

If `--skills-dir` is provided, those explicit directories are used instead of the default roots.

Recursive discovery is enabled by default:

```bash
go run ./cmd/cli --skills-recursive=true
```

To limit discovery to top-level skill directories only:

```bash
go run ./cmd/cli --skills-recursive=false
```

## MCP Servers

Register MCP servers with repeatable `--mcp` flags:

```bash
go run ./cmd/cli --mcp server-a --mcp server-b
```

The exact server string format depends on the MCP server integration you are using in `agentkit`.

## Sandbox Backends

Available backends:

- `host`
- `gvisor`
- `govm`

Select one with:

```bash
go run ./cmd/cli --sandbox-backend=host
go run ./cmd/cli --sandbox-backend=gvisor
go run -tags govm_native ./cmd/cli --sandbox-backend=govm
```

### Host

`host` is the default backend. No virtualization layer is added.

### gVisor

Enable with:

```bash
go run ./cmd/cli --sandbox-backend=gvisor
```

Default behavior:

- session workspace is auto-created under `<project>/workspace/<session-id>`
- guest working directory is `/workspace`
- project root can be mounted to `/project`

Project mount control:

```bash
go run ./cmd/cli --sandbox-backend=gvisor --sandbox-project-mount=ro
go run ./cmd/cli --sandbox-backend=gvisor --sandbox-project-mount=rw
go run ./cmd/cli --sandbox-backend=gvisor --sandbox-project-mount=off
```

### govm

Enable with:

```bash
go run -tags govm_native ./cmd/cli --sandbox-backend=govm
```

`govm` requires:

- a supported platform: `linux/amd64`, `linux/arm64`, or `darwin/arm64`
- a native build: `-tags govm_native`
- usable govm native runtime assets

Default `govm` behavior:

- `RuntimeHome = <project>/.govm`
- `OfflineImage = py312-alpine` unless overridden by `--sandbox-image`
- session workspace auto-created under `<project>/workspace/<session-id>`
- guest cwd is `/workspace`
- project root mounted to `/project` unless disabled

Examples:

```bash
go run -tags govm_native ./cmd/cli --sandbox-backend=govm --sandbox-project-mount=ro --repl
go run -tags govm_native ./cmd/cli --sandbox-backend=govm --sandbox-project-mount=rw --prompt "review this repo"
go run -tags govm_native ./cmd/cli --sandbox-backend=govm --sandbox-project-mount=off --sandbox-image py312-alpine --repl
```

Project mount modes:

- `ro`: mount project root read-only at `/project`
- `rw`: mount project root read-write at `/project`
- `off`: do not mount project root

## Environment and Runtime Notes

Relevant environment behavior:

- `AGENTSDK_TIMEOUT_MS` overrides the default timeout when set to a positive integer
- `USER` is passed into CLI request metadata
- model provider credentials are resolved by the configured model provider implementation

The CLI currently constructs an `AnthropicProvider` by default, using:

- `--model`
- `--system-prompt`

## Flags Reference

### General

- `--entry`: entry point type, one of `cli|ci|platform`
- `--project`: project root, default `.`
- `--session`: session id override
- `--timeout-ms`: request timeout in milliseconds
- `--print-effective-config`: print resolved config before running

### Model

- `--model`: model name
- `--system-prompt`: system prompt override

### Input and execution mode

- `--prompt`: prompt literal
- `--prompt-file`: read prompt from file
- `--repl`: force interactive shell
- `--stream`: stream events
- `--stream-format`: `json|rendered`
- `--verbose`: extra stream diagnostics
- `--waterfall`: `off|summary|full`
- `--acp`: run ACP server over stdio

### Config and runtime files

- `--claude`: optional `.claude` directory path
- `--config-root`: optional config root, default `<project>/.claude`

### Skills and MCP

- `--skills-dir`: additional or explicit skills directory, repeatable
- `--skills-recursive`: recursively discover `SKILL.md`
- `--mcp`: register MCP server, repeatable

### Sandbox

- `--sandbox-backend`: `host|gvisor|govm`
- `--sandbox-project-mount`: `ro|rw|off`
- `--sandbox-image`: offline image override for `govm`

### Tags

- `--tag`: attach `key=value` or boolean-style `key`

### Internal

- `--agentkit-gvisor-helper`: hidden internal helper mode

## Troubleshooting

### `no prompt provided`

This happens only when:

- you are not in interactive mode
- stdin is not a TTY
- and no prompt source was provided

Use one of:

```bash
go run ./cmd/cli
go run ./cmd/cli --repl
go run ./cmd/cli --prompt "hello"
echo "hello" | go run ./cmd/cli
```

### `govm native runtime unavailable`

This means the CLI selected `govm`, but native runtime support is not available.

Check:

- you used `-tags govm_native`
- your platform is supported
- govm native assets are available

Example:

```bash
go run -tags govm_native ./cmd/cli --sandbox-backend=govm --repl
```

### `invalid --sandbox-project-mount`

Allowed values are:

- `ro`
- `rw`
- `off`

### Sandbox works, but the run still fails

If `govm` or `gvisor` initializes successfully and the failure happens later during model execution, the problem is likely:

- provider credentials
- provider base URL / gateway
- upstream model service availability

That is separate from sandbox initialization.

## Related Docs

- [README.md](/home/vipas/workspace/saker-ai/godeps/agentkit/README.md)
- [docs/getting-started.md](/home/vipas/workspace/saker-ai/godeps/agentkit/docs/getting-started.md)
- [docs/acp-integration.md](/home/vipas/workspace/saker-ai/godeps/agentkit/docs/acp-integration.md)
- [examples/02-cli/README.md](/home/vipas/workspace/saker-ai/godeps/agentkit/examples/02-cli/README.md)
- [examples/13-govm-sandbox/README.md](/home/vipas/workspace/saker-ai/godeps/agentkit/examples/13-govm-sandbox/README.md)
