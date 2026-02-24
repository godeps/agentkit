# ADR 002: WAL Persistence

## Status
Accepted

## Context
Checkpoint/resume/fork semantics require durable logs that survive crashes and enable replay.

## Decision
Use a JSONL + WAL hybrid store for session transcripts and metadata.

## Consequences
- + Reproducible runs and resumable workflows.
- - Additional IO overhead and rotation rules to manage. 
