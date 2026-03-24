# Multimodal Skill Runtime Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Evolve `agentkit` into a multimodal skill runtime that treats artifacts, multimodal tool results, pipeline steps, checkpointing, and execution traceability as first-class concepts.

**Architecture:** Keep concrete media processing in skills and external tools, not in the runtime core. The runtime itself will provide the substrate: a unified artifact model, multimodal tool result contract, lightweight pipeline orchestration, resumable checkpoint/cache primitives, and timeline-grade observability. The plan is staged so each phase produces a usable capability without forcing a heavyweight general-purpose graph framework.

**Tech Stack:** Go, existing `pkg/api` runtime, `pkg/tool`, `pkg/message`, `pkg/runtime/tasks`, `pkg/core/events`, CLI examples, Go test suite.

---

### Task 1: Define the artifact model

**Files:**
- Create: `pkg/artifact/doc.go`
- Create: `pkg/artifact/types.go`
- Create: `pkg/artifact/types_test.go`
- Modify: `pkg/message/message.go`
- Modify: `pkg/model/interface.go`

**Step 1: Write the failing test**

Add tests for:
- `Artifact` identity and kind classification
- `ArtifactRef` referencing local paths, URLs, and generated artifacts
- metadata fields for media type, size, checksum, and origin
- serialization safety for runtime transport

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/artifact`
Expected: FAIL because the package and types do not exist yet.

**Step 3: Write minimal implementation**

Introduce:
- `Artifact`
- `ArtifactRef`
- `ArtifactKind`
- `ArtifactMeta`
- helper constructors for local file, URL, and generated artifacts

Keep this task focused on the schema only. Do not add caching or lineage yet.

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/artifact`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/artifact/doc.go pkg/artifact/types.go pkg/artifact/types_test.go pkg/message/message.go pkg/model/interface.go
git commit -m "feat(artifact): define multimodal artifact model"
```

### Task 2: Upgrade the tool result contract for multimodal outputs

**Files:**
- Modify: `pkg/tool/types.go`
- Modify: `pkg/api/agent.go`
- Modify: `pkg/api/options.go`
- Create: `pkg/tool/result_test.go`
- Modify: `pkg/api/agent_test.go`

**Step 1: Write the failing test**

Add tests verifying a tool can return:
- text output
- structured payload
- artifact refs
- content blocks
- preview metadata

Ensure history append behavior and API response conversion preserve multimodal data.

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/tool ./pkg/api -run 'Test(ToolResult|RuntimeToolExecutor)'`
Expected: FAIL because tool results only support the current narrower shape.

**Step 3: Write minimal implementation**

Extend the result contract to include:
- `Artifacts`
- `Structured`
- `Preview`
- optional summary text alongside existing output fields

Preserve backward compatibility for text-only tools.

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/tool ./pkg/api -run 'Test(ToolResult|RuntimeToolExecutor)'`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/tool/types.go pkg/tool/result_test.go pkg/api/agent.go pkg/api/options.go pkg/api/agent_test.go
git commit -m "feat(tool): add multimodal tool result contract"
```

### Task 3: Add artifact lineage and cache keys

**Files:**
- Create: `pkg/artifact/lineage.go`
- Create: `pkg/artifact/cache.go`
- Create: `pkg/artifact/lineage_test.go`
- Create: `pkg/artifact/cache_test.go`

**Step 1: Write the failing test**

Add tests for:
- lineage edges between source and derived artifacts
- deterministic cache key generation from tool name, params, and artifact refs
- preserving provenance across multi-step pipelines

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/artifact -run 'Test(Lineage|CacheKey)'`
Expected: FAIL because lineage and cache primitives do not exist.

**Step 3: Write minimal implementation**

Introduce:
- `LineageEdge`
- `LineageGraph`
- `CacheKey`
- helper functions to derive cache keys from runtime inputs

Do not implement persistent cache storage yet.

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/artifact -run 'Test(Lineage|CacheKey)'`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/artifact/lineage.go pkg/artifact/cache.go pkg/artifact/lineage_test.go pkg/artifact/cache_test.go
git commit -m "feat(artifact): add lineage and cache key primitives"
```

### Task 4: Introduce lightweight multimodal pipeline steps

**Files:**
- Create: `pkg/pipeline/doc.go`
- Create: `pkg/pipeline/types.go`
- Create: `pkg/pipeline/types_test.go`
- Modify: `pkg/api/options.go`

**Step 1: Write the failing test**

Add tests defining:
- `Step`
- `Batch`
- `FanOut`
- `FanIn`
- `Conditional`
- `Retry`
- `Checkpoint`

The tests should focus on declaration shape, not execution.

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/pipeline -run 'Test(Step|Batch|FanOut|FanIn|Conditional|Retry|Checkpoint)'`
Expected: FAIL because the package and types do not exist.

**Step 3: Write minimal implementation**

Define a lightweight pipeline DSL aimed at multimodal task execution, not a general graph engine.

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/pipeline -run 'Test(Step|Batch|FanOut|FanIn|Conditional|Retry|Checkpoint)'`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/pipeline/doc.go pkg/pipeline/types.go pkg/pipeline/types_test.go pkg/api/options.go
git commit -m "feat(pipeline): define multimodal pipeline step model"
```

### Task 5: Execute pipeline steps through the runtime

**Files:**
- Create: `pkg/pipeline/executor.go`
- Create: `pkg/pipeline/executor_test.go`
- Modify: `pkg/api/agent.go`
- Modify: `pkg/api/options.go`

**Step 1: Write the failing test**

Add tests for:
- sequential step execution
- fan-out over artifact sets
- fan-in aggregation ordering
- retrying a failed step only

Use stub tools/skills to avoid real media processing.

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/pipeline -run 'TestExecutor'`
Expected: FAIL because execution is not implemented.

**Step 3: Write minimal implementation**

Build a runtime-backed pipeline executor that:
- invokes tools and skills through existing runtime surfaces
- propagates artifacts between steps
- records lineage edges for derived outputs

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/pipeline -run 'TestExecutor'`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/pipeline/executor.go pkg/pipeline/executor_test.go pkg/api/agent.go pkg/api/options.go
git commit -m "feat(pipeline): execute multimodal pipeline steps"
```

### Task 6: Add checkpoint and resume primitives for pipeline steps

**Files:**
- Create: `pkg/runtime/checkpoint/store.go`
- Create: `pkg/runtime/checkpoint/memory.go`
- Create: `pkg/runtime/checkpoint/store_test.go`
- Modify: `pkg/api/agent.go`
- Modify: `pkg/api/options.go`

**Step 1: Write the failing test**

Add tests for:
- checkpoint emitted after a checkpointable step
- resume from checkpoint continues remaining steps only
- resumable approval / human-in-the-loop pause

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/runtime/checkpoint ./pkg/api -run 'Test(Checkpoint|Resume|Interrupt)'`
Expected: FAIL because checkpoint and resume support does not exist.

**Step 3: Write minimal implementation**

Implement:
- `CheckpointStore`
- in-memory store
- interrupt result containing checkpoint identifier
- resume entrypoint for pipeline-backed runs

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/runtime/checkpoint ./pkg/api -run 'Test(Checkpoint|Resume|Interrupt)'`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/runtime/checkpoint/store.go pkg/runtime/checkpoint/memory.go pkg/runtime/checkpoint/store_test.go pkg/api/agent.go pkg/api/options.go
git commit -m "feat(checkpoint): add pipeline checkpoint and resume"
```

### Task 7: Add artifact cache storage

**Files:**
- Create: `pkg/runtime/cache/store.go`
- Create: `pkg/runtime/cache/memory.go`
- Create: `pkg/runtime/cache/store_test.go`
- Modify: `pkg/pipeline/executor.go`

**Step 1: Write the failing test**

Add tests for:
- cache hit skipping an expensive step
- cache miss executing normally
- cache lookup keyed by artifact refs and params

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/runtime/cache ./pkg/pipeline -run 'Test(Cache|ExecutorCache)'`
Expected: FAIL because no cache store exists.

**Step 3: Write minimal implementation**

Implement:
- cache store interface
- in-memory cache
- executor integration

Keep persistence out of this task.

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/runtime/cache ./pkg/pipeline -run 'Test(Cache|ExecutorCache)'`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/runtime/cache/store.go pkg/runtime/cache/memory.go pkg/runtime/cache/store_test.go pkg/pipeline/executor.go
git commit -m "feat(cache): add artifact cache for pipeline steps"
```

### Task 8: Build a multimodal timeline

**Files:**
- Create: `pkg/api/timeline.go`
- Create: `pkg/api/timeline_test.go`
- Modify: `pkg/api/stream.go`
- Modify: `pkg/api/agent.go`
- Modify: `pkg/clikit/runtime_adapter.go`

**Step 1: Write the failing test**

Add tests that require timeline entries for:
- input artifacts
- generated artifacts
- tool calls and tool results
- cache hit/miss
- checkpoint create/resume
- token and latency snapshots

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/api -run 'TestTimeline'`
Expected: FAIL because no multimodal timeline abstraction exists.

**Step 3: Write minimal implementation**

Implement a timeline collector and CLI renderer without removing existing JSON stream output.

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/api -run 'TestTimeline'`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/api/timeline.go pkg/api/timeline_test.go pkg/api/stream.go pkg/api/agent.go pkg/clikit/runtime_adapter.go
git commit -m "feat(debug): add multimodal execution timeline"
```

### Task 9: Separate structured results from text output

**Files:**
- Modify: `pkg/api/options.go`
- Modify: `pkg/api/agent.go`
- Create: `pkg/api/result_test.go`

**Step 1: Write the failing test**

Add tests verifying:
- `Result.Output` remains the final text answer
- `Result.Structured` carries structured output independently
- `Result.Artifacts` carries final artifact refs independently

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/api -run 'TestResult'`
Expected: FAIL because these fields do not exist yet.

**Step 3: Write minimal implementation**

Add new public response fields for structured output and final artifacts while preserving backward compatibility for text-only clients.

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/api -run 'TestResult'`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/api/options.go pkg/api/agent.go pkg/api/result_test.go
git commit -m "feat(api): separate text structured and artifact outputs"
```

### Task 10: Add multimodal skill-first examples

**Files:**
- Create: `examples/14-artifact-pipeline/README.md`
- Create: `examples/14-artifact-pipeline/main.go`
- Create: `examples/15-resumable-review/README.md`
- Create: `examples/15-resumable-review/main.go`
- Create: `examples/16-timeline/README.md`
- Create: `examples/16-timeline/main.go`
- Modify: `README.md`

**Step 1: Write the failing test**

Add smoke tests or example build tests ensuring the examples compile.

**Step 2: Run test to verify it fails**

Run: `go test ./examples/...`
Expected: FAIL because the examples do not exist.

**Step 3: Write minimal implementation**

Add examples covering:
- artifact-based skill pipeline
- resumable human review flow
- timeline inspection for a multimodal run

Use stubbed or lightweight skills so examples remain runnable.

**Step 4: Run test to verify it passes**

Run: `go test ./examples/...`
Expected: PASS

**Step 5: Commit**

```bash
git add examples/14-artifact-pipeline examples/15-resumable-review examples/16-timeline README.md
git commit -m "docs(examples): add multimodal runtime examples"
```

### Task 11: Final verification

**Files:**
- Modify: any touched files from Tasks 1-10 as needed

**Step 1: Format**

Run: `gofmt -w ./pkg ./examples`

**Step 2: Run targeted tests**

Run: `go test ./pkg/artifact ./pkg/pipeline ./pkg/runtime/checkpoint ./pkg/runtime/cache ./pkg/api`
Expected: PASS

**Step 3: Run full verification**

Run: `go test ./...`
Expected: PASS

**Step 4: Optional race verification**

Run: `go test -race ./...`
Expected: PASS or a clearly documented residual list.
