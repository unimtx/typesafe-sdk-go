# Changelog

## [Unreleased]

## [0.2.0] - 2026-09-19

- Add allocation-free singular Noul, Choice, and Score response accessors with
  distinct missing-answer and type-mismatch errors.
- Validate nonempty question IDs, Choice's 1–255 options, Score's 2–10 levels,
  and meaningful Noul content locally for modeled and known raw questions.
  This intentionally tightens pre-1.0 behavior: request shapes that were
  previously sent to the service now fail locally when they violate these
  documented bounds or contain no meaningful question.
- Document transport, timeout, and caller-cancellation error categories.

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
[0.2.0]: https://github.com/unimtx/typesafe-sdk-go/releases/tag/v0.2.0
[Unreleased]: https://github.com/unimtx/typesafe-sdk-go/compare/v0.2.0...HEAD
