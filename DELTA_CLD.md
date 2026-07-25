# Claude beta Messages API compatibility delta

Audit date: 2026-07-24

Repository revision: `06f6bd47cefa`

Target: `POST /v1/messages`, compared with Anthropic's
[beta Messages create reference](https://platform.claude.com/docs/en/api/cli/beta/messages/create)

## Executive summary

Wingman's Anthropic endpoint implements the core Messages flow: synchronous and
streaming text, images and PDFs, multi-turn messages, client tools and tool
results, tool choice, stop sequences, structured JSON output, thinking,
selected computer/bash/text-editor/tool-search tools, basic usage, and a
compaction path.

It is **not wire-compatible with the complete current beta contract**. The most
important gaps are:

1. Required request validation is incomplete, and `max_tokens: 0` has the wrong
   behavior.
2. Many documented parameters are silently ignored rather than honored or
   rejected.
3. Required/documented response-envelope and usage fields are absent in both
   JSON and SSE responses.
4. The current content-block and tool unions are only partially implemented.
5. Cache controls, citations, context management, refusal/fallback data, and
   server-tool/MCP/container state are lost or unsupported.

The silent-ignore behavior is the highest compatibility risk: a client can
receive HTTP 200 even though Wingman did not apply important requested
semantics.

## Scope and method

The audit used the linked live reference as the authority. Because that page is
dynamically rendered, the generated types in Wingman's official
`github.com/anthropics/anthropic-sdk-go v1.58.0` dependency (released
2026-07-16) were also used to enumerate nested request/response unions. The
linked live reference wins if it differs from that SDK snapshot.

Primary implementation files inspected:

- `server/anthropic/models.go`
- `server/anthropic/handler_messages.go`
- `server/anthropic/convert.go`
- `server/anthropic/accumulator.go`
- `server/anthropic/handler.go`
- `pkg/provider/completer.go`
- `pkg/provider/anthropic/completer.go`
- `test/anthropic/**`
- `API.md`

This is a compatibility audit, not a requirement that every backend emulate
every Anthropic-only feature. A backend may reject an unsupported capability
with a clear Anthropic-shaped 4xx error. Silently accepting and discarding it is
not compatible behavior.

Status terms:

- **Supported**: represented and materially honored end-to-end.
- **Partial**: accepted but lossy, backend-dependent, or missing documented
  fields.
- **Ignored**: decoded or accepted, but not passed to completion behavior.
- **Missing**: not represented and therefore normally ignored or rejected.

## Current coverage

| Area | Status | Notes |
|---|---|---|
| `model`, `messages`, normal positive `max_tokens` | Partial | Core flow works, but required-field and range validation are incomplete. |
| Synchronous and SSE responses | Partial | Core event sequence works; current beta envelope, content, and usage shapes do not. |
| Text and multi-turn messages | Supported | Includes assistant history/prefill paths, subject to backend restrictions. |
| Image base64/URL and PDF base64/URL | Partial | Common forms work; Files API sources and document metadata/citations do not. |
| System prompt | Partial | Text is flattened; block cache controls/citations are discarded. |
| Client tools and tool results | Partial | Basic fields work; `strict`, `is_error`, caching, callers, examples, and newer fields are lost. |
| `tool_choice` | Partial | Known modes map; invalid/underspecified choices are not consistently rejected. |
| Structured output | Supported/partial | `output_config.format` and deprecated `output_format` map to the common schema. Exact validation behavior varies by backend. |
| Thinking | Partial | Adaptive/display works; fixed `budget_tokens` is converted to a coarse effort rather than preserved. |
| Context management | Partial | Only compaction threshold is represented. |
| Prompt caching | Partial | Anthropic upstream is auto-cached, but request-defined placement/TTL is not honored. |
| Computer, bash, text editor, tool search | Partial | Selected variants are normalized; variant-specific options are lost. |
| MCP, code execution, memory, web tools, advisor, containers/skills | Missing | The public Messages surface cannot represent these current beta features. |

## P0: contract correctness

### CLD-001 — Required fields and `max_tokens` semantics

Official request requirements:

- `model` is required.
- `messages` is required.
- `max_tokens` is required and may be `0` for prompt-cache prewarming.

Current behavior:

- `MessageRequest` uses non-pointer value fields and has no presence-aware
  validation.
- A missing `model` can resolve to the default completer registered under the
  empty model key.
- A missing or empty `messages` array is not rejected at the HTTP boundary.
- `max_tokens <= 0` is not put into `CompleteOptions`.
- Consequently, `max_tokens: 0` selects the backend's normal default and can
  generate a response instead of prewarming the prompt cache.
- Negative values are also treated as "unspecified" instead of rejected.

Required change:

- Decode required scalar fields with presence-aware types or a validation
  layer.
- Reject missing `model`, missing `messages`, missing `max_tokens`, and negative
  `max_tokens` with `invalid_request_error`.
- Implement `max_tokens: 0` cache-prewarm semantics, or explicitly reject it as
  unsupported before calling a backend. Do not generate normal output.

### CLD-002 — Unknown and unsupported input is silently discarded

`json.Decoder` is used without unknown-field or trailing-payload validation.
Several unknown top-level fields therefore disappear. Unknown content-block
types also fall through `toMessage` without an error.

Examples:

- `fallbacks`, `mcp_servers`, `speed`, or `service_tier` can be accepted and
  ignored.
- A `search_result`, `mcp_tool_result`, `container_upload`,
  `mid_conv_system`, or `fallback` input block can be dropped from the prompt.
- An unknown `thinking.type`, effort, output format, or tool choice can fall
  back to unrelated/default behavior.

Required change:

- Validate discriminated unions and documented constraints explicitly.
- For a documented but unsupported capability, return a clear Anthropic-shaped
  4xx error naming the field or block.
- For an unknown field, choose and document a compatibility policy. Strict
  rejection is safest for semantics-bearing fields; deliberate pass-through is
  acceptable only when the downstream provider actually receives the value.
- Ensure the request body contains exactly one JSON value.

### CLD-003 — Missing response envelope fields

The current beta response model includes additional envelope state. Wingman's
`Message` does not expose:

| Response field | Wingman | Impact |
|---|---|---|
| `container` | Missing | Container/code-execution state cannot be returned or reused. |
| `context_management` | Missing | Applied context edits are not reported. |
| `diagnostics` | Missing | Opted-in cache-miss diagnostics cannot be returned. |

The same divergence appears in streaming:

- `message_start.message` lacks these documented message fields.
- `message_delta` lacks top-level `context_management`.
- `message_delta.delta` lacks `container`.

Required change:

- Add the documented keys with correct object/null behavior in non-streaming
  and SSE shapes.
- Preserve backend-returned values where supported.
- Where a feature was not requested or cannot apply, emit the documented null
  or empty form rather than silently changing the schema.

### CLD-004 — Required response usage fields are absent

Wingman currently returns basic input/output/cache counts and optional thinking
tokens. It does not return the full current usage contract:

| Usage field | Non-streaming | `message_delta.usage` |
|---|---:|---:|
| `cache_creation` TTL breakdown | Type exists but is never populated | Missing |
| `iterations` | Missing | Missing |
| `server_tool_use` | Missing | Missing |
| `service_tier` | Type exists but is never populated | Not part of delta usage |
| `inference_geo` | Type exists but is never populated | Not part of delta usage |
| `speed` | Missing | Not part of delta usage |
| `output_tokens_details` | Partial | Partial |

`iterations` is especially relevant to compaction, advisor calls, server-side
fallbacks, and multiple sampling loops. The common `provider.Usage` type has no
place to retain it.

Required change:

- Extend the common usage model or add an Anthropic-native side channel that
  preserves these fields.
- Emit documented empty/zero/null values when applicable.
- Add sync and SSE golden tests that assert field presence, not only selected
  token totals.

### CLD-005 — Text citations and streaming citation deltas are missing

Documented text response blocks carry `citations` (an empty array when none are
present). Wingman's `ContentBlock` has no citations field, so ordinary text
blocks already differ in shape.

The SSE delta union also lacks:

- `citations_delta`
- its citation-location union
- `thinking_delta.estimated_tokens` when the corresponding beta is enabled

Required change:

- Model all citation location variants and preserve them through the provider
  layer.
- Include `citations` in text block starts and final text blocks.
- Add `citations_delta` and beta-gated `estimated_tokens` streaming support.

## P1: missing request semantics

### CLD-006 — Top-level request fields missing or ignored

| Official JSON/header field | Current state | Required implementation behavior |
|---|---|---|
| `fallback_credit_token` | Missing | Preserve/reforward credit-token retry semantics or reject. |
| `fallbacks` (CLI `--fallback`) | Missing | Support ordered server-side refusal fallback attempts or reject. |
| `container` | Missing | Support ID reuse and `{id, skills}` form or reject. |
| `inference_geo` | Missing | Honor routing constraint and report actual geo, or reject. |
| `speed` (`standard`/`fast`) | Missing | Honor and report selected speed, or reject. |
| top-level `cache_control` | Missing | Apply the automatic last-cacheable-block marker and TTL. |
| `diagnostics.previous_message_id` | Missing | Return cache divergence diagnostics or reject. |
| `mcp_servers` | Missing | Support connector server definitions and MCP toolsets or reject. |
| `service_tier` | Missing | Honor `auto`/`standard_only` and report actual tier, or reject. |
| `output_config.task_budget` | Missing | Preserve `{type:"tokens", total, remaining}` semantics or reject. |
| `metadata.user_id` | **Ignored** | It is decoded but never reaches the provider or abuse-tracking layer. |
| `top_p` | **Ignored** | It is decoded but never added to `CompleteOptions`. |
| `top_k` | **Ignored** | It is decoded but never added to `CompleteOptions`. |
| `anthropic-user-profile-id` header | Missing | Attribute the request when the required beta is enabled, or reject. |
| `anthropic-beta` header | Ignored | No beta validation, gating, or behavior selection. |
| `anthropic-version` header | Ignored | No version validation or compatibility selection. |

`API.md` currently advertises `top_p`, `top_k`, and `metadata`, even though the
handler does not honor them.

### CLD-007 — Cache control is not request-compatible

The beta contract permits cache control at the request level and on cacheable
system, content, and tool blocks, with `5m` and `1h` TTLs.

Current behavior:

- Top-level `cache_control` is not modeled.
- `SystemBlock.CacheControl` is parsed but discarded when system text is
  flattened.
- Message `ContentBlockParam` has no `cache_control` field.
- `ToolParam` has no `cache_control` field.
- `CacheControl` has no `ttl`.
- The Anthropic provider unconditionally sets a top-level ephemeral cache
  marker, which is not equivalent to honoring caller-selected placement/TTL.

Required change:

- Preserve explicit cache markers and TTL through the common provider model.
- Do not replace caller cache policy with an unconditional default.
- If another backend cannot emulate placement/TTL, reject the unsupported
  combination rather than claiming it was used.

### CLD-008 — Thinking configuration is lossy and under-validated

Current behavior:

- `thinking.type: "enabled"` with `budget_tokens` becomes adaptive thinking plus
  a coarse effort derived from the budget.
- The exact budget is not sent to Anthropic or other providers.
- The documented `budget_tokens >= 1024` and `< max_tokens` constraints are not
  validated at the server boundary.
- Unknown thinking types become adaptive rather than erroring.
- Invalid `display` values are not rejected.

Required change:

- Represent fixed-budget and adaptive thinking separately in
  `provider.ReasoningOptions`.
- Preserve the exact fixed budget for Anthropic backends.
- Validate the tagged union and model-specific restrictions before starting a
  response.

### CLD-009 — Context management only preserves a compaction threshold

Official edit variants include:

- `clear_tool_uses_20250919`
- `clear_thinking_20251015`
- `compact_20260112`

Current behavior:

- Only edit types beginning with `compact` have any effect.
- Compaction only retains `trigger.value`.
- `instructions` and `pause_after_compaction` are not represented.
- Clear-tool-use configuration (`trigger`, `keep`, `clear_at_least`,
  `clear_tool_inputs`, `exclude_tools`) is ignored.
- Clear-thinking `keep` is ignored.
- Applied edit details are not returned.
- A paused compaction cannot produce the documented `stop_reason:
  "compaction"`.

Required change:

- Model each edit as a strict tagged union.
- Preserve compaction instructions and pause behavior.
- Return applied edits and the correct stop reason.
- Explicitly reject strategies that the selected backend cannot support.

### CLD-010 — Message/content input union is incomplete and lossy

Supported input blocks are effectively limited to:

- `text`
- `image` with base64 or URL source
- `document` with base64, URL, or plain-text source
- `thinking`, `redacted_thinking`
- `tool_use`, `tool_result`
- `compaction`
- lossy marker handling for a few server-tool blocks

Missing or incomplete current beta input shapes include:

- `search_result`
- advisor, code-execution, bash-code-execution, text-editor-code-execution, and
  tool-search result blocks
- native `mcp_tool_use` / `mcp_tool_result`
- `container_upload`
- `mid_conv_system`
- `fallback`
- Files API `source.type: "file"` for images and documents
- document `context`, `title`, and `citations`
- text-block citations
- per-block cache controls

Additional correctness issues:

- `tool_result.is_error` is parsed but discarded because
  `provider.ToolResult` has no error flag.
- Document `source.type: "content"` is declared in the local type comment but
  is not implemented by `toFile`; it can become an empty file.
- Unknown block types are silently skipped.
- Server-tool history is converted to human-readable marker text instead of
  round-tripped in its native shape. This can change subsequent model behavior
  and signature validation.

Required change:

- Implement a strict content-block union with lossless native payload storage.
- Preserve native blocks through the common provider layer where possible.
- Reject unsupported blocks before completion instead of dropping them.

### CLD-011 — Tool union and custom-tool fields are incomplete

Custom tool fields missing at the public Anthropic boundary:

- `strict`
- `eager_input_streaming`
- `allowed_callers`
- `cache_control`
- `input_examples`

`defer_loading` is represented for custom tools, but other documented
tool-search interactions and validation are incomplete.

Built-in tools currently accepted by prefix:

- computer use
- bash
- text editor
- tool search

Missing built-in families include:

- code execution variants
- memory
- web search and web fetch variants
- advisor
- MCP toolsets

Variant-specific options are also lost, including such fields as web domain
filters/location/max uses, code-execution callers, strict/deferred behavior,
computer zoom, and MCP tool configuration. The response layer cannot emit most
corresponding server-tool result blocks.

Required change:

- Model tools as a tagged union rather than one permissive `ToolParam`.
- Preserve common custom-tool fields in `provider.Tool`.
- Add native server-tool result support.
- Return a field-specific 4xx for a known but unsupported built-in variant.

## P1: response and stop-state fidelity

### CLD-012 — Content response union is incomplete

Wingman can emit:

- `text`
- `thinking`
- `redacted_thinking`
- client `tool_use`
- `compaction`
- a limited `server_tool_use` representation for tool search

It cannot natively emit the remaining documented server/MCP/container/fallback
blocks:

- `web_search_tool_result`
- `web_fetch_tool_result`
- `advisor_tool_result`
- `code_execution_tool_result`
- `bash_code_execution_tool_result`
- `text_editor_code_execution_tool_result`
- `tool_search_tool_result`
- `mcp_tool_use`
- `mcp_tool_result`
- `container_upload`
- `fallback`

The common `provider.Content` type cannot retain these shapes, so even an
Anthropic upstream response loses them before the public response is built.

Required change:

- Add a lossless provider-native content variant or explicit common variants.
- Preserve block order, caller data, IDs, encrypted content, and result/error
  unions through sync accumulation and SSE.

### CLD-013 — Stop reasons and refusal/fallback details are incomplete

Wingman defines/emits:

- `end_turn`
- `max_tokens`
- `stop_sequence`
- `tool_use`
- `refusal`

Missing stop reasons:

- `pause_turn`
- `compaction`
- `model_context_window_exceeded`

The Anthropic provider currently folds `model_context_window_exceeded` into the
generic incomplete state, which the public endpoint re-emits as `max_tokens`.

Current `stop_details` only carries `type`, `category`, and `explanation`.
Current refusal details also include fallback-related fields such as:

- `fallback_credit_token`
- `fallback_has_prefill_claim`
- `recommended_model`

These are dropped in both JSON and SSE. Fallback boundary blocks and fallback
iteration usage are also absent.

Required change:

- Preserve the native stop reason in `provider.Completion`.
- Extend refusal details and fallback state.
- Add non-streaming and streaming round-trip tests for every stop reason.

## P2: validation, errors, and tests

### CLD-014 — Validation behavior differs from the reference

Examples:

- Invalid or unknown `tool_choice.type` can be ignored.
- `tool_choice: {type:"tool"}` does not validate a non-empty name.
- Unknown effort and output-format types can be ignored.
- `metadata.user_id` length/opacity guidance is not enforced.
- Sampling ranges are not validated at the HTTP boundary.
- Current reference deprecates temperature for newer models; Wingman can still
  forward arbitrary values to non-Anthropic backends.
- Required content-block fields and role/block compatibility are not
  consistently checked.
- Error messages generally come from conversion or an upstream backend rather
  than matching Anthropic field paths and constraints.

Required change:

- Add request validation before policy/provider execution.
- Use stable field paths in errors (`messages.2.content.0...`, `tools.1...`).
- Keep error type/status/header parity, including `request-id` if Wingman intends
  full client compatibility.

### CLD-015 — Differential tests do not cover the current beta surface

Existing tests are useful for core text/tools/streaming/thinking/cache usage and
selected built-in tools. Missing coverage includes:

- Required-field and `max_tokens: 0` behavior
- Explicit ignored-field detection for `top_p`, `top_k`, and metadata
- All missing top-level beta fields
- Response envelope field presence
- Full usage schema and iteration records
- Citations and `citations_delta`
- MCP/container/server-tool blocks
- All context-management edit variants and applied-edit responses
- All stop reasons and expanded refusal details
- Strict rejection of unsupported/unknown content and tools
- Beta header gating

Required change:

- Generate a field/block inventory test from the official SDK schema snapshot.
- Add golden JSON and SSE fixtures for required field presence.
- Add live differential tests for small, deterministic requests where account
  betas are available.
- Add explicit capability tests proving unsupported features fail rather than
  disappear.

## Recommended implementation order

1. **Make acceptance trustworthy.** Add presence/range/union validation,
   implement `max_tokens: 0`, and reject all currently ignored semantics.
2. **Fix baseline response shape.** Add required message, text citation, usage,
   and SSE envelope fields, even when values are empty/null.
3. **Preserve native data.** Extend `provider.Content`, `Usage`,
   `StopDetails`, and options with lossless/native variants so an Anthropic
   upstream can round-trip without degradation.
4. **Complete core controls.** Implement top-level routing/speed/tier/cache,
   metadata, fixed thinking budgets, and all context-management edits.
5. **Add advanced beta families.** MCP, containers/skills, server tools,
   fallbacks, diagnostics, user profiles, and task budgets.
6. **Lock with schema and differential tests.** Update `API.md` only after each
   field is honored or explicitly documented as rejected.

## Suggested compatibility policy across backends

For each request feature, resolve one of these outcomes before generation:

1. Native support: preserve and forward it.
2. Exact safe emulation: translate it and document the mapping.
3. Unsupported: return `400 invalid_request_error` identifying the field.

Avoid best-effort acceptance for routing, budget, caching, tool execution,
context management, or safety/refusal features. Those fields affect cost,
location, side effects, or conversation correctness and must not be silently
discarded.

## Verification performed

The focused existing unit test suites pass:

```text
go test ./server/anthropic ./pkg/provider/anthropic
ok github.com/adrianliechti/wingman/server/anthropic
ok github.com/adrianliechti/wingman/pkg/provider/anthropic
```

This confirms the current implementation baseline; it does not close the
contract gaps above.
