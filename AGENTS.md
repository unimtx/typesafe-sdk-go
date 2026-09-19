# AGENTS.md

Contributor instructions for humans and AI coding agents. This file owns core
constraints and workflow; detailed API and implementation rules live in DESIGN.md.
The repository is currently at the design stage. Missing code, fixtures, or tools
are work to implement, not checks that have passed.

## Project

Build an **unofficial, idiomatic Go SDK** for the TypeSafe HTTP API, using the
official JavaScript SDK as an implementation reference. Prioritize **Go idioms,
then the HTTP API contract, then JS SDK conventions** when designing the public
API. This does not permit invalid wire formats: adapt Go representations to the
HTTP contract rather than reproducing JS mechanics. Keep the API discoverable
through `go doc`, keyed structs, small constructors, contexts, and normal errors.

- Module: `github.com/unimtx/typesafe-sdk-go`; root package: `typesafe`.
- Public packages: root and `option` only; other implementation stays private.
- README and package docs must state **unofficial and unaffiliated with TypeSafe**.
- MIT license: preserve this project's notice and TypeSafe's notice for copied
  or adapted upstream code, tests, and substantial documentation material.

## Reading guide and document ownership

| Document | Owns | Read when |
|---|---|---|
| [DESIGN.md](DESIGN.md) | Go API, JSON rules, ownership, transport, implementation sequence, detailed tests, releases | Before changing SDK behavior or public API; read affected sections |
| [UPSTREAM.md](UPSTREAM.md) | Exact upstream pins and source references | Before implementing or syncing upstream behavior |
| [PARITY.md](PARITY.md) | Adaptation rationale, exceptions, unresolved conflicts | Before changing SDK behavior or proposing a divergence |
| [Primitives coverage](docs/primitives-coverage.md) | Official examples mapped to Go usage and test evidence | When changing questions, answers, serialization, or their examples/tests |

Do not duplicate API signatures/defaults across these documents. DESIGN.md owns
exact behavior; PARITY.md explains differences; the coverage matrix tracks
acceptance evidence. Update their links together when moving content. Previously
recorded design decisions do not require repeated approval to implement.

## Sources of truth

- Go conventions and DESIGN.md govern public shape, option scopes, zero values,
  contexts, and errors. Prefer compile-time misuse prevention where practical.
- The [HTTP API reference](https://docs.typesafe.ai/api.md) and
  [structured-entry guide](https://docs.typesafe.ai/primitives/advanced.md) define
  wire vocabulary, shapes, and inference semantics.
- The **JS tag and commit pinned in UPSTREAM.md** provide evidence for defaults,
  validation, environment, headers, retry, and errors. Adopt the behaviors stated
  in DESIGN.md; exact JS parity is not a goal. Read pinned source/tests, never
  implement from memory or from a moving `main` branch.
- Pinned Python is corroborating evidence and a source of naming/ergonomics;
  newer Python-only features do not expand this port's scope automatically.
- PARITY.md records deliberate adaptations and omissions. Every exported symbol
  needs an HTTP capability mapping, SDK reference, or documented Go rationale;
  a matching JS export is not required.

Check PARITY.md before treating a difference as unresolved. For a new material
conflict, record evidence and a proposed resolution, ask about the affected
behavior, and continue independent work. A narrow example does not override
richer shapes explicitly supported elsewhere in official documentation. If pins
or references are missing, establish them before implementing dependent behavior.

## Core constraints

1. Runtime code uses the standard library only. Pin development tools separately.
2. Minimum Go is **1.24.0**. Use `encoding/json` v1 and no experimental or later-only
   APIs, including `new(expr)`, `errors.AsType`, and stable `testing/synctest`.
   Raising the floor requires a maintainer decision. Verify the latest stable
   CI version from official Go releases; do not guess future language features.
3. Prefer readable, focused functions and simple Go idioms. Fix the underlying
   problem, keep changes proportional, and avoid speculative abstractions or
   unrelated refactors. Do not copy another community Go SDK.
4. Network operations take `context.Context` first. Client configuration is
   immutable and supports concurrent calls under DESIGN.md's ownership rules.
5. Invalid SDK-controlled input returns errors, never panics. No `Must*`,
   `os.Exit`, `init()` side effects, or direct stdout/stderr output. Explicitly
   supplied loggers control their own destination. Do not recover arbitrary
   panics from caller callbacks, marshalers, transports, or loggers.
6. No mutable package-level configuration. Sentinel errors are initialized once
   and never reassigned by SDK code. Defaults containing maps/slices are fresh.
7. Keep credentials private and diagnostics safe, following DESIGN.md's redaction
   and raw-body boundaries. Never put real credentials in tests or examples.
8. Do not add public API without an HTTP/SDK basis or explicit Go rationale;
   respect [non-goals](DESIGN.md#37-non-goals). API contract changes update
   DESIGN.md and PARITY.md together; upstream pins change only deliberately.
9. Ordinary tests run offline. Live API tests require an explicit user request;
   they use real credentials and can incur charges.
10. Agents never merge PRs or push release tags. Never move/delete a published
    tag. Release/version policy lives in DESIGN.md.

## Working procedure

- Inspect the working tree and preserve unrelated user edits.
- Read the relevant design, pinned upstream source, and tests. Cite immutable
  commit URLs with file/line references in the PR description.
- For changed wire behavior, add/update a focused conformance fixture, observe
  failure, then implement. Go-only bugs may use focused unit tests instead.
- Keep PRs small. Update parity decisions, affected example coverage, user-facing
  changelog, and upstream attribution when applicable.
- Keep README snippets synchronized with compiled examples. Explain defaults,
  nil/zero behavior, and ownership in exported Go doc comments.
- Source inspection and mocked tests do not establish live API acceptance or
  guarantee the model outputs shown in documentation.

## Commands

Once the module exists, run applicable checks from its root:

```sh
go build ./...
go vet ./...
go test -race ./...
gofmt -l .              # must print nothing for project Go files
go run honnef.co/go/tools/cmd/staticcheck@v0.8.1 ./...
go run ./internal/tools/surfacecheck
```

CI tests the actual minimum toolchain and latest stable Go. Report missing tools/toolchains;
do not silently install arbitrary latest versions or claim absent checks passed.
Run `go fix ./...` only for an intentional modernization task, review its edits,
and retain the minimum Go version.

Only with an explicit request for live tests:

```sh
go test -tags=integration ./integration/...
```

Documentation-only changes need link, reference, consistency, and snippet checks;
do not run nonexistent module checks. See DESIGN.md for detailed test requirements.

## Definition of done

- Applicable build, vet, race, formatting, staticcheck, and implemented surface
  checks pass; library changes are checked on the floor and latest stable Go.
- Exports match DESIGN.md and the capability map. Relevant fixtures and examples
  pass; affected coverage rows link actual evidence, not planned filenames.
- Update DESIGN.md, PARITY.md, UPSTREAM.md, CHANGELOG.md, and attribution as needed.
- Report changes, verification performed, missing checks, and unresolved issues.
  Do not claim implementation or live conformance from documentation alone.
