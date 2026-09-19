# Upstream baseline

Reviewed on **2026-09-19**. These are source pins for the design and
implementation. Offline conformance evidence is recorded in tests and the
coverage matrix. Separately authorized live-service verification is recorded in
[`docs/live-coverage.md`](docs/live-coverage.md); neither kind of evidence
guarantees future service behavior.
[DESIGN.md](DESIGN.md) owns the implementation contract; [PARITY.md](PARITY.md)
explains adaptations and intentional omissions. Source pins and evidence are
owned here; they do not impose exact JS parity. Public design follows Go idioms,
wire behavior follows the HTTP contract, and JS supplies implementation evidence.

## Pins

| Role | Repository | Tag | Commit |
|---|---|---|---|
| Implementation reference | [typesafe-ai/typesafe-sdk-js](https://github.com/typesafe-ai/typesafe-sdk-js) | `v0.6.0` | `66880ccded6cb642dc1809620c2b108c33730214` |
| Secondary cross-check | [typesafe-ai/typesafe-sdk-python](https://github.com/typesafe-ai/typesafe-sdk-python) | `v0.7.0` | `2ce5c65f13646cab6e6f782328194c9d85f3300a` |

Both tags and commit IDs were verified from their repositories. They were the
latest available stable tags at review time. Python v0.7.0 is newer than the JS
baseline; its Pydantic/response-model additions are not Go port requirements.
The Go release version is independent; the initial implementation targets 0.1.0
and remains unreleased until a maintainer publishes its tag.

Optional reference checkouts belong under gitignored `.upstream/js` and
`.upstream/python`. Verify HEAD against the full commit above after checkout;
do not implement against a moving branch or trust a tag name without its commit.

## HTTP documentation

These official pages were read during the review. They are live, unversioned
documents; the access date is not an immutable specification version.

- [HTTP API](https://docs.typesafe.ai/api.md): evaluation schema, question IDs,
  fractional scores, usage fields, and HTTP error statuses.
- [Primitives overview](https://docs.typesafe.ai/primitives): shared state, mixed
  questions, answer composition, and dependent follow-up calls.
- [Choice](https://docs.typesafe.ai/primitives/choice),
  [Score](https://docs.typesafe.ai/primitives/score), and
  [Noul](https://docs.typesafe.ai/primitives/noul): worked request/response and
  application examples, cross-checked in the
  [primitives coverage matrix](docs/primitives-coverage.md).
- [Structured entries](https://docs.typesafe.ai/primitives/advanced.md): structured
  or null instructions and descriptions across all three primitives.
- [State](https://docs.typesafe.ai/concepts/state.md): one shared state, independently
  evaluated questions, string/object/array input.
- [Documentation index](https://docs.typesafe.ai/llms.txt): discover related pages.

HTTP documentation alone does not settle every SDK input or runtime detail.
See PARITY.md for known differences and limits of this review. Record source
references in each conformance fixture; snapshot relevant documented facts when
adding fixtures rather than assuming a live page never changes.

## Immutable JS reference index

| Subject | Source |
|---|---|
| Public package exports | [src/index.ts:1–22](https://github.com/typesafe-ai/typesafe-sdk-js/blob/66880ccded6cb642dc1809620c2b108c33730214/src/index.ts#L1-L22) |
| Entry and question fields | [src/types.ts:1–66](https://github.com/typesafe-ai/typesafe-sdk-js/blob/66880ccded6cb642dc1809620c2b108c33730214/src/types.ts#L1-L66) |
| Answers, usage, model cards | [src/types.ts:72–149](https://github.com/typesafe-ai/typesafe-sdk-js/blob/66880ccded6cb642dc1809620c2b108c33730214/src/types.ts#L72-L149) |
| Requests, retry fields, option scopes | [src/types.ts:155–248](https://github.com/typesafe-ai/typesafe-sdk-js/blob/66880ccded6cb642dc1809620c2b108c33730214/src/types.ts#L155-L248) |
| Constructors and limited local validation | [src/questions.ts:16–89](https://github.com/typesafe-ai/typesafe-sdk-js/blob/66880ccded6cb642dc1809620c2b108c33730214/src/questions.ts#L16-L89) |
| Retry validation and timeout classification | [src/client.ts:70–157](https://github.com/typesafe-ai/typesafe-sdk-js/blob/66880ccded6cb642dc1809620c2b108c33730214/src/client.ts#L70-L157) |
| Configuration and environment | [src/client.ts:259–289](https://github.com/typesafe-ai/typesafe-sdk-js/blob/66880ccded6cb642dc1809620c2b108c33730214/src/client.ts#L259-L289), [src/env.ts:15–23](https://github.com/typesafe-ai/typesafe-sdk-js/blob/66880ccded6cb642dc1809620c2b108c33730214/src/env.ts#L15-L23) |
| Request forwarding and nullish model fallback | [src/client.ts:311–324](https://github.com/typesafe-ai/typesafe-sdk-js/blob/66880ccded6cb642dc1809620c2b108c33730214/src/client.ts#L311-L324) |
| Protected headers, body serialization, retries | [src/client.ts:350–400](https://github.com/typesafe-ai/typesafe-sdk-js/blob/66880ccded6cb642dc1809620c2b108c33730214/src/client.ts#L350-L400) |
| Body timeout, aborts, JSON/text parsing | [src/client.ts:403–489](https://github.com/typesafe-ai/typesafe-sdk-js/blob/66880ccded6cb642dc1809620c2b108c33730214/src/client.ts#L403-L489) |
| Error messages, classes, retry/timeout metadata | [src/errors.ts:15–122](https://github.com/typesafe-ai/typesafe-sdk-js/blob/66880ccded6cb642dc1809620c2b108c33730214/src/errors.ts#L15-L122) |
| Retry defaults, header parsing, backoff | [src/retry.ts:5–67](https://github.com/typesafe-ai/typesafe-sdk-js/blob/66880ccded6cb642dc1809620c2b108c33730214/src/retry.ts#L5-L67) |
| Logger defaults and redaction | [src/logging.ts:4–79](https://github.com/typesafe-ai/typesafe-sdk-js/blob/66880ccded6cb642dc1809620c2b108c33730214/src/logging.ts#L4-L79) |
| Runtime header format | [src/runtime.ts:20–31](https://github.com/typesafe-ai/typesafe-sdk-js/blob/66880ccded6cb642dc1809620c2b108c33730214/src/runtime.ts#L20-L31) |
| Request ID, raw/parsed HTTP response ownership | [src/api-promise.ts:1–58](https://github.com/typesafe-ai/typesafe-sdk-js/blob/66880ccded6cb642dc1809620c2b108c33730214/src/api-promise.ts#L1-L58) |
| Models endpoint, envelope check, list return | [src/resources/models.ts:14–28](https://github.com/typesafe-ai/typesafe-sdk-js/blob/66880ccded6cb642dc1809620c2b108c33730214/src/resources/models.ts#L14-L28) |
| Header and body-delivery regressions | [test/release-regressions.test.ts:15–153](https://github.com/typesafe-ai/typesafe-sdk-js/blob/66880ccded6cb642dc1809620c2b108c33730214/test/release-regressions.test.ts#L15-L153) |
| Model response shape and raw fields | [test/client.test.ts:164–195](https://github.com/typesafe-ai/typesafe-sdk-js/blob/66880ccded6cb642dc1809620c2b108c33730214/test/client.test.ts#L164-L195) |
| Request extras including null | [test/client.test.ts:413–425](https://github.com/typesafe-ai/typesafe-sdk-js/blob/66880ccded6cb642dc1809620c2b108c33730214/test/client.test.ts#L413-L425) |
| Release notes | [docs/changelog.md](https://github.com/typesafe-ai/typesafe-sdk-js/blob/66880ccded6cb642dc1809620c2b108c33730214/docs/changelog.md) |
| Copyright to retain when porting | [LICENSE](https://github.com/typesafe-ai/typesafe-sdk-js/blob/66880ccded6cb642dc1809620c2b108c33730214/LICENSE) |

## Immutable Python cross-checks

- [Response types:65–73](https://github.com/typesafe-ai/typesafe-sdk-python/blob/2ce5c65f13646cab6e6f782328194c9d85f3300a/src/typesafe_sdk/_core/response_types.py#L65-L73): unreported token counts use None.
- [Response types:79–125](https://github.com/typesafe-ai/typesafe-sdk-python/blob/2ce5c65f13646cab6e6f782328194c9d85f3300a/src/typesafe_sdk/_core/response_types.py#L79-L125): unknown answer handling and grouped answer accessors.
- [Response types:142–165](https://github.com/typesafe-ai/typesafe-sdk-python/blob/2ce5c65f13646cab6e6f782328194c9d85f3300a/src/typesafe_sdk/_core/response_types.py#L142-L165): model metadata and wrapped list response.
- [Response validation error:176–196](https://github.com/typesafe-ai/typesafe-sdk-python/blob/2ce5c65f13646cab6e6f782328194c9d85f3300a/src/typesafe_sdk/_core/errors.py#L176-L196): dedicated response-shape error with field-path and HTTP metadata.
- [Question types:14–35](https://github.com/typesafe-ai/typesafe-sdk-python/blob/2ce5c65f13646cab6e6f782328194c9d85f3300a/src/typesafe_sdk/_core/question_types.py#L14-L35): optional nullable instructions and noul criteria.
- [Changelog](https://github.com/typesafe-ai/typesafe-sdk-python/blob/2ce5c65f13646cab6e6f782328194c9d85f3300a/docs/changelog.md): v0.7.0 scope relative to v0.6.0.

## Go language baseline

[Go 1.24 release notes](https://go.dev/doc/go1.24) confirm `omitzero` and
`slog.DiscardHandler`. Use [Go release history](https://go.dev/doc/devel/release)
for the current stable CI version. A minimum language/library version is not a
claim that the Go project still provides security maintenance for that release.
At the 2026-09-19 implementation check, the official release history listed
Go 1.27.1 (released 2026-09-01) as current stable; both Go 1.24.0 and 1.27.1
were run locally rather than inferred from CI labels.

The [Go specification on variadic arguments](https://go.dev/ref/spec#Passing_arguments_to_..._parameters)
requires an expanded slice to be assignable to the variadic slice type; []string
is not []any. [Type inference](https://go.dev/ref/spec#Type_inference) explains
why generic Score can accept typed slices but needs explicit Entry type arguments
for heterogeneous entries, only untyped nil arguments, or no levels.

## Primitive shape decisions

The official [Choice request structure](https://docs.typesafe.ai/primitives/choice)
defines criteria as an option-name/description map. [Score](https://docs.typesafe.ai/primitives/score)
defines an ordered array whose positions are level indices, and a fractional
expected score. [Noul](https://docs.typesafe.ai/primitives/noul) defines optional
true/false descriptions and returns a single yes probability, without a separate
confidence field. These shapes motivate the Go map/slice/fixed-struct distinction.

[Go 1.24 encoding/json.Marshal](https://pkg.go.dev/encoding/json@go1.24.0#Marshal)
documents sorted map keys and nil map/slice encoding. ChoiceQuestion normalizes
its nil criteria map to an object; Score rejects nil/short scales at call time.
Pointer and omitzero rules distinguish omitted Noul criteria from an empty object.
These are Go representation decisions, not claims about live model equivalence.
