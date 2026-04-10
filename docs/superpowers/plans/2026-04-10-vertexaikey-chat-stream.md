# VertexAI API Key Chat/Stream Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `VertexAI (API Key)` adapt one-api's standard chat and streaming chat flow without requiring client-side request or response changes.

**Architecture:** Keep one-api's standard request and response contracts unchanged, and give `vertexaikey` its own Vertex-specific request models and conversion logic. Reuse only the standard output shaping path where the Vertex response shape is already compatible, and add adapter tests that lock request JSON and request URL behavior.

**Tech Stack:** Go, Gin, existing relay adaptor framework, Go tests

---

### Task 1: Lock the desired adapter behavior with tests

**Files:**
- Create: `relay/adaptor/vertexaikey/adaptor_test.go`

- [ ] **Step 1: Write failing tests for request URL generation**
- [ ] **Step 2: Write failing tests for chat request conversion JSON shape**
- [ ] **Step 3: Write failing tests for stream request conversion invariants**
- [ ] **Step 4: Run the focused adapter tests and confirm failure**

### Task 2: Build Vertex-specific request conversion

**Files:**
- Create: `relay/adaptor/vertexaikey/model.go`
- Create: `relay/adaptor/vertexaikey/convert.go`
- Modify: `relay/adaptor/vertexaikey/adaptor.go`

- [ ] **Step 1: Add Vertex request structs with camelCase JSON tags**
- [ ] **Step 2: Convert one-api standard chat input into Vertex `generateContent` request bodies**
- [ ] **Step 3: Keep stream URL routing on `streamGenerateContent?alt=sse`**
- [ ] **Step 4: Re-run the focused adapter tests and confirm they pass**

### Task 3: Verify and document the service-level validation path

**Files:**
- Modify: `docs/superpowers/plans/2026-04-10-vertexaikey-chat-stream.md`

- [ ] **Step 1: Run focused Go tests for `vertexaikey`**
- [ ] **Step 2: Run a broader relay test command if the workspace supports it**
- [ ] **Step 3: Record the manual channel/service validation path using `controller/channel-test.go`**

**Manual validation path after starting the service:**
- Create or edit a `VertexAI (API Key)` channel that points at `https://aiplatform.googleapis.com` and includes a Gemini model such as `gemini-2.0-flash`.
- Use the existing channel test flow backed by `controller/channel-test.go` to send a standard chat request through `/v1/chat/completions`.
- Confirm the channel test succeeds for both normal chat and stream-enabled requests, and compare the returned standard response shape with an equivalent Gemini channel.
