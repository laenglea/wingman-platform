# Gemini GenerateContent API compatibility delta

Audit date: 2026-07-24  
Repository revision: `06f6bd47cefa`  
Primary targets:

- `POST /v1beta/models/{model}:generateContent`
- `POST /v1beta/models/{model}:streamGenerateContent`

Compared with Google's
[GenerateContent API reference](https://ai.google.dev/api/generate-content)
and the official
[Gemini API v1beta discovery document](https://generativelanguage.googleapis.com/$discovery/rest?version=v1beta),
revision `20260721`.

## Executive summary

Wingman implements a useful core Gemini subset: lower-camel-case text and
inline-media requests, text-only system instructions, multi-turn messages,
basic function calling and function responses, structured JSON output, thought
parts/signatures, three generation controls, synchronous responses, JSON-array
streaming, SSE streaming, and basic token usage.

It is **not wire- or behavior-compatible with the complete current
GenerateContent contract**. The highest-risk differences are:

1. Official REST examples using protobuf snake-case names and singleton
   `contents`/`parts` objects do not decode correctly.
2. Safety settings and many other semantics-bearing request fields are accepted
   and silently ignored.
3. Upstream Gemini prompt-block, safety-rating, and finish-reason information is
   discarded; a blocked or truncated response can be rewritten as a successful
   candidate with `finishReason: "STOP"`.
4. `fileData` URIs are converted into inline bytes containing the URI string,
   while generated image/audio/file parts are dropped from the Gemini response.
5. Most generation controls, multiple candidates, built-in tools, grounding,
   citations, logprobs, and current response metadata are unsupported.
6. The adjacent advertised `countTokens` endpoint uses a different request
   shape from Google and returns a local heuristic rather than the selected
   model's tokenizer result.

The silent-ignore behavior is the principal correctness problem. A backend does
not have to emulate every Google-specific feature, but Gemini compatibility
requires either honoring a requested capability or returning a clear
Google-shaped 4xx error. Returning HTTP 200 after discarding safety, sampling,
tool, storage, or service-tier instructions is not compatible behavior.

## Scope and method

The live Google reference was last updated 2026-07-21. Its v1beta discovery
document, also revision `20260721`, was used to enumerate exact schema fields.
Wingman's `google.golang.org/genai v1.64.0` dependency, released 2026-07-16, was
used as a secondary check on nested types. The live reference/discovery schema
wins if the SDK snapshot differs.

Primary implementation files inspected:

- `server/gemini/models.go`
- `server/gemini/handler_generate.go`
- `server/gemini/convert.go`
- `server/gemini/accumulator.go`
- `server/gemini/handler_tokens.go`
- `server/gemini/handler.go`
- `pkg/provider/completer.go`
- `pkg/provider/provider.go`
- `pkg/provider/google/completer.go`
- `test/gemini/**`

Status terms:

- **Supported**: represented and materially honored end-to-end.
- **Partial**: accepted but lossy, backend-dependent, or missing documented
  response information.
- **Ignored**: decoded or accepted, but not applied to completion behavior.
- **Missing**: not represented and therefore normally ignored.

## Current coverage

| Area | Status | Notes |
|---|---|---|
| `models/{model}:generateContent` | Partial | Route and core text flow work. Request and response contracts are subsets. |
| `models/{model}:streamGenerateContent` | Partial | Both JSON-array and `alt=sse` framing work; chunk semantics and metadata differ. |
| Text and multi-turn `user`/`model` content | Supported | Invalid roles are coerced to `user` instead of rejected. |
| Text-only `systemInstruction` | Partial | Lower-camel array form works; parts are flattened with newlines. |
| `inlineData` input | Supported/partial | Base64 is decoded and passed as a generic file; part metadata and per-part resolution are lost. |
| `fileData` input | Incorrect | The URI string becomes inline payload bytes. |
| Generated image/audio/file output | Missing | Provider file content is discarded by the Gemini response converter. |
| Function declarations/calls/responses | Partial | Basic synchronous functions work; current schemas and non-blocking/server-side forms do not. |
| Built-in Google tools | Missing | Search, URL context, code execution, file search, Maps, computer use, and MCP are ignored. |
| Safety settings and safety feedback | Incorrect | Request settings are ignored and response safety state is discarded. |
| Generation controls | Partial | Only `stopSequences`, `temperature`, and `maxOutputTokens` are directly forwarded. |
| Structured output | Partial | Schemas work in common cases, but MIME/schema validation and schema dialect are changed. |
| Thinking | Partial | Include/level generally work; exact budgets and zero-budget behavior do not. |
| Candidate count and logprobs | Missing | Only the first candidate's content survives. |
| Usage metadata | Partial | Basic scalar counts survive; modality, tool-use, and service-tier detail does not. |
| Prompt caching, service tier, storage | Missing | `cachedContent`, `serviceTier`, and `store` are not represented. |

## P0: contract correctness

### GNI-001 — Published REST request forms are incompatible

Google's own shell examples use both protobuf field names and singleton forms.
For example, the current system-instruction example sends:

```json
{
  "system_instruction": {
    "parts": {"text": "You are a cat."}
  },
  "contents": {
    "parts": {"text": "Hello there"}
  }
}
```

Other official shell examples use `generation_config`,
`response_mime_type`, `response_schema`, `function_declarations`,
`tool_config`, and `function_calling_config`.

Wingman decodes directly into Go structs containing only lower-camel JSON tags
and slice-typed `contents`/`parts` fields
(`server/gemini/models.go:18-24`, `server/gemini/models.go:33-47`,
`server/gemini/handler_generate.go:147-153`).

Consequences:

- Singleton `contents` or `parts` causes a 400 JSON type error.
- Snake-case `system_instruction` is silently ignored when the rest of the body
  happens to decode.
- Snake-case nested tool/config fields are silently ignored, often leaving an
  empty tool or default generation behavior.
- Existing tests exercise only Wingman's lower-camel array form, so they do not
  catch this delta.

Required change:

- Use protobuf-compatible JSON decoding or custom compatibility unmarshallers
  that accept both lower-camel and original snake-case names.
- Accept the singleton repeated-field forms that Google's published REST
  samples accept, normalizing them to arrays internally.
- Add the current official shell payloads as handler-level conformance tests.

### GNI-002 — Required fields, unions, ranges, and unknown input are not validated

`contents` is required by `generateContent`, but Wingman does not check its
presence or require a non-empty prompt. The decoder also does not reject unknown
fields or trailing JSON.

Other missing validation includes:

- `Content.role` must be `user` or `model`; unknown and empty roles are coerced
  to `user` (`server/gemini/convert.go:51-61`).
- The `Part` data fields form a union, but a single Wingman part may contain
  text, inline data, a function call, and a function response; these are split
  and processed together rather than rejected.
- Tool and function names, allowed-function modes, duplicate safety categories,
  stop-sequence count, numeric ranges, and mutually exclusive schema fields are
  not checked.
- An unknown function-calling mode falls back to backend defaults rather than
  producing `INVALID_ARGUMENT`.
- Unknown documented fields disappear because `json.Decoder` is used without a
  presence-aware validation layer.
- A missing configured model is a plain error and defaults to HTTP 400 rather
  than the expected not-found classification.

Required change:

- Add presence-aware request validation before resolving or calling a backend.
- Validate tagged unions, enum values, mutual exclusion, numeric limits, and
  model/tool constraints.
- Ensure the body contains exactly one JSON value.
- Reject unsupported semantics explicitly instead of silently dropping them.

### GNI-003 — Safety policy and safety outcomes are not preserved

The request model declares `safetySettings`
(`server/gemini/models.go:18-25`), but `parseGenerateRequest` never reads or
forwards it (`server/gemini/handler_generate.go:147-259`). The caller therefore
receives no indication that its requested category thresholds were ignored.

The reverse path is also lossy. The Google provider copies only the first
candidate's content and aggregate usage into `provider.Completion`; it discards
`PromptFeedback`, `Candidate.FinishReason`, `FinishMessage`, and
`SafetyRatings` (`pkg/provider/google/completer.go:65-89`). The Gemini handler
then derives a finish reason from the provider-neutral status, whose empty
default maps to `STOP` (`server/gemini/convert.go:338-356`).

A prompt-block response from Google has prompt feedback and no candidates.
Wingman's accumulator nevertheless always returns a non-nil empty message, so
the non-streaming handler can emit a fabricated candidate with `STOP` instead of
the documented `promptFeedback` and empty candidate list. The same defaulting
occurs in the final stream chunk.

Impact:

- Caller-selected safety thresholds do not take effect.
- Safety blocks, recitation, prohibited content, SPII, malformed calls, missing
  thought signatures, image safety, and other current finish reasons can be
  misreported as a normal stop.
- Clients cannot distinguish a blocked prompt from a successful empty response.

Required change:

- Preserve safety configuration through the provider layer where supported, or
  reject it before inference.
- Add provider-neutral/native fields for prompt feedback, candidate safety, and
  exact finish reasons.
- Never synthesize a successful candidate when the backend returned only prompt
  feedback.

### GNI-004 — URI media is corrupted and generated media is dropped

For a request `Part.fileData`, Wingman stores `fileUri` as `[]byte(uri)` in a
generic `provider.File` (`server/gemini/convert.go:151-158`). The Google
provider later converts every generic file with `genai.NewPartFromBytes`
(`pkg/provider/google/completer.go:209-220`). The model therefore receives
inline bytes such as `gs://bucket/object` or a Files API URI, not a URI
reference to the media.

The same corruption affects `FunctionResponse.parts[].fileData`.

On output, the Google provider correctly converts upstream `inlineData` and
`fileData` to `provider.File` (`pkg/provider/google/completer.go:435-479`), but
the public Gemini converter handles only reasoning, text, and tool calls
(`server/gemini/convert.go:242-336`). Image, audio, video, and other generated
files disappear from both synchronous and streaming responses.

The public `Part` type is also missing `videoMetadata`, per-part
`mediaResolution`, `partMetadata`, `executableCode`, `codeExecutionResult`,
`toolCall`, and `toolResponse`.

Required change:

- Give provider files an explicit inline-bytes versus URI-source
  representation and preserve it in both directions.
- Serialize provider file output as `inlineData` or `fileData`, including MIME
  type and display name where available.
- Add the remaining `Part` variants or reject them explicitly.
- Add image, audio, video, PDF URI, and generated-image round-trip tests.

### GNI-005 — Most generation controls are silent no-ops

`GenerationConfig` declares many current controls, but the handler forwards only:

- `stopSequences`
- `temperature`
- `maxOutputTokens`
- a lossy structured-output schema
- a lossy thinking configuration

See `server/gemini/handler_generate.go:198-257`.

Fields that are decoded but have no effect:

| Field | Current result |
|---|---|
| `topP` | Ignored |
| `topK` | Ignored |
| `candidateCount` | Ignored; only one candidate is ever returned |
| `seed` | Ignored |
| `presencePenalty` | Ignored |
| `frequencyPenalty` | Ignored |
| `responseLogprobs` | Ignored |
| `logprobs` | Ignored |
| `responseMimeType` | Ignored unless a schema causes Wingman to force JSON |
| `responseModalities` | Ignored |
| `mediaResolution` | Ignored |

Current official fields absent from Wingman's request type include
`speechConfig`, `imageConfig`, `audioTranscriptionConfig`,
`enableEnhancedCivicAnswers`, `enableAffectiveDialog`, `responseFormat`, and
`translationConfig`.

At the request top level, `cachedContent`, `serviceTier`, and `store` are also
missing. They are accepted as unknown JSON and silently discarded.

Candidate handling compounds this problem: the Google provider reads only
`resp.Candidates[0]` (`pkg/provider/google/completer.go:81-85`), and the Gemini
handlers always build one candidate at index zero.

Required change:

- Extend `provider.CompleteOptions` for portable controls and add a
  protocol-native option channel for controls that cannot be generalized.
- Preserve all candidates or explicitly reject `candidateCount != 1`.
- Reject unsupported fields with `INVALID_ARGUMENT` until they are implemented.
- Track field presence so explicit `false`/zero values are distinguishable from
  omission.

## P1: feature and fidelity gaps

### GNI-006 — Tool support is limited to basic client functions

The current `Tool` union includes:

- `functionDeclarations`
- `codeExecution`
- `googleSearch` and deprecated `googleSearchRetrieval`
- `computerUse`
- `urlContext`
- `fileSearch`
- `mcpServers`
- `googleMaps`

Wingman's public type contains only `functionDeclarations`
(`server/gemini/models.go:96-107`). Sending any built-in tool decodes to an
empty tool and produces no error.

Function-specific gaps:

- `FunctionDeclaration.response`, `responseJsonSchema`, and `behavior` are
  missing.
- The distinction between OpenAPI `parameters` and JSON Schema
  `parametersJsonSchema` is erased; both become a normalized common schema.
- `FunctionResponse.scheduling` and `willContinue` are missing.
- `ToolConfig.retrievalConfig` and `includeServerSideToolInvocations` are
  missing.
- Server-side `Part.toolCall` and `Part.toolResponse` cannot round-trip.
- Function-call arguments that fail internal JSON parsing are replaced with
  `{}` instead of surfacing `MALFORMED_FUNCTION_CALL`
  (`server/gemini/convert.go:292-301`).

Required change:

- Model the complete tool union and function schemas.
- Preserve server-side call/response parts when requested.
- For cross-provider models, return a clear unsupported-tool error when a
  Google-hosted tool cannot be emulated.

### GNI-007 — `VALIDATED` and thinking budgets change meaning

Function calling:

- Incoming `VALIDATED` is mapped to common tool choice `ANY` plus per-tool
  strictness (`server/gemini/handler_generate.go:169-195`).
- On the Google path, `ANY` is assigned before strictness is inspected.
  Strictness upgrades only empty/`AUTO` mode, so the outgoing Gemini mode
  remains `ANY`, not `VALIDATED`
  (`pkg/provider/google/completer.go:147-179`).

Thinking:

- A positive `thinkingBudget` is converted to coarse low/medium/high effort,
  losing the exact token budget.
- `thinkingBudget: 0` by itself fails the `wantThinking` condition and is
  ignored, so it does not request disabled/minimal thinking
  (`server/gemini/handler_generate.go:225-255`).
- If both level and budget are present, a recognized level wins silently rather
  than enforcing the documented/model-specific constraints.
- The common effort mapping changes exact Google behavior even when the selected
  backend is Google.

Required change:

- Preserve `VALIDATED` as a distinct common/native mode.
- Represent exact thinking budget separately from level/effort.
- Validate level/budget combinations and preserve explicit zero.

### GNI-008 — Structured-output MIME and schema semantics are altered

Current behavior:

- `responseJsonSchema` takes precedence if both schema fields are present;
  mutual exclusion is not validated.
- Any accepted schema causes downstream Google configuration to force
  `responseMimeType = "application/json"`, regardless of the caller's MIME type
  (`pkg/provider/google/completer.go:194-197`).
- `responseSchema` and `responseJsonSchema` are both carried through the same
  arbitrary map and later sent as `ResponseJsonSchema`; the OpenAPI versus JSON
  Schema distinction is lost.
- A schema without a compatible MIME type, which Google documents as invalid,
  can succeed through Wingman.
- `text/x.enum` and MIME-only output requests are ignored.

Required change:

- Validate schema mutual exclusion and compatible MIME types.
- Preserve the requested schema dialect.
- Do not force a different MIME type unless it is an explicitly documented
  compatibility transformation.

### GNI-009 — Response metadata is a small subset of the current schema

Top-level response gaps:

| Official field | Wingman |
|---|---|
| `candidates` | Partial; exactly one synthesized candidate |
| `promptFeedback` | Type exists but is never populated |
| `usageMetadata` | Partial scalar subset |
| `modelVersion` | Present, but Google provider does not preserve upstream `ModelVersion`; requested route model is used as fallback |
| `responseId` | Present; a new Wingman ID is generated instead of preserving the upstream ID |
| `modelStatus` | Missing |

Candidate fields missing or never populated:

- `finishMessage`
- `citationMetadata`
- `groundingAttributions`
- `groundingMetadata`
- `avgLogprobs`
- `logprobsResult`
- `urlContextMetadata`
- actual `safetyRatings` and `tokenCount`

The current official finish-reason enum contains many values beyond Wingman's
effective `STOP`, `MAX_TOKENS`, `SAFETY`, and `OTHER` mapping, including
`RECITATION`, `LANGUAGE`, `BLOCKLIST`, `PROHIBITED_CONTENT`, `SPII`,
`MALFORMED_FUNCTION_CALL`, image-specific reasons, `UNEXPECTED_TOOL_CALL`,
`TOO_MANY_TOOL_CALLS`, `MISSING_THOUGHT_SIGNATURE`, `MALFORMED_RESPONSE`, and
`ESCALATION`.

Usage fields are declared more broadly in `models.go`, but the common
`provider.Usage` retains only input, output, reasoning, and cache counts.
Consequently modality details, tool-use prompt tokens, and response
`serviceTier` cannot be emitted (`server/gemini/convert.go:359-378`).

Required change:

- Preserve the native response ID, model version, candidate list, exact finish
  state, safety/grounding/citation/logprob metadata, and detailed usage.
- Avoid generating replacement identifiers unless the backend supplied none.

### GNI-010 — Streaming framing is compatible, but chunk semantics are not

Correct behavior:

- `alt=sse` produces `text/event-stream` with `data: {json}` records.
- Without SSE, the method produces a JSON array of response objects.
- No OpenAI-style `[DONE]` marker is added.

Differences:

- All function calls are withheld until a synthetic final chunk.
- If a response mixes streamed text/reasoning with a function call, the final
  chunk serializes the entire accumulated message, duplicating text already
  emitted (`server/gemini/accumulator.go:110-122`).
- Signature-only reasoning and summary-only reasoning chunks are filtered out
  by the streaming path.
- Usage-only/provider-metadata-only chunks are suppressed, although a final
  aggregate may still be emitted.
- The final finish reason is synthesized from reduced provider state.
- A mid-stream error is encoded as a nonstandard
  `GenerateContentResponse.error` object
  (`server/gemini/models.go:147-155`,
  `server/gemini/accumulator.go:130-164`). Google's successful response schema
  has no `error` field.

Required change:

- Preserve incremental content order and emit only un-emitted call parts in any
  synthetic final chunk.
- Preserve metadata-only chunks where the source protocol returns them.
- Define mid-stream failure behavior separately from the successful response
  schema and test it with official SDK clients.

### GNI-011 — Error and authentication behavior is only partially Google-shaped

`writeError` generally returns the Google JSON error envelope with `code`,
`message`, and `status`, which is a good baseline. Remaining differences:

- Several distinct 4xx conditions collapse to `INVALID_ARGUMENT`.
- Upstream non-503/504 5xx statuses are normalized to 502 while the status
  string remains `INTERNAL`.
- Authentication failure in the shared server middleware is a bare 401 with no
  Google error body.
- Google API-key authentication via `key=` or `x-goog-api-key` is not an
  authorizer. It works only when Wingman is otherwise anonymous; a configured
  static authorizer expects `Authorization: Bearer ...`, so an unmodified
  Gemini client no longer authenticates.
- Global Google query behavior such as partial response `fields`,
  `prettyPrint`, `$.xgafv`, and `alt=proto` is not implemented.

Required change:

- Return Google-shaped errors from shared middleware on `/v1beta`.
- Either support the Gemini API-key locations or document that auth is a
  deliberate deployment-level incompatibility.
- Keep provider/gateway normalization separate from the public error status
  classification.

### GNI-012 — Additional GenerateContent resource forms are not routed

The v1beta discovery document exposes GenerateContent methods for `models`,
`tunedModels`, and `dynamic` resources. The linked reference also contains a
tuned-model request example.

Wingman attaches only:

```text
/v1beta/models/{model}:generateContent
/v1beta/models/{model}:streamGenerateContent
```

See `server/gemini/handler.go:24-28` and `server/server.go:86-88`.

If the compatibility claim is intentionally limited to configured model aliases
under `models/`, this should be documented as a boundary. Otherwise, add the
resource routes and preserve the full resource name during model resolution.

## Adjacent advertised endpoint: `countTokens`

### GNI-013 — `countTokens` is not contract-compatible

Although `countTokens` is not the primary method in the linked reference,
Wingman advertises it in the same Gemini-compatible API surface and shares the
same content/tool types.

Official v1beta shape:

```json
{
  "contents": [],
  "generateContentRequest": {}
}
```

`contents` and `generateContentRequest` are mutually exclusive. The nested
request is how system instructions, tools, cached content, and generation
context are counted. The response can include `totalTokens`,
`cachedContentTokenCount`, `promptTokensDetails`, and `cacheTokensDetails`.

Wingman's request instead declares nonstandard top-level
`systemInstruction` and `tools`, and has no `generateContentRequest`
(`server/gemini/models.go:200-210`). The existing integration test explicitly
sends a different request body to Google and Wingman for this reason
(`test/gemini/features/count_tokens_test.go:16-24`).

The handler:

- does not resolve the route model;
- does not perform the model policy check;
- does not call a model tokenizer;
- returns a character-count heuristic;
- treats every inline media part as 1,300 tokens regardless of MIME, dimensions,
  or model;
- ignores `fileData`, function-response media, most part types, cached content,
  and modality details.

See `server/gemini/handler_tokens.go:8-107`.

Required change:

- Implement the official request union.
- Resolve and authorize the route model.
- Add a provider tokenizer/count interface and use the selected model's actual
  count where available.
- If only an estimate is possible for a backend, expose that as a documented
  Wingman extension rather than an exact Gemini `countTokens` result.

## Request field matrix

### `GenerateContentRequest`

| Official field | Parser | Effective behavior |
|---|---|---|
| `contents` | Present | Partial content union; requiredness not validated |
| `tools` | Present | Function declarations only |
| `toolConfig` | Present | Partial function-calling config only |
| `safetySettings` | Present | **Ignored** |
| `systemInstruction` | Present | Text-only lower-camel array form |
| `generationConfig` | Present | Partial; see below |
| `cachedContent` | Missing | Ignored |
| `serviceTier` | Missing | Ignored |
| `store` | Missing | Ignored |

### `GenerationConfig`

| Behavior | Fields |
|---|---|
| Directly effective | `stopSequences`, `temperature`, `maxOutputTokens` |
| Effective but lossy | `responseSchema`, `responseJsonSchema`, `thinkingConfig` |
| Parsed but ignored | `topP`, `topK`, `candidateCount`, `seed`, `presencePenalty`, `frequencyPenalty`, `responseLogprobs`, `logprobs`, `responseMimeType`, `responseModalities`, `mediaResolution` |
| Missing | `speechConfig`, `imageConfig`, `audioTranscriptionConfig`, `enableEnhancedCivicAnswers`, `enableAffectiveDialog`, `responseFormat`, `translationConfig` |

### `Part`

| Official part data | Current state |
|---|---|
| `text` | Supported |
| `inlineData` | Supported input; output dropped |
| `fileData` | Parsed but URI semantics corrupted; output dropped |
| `functionCall` | Partial |
| `functionResponse` | Partial |
| `thought`, `thoughtSignature` | Partial; best support is on non-streaming/basic tool paths |
| `videoMetadata`, per-part `mediaResolution`, `partMetadata` | Missing |
| `executableCode`, `codeExecutionResult` | Missing |
| `toolCall`, `toolResponse` | Missing |

## Existing tests and blind spots

Local unit verification performed for this audit:

```text
GOCACHE=/tmp/wingman-go-build go test ./server/gemini ./pkg/provider/google
ok github.com/adrianliechti/wingman/server/gemini
ok github.com/adrianliechti/wingman/pkg/provider/google
```

The live differential tests require configured external services and API keys,
so they were not run as part of this static audit.

Current differential tests establish useful basic text, multi-turn, function,
structured-output, thinking, usage, and framing coverage. However:

- `test/gemini/rules.go` ignores `finishReason`, `finishMessage`,
  `safetyRatings`, `promptFeedback`, `modelVersion`, token-detail fields, and
  service tier—the exact fields involved in several critical findings.
- Tests require successful responses and do not compare prompt blocking,
  candidate safety, truncation, invalid input, or official error shapes.
- There are no handler conformance cases for snake-case/singleton REST forms,
  URI media, generated media, built-in tools, multiple candidates, logprobs,
  service tier, cache, storage, or the current new generation configs.
- `countTokens` deliberately uses different request shapes and only checks that
  both totals are positive, not that Wingman implements the Google contract.
- `server/gemini/convert_test.go` currently covers only two usage conversions.

## Recommended implementation order

1. **Make unsupported behavior explicit.** Add presence-aware decoding and
   validation. Return `INVALID_ARGUMENT` for documented but unsupported
   semantics instead of HTTP 200 with defaults.
2. **Fix safety and termination fidelity.** Preserve prompt feedback, exact
   finish reasons, candidate safety ratings, and no-candidate blocked responses.
3. **Fix REST decoding.** Accept official snake-case and singleton examples,
   then lock them in as golden handler tests.
4. **Fix media identity and output.** Distinguish URI references from inline
   bytes and serialize generated media.
5. **Preserve generation controls and candidates.** Add portable/common options
   plus a Gemini-native side channel; either support multiple candidates or
   reject them.
6. **Expand tools and current part variants.** Start with code execution,
   Google Search/URL context, and server-side call round-tripping; explicitly
   reject non-emulatable tools on other backends.
7. **Correct thinking, `VALIDATED`, structured output, and streaming edge
   cases.**
8. **Preserve response metadata and detailed usage.**
9. **Replace or relabel heuristic `countTokens`.**
10. **Strengthen differential tests.** Stop ignoring safety/finish fields and
    add negative, multimodal, multi-candidate, and stream-order assertions.

## Compatibility conclusion

The current endpoint is accurately described as a **Gemini-shaped core
translation API**, not a complete Gemini GenerateContent-compatible API. Basic
Gemini SDK and CLI workflows can work when they serialize lower-camel arrays
and stay within text, inline media, simple functions, basic structured output,
and level-based thinking. Applications relying on safety policy, Files API URI
parts, generated media, built-in tools, precise generation controls, multiple
candidates, exact finish state, grounding/logprobs, caching/tier/storage, or
exact token counting can receive materially incorrect results without an
error.

