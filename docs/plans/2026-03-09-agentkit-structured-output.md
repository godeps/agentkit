# Agentkit Structured Output Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add first-class structured output support to Agentkit across runtime request construction and both OpenAI transport implementations.

**Architecture:** Extend the provider-agnostic `model.Request` shape with structured output metadata, expose it through `api.Options`, and map it in both OpenAI request builders. Keep the agent loop unchanged and scope first-version support to OpenAI-compatible transports only.

**Tech Stack:** Go, openai-go SDK, Agentkit runtime/model packages, standard `go test`.

---

### Task 1: Extend the model request shape

**Files:**
- Modify: `pkg/model/interface.go`
- Test: `pkg/model/openai_test.go`

**Step 1: Write the failing test**

- Add a model test that constructs a `model.Request` with structured output fields and verifies provider builders can access it.
- Cover `json_object` and `json_schema` input shapes.

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/model -run OpenAI`

Expected: FAIL because `Request` does not yet carry structured output fields.

**Step 3: Write minimal implementation**

- Add `ResponseFormat` and `OutputJSONSchema` types.
- Add `Request.ResponseFormat`.

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/model -run OpenAI`

Expected: PASS for the new compile/runtime expectations.

**Step 5: Commit**

```bash
git add pkg/model/interface.go pkg/model/openai_test.go
git commit -m "feat: add structured output request types"
```

### Task 2: Add runtime configuration and request injection

**Files:**
- Modify: `pkg/api/options.go`
- Modify: `pkg/api/agent.go`
- Test: `pkg/api/agent_test.go`
- Test: `pkg/api/options_test.go`

**Step 1: Write the failing tests**

- Add a test proving `Options.OutputSchema` survives option freezing/copying.
- Add a runtime test proving generated `model.Request` includes `ResponseFormat`.

**Step 2: Run tests to verify they fail**

Run: `go test ./pkg/api -run 'Options|Agent'`

Expected: FAIL because `Options` and runtime request construction do not yet include structured output configuration.

**Step 3: Write minimal implementation**

- Add `OutputSchema *model.ResponseFormat` to `api.Options`.
- Preserve it through option freezing/copying.
- Inject it into the request built in `pkg/api/agent.go`.

**Step 4: Run tests to verify they pass**

Run: `go test ./pkg/api -run 'Options|Agent'`

Expected: PASS.

**Step 5: Commit**

```bash
git add pkg/api/options.go pkg/api/agent.go pkg/api/agent_test.go pkg/api/options_test.go
git commit -m "feat: wire structured output through runtime options"
```

### Task 3: Implement chat completions structured output mapping

**Files:**
- Modify: `pkg/model/openai.go`
- Test: `pkg/model/openai_test.go`

**Step 1: Write the failing tests**

- Add tests for `json_object` mapping.
- Add tests for `json_schema` mapping with `Name`, `Description`, `Schema`, and `Strict`.
- Add tests for invalid schema inputs returning an error.

**Step 2: Run tests to verify they fail**

Run: `go test ./pkg/model -run OpenAI`

Expected: FAIL because `buildParams` does not set `ResponseFormat`.

**Step 3: Write minimal implementation**

- Add mapping logic in `buildParams`.
- Add validation/marshal errors with clear messages.

**Step 4: Run tests to verify they pass**

Run: `go test ./pkg/model -run OpenAI`

Expected: PASS.

**Step 5: Commit**

```bash
git add pkg/model/openai.go pkg/model/openai_test.go
git commit -m "feat: support structured output in openai chat completions"
```

### Task 4: Implement Responses API structured output mapping

**Files:**
- Modify: `pkg/model/openai_responses.go`
- Test: `pkg/model/openai_responses_test.go`

**Step 1: Write the failing tests**

- Add the same `json_object`, `json_schema`, and invalid-schema coverage for the Responses API builder.

**Step 2: Run tests to verify they fail**

Run: `go test ./pkg/model -run Responses`

Expected: FAIL because `buildResponsesParams` does not set structured output.

**Step 3: Write minimal implementation**

- Map `Request.ResponseFormat` into `responses.ResponseNewParams`.
- Keep behavior aligned with chat completions.

**Step 4: Run tests to verify it passes**

Run: `go test ./pkg/model -run Responses`

Expected: PASS.

**Step 5: Commit**

```bash
git add pkg/model/openai_responses.go pkg/model/openai_responses_test.go
git commit -m "feat: support structured output in openai responses API"
```

### Task 5: Document support boundaries

**Files:**
- Modify: `docs/api-reference.md`
- Modify: `README.md`
- Modify: `README_zh.md`

**Step 1: Write the doc updates**

- Document `model.ResponseFormat`
- Document `model.OutputJSONSchema`
- Document `api.Options.OutputSchema`
- Explicitly state first-version support scope: OpenAI-compatible chat completions and Responses API

**Step 2: Verify references are accurate**

Run: `rg -n "OutputSchema|ResponseFormat|structured output|json_schema" docs README.md README_zh.md`

Expected: new references appear only in the intended docs.

**Step 3: Commit**

```bash
git add docs/api-reference.md README.md README_zh.md
git commit -m "docs: describe structured output support"
```

### Task 6: Final regression verification

**Files:**
- Review: `pkg/model/interface.go`
- Review: `pkg/api/options.go`
- Review: `pkg/api/agent.go`
- Review: `pkg/model/openai.go`
- Review: `pkg/model/openai_responses.go`

**Step 1: Run focused package tests**

Run: `go test ./pkg/model ./pkg/api`

Expected: PASS.

**Step 2: Run full test suite**

Run: `go test ./...`

Expected: PASS.

**Step 3: Review final diff**

Run: `git diff --stat HEAD~5..HEAD`

Expected: only planned runtime/model/doc files are touched.

**Step 4: Commit any final cleanup**

```bash
git add -A
git commit -m "chore: finalize structured output rollout"
```
