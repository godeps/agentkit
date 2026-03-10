# Agentkit Tool Multimodal And Image Read Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add first-class multimodal tool results to Agentkit history flow and ship a builtin `image_read` tool that returns image content blocks.

**Architecture:** Extend `tool.ToolResult` with provider-agnostic `model.ContentBlock` payloads, then adapt the runtime tool executor to persist normal tool text results while appending a follow-up multimodal `user` message for providers that only accept content blocks on user turns. Build `image_read` on the existing file sandbox pattern so local image reads stay inside the configured root and builtin registration remains consistent with the other file tools.

**Tech Stack:** Go, Agentkit runtime/api/tool packages, standard library image MIME sniffing, `go test`

---

### Task 1: Extend tool result payload shape

**Files:**
- Modify: `pkg/tool/result.go`
- Test: `pkg/api/multimodal_test.go`

**Step 1: Write the failing test**

- Add a conversion-level test that a `tool.ToolResult` can carry `model.ContentBlock` values without the runtime dropping them when converting into history-facing message blocks.

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/api -run Multimodal`

Expected: FAIL because there is no tool-result content block path yet.

**Step 3: Write minimal implementation**

- Add `ContentBlocks []model.ContentBlock` to `tool.ToolResult`.

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/api -run Multimodal`

Expected: PASS.

### Task 2: Append multimodal tool outputs into history

**Files:**
- Modify: `pkg/api/agent.go`
- Test: `pkg/api/agent_test.go`

**Step 1: Write the failing test**

- Add a runtime test where a tool returns text plus image content blocks.
- Assert history contains:
  - a `tool` message with the textual result
  - a follow-up `user` message with converted content blocks

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/api -run ToolResult`

Expected: FAIL because the runtime only appends the text tool result today.

**Step 3: Write minimal implementation**

- Update the local `appendToolResult` helper in `runtimeToolExecutor.Execute`.
- Keep the existing `tool` message unchanged for text.
- When `ContentBlocks` is non-empty, append a synthetic `user` message carrying those blocks and a short marker string.

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/api -run ToolResult`

Expected: PASS.

### Task 3: Add builtin `image_read`

**Files:**
- Create: `pkg/tool/builtin/image_read.go`
- Test: `pkg/tool/builtin/image_read_test.go`

**Step 1: Write the failing tests**

- Add a test for reading a local PNG file and returning one image content block.
- Add a test for unsupported / non-image input returning an error.
- Add a test for sandbox/root-relative path handling.

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/tool/builtin -run ImageRead`

Expected: FAIL because the tool does not exist.

**Step 3: Write minimal implementation**

- Follow the `ReadTool` sandbox pattern.
- Accept `file_path`.
- Read bytes from sandboxed local path.
- Detect MIME type and allow only common image types.
- Return a success text summary and one base64-backed `model.ContentBlock`.

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/tool/builtin -run ImageRead`

Expected: PASS.

### Task 4: Register and expose the builtin

**Files:**
- Modify: `pkg/api/agent.go`
- Modify: `pkg/api/runtime_helpers_tools_test.go`
- Modify: `pkg/api/helpers_test.go`

**Step 1: Write the failing tests**

- Add a builtin-key test expecting `image_read` in the default set.
- Add a registration test verifying `EnabledBuiltinTools: []string{"image_read"}` wires the new tool.

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/api -run 'Builtin|RegisterTools'`

Expected: FAIL because builtin factories and ordering do not include `image_read`.

**Step 3: Write minimal implementation**

- Register `image_read` in builtin factories.
- Add `image_read` to builtin ordering and comments describing available builtins if needed.

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/api -run 'Builtin|RegisterTools'`

Expected: PASS.

### Task 5: Final verification

**Files:**
- Review: `pkg/tool/result.go`
- Review: `pkg/api/agent.go`
- Review: `pkg/tool/builtin/image_read.go`

**Step 1: Run focused tests**

Run: `go test ./pkg/api ./pkg/tool/builtin`

Expected: PASS.

**Step 2: Run broader regression**

Run: `go test ./...`

Expected: PASS.
