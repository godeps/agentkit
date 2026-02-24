# ADR 001: Event Channels

## Status
Accepted

## Context
The architecture requires three logical channels (progress, control, monitor) to keep observability, orchestration, and control-plane signals independent.

## Decision
Adopt an EventBus abstraction with named channels and bookmarking support.

## Consequences
- + Clear separation of concerns.
- - Requires consumers to subscribe to multiple streams.
