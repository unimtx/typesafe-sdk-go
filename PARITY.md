# Parity decisions

This records the **intended contract and rationale**, while tests and the coverage
matrix record implementation evidence. Source paths below refer to the immutable pins and linked line ranges in
[UPSTREAM.md](UPSTREAM.md). [DESIGN.md](DESIGN.md) defines exact Go signatures
and behavior; [AGENTS.md](AGENTS.md) defines contributor constraints. The
[primitives coverage matrix](docs/primitives-coverage.md) tracks example acceptance
evidence separately from these design decisions.

## Design review decision — 2026-09-19

Priority: **Go idioms > HTTP API contract > JS SDK reference**. Go public shape
must remain idiomatic while serialized requests obey the HTTP contract. JS is
useful implementation evidence, not a requirement to reproduce every behavior.
The five external review suggestions are adopted with these qualifications:

| Suggestion | Decision and boundary |
|---|---|
| Plain-string request Model | Adopt. Empty means default. Pointers are idiomatic when absence/zero is meaningful; preserve them for Usage rather than declaring pointers inherently unidiomatic. Explicit empty-model passthrough is intentionally lost. |
| Compile-time option scopes | Adopt sealed ClientOption/RequestOption/CommonOption. Factory return types enforce scope; runtime validation still checks values. Use WithTimeout, without a redundant synonym. |
| Minimal sealed Question | Adopt a private marker only. Concrete types retain MarshalJSON. The previous sealed interface already excluded arbitrary external marshalers; this is interface simplification, not closing a type-safety hole. |
| Generic Score builder | Adopt to accept typed slices without conversion loops. Mixed concrete entries, all untyped nils, or no levels require explicit Entry type arguments; Criteria remains []Entry and results are not generic. |
| Remove SetExtraFields | Adopt for the initial API. JS really does forward extras; this is a deliberate capability omission. RawQuestion covers question-level extensions, not arbitrary top-level request fields. Existing primitive examples need neither escape hatch. |

Consequences: blank explicit client credentials/configuration are local errors;
State must encode as string/object/array, unlike JS's acceptance of null. Keep
structured/null question descriptions supported by the advanced guide. The
AGENTS/DESIGN/PARITY split remains: core contributor constraints, exact contract,
and rationale respectively. These decisions supersede earlier JS-parity choices.

## Choice, Score, and Noul review — 2026-09-19

This review applies the same priority to all three primitives. It supersedes the
previous requirement to preserve Choice author order through an ordered slice.
Exact signatures and nil/ownership rules live in DESIGN.md.

| Primitive | Decision | Reason and tradeoff |
|---|---|---|
| Choice | Use a named map for criteria; remove ChoiceOption and Option. | Go key/value syntax maps directly to the HTTP object. Standard JSON encoding sorts keys, so insertion order is lost. No duplicate-name validator or ordered-object encoder is needed. RawQuestion remains available for explicit wire order. |
| Score | Retain []Entry and the generic variadic helper. | HTTP array positions define levels. The generic helper handles typed slices; literals handle mixed structured descriptions clearly. Extra wrappers or numeric level maps add no value. Nil/empty/one-level scales are local errors at call time; results remain fractional. |
| Noul | Retain the single-instruction helper and optional pointer to fixed criteria fields. | Simple calls need no nil placeholder; advanced calls use named True/False descriptions. The pointer preserves omitted versus empty criteria, and fixed fields avoid key typos. No variadic optional-argument emulation, criteria builder, or boolean response conversion. |

Go maps provide deterministic serialized order, not a guarantee that model
outputs match differently ordered JS requests. Existing official examples remain
representable; adapt expected wire ordering explicitly. No live equivalence has
been established. Helpers copy outer criteria containers, while literals and
nested Entry data retain ordinary Go sharing semantics.

## Divergences

| Area | Go decision and reason | Upstream reference |
|---|---|---|
| Language surface | Port capabilities, not identical exported symbols/parameters. Contexts replace AbortSignal, errors replace throws, and TS generics/Promise composition have no literal Go counterpart. | JS `src/index.ts:1–22`, `src/types.ts:196–248`, `src/api-promise.ts:21–78` |
| Option scopes | Distinct sealed client/call interfaces with a shared CommonOption. Mis-scoped options fail compilation; values are validated at construction/call time. | JS `src/types.ts:196–248`; Go design decision |
| Configuration validation | Blank explicit API key/base URL/default model fails locally instead of reaching the service; base URL must be an absolute HTTP(S) URL without userinfo, query, or fragment. Invalid explicit options do not silently fall back. | JS `src/client.ts:259–289` is more permissive; Go design decision |
| State shape | Reject serialized null/boolean/number root State locally; inspect encoded shape so structs and custom marshalers work. Question descriptions still admit null. | HTTP API and state guide; JS `src/types.ts:10–17` admits null |
| Defensive Go inputs | Reject nil clients/options/question pointers, invalid raw JSON, and unsupported JSON values rather than panic. Custom callbacks and transports remain responsible for their own behavior. | JS `src/questions.ts:69–89`, `src/client.ts:70–148` |
| Documented question limits | Reject empty question IDs, Choice counts outside 1–255, Score counts outside 2–10, and Noul questions with neither non-null instructions nor a non-null outcome description. Apply the same rules to known RawQuestion kinds. | Official Choice maximum, Score bounds, Noul, and HTTP documentation; an empty Choice has no selectable result; JS only checks the Score minimum |
| Choice criteria | Named map from option names to Entry descriptions; question marshaling normalizes nil to {}. Standard encoding sorts keys, including numeric-looking keys lexicographically. Go map replacement semantics apply; no separate duplicate-name check or ChoiceOption/Option API. | HTTP Choice criteria object; JS `src/types.ts:34–46`, `src/questions.ts:49–62` |
| Entry ordering | Go maps sort string keys; structs and RawMessage allow callers to choose member order. RawMessage may be compacted/escaped; it is not a byte-passthrough guarantee. | JS `src/client.ts:362`; official structured-entry guide |
| Question helpers | Choice accepts a criteria map and shallow-copies it; generic Score copies ordered typed levels into []Entry while preserving nil/empty slices. Noul takes instructions only; literals supply criteria. Nested Entry values remain shared. | JS `src/questions.ts:16–62`; Go variadic assignability/type inference |
| Omitted/null noul fields | Keep an optional pointer to a struct with fixed True/False description fields; support either field alone. Nil omits an optional field, an empty string remains present, and RawMessage can express null. RawQuestion can express a null whole criteria object or omitted instructions. NoulAnswer remains a probability, not bool. | JS `src/types.ts:20–31,73–78`; official Noul and structured-entry guides |
| Request model | Plain string; empty selects the default. A private resolved payload always sends model. Deliberately omit JS's distinction between absent and explicit empty model for a useful Go zero value. | JS `src/client.ts:315–319`; HTTP request model field |
| Extra fields | No SetExtraFields or arbitrary top-level request fields initially. Entry holds free-form domain data; RawQuestion supports question-level extensions. Add supported HTTP fields deliberately rather than a generic merge API. | JS `src/client.ts:315–324,362`, `test/client.test.ts:413–425` prove forwarding exists |
| Usage | `*int` token fields preserve absent/null versus zero. HTTP docs do not mark the individual counts required; Python permits None. JS's required numeric static types do not enforce runtime validation. | JS `src/types.ts:126–142`, `src/client.ts:342–345`; Python `_core/response_types.py:65–73` |
| Answer decoding | Concrete Go variants with int score-map keys; unknown string types retain their raw JSON as UnknownAnswer. Missing/invalid discriminators and incompatible known field types error. Python drops unknown answers; JS keeps unvalidated parsed data. | JS `src/types.ts:72–142`, `src/client.ts:342–345`; Python `_core/response_types.py:79–96` |
| Score legends | `map[int]Entry` admits structured/null descriptions. Do not narrow to strings or use integer Score values; Score is a fractional expectation. | JS `src/types.ts:98–114`; official structured-entry guide |
| Success parsing | Parse irrespective of Content-Type and return ResponseValidationError for malformed/incompatible typed success bodies. Require documented top-level and known-variant fields plus request/answer discriminator agreement; retain unknown fields and answer variants. Decode errors are not retried. | JS `src/client.ts:342–345,472–489` uses an unchecked cast; Python `_core/errors.py:176–196` has a dedicated validation error |
| Model listing | Return `[]ModelCard` after validating/unwrapping `{models:[...]}`, matching JS. No public ListModelsResponse even though Python has one. Raw envelope access uses WithResponseInto. | JS `src/resources/models.ts:14–28`; Python `_core/response_types.py:142–165` |
| Raw HTTP access | WithResponseInto provides a sanitized buffered snapshot alongside typed decoding, including on errors. Body ownership differs from JS; it is an independent memory reader. There is no separate raw-only Promise path that bypasses typed decoding. | JS `src/api-promise.ts:36–53` |
| Error form | Sentinels and errors.As replace classes. APIError adds method/path/request-ID context and dump helpers; ResponseValidationError adds field-path/cause/response context for incompatible 2xx bodies; RetryAfter preserves rate-limit metadata, TimeoutError.Duration preserves timeout metadata. | JS `src/errors.ts:15–122`; Python `_core/errors.py:176–196` |
| Logging | `slog` with a discard default instead of console. SDK verbosity and the caller's handler filtering both apply; setting debug cannot enable a handler that discards debug events. | JS `src/logging.ts:20–47` |
| Redaction | Fully redact known credential headers instead of retaining JS's last four characters. Sanitize stored request/response snapshots, not the actual HTTP request. Raw bodies and arbitrary caller fields are not generally redacted. | JS `src/logging.ts:53–79` |
| Identity headers | Identify this independent port as `typesafe-sdk-go/<Version>` and the actual Go runtime/platform. Protect the same header names as JS. | JS `src/client.ts:352–367`, `src/runtime.ts:20–31` |
| Immutability | Copy mutable configuration; expose only BaseURL, DefaultModel, Timeout, RetryPolicy accessors. Caller-owned Models field/components and request data must not be mutated concurrently. | JS `src/client.ts:234–256,279–288` |
| Runtime | Standard Go HTTP transport, durations as time.Duration, no browser/WASM target. Injected transport/client settings remain effective; no promise to kill a transport ignoring context. | JS `src/types.ts:174–209,222–248` |
| Release policy | Independent Go semver with exact JS reference pins and explicit capability coverage. Go fixes can ship immediately; JS-only changes need not force a Go release. | Repository policy; JS and Python already have distinct release histories |

## Go-only additions

These are approved design adaptations; they are not a license to grow the API
without evidence. Record newly introduced exports in the capability mapping.

- NewClient and option.ClientOption/RequestOption/CommonOption with scoped With*
  factories: Go configuration/call options with compile-time scope enforcement.
- `Entry` alias, sealed `Question`/`Answer`, concrete question/answer structs,
  ChoiceCriteria and NoulCriteria: native map/slice/struct representations of the
  corresponding HTTP shapes. No public choice-option value type is needed.
- RawQuestion: an escape hatch for question JSON without a dedicated modeled
  field/type. Arbitrary top-level request extras are not supported.
- `UnknownAnswer` and RawJSON on SystemOneResponse/answers: preserve future fields
  and answer kinds. RawJSON is an immutable received snapshot, not a live view.
- Noul, Choice, Score singular accessors: allocation-free typed lookups with
  distinct missing/type-mismatch errors. Nouls, Choices, Scores remain grouped
  typed maps inspired by Python `_core/response_types.py:112–125`; direct type
  assertions also work.
- ErrTypeSafe and class-specific sentinels, Error, APIError,
  ResponseValidationError, TimeoutError, APIError.RetryAfter, and error dump
  methods: Go error handling and diagnostics.
- Client/String/GoString/LogValue safety methods and read-only accessors;
  WithHTTPClient, WithLogger, WithResponseInto adapt fetch/logger/response access.
- DefaultBaseURL, DefaultModel, DefaultRetryPolicy, and named LogLevel constants
  expose useful fixed defaults/values. Some JS defaults are module-internal,
  rather than package-entrypoint exports; their Go export is deliberate.

## Resolved questions

- **Models endpoint and result:** GET `/v1/models` returns a wire envelope; JS
  unwraps it to a list. Go follows JS and preserves envelope access separately.
- **Timeout hierarchy:** APITimeoutError extends APIConnectionError. Go timeout
  errors match both sentinels; caller aborts do not become attempt timeouts.
- **Protected headers:** all seven SDK-controlled headers are protected, not
  just Authorization and Content-Type. GET removes a supplied Content-Type;
  attempt zero removes a supplied retry-count header.
- **Extra forwarding:** JS shallow-spreads the request and serializes its nested
  questions directly; validateQuestions does not rebuild them. Request,
  question, and nested criteria extras therefore reach serialization. We omit
  the general-purpose mutation API intentionally; absence of JS forwarding is
  not the justification. RawQuestion retains unknown question fields.
- **Question IDs:** wire-visible keys, excluded from model inference according
  to the HTTP reference. "Client-side only" was incorrect.
- **Optional scalar:** request Model deliberately collapses absent/empty into
  default selection. Meaningful absent/zero distinctions still use pointers,
  including Usage counts. Exact JS nullish fallback is not a requirement.
- **Official documentation breadth:** the advanced guide explicitly supports
  structured/null question entries even where the concise HTTP page shows
  string-only descriptions. Keep the richer shape.

## Open questions

These do not justify speculative validation or live API calls.

- The HTTP reference marks instructions required with string/object/array shapes,
  while the advanced guide explicitly permits null and JS also accepts omission.
  **Decision:** retain the richer documented null shape, allow criteria-only Noul,
  and reject a Noul only when neither instructions nor a true/false description
  supplies a non-null value. Root State has no such advanced-guide exception:
  follow its documented string/object/array shape.
- Key-order effects on model inference are not a universal API guarantee. Keep
  the deliberate Go wire ordering and user-supplied structure; do not claim that
  arbitrary reorderings are semantically identical. Question independence and
  exclusion of IDs from inference do not prove all other order invariance.
- The minimum Go version remains 1.24 as a compatibility target. As of the
  2026-09-19 check, current stable is Go 1.27.1 and Go's two-newer-major release
  policy means 1.24 is no longer upstream-maintained. The implementation still
  passes the actual 1.24.0 toolchain; raising the floor remains a separate
  maintainer decision rather than a silent SDK feature change.
