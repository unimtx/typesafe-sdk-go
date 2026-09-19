# Live API coverage

Live tests verify that the offline contract still works against the deployed
TypeSafe API. They are deliberately separate from ordinary tests because they
require credentials, use a remote service, and may incur charges.

Run them only when explicitly intended:

```sh
go test -tags=integration ./integration/...
```

The suite reads `TYPESAFE_API_KEY` through the normal client configuration. It
never prints or stores the credential. Automatic retries are disabled so one
run makes at most two HTTP requests:

| Request | Acceptance boundary |
|---|---|
| `GET /v1/models` | A nonempty list is returned and every model has a name. |
| `POST /v1/systemone` | One mixed request returns Choice, Score, and Noul answers with the expected IDs and basic documented ranges/shapes. |

The assertions intentionally avoid exact probabilities, scores, token counts,
model names, and model-generated choices. A successful run establishes only
that the deployed service accepted these request shapes and returned compatible
response shapes at that time.

## Recorded runs

| Date | SDK version | Result | Notes |
|---|---|---|---|
| 2026-09-19 | 0.1.0 development tree | Passed | Models.List and one mixed Choice, Score, and Noul request; two HTTP requests with retries disabled. |
