# Changelog

## [Unreleased]

## [0.1.1] - 2026-09-19

- Add an explicitly invoked, two-request live conformance suite for Models.List
  and a mixed Choice, Score, and Noul SystemOne request.
- Add a weekly stable-release monitor for the pinned TypeSafe JavaScript and
  Python SDKs, with a deduplicated drift issue and no automatic pin changes.
- Add `ResponseValidationError` and `ErrResponseValidation` for incompatible 2xx
  response bodies, with JSON field paths, request IDs, causes, sanitized response
  snapshots, and required-field/request-answer correlation checks.
- Document client options, retry defaults, server-directed retry delays, and
  client-wide versus per-request timeout behavior.

## [0.1.0] - 2026-09-19

- Add the initial unofficial TypeSafe Go SDK with Choice, Score, Noul, raw
  questions, typed and unknown answers, raw JSON retention, and usage metadata.
- Add immutable client configuration, scoped options, SystemOne, Models.List,
  retry/timeout/cancellation behavior, structured logging, redacted HTTP
  snapshots, and Go error categories.
- Add offline conformance fixtures, compiled examples, public surface checking,
  primitive coverage evidence, and Go 1.24/latest-stable CI.
- Add runnable Choice, Score, and Noul programs under `examples` and link them
  from the README.

[0.1.0]: https://github.com/unimtx/typesafe-sdk-go/releases/tag/v0.1.0
[0.1.1]: https://github.com/unimtx/typesafe-sdk-go/releases/tag/v0.1.1
[Unreleased]: https://github.com/unimtx/typesafe-sdk-go/compare/v0.1.1...HEAD
