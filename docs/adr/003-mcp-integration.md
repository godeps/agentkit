# ADR 003: MCP Integration

## Status
Proposed

## Context
MCP unlocks interoperability across IDEs, terminals, and custom shells.

## Decision
Expose an MCP client with both stdio and SSE transports so the agent runtime can communicate with hosts that prefer either mechanism.

## Consequences
- + Works with Claude Desktop and other MCP consumers out of the box.
- - Requires transport negotiation logic and testing across multiple hosts.
