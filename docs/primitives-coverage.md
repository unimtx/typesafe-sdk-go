# Primitives example coverage

Reviewed on **2026-09-19** against these five official pages, including their
embedded playground requests, Python snippets, response JSON, and illustrative
input/output variations:

- [Primitives overview](https://docs.typesafe.ai/primitives)
- [Choice](https://docs.typesafe.ai/primitives/choice)
- [Score](https://docs.typesafe.ai/primitives/score)
- [Noul](https://docs.typesafe.ai/primitives/noul)
- [Advanced: structure](https://docs.typesafe.ai/primitives/advanced)

**Design result:** the current public API can express all examples on these
pages using modeled requests/questions/answers and ordinary Go control flow.
These examples need no RawQuestion escape hatch or generic extra-field API.
Choice key order is adapted to Go's sorted map encoding; Score level order is
preserved. Coverage means representable data/workflows, not guaranteed identical
model behavior after reordering. It covers these pages, not every linked
cookbook, demo, or future documentation edit.

**Implementation status: verified offline for every row below.** The shared
fixture file is [`testdata/conformance/primitives.json`](../testdata/conformance/primitives.json),
contains 25 base/variant cases across the 18 matrix IDs and is exercised through
the public API by `TestPrimitiveConformanceFixtures`; compiled
usage is in [`example_test.go`](../example_test.go), with runnable primitive
programs under [`examples`](../examples). Fixed mocked outputs are not
live-service evidence. The matrix owns example coverage;
[DESIGN.md](../DESIGN.md) owns exact API/behavior, [UPSTREAM.md](../UPSTREAM.md)
pins the SDK baseline, and [PARITY.md](../PARITY.md) records adaptations.

## How to turn a row into evidence

- Add language-neutral request/response fixtures under `testdata/conformance/`
  for wire cases, with the case ID, source page/section, retrieval date, and the
  relevant pinned JS source/test reference. Identify documentation-derived
  cases honestly; do not invent a matching upstream test.
- Add complete Go examples in `example_test.go` and deterministic offline tests
  for workflows. The same fixture may serve equivalent playground/Python forms;
  different inputs or shapes remain named variants, not silently dropped cases.
- Test through the public API using httptest/injected transport. Use DESIGN.md's
  wire-comparison rules, with explicit expected Go ordering. For examples where
  source object order must be retained, use keyed structs or json.RawMessage
  within Entry; Go maps cannot preserve insertion order. ChoiceCriteria is a map:
  adapt source fixtures to sorted option names and verify deterministic encoding.
  Order-controlled RawQuestion cases are supplemental, not needed by these examples.
- Feed published response JSON as a mock to verify decoding and business logic.
  Never assert that a live model must reproduce documentation probabilities,
  confidence, scores, token counts, latency, or pricing.
- After verification, add actual fixture paths and test/example function links
  in the evidence column. Mark a row verified only after all listed variants
  have the relevant request, response, and workflow checks. Compilation alone
  proves an example's API usage, not its wire correctness.
- New official examples require a new row or an explicit variant in an existing
  row. SDK changes affecting these capabilities must revisit the affected rows.

## Overview examples

Source: [Primitives overview](https://docs.typesafe.ai/primitives).
All rows are implemented and verified with offline fixtures and workflow tests.

| ID | Official example / variants | Go expression | Required acceptance checks | Evidence |
|---|---|---|---|---|
| P01 | Define a refund question; yes/no prompt variations | Noul inside Questions | ID is the wire map key, fixed noul discriminator, instructions preserved, no ID inserted into the question | P01 fixture; `TestPrimitiveConformanceFixtures` |
| P02 | Refer to fields in a nested ticket/order/policy state | Structured State and string instructions containing field paths | Preserve arrays, nested numbers/strings, and literal backticks/paths; SDK does not interpret paths | P02 fixture; `TestPrimitiveConformanceFixtures` |
| P03 | Mixed Choice/Noul/Score request; both the API-outage playground and flight/refund Python example | One SystemOneRequest with all three typed questions | One HTTP request per input; shared state; answer IDs and value variants preserved; typed lookup examples compile | P03 fixture; `ExampleClient_SystemOne` |
| P04 | Speculative questions and dependent follow-up requests | Select needed answers in Go; construct another SystemOneRequest only for a real dependency | Mock one-call fan-out and two-call dependency separately; second payload uses the first answer; no automatic hidden chaining | P04 fixture; `TestDependentAndApplicationWorkflows` |

P04 covers the workflow described on the overview page. Linked cookbook
implementations are outside this matrix's audited scope. Illustrative comparisons
of Choice/Score/Noul use the same modeled shapes as the detailed cases below.

## Choice examples

Source: [Choice](https://docs.typesafe.ai/primitives/choice).
All rows are implemented and verified offline.

| ID | Official example / variants | Go expression | Required acceptance checks | Evidence |
|---|---|---|---|---|
| C01 | Basic team routing, playground and Python forms; language/meeting/category prompt variations | Choice with a ChoiceCriteria map | Criteria serialize as a sorted-key object, descriptions as strings, selected option/confidence/full probability map decode | C01 fixture; `ExampleChoice`; `examples/choice` |
| C02 | Five-question triage and its conditional routing code | Questions with five Choice values; nil descriptions for tone; Choices or type assertions | All five questions share one call; nil descriptions remain null; unused speculative answers can be ignored; confidence branches and secondary-team probability iteration work with fixed mocks | C02 fixture; `TestPrimitiveConformanceFixtures` |
| C03 | Distinguish return policy from return status using structured instructions and option descriptions | Entry objects/arrays in Instructions and ChoiceCriteria values | Preserve arbitrary nested fields; do not stringify objects or reserve user-selected keys; structured request and published response decode | C03 fixture; `TestPrimitiveConformanceFixtures` |

Thresholds, routing destinations, notification functions, and refund approvals
are application code in C02, not SDK operations. Tests use local fakes for those
application actions. They must not send messages or perform refunds.

## Score examples

Source: [Score](https://docs.typesafe.ai/primitives/score).
All rows are implemented and verified offline.

| ID | Official example / variants | Go expression | Required acceptance checks | Evidence |
|---|---|---|---|---|
| S01 | Basic bug severity, playground and Python forms; clothing/experience scales | Score with ordered Entry levels; ScoreAnswer | Criteria are an array, never an integer-keyed request object; score remains fractional; response legend/probabilities decode to integer-keyed maps | S01 fixture; `ExampleScore`; `examples/score` |
| S02 | Reading scores across input variations; numeric-string level counterexample | Different State strings with ordinary Score criteria | Decode sample boundary and fractional values without rounding; encode numeric descriptions as strings; do not reject a valid but poorly worded rubric | S02 fixture; `TestPrimitiveConformanceFixtures` |
| S03 | Composite triage with severity, frustration, and report quality | Three Score questions; retain typed question values for criteria lengths; use Scores for answers | One call; normalize by each scale's own length minus one; combine weights in caller code; verify calculation against fixed response values with floating-point tolerance | S03 fixture; `TestDependentAndApplicationWorkflows` |
| S04 | Structured level descriptions and the returned structured legend; alternate description variations | Entry objects in score levels and Legend values | Round-trip request objects/arrays with chosen order; decode full object-valued legend; preserve score/confidence/probabilities and nested description data | S04 fixture; `TestPrimitiveConformanceFixtures` |

S03 does not need a normalization or weighted-score helper in the SDK. A Go
example can keep its questions in typed variables before inserting them into
Questions; a Question interface lookup otherwise needs a type assertion before
accessing Criteria. Demonstrate this explicitly rather than copying Python's
dynamic field access. Application logic may round a score, but the SDK must not.

## Noul examples

Source: [Noul](https://docs.typesafe.ai/primitives/noul).
All rows are implemented and verified offline.

| ID | Official example / variants | Go expression | Required acceptance checks | Evidence |
|---|---|---|---|---|
| N01 | Human escalation and repeat contact together; question/statement prompt variations | Noul for the simple question; NoulQuestion with NoulCriteria for the clarified one | One request with criteria omitted for one question and true/false descriptions for the other; decode the two answers' yes probabilities; threshold in caller code; no invented confidence or probability-map field | N01 fixture; `ExampleNoul`; `examples/noul` |

## Structured-entry examples

Source: [Advanced: structure](https://docs.typesafe.ai/primitives/advanced).
All rows are implemented and verified offline.

| ID | Official example / variants | Go expression | Required acceptance checks | Evidence |
|---|---|---|---|---|
| A01 | Invoice verification/extraction with Noul, Choice, and two Scores | Entry objects as instructions; nil Choice descriptions; four questions in one request | Preserve nested schema-like fields and literal names; send null descriptions; keep distinct ordered score scales; no local interpretation of instruction contents | A01 fixture; `TestPrimitiveConformanceFixtures` |
| A02 | Instruction object containing a list of fields to compare | Entry object containing an array | Preserve the nested array and literal field paths; also include an array-valued Instructions case to cover the page's accepted-shape table | A02 fixture; `TestPrimitiveConformanceFixtures` |
| A03 | Structured option descriptions for billing/orders/account | ChoiceCriteria values containing objects with nested arrays | Preserve all caller-defined fields and arrays; no new SDK fields for rubric vocabulary | A03 fixture; `TestPrimitiveConformanceFixtures` |
| A04 | Classification taxonomy with object and array subtrees | Dynamically build a ChoiceCriteria map; each value carries the corresponding subtree | First request retains nested taxonomy data; mock next-level selection and construct a second request with selected children encoded by sorted key; no implicit traversal in SDK | A04 fixture; `TestDependentAndApplicationWorkflows` |
| A05 | Structured score levels for evaluating change scope | Score with Entry instructions and object-valued levels | Preserve level order and each level's nested arrays; use a synthetic structured response if needed and label it as synthetic | A05 fixture (synthetic response); `TestPrimitiveConformanceFixtures` |
| A06 | Structured true/false descriptions for credential-request detection | NoulQuestion with Entry Instructions and NoulCriteria.True/False objects | Preserve nested state, instruction, and both criteria objects; use fabricated message data; no special security-classification API | A06 fixture; `TestPrimitiveConformanceFixtures` |

## Shape and boundary checklist

The matrix groups examples by behavior rather than copying entire official
pages. All 13 embedded playground request examples from the five reviewed pages
are represented: P03, C01–C03, S01/S03/S04, N01, A01/A03–A06. Python equivalents,
standalone snippets, prompt variations, and tabled outputs are included as
variants or separate rows above. These counts are an audit snapshot, not a
permanent assumption about the live documentation.

Also test the accepted-shape table from the advanced page: string, object, array,
and null at Instructions, Choice descriptions, Score levels, and Noul true/false
descriptions. Follow DESIGN.md's nil/omission rules, especially explicit null in
NoulCriteria. Supplemental shape cases are design tests, not claims that every
combination appears as a standalone official example.

Keep these translation boundaries explicit:

- Playground `selectedModels` is UI metadata. Use the request's Model/client
  default; do not introduce a selectedModels wire field or multi-model endpoint.
- A JSON Choice criteria object becomes ChoiceCriteria, a named map. Names are
  keys and descriptions are Entry values. No ChoiceOption/Option constructor or
  duplicate-name check is needed; map assignments replace existing values.
  Encoding sorts keys, including numeric-looking keys; author order is not kept.
  Include nil/empty maps, null/structured values, and helper copy-isolation tests.
- Python's direct answer attributes become type assertions or typed answer maps.
  Python context managers do not imply a Go Client.Close method.
- Scores are ordered level descriptions on input and fractional values on output;
  a response legend is not the request criteria shape. Add Go-specific compilation
  variants for []string/[]struct/[]Entry expansion and explicit Score[Entry] for
  mixed or all-null descriptions. These supplement S01/S04/A05, not new official
  examples. Direct literals with []Entry remain supported. Verify nil, empty,
  and one-level criteria fail locally when called, and response scores remain
  fractional rather than rounded or converted to probabilities.
- Noul uses the single-instruction helper or a keyed NoulQuestion with an optional
  *NoulCriteria. Supplement N01/A06 with absent, empty, one-sided, and explicit-null
  criteria-field cases. True/False are descriptions, not bool inputs; the result
  is a yes probability and the application chooses its threshold.
- Root State is string/object/array; nil State is rejected locally. Null question
  instructions/descriptions remain supported. Empty Model selects the default;
  serialization of a client request includes the resolved model.
- Official pages describe Choice with up to 255 options and Score with 2–10
  levels. SystemOne also rejects an empty Choice, which has no selectable result,
  and applies the bounds to modeled and known raw questions.
- Ordinary examples need no extra-field escape hatch. Unknown/future types,
  invalid inputs, retries, cancellation, and diagnostics are separate design
  conformance tests, not gaps in these primitive examples.
