# SDK design and implementation

This is the authoritative Go API and implementation contract for
`typesafe-sdk-go`. It describes the implemented public and runtime contract;
tests and coverage documents separately record verification evidence.
Apply the contributor constraints in [AGENTS.md](AGENTS.md); use
[UPSTREAM.md](UPSTREAM.md) for pinned evidence and [PARITY.md](PARITY.md) for
adaptation rationale and unresolved conflicts. Keep signatures and behavioral
rules here rather than duplicating them in contributor instructions.

Design priority is **Go idioms > HTTP API contract > JS SDK reference**. Use Go
representations and compile-time constraints for the public API, preserve valid
HTTP shapes and semantics on the wire, and adopt JS behavior where it serves
these goals. Compatibility means the documented capabilities, not every JS
input quirk or export. The review decisions are recorded in PARITY.md.

Before changing SDK behavior, read the relevant sections below and the pinned
source/tests. Changes to question/answer types or serialization must also check
[primitives coverage](docs/primitives-coverage.md). Documentation examples are
acceptance inputs, not evidence that a live service returns fixed model outputs.

- [Layout and implementation order](#1-layout-and-initial-implementation-order)
- [Implementation details](#2-implementation-details)
- [Public API contract](#3-public-api-contract)
- [Runtime behavior](#4-runtime-behavior)
- [Examples](#5-examples-and-ai-facing-documentation)
- [Testing](#6-testing-and-surface-coverage)
- [Releases and upstream sync](#7-versioning-releases-and-upstream-sync)

## 1. Layout and initial implementation order

```text
typesafe-sdk-go/
  *.go                       package typesafe
  option/                    package option: scoped options, RetryPolicy, With* factories
  internal/                  private implementation; no compatibility promise
  internal/tools/surfacecheck/  eventual capability/surface checker
  integration/               tests behind the integration build tag
  testdata/conformance/       language-neutral JSON fixtures with source references
  example_test.go             compiled usage examples
  examples/                   runnable Choice, Score, and Noul programs
  version.go                 Go SDK Version
  AGENTS.md                  contributor constraints and reading guide
  DESIGN.md                  API and implementation contract
  docs/primitives-coverage.md official-example acceptance matrix
  UPSTREAM.md                exact upstream pins and reference index
  PARITY.md                  adaptations and unresolved questions
  CHANGELOG.md
  .upstream/js/              optional, gitignored pinned reference checkout
  .upstream/python/          optional, gitignored pinned reference checkout
  .github/workflows/         checks, releases, and upstream monitoring
```

Implement in small layers:
request/response types and serialization, errors and options, transport and
retries, client/services, then examples and release checks. Start with a reviewed
capability mapping; do not block the first working endpoint on a TypeScript
parser or build a general-purpose code generator.

### Implementation checkpoints

These are dependency-ordered work slices, not completion claims. Keep internals
private and introduce subpackages only when they clarify a real boundary.

| Layer | Responsibility | Evidence before moving on |
|---|---|---|
| Types and JSON | Questions, criteria maps/slices/structs, answers, raw retention | Request/response fixtures and helper/literal equivalence |
| Options and errors | Enforce option scopes by type, validate values, copy owned data, map errors | Positive/negative compilation, precedence, copy-isolation, and error tests |
| Transport | Own serialized bytes, attempt timeouts, body buffers, retry waits, diagnostics | Offline HTTP, cancellation, retry, body-failure, and redaction tests |
| Client and services | Connect SystemOne and Models to the shared transport and decoders | End-to-end conformance cases with httptest |
| Documentation and delivery | Compiled examples, capability map, CI, release metadata | Example coverage, supported-toolchain checks, and release consistency |

Within a call, resolve options and model, serialize once and validate the
serialized payload, run HTTP attempts with the same bytes, then decode and
expose buffered snapshots. Domain types own JSON semantics; transport owns
network resources and retries; public methods connect these responsibilities.
Business routing, score weighting, and taxonomy traversal remain caller code.

## 2. Implementation details

Language/runtime constraints, supported public packages, and error/side-effect
policies live in [AGENTS.md](AGENTS.md#core-constraints). The details below define
how SDK objects satisfy those constraints.

- `context.Context` is the first parameter of network operations. Never store
  a call context in client configuration. Private HTTP snapshots may retain
  only detached contexts, not a caller's values or cancellation lifecycle.
- Client configuration is immutable after construction. Copy option-owned
  maps/slices, including `RetryPolicy.HTTPStatuses`, and return defensive copies
  from accessors. Never mutate an injected `*http.Client` or its transport.
  Concurrent calls are supported when caller-provided components are safe for
  concurrent use; callers must not mutate request data while a call uses it.
- Keep the API key unexported with no accessor. `Client.String`, `GoString`,
  and `LogValue` must redact it for value and pointer formatting and `slog`.
  Apply the same protection to any service holding client configuration.
  JSON marshaling a client must not expose credentials. See
  [runtime behavior](#4-runtime-behavior) for the boundary between credential
  redaction and caller-owned body data.
- Exported identifiers have Go doc comments starting with their names. Explain
  defaults, option scope, nil/zero semantics, ownership, and relevant wire names.
  Keep README snippets synchronized with compiled examples.

## 3. Public API contract

### 3.1 Names and core signatures

| JS capability | Go API |
|---|---|
| `TypeSafeClient` | `Client` |
| constructor config | `NewClient(opts ...option.ClientOption) (*Client, error)` |
| `systemOne(request, options)` | `(*Client).SystemOne(ctx context.Context, req SystemOneRequest, opts ...option.RequestOption) (*SystemOneResponse, error)` |
| `client.models.list()` | `client.Models.List(ctx, opts...)` |
| `SystemOneResult` | `SystemOneResponse` |
| `NoulResponse`, `ChoiceResponse`, `ScoreResponse` | `NoulAnswer`, `ChoiceAnswer`, `ScoreAnswer` |
| `noul`, `choice`, `score` | `Noul`, `Choice`, `Score` |
| choice criteria | `ChoiceCriteria` map: option name to Entry description |
| `TypeSafeError`, `VERSION` | `Error`, `Version` |

Use `SystemOneRequest`, not a positional `state, questions, model` call or a
`Params` alias. Required wire fields always serialize in the resolved payload.
Use useful Go zero values: an empty request Model selects the client default.
Pointers remain appropriate when absence and zero have distinct supported
meanings, such as unreported versus zero token counts; do not add pointer
ceremony merely to reproduce JS input quirks. JSON tags use the wire name;
Go initialisms use `APIKey`, `BaseURL`, `RequestID`, and `HTTPStatuses`.

`DefaultBaseURL`, `DefaultModel`, and `Version` are exported constants. Also
provide the documented `option.LogLevel` constants. Environment names stay
private. Durations use `time.Duration`; fractional values use `float64`;
integer counts and level indices use `int`.

### 3.2 Client and options

`NewClient` reads the environment once, applies options in order, and validates
without network activity. Client has one exported field, `Models ModelService`,
and read-only `BaseURL() string`, `DefaultModel() string`, `Timeout()
time.Duration`, and `RetryPolicy() option.RetryPolicy` methods, in addition to
network and safe-formatting methods. Callers must not reassign `Models` during
use; its exported field is a discoverability compromise, not a promise that Go
can make exported fields readonly. A nil or zero Client, or zero ModelService,
returns `*Error` from network calls. There is no `Close` method.

Use three sealed interfaces so option scope is checked by the Go compiler:

```go
// Scope constraints in package option; internal application methods omitted.
type ClientOption interface { clientOption() }
type RequestOption interface { requestOption() }
type CommonOption interface {
    ClientOption
    RequestOption
}
```

Factories return the interface listed below. CommonOption values are accepted
by both NewClient and network methods without conversions; client-only and
call-only options cannot cross those boundaries. Keep application machinery in
an internal package (public aliases to internal sealed types are acceptable).
Do not expose Apply methods, use reflection, or export mutable configuration to
bridge packages. Marker methods alone do not apply settings: use a private,
shared implementation with typed application functions. Custom option
implementations are not a supported extension point.

| Option | Return type | JS counterpart / default |
|---|---|---|
| `WithAPIKey(string)` | `ClientOption` | `apiKey` / environment, otherwise missing-key error |
| `WithBaseURL(string)` | `ClientOption` | `baseURL` / environment / `https://api.typesafe.ai`; strip trailing slashes |
| `WithDefaultModel(string)` | `ClientOption` | `defaultModel` / environment / `jev-latest` |
| `WithTimeout(time.Duration)` | `CommonOption` | `timeout` / 10 seconds per attempt |
| `WithMaxRetries(int)` | `CommonOption` | `retry.maxRetries` / 2; zero disables retries |
| `WithRetryPolicy(func(*option.RetryPolicy))` | `CommonOption` | partial `retry`; mutate an isolated copy of the current policy |
| `WithHeader(name, value string)` | `CommonOption` | `defaultHeaders` / per-call `headers`; case-insensitive, later wins |
| `WithHTTPClient(*http.Client)` | `ClientOption` | `fetch`; default is a dedicated zero-value `http.Client` |
| `WithLogger(*slog.Logger)` | `ClientOption` | `logger`; default discards output |
| `WithLogLevel(option.LogLevel)` | `ClientOption` | `logLevel` / environment / `warn` |
| `WithResponseInto(**http.Response)` | `RequestOption` | raw/metadata access corresponding to `withResponse` and `asResponse` |

Use WithTimeout consistently; do not introduce a WithRequestTimeout synonym.
For dynamically assembled options, use []ClientOption or []RequestOption for the
receiving API. Go does not implicitly convert a slice of CommonOption to either
slice; append individual common options to the appropriately typed slice.

- The environment variables are `TYPESAFE_API_KEY`, `TYPESAFE_BASE_URL`,
  `TYPESAFE_DEFAULT_MODEL`, and `TYPESAFE_LOG_LEVEL`. Trim environment values and
  ignore blank ones. Explicit API key, base URL, and default model options must
  not be empty or whitespace-only: return *Error instead of silently falling
  back or sending a blank credential. Nonblank explicit values are not trimmed.
  Reject base URLs lacking an http/https scheme or host, or containing userinfo,
  query, or fragment; preserve an allowed path prefix. This is deliberate Go
  configuration validation, not exact JS input parity.
- Validate the selected value, so a valid explicit option can override an
  invalid environment setting. Later options win; call options override client
  values only at supported scopes. No environment rereads during calls.
- `LogLevel` is a string type with `LogLevelDebug`, `LogLevelInfo`,
  `LogLevelWarn`, `LogLevelError`, `LogLevelOff`; values are case-sensitive.
- `RetryPolicy` has `MaxRetries int`, `BackoffInitial time.Duration`,
  `BackoffMax time.Duration`, `BackoffJitter float64`, `HTTPStatuses []int`,
  `RespectRetryAfter bool`, `MaxRetryAfter time.Duration`,
  `RetryConnectionErrors bool`, and `RetryTimeouts bool`. The last two map to
  JS `apiConnectionError` and `apiTimeoutError`. `DefaultRetryPolicy()` returns
  a fresh copy; it is not a mutable global.
- Validate timeout > 0; retries >= 0; all retry durations >= 0; finite jitter in
  [0, 1]; statuses in [100, 999]. Zero backoff is valid. An empty status slice
  disables status retries. Do not require backoff max >= initial; JS does not.
- Reject nil options, nil policy callbacks, nil HTTP clients/loggers, and a nil
  response destination with `*Error`. Callbacks must not retain/mutate their
  argument after returning; copy the result before storing it.
- An injected client's own timeout/redirect/transport policy remains effective.
  A custom transport must honor request contexts and return closable bodies;
  do not promise to forcibly terminate arbitrary non-cooperative Go code.

### 3.3 Requests and questions

```go
type Entry = any

type SystemOneRequest struct {
    State     Entry     `json:"state"`
    Questions Questions `json:"questions"`
    Model     string    `json:"model,omitzero"`
}

type Questions map[string]Question
```

- `Model == ""` inherits the client default; a nonempty value selects a model.
  Resolve into a private wire payload without mutating the request; the HTTP
  body always includes `model`. Direct json.Marshal of the public request is
  not a complete client request: it cannot resolve client defaults.
- Entry represents JSON-compatible data. State must encode as a string, object,
  or array under the HTTP contract; reject null (including typed nils), numbers,
  and booleans at its root before sending. Question instructions/descriptions
  also admit null as documented by the advanced guide. Nested data may contain
  any JSON value. Structs and json.RawMessage are supported; any cannot enforce
  these shapes at compile time. Unsupported Go values fail serialization locally.
- Question IDs **are sent as keys of `questions`** and returned under `answers`.
  They are not sent to the underlying model for inference, and do not belong in
  an individual question value. Encode question keys in sorted order.
- `Question` is the minimal sealed interface `interface { isQuestion() }`.
  It does not embed json.Marshaler. Value and pointer forms of NoulQuestion,
  ChoiceQuestion, ScoreQuestion, and RawQuestion implement it; their concrete
  types implement MarshalJSON for direct encoding and fixed discriminators.
  Reject nil interfaces and typed-nil question pointers at call time. Sealing
  already prevents a foreign marshaler alone from becoming a Question; this
  choice keeps serialization out of the interface's public method set.

The three primitives use different criteria shapes because their semantics
are different. Keep keyed structs as the complete API and small helpers for
common cases; do not introduce a shared criteria union or fluent builder.

```go
type ChoiceCriteria map[string]Entry

type ChoiceQuestion struct {
    Instructions Entry          `json:"instructions"`
    Criteria     ChoiceCriteria `json:"criteria"`
}

type ScoreQuestion struct {
    Instructions Entry   `json:"instructions"`
    Criteria     []Entry `json:"criteria"`
}

type NoulCriteria struct {
    True  Entry `json:"true,omitzero"`
    False Entry `json:"false,omitzero"`
}

type NoulQuestion struct {
    Instructions Entry         `json:"instructions"`
    Criteria     *NoulCriteria `json:"criteria,omitzero"`
}

func Choice(instructions Entry, criteria ChoiceCriteria) ChoiceQuestion
func Score[T any](instructions Entry, levels ...T) ScoreQuestion
func Noul(instructions Entry) NoulQuestion
```

- All three concrete question types inject their fixed `type` in MarshalJSON.
  Instructions always serialize, with nil becoming null. RawQuestion can express
  omission. Helpers perform no validation and never fail or panic on SDK-controlled
  input; SystemOne performs local validation before network activity.
- **Choice:** names are map keys, descriptions are Entry values. No ChoiceOption
  type or Option helper. Values may be structured or null; nil descriptions
  remain null. Dynamic callers use ordinary map assignment. Reassigning a key
  replaces its value; the SDK cannot detect an overwritten map entry and adds
  no duplicate-option check. Duplicate constant keys in literals are Go errors.
- Choice criteria encode as a JSON object with sorted keys under encoding/json
  v1, including lexicographic ordering of numeric-looking string keys. No author
  order or option-priority semantics are promised. Normalize nil criteria to an
  empty object in ChoiceQuestion.MarshalJSON without changing caller data; empty
  criteria also encode as {}. No separate ChoiceCriteria marshaler is needed.
  Zero ChoiceQuestion remains directly encodable, but SystemOne rejects fewer
  than one or more than 255 choices before network activity.
- **Score:** Criteria stays []Entry. Array position defines the level, so never
  sort it or replace it with an integer-keyed map. Nil slices encode as null and
  non-nil empty slices as []; neither is a usable scale. Reject nil, empty, and
  one-level criteria at call time, including known raw score questions.
  The response Score is a fractional expected value, not an integer, probability,
  or the most likely level. Normalization and rounding belong to caller code.
- Retain the generic variadic Score helper: it accepts direct string levels and
  []string, []struct, or []Entry expansion without a conversion helper. It returns
  the same non-generic ScoreQuestion. Homogeneous arguments infer T; heterogeneous
  concrete arguments, only untyped nil arguments, and no levels require an
  explicit type argument (use Entry). For mixed structured rubrics, prefer a
  keyed ScoreQuestion with []Entry in examples. T any does not validate JSON
  shapes or make numeric level descriptions supported. No additional ScoreOf,
  ScoreStrings, ScoreCriteria wrapper, or typed-result machinery is needed.
- **Noul:** keep Noul(instructions) for the common question without criteria.
  It is equivalent to NoulQuestion{Instructions: instructions}. Custom criteria
  use a keyed NoulQuestion with &NoulCriteria{True: ..., False: ...}; the fields
  are descriptions, not booleans. Retain the pointer to distinguish an omitted
  criteria object from an explicitly supplied empty object. Fixed field names
  prevent misspelled outcome keys; do not use map[string]Entry or map[bool]Entry.
  Do not emulate optional arguments with variadic criteria or add NoulWithCriteria.
- Nil Noul criteria is omitted; &NoulCriteria{} sends {}. Either True or False
  may be specified alone. Nil Entry fields omit their keys; an empty string is
  present, and json.RawMessage("null") explicitly sends null. A typed-nil value
  inside Entry is not a nil interface and encodes as null when supported by its
  marshaler. Explicit null for the whole criteria object uses RawQuestion.
  SystemOne requires either non-null instructions or at least one non-null true
  or false description. Criteria-only Noul questions are accepted; Noul(nil)
  without meaningful criteria is rejected locally.
- **Ownership:** Choice shallow-copies the input map, preserving nil; Score copies
  levels into a new []Entry, preserving nil versus non-nil empty slices. Replacing
  caller map entries or slice elements later must not change the built question.
  Entry contents (including Instructions and nested maps/slices/pointers) remain
  shared. Keyed literals follow ordinary Go assignment and share maps, slices,
  and criteria pointers; do not mutate shared data during a call. Helper and
  equivalent literal encodings must match, including nil/empty cases.

- `RawQuestion` is `struct { JSON json.RawMessage }`. Its JSON must be a valid
  object with a nonempty string `type`; unknown types are forwarded. Known raw
  Choice, Score, and Noul questions undergo the same criteria-count/content
  validation as modeled questions. Do not normalize its object order or strip
  unknown fields.
- Use `encoding/json` order deliberately: maps sort string keys, structs follow
  field order, RawMessage preserves member order but may be compacted/escaped
  when embedded. **Do not promise byte-for-byte request passthrough.** Avoid
  converting structured Entry data to a map or to a JSON string. Preserve
  chosen ordering without claiming the API guarantees order-invariant output.

### 3.4 Serialization and validation

Do not provide SetExtraFields or generic JSON mutation options in the initial
API. Model the documented HTTP fields. Entry already carries arbitrary nested
state/instruction/description data, and RawQuestion can carry a future question
field or discriminator. RawQuestion does **not** provide arbitrary top-level
request fields: that JS capability is deliberately omitted. Add newly supported
HTTP fields as reviewed typed fields rather than prebuilding an override system.

- Serialize the resolved payload once into owned bytes and reuse those bytes
  for retries; do not re-run caller marshalers for validation or each attempt.
  Inspect serialized JSON without rebuilding Entry objects, so chosen member
  order and caller marshaler output remain intact.
- Local checks cover nonempty Questions and nonempty question IDs; one to 255
  Choice criteria; two to 10 Score criteria; meaningful Noul instructions or
  criteria; invalid raw JSON; nil questions/options; supported root State shape;
  and serialization errors. Known RawQuestion kinds receive the same checks.
  They return *Error before network activity. Use the serialized shape for
  custom marshalers and typed-nil State values rather than a Go type allowlist.
- Do not add a full JSON-schema validator, model allowlists, or speculative
  limits. Add targeted client checks only for a documented contract and clear
  usability benefit, recording intentional differences from JS.

### 3.5 Responses, models, and raw HTTP access

```go
type SystemOneResponse struct {
    Model     string            `json:"model"`
    Answers   map[string]Answer `json:"answers"`
    Usage     Usage             `json:"usage"`
    RequestID string            `json:"-"`
    // private original JSON
}

type Usage struct {
    InputTokens  *int `json:"input_tokens,omitzero"`
    OutputTokens *int `json:"output_tokens,omitzero"`
}

func (ModelService) List(ctx context.Context, opts ...option.RequestOption) ([]ModelCard, error)

func (*SystemOneResponse) Noul(name string) (NoulAnswer, error)
func (*SystemOneResponse) Choice(name string) (ChoiceAnswer, error)
func (*SystemOneResponse) Score(name string) (ScoreAnswer, error)
```

- Ordinary exported value structs, using custom decoding where needed for
  tagged answers and raw retention. No generated union wrappers or per-field
  metadata. Unknown fields are accepted; do not use `DisallowUnknownFields`.
- `Usage` pointers distinguish unreported/null counts from a real zero. This is
  a deliberate tolerant interpretation supported by Python and the HTTP docs;
  JS's static type is narrower than its unchecked JSON parser.
- `Answer` is sealed with `RawJSON() string`. Decode into **value** variants:
  `NoulAnswer`, `ChoiceAnswer`, `ScoreAnswer`, and `UnknownAnswer{Type string}`.
  Known variants are discriminated by their Go type; an unknown nonempty string
  discriminator retains its whole JSON. Missing/invalid discriminator or invalid
  known field types cause a decode error, not an UnknownAnswer fallback.
- NoulAnswer has `Noul float64`: the probability of yes, not a bool. The SDK
  does not choose a threshold or add Confidence/Probabilities fields absent from
  the response. Applications can compare the value with their chosen threshold.
  ChoiceAnswer has `Choice string`,
  `Confidence float64`, and `Probabilities map[string]float64`. ScoreAnswer has
  `Score float64`, `Confidence float64`, `Legend map[int]Entry`, and
  `Probabilities map[int]float64`; non-integer level keys cause a decode error.
  Legends support structured and null descriptions, not just strings.
- `SystemOneResponse.RawJSON()` and each answer's `RawJSON()` return original
  received JSON, not a re-marshaling. Retain owned copies; return `""` for
  manually constructed values. Later field edits do not change RawJSON.
- `Noul(name)`, `Choice(name)`, and `Score(name)` perform allocation-free single
  lookups and distinguish a missing answer from a different answer type.
  `Nouls()`, `Choices()`, and `Scores()` return fresh typed maps keyed by ID.
  Ordinary type assertions/type switches on Answers remain available.
  Returned answer values may share nested response maps; these are caller-owned.
- Models uses **GET `/v1/models`**, decoding `{ "models": [...] }` and returning
  the inner list, matching JS. Missing/null/non-array `models` is an error.
  ModelCard has string fields `Name`, `Description`, `ReleaseDate` with matching
  snake_case tags; do not parse the date into time.Time. There is no public
  ListModelsResponse. Raw model fields and the outer envelope remain available
  through WithResponseInto.
- Parse JSON regardless of Content-Type. For non-2xx errors preserve parsed JSON
  or text (nil for an empty body). A typed Go success result cannot represent
  arbitrary text: malformed or incompatible 2xx bodies return
  `*ResponseValidationError` without retries. Validate required top-level fields,
  required fields of known answer/model variants, answer presence and discriminator
  agreement with each request question, and integer score-map keys. Continue to
  accept unknown fields and unknown answer variants; optional usage token counts
  may be absent or null. This targeted validation differs from JS's unchecked
  type assertion and does not introduce a generated full-schema validator.
- WithResponseInto resets its destination to nil on entry, then receives the
  final HTTP response snapshot on success, HTTP error, or decode error. No final
  response means nil. The SDK reads and closes the network body within the
  attempt timeout. Exposed Body is an independent in-memory reader over buffered
  bytes, which the caller closes; it does not hold a connection. Error snapshots
  and dumps use separate readers. Never return an already-closed network body.
  Snapshots redact credential headers, including Response.Request, without
  modifying the actual outgoing request. A destination must not be shared by
  concurrent calls. On a body-read failure, expose available final metadata and
  partial bytes if a response was obtained, and still return the transport error.

### 3.6 Errors

```go
type APIError struct {
    StatusCode int
    RequestID  string
    Header     http.Header
    Body       any
    RetryAfter *time.Duration
    Request    *http.Request
    Response   *http.Response
}

func (e *APIError) Error() string
func (e *APIError) DumpRequest(body bool) ([]byte, error)
func (e *APIError) DumpResponse(body bool) ([]byte, error)

type ResponseValidationError struct {
    Method      string
    Path        string
    RequestID   string
    FieldPath   string
    Response    *http.Response
    Cause       error
}
```

- `*Error` handles configuration, local request validation, and encoding
  failures; preserve underlying causes with Unwrap when present.
  `*ResponseValidationError` handles incompatible 2xx response bodies and matches
  both `ErrResponseValidation` and `ErrTypeSafe`. It exposes the method, endpoint
  path, request ID, `$` or an RFC 6901 JSON Pointer field path, a sanitized
  independently buffered response snapshot, and the underlying cause. Its error
  string never includes the response body. Callers use `errors.Is` for categories
  and `errors.As` for details; do not require string matching.
- All non-2xx HTTP errors use `*APIError`. Match the following sentinel mapping:

  | JS class | Go sentinel |
  |---|---|
  | `BadRequestError` | `ErrBadRequest` (400) |
  | `AuthenticationError` | `ErrAuthentication` (401) |
  | `PermissionDeniedError` | `ErrPermissionDenied` (403) |
  | `NotFoundError` | `ErrNotFound` (404) |
  | `UnprocessableEntityError` | `ErrUnprocessableEntity` (422) |
  | `RateLimitError` | `ErrRateLimit` (429) |
  | `InternalServerError` | `ErrInternalServer` (>= 500, including 529) |
  | `APIConnectionError` | `ErrConnection` |
  | `APITimeoutError` | `ErrTimeout` **and** `ErrConnection` |
  | `APIUserAbortError` | `ErrUserAbort` and the caller's context error |

- An unlisted HTTP status remains APIError without a status-specific sentinel.
  Caller cancellation/deadline expiration matches `context.Canceled` or
  `context.DeadlineExceeded`, is not an SDK attempt timeout, and is never retried.
- `*TimeoutError` exposes `Duration time.Duration`, implements Error and Unwrap,
  and matches ErrTimeout, ErrConnection, and ErrTypeSafe. Other transport wrappers
  stay private and preserve causes. Retry classification must check timeout
  before general connection errors, just as JS does.
- RetryAfter represents JS RateLimitError.retryAfterMs: populate for 429 from
  the server's valid retry header, nil if absent/invalid. A valid zero is not nil;
  preserve a delay above the automatic-retry cap in the error metadata.
- Error strings include method/path, status, request ID when present, and the
  useful upstream message. Follow JS extraction for `error`, `error.message`,
  `message`, `detail`, and validation locations; truncate fallback raw-body
  summaries as upstream does rather than dumping an unlimited body. The Go
  method/path/request-ID prefix is documented formatting, not byte-for-byte JS
  parity. Keep the complete body in Body and HTTP snapshots.
- Public HTTP fields are sanitized copies. Dumps operate on private buffered
  copies, return serialization errors, redact headers, and do not consume the
  caller's response reader or issue network requests. SDK-generated formatting
  must not reveal the configured API key through a request pointer or cause.

### 3.7 Non-goals

Do not add TS compile-time inference, label enums, typed question handles that
embed IDs, Python's async/sync split or response-model machinery, generic optional
wrappers, generated-SDK unions, `Must*`, model-name constants, public generic HTTP
methods, middleware/query/JSON-path option frameworks, streaming, pagination,
file upload, SetExtraFields or generic extra-field maps, or a CLI. Compose a
RoundTripper through WithHTTPClient for transport customization. Browser/WASM
support is outside this port's supported targets.
The generic Score builder is allowed; TS result inference is not its purpose.
New API capabilities require HTTP evidence or a documented Go rationale and an
explicit scope change; a JS export is not a prerequisite.

## 4. Runtime behavior

These defaults are deliberately adopted from the pins in UPSTREAM.md, subject
to the adaptations below. Review changes rather than automatically copying all
JS behavior; update references and conformance cases when syncing a release.

| Setting | Contract |
|---|---|
| Base URL / model | `https://api.typesafe.ai` / `jev-latest` |
| Evaluation | POST `/v1/systemone`, JSON body with resolved model |
| Discovery | GET `/v1/models`, no body |
| Timeout | 10 seconds per attempt, through complete body reading; no SDK total retry budget |
| Caller context | Bounds the entire call, including attempts and backoff |
| Retries | 2 after the first attempt; 408, 429, 500–599; connection failures and attempt timeouts |
| Backoff | 500 ms × 2^attempt, capped at 5 s, subtract random fraction up to 25%, round to milliseconds |
| Server delay | Prefer valid `retry-after-ms`, then numeric seconds or HTTP date from `Retry-After`; use delays <= 60 s, otherwise backoff; past dates yield zero |
| Retry body | Same owned serialized bytes on each attempt; increment retry header only on retries |
| Logging | info summaries; debug headers/bodies; default discard, default level warn |

- Preserve the pinned Retry-After parser's tested edge cases, including valid
  zero, invalid/negative values, fallback between headers, and millisecond
  precedence. Inject time/randomness/waiting privately for deterministic tests.
  Avoid overflow for large configured retry counts/durations.
- Protect **all** SDK headers case-insensitively: Authorization, Accept,
  User-Agent, X-TypeSafe-SDK, X-TypeSafe-Runtime, Content-Type, and
  X-TypeSafe-Retry-Count. Merge user headers first, then overwrite/remove these.
  Content-Type is `application/json` only with a body (remove even a caller's
  value on GET). Remove a caller's retry count on attempt zero.
- Authorization is `Bearer <key>`, Accept is `application/json`. Identify this
  port with `typesafe-sdk-go/<Version>` for User-Agent and X-TypeSafe-SDK; record
  the intentional identification difference from JS. X-TypeSafe-Runtime is
  `go/<version> (<GOOS>; <GOARCH>)`, stripping a leading `go` from runtime.Version
  before adding `go/`. Do not fabricate Node metadata.
- Redact authorization, proxy-authorization, x-api-key, cookie, and set-cookie
  in diagnostics. Go uses full-value redaction instead of JS's four-character
  key suffix. Default client/error formatting must be safe. Bodies and arbitrary
  caller headers are **not generally secret-scrubbed**: debug logging and dumps
  with bodies may contain user data or credentials placed in a body. Do not
  claim universal secret removal while promising unmodified raw payloads.
- The SDK does not mutate caller request data. Close every network response body
  before retry/return, including failures. Mid-body failures are transport
  failures subject to retry policy; malformed fully delivered JSON is not.
- README must explain that retries can repeat an evaluation whose response was
  lost, possibly incurring duplicate usage. Do not invent idempotency headers.

## 5. Examples and AI-facing documentation

Maintain complete, compiling `Example` functions in `example_test.go`, package
`typesafe_test`. Network examples have **no Output comment**, so `go test`
compiles but does not execute them. Add Output examples for local serialization
or deterministic httptest-backed examples where useful. No undefined `ctx`,
`req`, or `key` placeholders in a purported complete example.

Keep standalone programs for the three core primitives under `examples/choice`,
`examples/score`, and `examples/noul`. They may call the live service only when a
user explicitly runs them; ordinary tests build but never execute them. Their
README must state that live calls can incur usage.

The README quickstart mirrors this example (add package/imports in the file):

```go
ctx := context.Background()
client, err := typesafe.NewClient() // reads TYPESAFE_API_KEY
if err != nil {
    log.Fatal(err)
}
resp, err := client.SystemOne(ctx, typesafe.SystemOneRequest{
    State: map[string]any{"document": "I was charged twice. Please fix this ASAP."},
    Questions: typesafe.Questions{
        "billing": typesafe.Noul("Is this ticket about billing?"),
        "tone": typesafe.Choice("What is the customer's tone?", typesafe.ChoiceCriteria{
            "calm":       nil,
            "frustrated": nil,
            "angry":      nil,
        }),
        "urgency": typesafe.Score("How urgent is this ticket?",
            "can wait", "this week", "today"),
    },
})
if err != nil {
    log.Fatal(err)
}
tone, err := resp.Choice("tone")
if err != nil {
    log.Fatal(err)
}
fmt.Println(tone.Choice, tone.Confidence)
```

Use the complete keyed forms for described/structured criteria (these are usage
fragments to incorporate into complete Example functions):

```go
route := typesafe.ChoiceQuestion{
    Instructions: "Which team should handle this?",
    Criteria: typesafe.ChoiceCriteria{
        "returns":  "Returns, exchanges, and damaged items",
        "shipping": "Delivery progress and missing parcels",
        "billing":  "Charges and payment problems",
    },
}
severity := typesafe.ScoreQuestion{
    Instructions: "How severe is the issue?",
    Criteria: []typesafe.Entry{
        "Cosmetic only",
        map[string]any{"impact": "A feature is unavailable", "workaround": true},
        "Blocks all work",
    },
}
repeatContact := typesafe.NoulQuestion{
    Instructions: "Has the customer contacted support about this before?",
    Criteria: &typesafe.NoulCriteria{
        True:  "Mentions a previous request or ticket",
        False: "No indication of previous contact",
    },
}
```

Typed Score slices remain convenient; no string-to-Entry conversion is needed:

```go
levels := []string{"low", "medium", "high"}
q := typesafe.Score("Severity?", levels...)
nullLevels := typesafe.Score[typesafe.Entry]("Rank?", nil, nil)
```

The [primitives coverage matrix](docs/primitives-coverage.md) specifies the
official-example acceptance cases. Also include focused examples for:

- Client defaults and per-call timeout/retry/header overrides; WithResponseInto
  reader ownership and closure.
- Keyed ChoiceQuestion literals with named/described ChoiceCriteria entries,
  and NoulQuestion with both or only one true/false description; omitted, empty,
  and explicitly null fields must be explained alongside these examples.
- A per-request model using `Model: "jev-latest"`; omitted/empty Model inherits
  the client default, and the outgoing body always contains the resolved model.
- Dynamic IDs and reuse of a question value; Score with []string, []struct, and
  []Entry expansion; explicit Score[Entry] for mixed/nil entries; dynamic Choice maps.
- Structured Entry values, ordered structs/RawMessage, and nil versus omission.
- RawQuestion for an unknown question field/type, and response RawJSON.
- Models.List; errors.Is categories and errors.As APIError/TimeoutError details.

Document the central workflow: send state plus independent typed questions;
consume answers in ordinary Go control flow. Do not introduce chat messages,
roles, tools, or completion terminology absent from this API. README and package
Go docs serve SDK users and coding agents; AGENTS.md serves repository contributors.

## 6. Testing and surface coverage

Track official primitive examples in the [coverage matrix](docs/primitives-coverage.md);
link implemented fixtures/tests there as each case is verified.

1. Conformance fixtures contain source commit/test references, inputs, expected
   HTTP method/path/header subset/body, mocked responses, and result/error
   expectations. Go-specific differences are explicitly marked. Cross-check
   Python where relevant; do not require identical language representations.
2. A single `assertWireEqual` helper must understand JSON member order: ignore
   top-level envelope order and Questions key order. For modeled Choice criteria,
   adapt official fixture key order to Go's sorted order and assert that order;
   for RawQuestion preserve the supplied object member order. Preserve order in
   score arrays and structured Entry values, using expected Go ordering for maps
   and author ordering for structs/RawMessage. Do not decode everything into maps
   and then claim to check order. Compare decoded string values so equivalent
   escaping/whitespace does not cause false failures. Reordered Choice fixtures
   prove request shape and deterministic encoding, not identical model behavior.
3. Tests run offline under `-race` using httptest or an injected transport.
   Exercise environment precedence/trim/explicit-empty values; partial retry
   updates and copy isolation; all protected headers; zero/nil options/clients;
   timeout through body reading; failures mid-body; cancellation in backoff;
   Retry-After precedence and limits; every errors.Is relationship; raw response
   ownership; credential redaction; model envelope validation; unknown answers;
   fractional scores and structured legends; omitted/null/zero usage; choice
   sorted-key map encoding/replacement and map-copy isolation; helper versus
   literal encoding; empty/default model resolution;
   serialized State shape and raw-score validation; omission/null semantics;
   blank credentials/invalid base URLs; and JSON encoding errors.
   Check positive and expected-negative compilation cases for option scope and
   generic Score inference using the minimum toolchain. Keep intentionally
   invalid snippets in testdata, not normal build packages; assert the relevant
   type error, not merely any compiler failure. Score tests cover nil/empty/one
   level rejection, retained level order, structured/null levels, and copied outer
   slices. Noul tests cover omitted/empty/one-sided criteria, empty strings and
   explicit null, fractional probabilities, and caller-owned thresholding.
   Verify the removed Option/ChoiceOption API is absent in the surface baseline.
4. Do not use real retry sleeps for schedule assertions. Keep clocks and sleep
   hooks private, avoiding a public testing API. Fuzz custom JSON boundaries
   when useful; never send fuzz input to the live service.
5. Build a capability map starting with documented HTTP features and recording
   **exports of JS src/index.ts** as reference coverage, not obligations. Each
   reviewed JS export has a Go counterpart or an explicit exclusion/rationale;
   Go additions need no JS counterpart. Record expected Go signatures separately.
   A later surfacecheck verifies mapping coverage and Go declarations; it must
   not compare Go and TS parameter lists for literal equality. Check in the
   baseline so normal Go tests do not require Node, Python, or live GitHub.
6. CI checks the Go floor and latest stable, formatting, vet, race tests, and the
   pinned staticcheck version. Tool installation may require a newer host Go;
   library compatibility must still be tested with an actual floor toolchain.

## 7. Versioning, releases, and upstream sync

This independent Go module uses **its own semantic versions**. `Version` equals
its Go tag without `v`; UPSTREAM.md separately records the exact JS reference
version and commit, with supported capabilities and omissions in PARITY.md.
This allows Go bug/security fixes without waiting for a JS release. Do not use
build-metadata or ad hoc `-go.N` suffixes as patch releases.
An upstream major version does not automatically change this Go module's path.

- Preserve published Go API compatibility. Before v1, explain breaking changes
  in the changelog and use a new minor release; after v1, a breaking release
  requires a maintainer decision and the appropriate `/vN` module path.
- Tag CI checks the Go tag, Version, and changelog agree, and that upstream pins
  and parity coverage are valid. It must not require Go and JS versions to match.
- For a bad Go release, use `retract` when appropriate; publishing restrictions
  are defined in [AGENTS.md](AGENTS.md#core-constraints).
- New upstream release workflow: inspect pinned-to-target `src` and `test` diffs
  and changelogs; classify wire/behavior/API/type/docs/build changes; update
  fixtures first, then implementation/mapping, pins, parity, and changelog.
  Docs/build-only upstream changes do not force a Go release.
- Upstream-watch compares semantic versions (not lexical order or tag dates),
  normally targets stable JS releases, and deduplicates issues. It reports an
  available release; it does not automatically change pins or publish packages.
- A PR describes the concrete Go behavior, source commit/line references,
  adaptations, and validation. If a new conflict needs a maintainer decision,
  isolate it and explain the blocker instead of declaring the whole port done.
