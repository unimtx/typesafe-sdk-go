# Capability map

Reviewed against TypeSafe JavaScript SDK `v0.6.0` at commit
`66880ccded6cb642dc1809620c2b108c33730214`. This is reference coverage, not a
promise of identical language APIs. Exact Go behavior is owned by `DESIGN.md`;
the checked declaration baseline is `internal/tools/surfacecheck/surface.txt`.

## HTTP capabilities

| HTTP capability | Go surface | Offline evidence |
|---|---|---|
| `POST /v1/systemone` | `(*Client).SystemOne(context.Context, SystemOneRequest, ...option.RequestOption)` | `TestPrimitiveConformanceFixtures`, `TestSystemOneWireHeadersDecodeAndSnapshot` |
| `GET /v1/models` | `(ModelService).List(context.Context, ...option.RequestOption)` | `TestModelsListGETAndEnvelopeValidation` |
| Choice question/answer | `Choice`, `ChoiceQuestion`, `ChoiceCriteria`, `ChoiceAnswer` | C01–C03 fixtures |
| Score question/answer | generic `Score`, `ScoreQuestion`, `ScoreAnswer` | S01–S04 fixtures and compilation cases |
| Noul question/answer | `Noul`, `NoulQuestion`, `NoulCriteria`, `NoulAnswer` | N01 fixture and nil/omission tests |
| Future question/answer forms | `RawQuestion`, `UnknownAnswer` | `TestRawQuestionPreservesMemberOrder`, `TestDecodeAnswersAndRawJSON` |
| Raw response and original JSON | `option.WithResponseInto`, `SystemOneResponse.RawJSON`, `Answer.RawJSON` | snapshot, decode-error, and raw JSON tests |
| Status/transport errors | sentinels, `Error`, `APIError`, `TimeoutError` | `TestAPIErrorCategoriesBodyAndDumps`, timeout/cancellation tests |
| Retry, timeout, cancellation | `RetryPolicy`, common options, request contexts | retry and reliability tests |

## Pinned JavaScript export review

| JavaScript `src/index.ts` export | Go counterpart or exclusion |
|---|---|
| `TypeSafeClient` | `typesafe.Client`, constructed by `NewClient` |
| `APIPromise`, `WithResponse` | Omitted. Go methods are synchronous and context-aware; raw snapshots use `WithResponseInto`. |
| `Models` | `typesafe.ModelService`, exposed as `Client.Models` |
| `choice`, `score`, `noul` | `typesafe.Choice`, generic `Score`, and `Noul` |
| `TypeSafeError` | `typesafe.Error` plus `ErrTypeSafe` |
| `APIError` and HTTP subclasses | `typesafe.APIError` plus status sentinels |
| `APIConnectionError` | private wrapper matching `ErrConnection` |
| `APITimeoutError` | `typesafe.TimeoutError`, matching both timeout and connection sentinels |
| `APIUserAbortError` | private wrapper matching `ErrUserAbort` and the caller context error |
| `ENV`, `EnvVar` | Intentionally private environment names |
| `LOG_LEVELS`, `LogLevel` | `option.LogLevel` and five named constants; no mutable slice |
| `VERSION` | `typesafe.Version` |
| `JsonValue`, `EntryType`, `Description` | Unified as `typesafe.Entry`; call-time checks enforce the narrower root State shape |
| `NoulQuestion`, `ChoiceQuestion`, `ScoreQuestion`, `Question`, `Questions`, `ChoiceCriteria` | Corresponding Go question types; Score criteria are `[]Entry`, and Noul criteria use `NoulCriteria` |
| `ScoreCriteria` | `[]typesafe.Entry`; the helper is generic only to accept expanded typed slices |
| `NoulResponse`, `ChoiceResponse`, `ScoreResponse` | `NoulAnswer`, `ChoiceAnswer`, `ScoreAnswer` |
| `Usage`, `SystemOneResult`, `ModelCard` | `Usage`, `SystemOneResponse`, `ModelCard` |
| `SystemOneRequest`, `SystemOneRequestPayload` | Public `SystemOneRequest`; the always-resolved payload remains private |
| `RetryPolicy` | `option.RetryPolicy` with Go durations and copied status slices |
| `RequestOptions`, `TypeSafeClientConfig` | Sealed `option.RequestOption`, `ClientOption`, and `CommonOption` factories |
| `LogLevel`, `Logger` | `option.LogLevel` constants and injected `*slog.Logger` |
| `Fetch` | `option.WithHTTPClient(*http.Client)`; transports compose through `RoundTripper` |
| TypeScript conditional result types (`ResultFor`, `ScoreOf`, `ScoreLegend`) | Omitted; concrete Go answer variants and ordinary type assertions are used |
| browser guard/configuration | Omitted; browser/WASM is outside supported targets |
| arbitrary request extra-field forwarding | Intentionally omitted; documented fields are typed and question-level evolution uses `RawQuestion` |

## Go additions and signatures

The Go-only additions are grounded in normal Go configuration, ownership, and
error handling:

```go
func NewClient(opts ...option.ClientOption) (*Client, error)
func (c *Client) SystemOne(ctx context.Context, req SystemOneRequest, opts ...option.RequestOption) (*SystemOneResponse, error)
func (s ModelService) List(ctx context.Context, opts ...option.RequestOption) ([]ModelCard, error)

func Choice(instructions Entry, criteria ChoiceCriteria) ChoiceQuestion
func Score[T any](instructions Entry, levels ...T) ScoreQuestion
func Noul(instructions Entry) NoulQuestion

func option.WithRetryPolicy(func(*option.RetryPolicy)) option.CommonOption
func option.WithResponseInto(**http.Response) option.RequestOption
```

Other Go additions are the scoped option interfaces, raw JSON retention,
future-answer preservation, grouped typed accessors, sentinel categories,
redacted HTTP dumps, immutable accessors, and safe formatting. They are tracked
in the exact checked surface baseline rather than duplicated here.
