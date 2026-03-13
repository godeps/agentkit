# CLI Documentation Design

## Goal

Add a complete Markdown usage guide for `cmd/cli` that documents the current, verified behavior of the CLI.

## Decision

Create a new primary manual at `docs/cli.md` and link to it from the root `README.md`.

## Scope

The document will cover:

- interactive mode
- prompt/input resolution
- non-interactive and streaming modes
- ACP mode
- project/config layout
- skills and MCP
- sandbox backends, including govm defaults
- complete flag reference
- troubleshooting

The document will not cover SDK API usage or future CLI/TUI plans.

## Validation

Verify the document content against:

- `cmd/cli/main.go`
- `go run ./cmd/cli --help`
- interactive mode behavior
- current govm sandbox activation flow
