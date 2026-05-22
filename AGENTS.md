# AGENTS.md

This file provides guidance to AI coding agents (Claude Code, Cursor, Copilot, etc.) when working with code in this repository.
CLAUDE.md is a symlink to this file.

## What This Is

`elastic-transport-go` is the shared HTTP transport layer for Elastic Go clients (e.g. `go-elasticsearch`). It provides connection pooling, node discovery, retry logic, request compression, OpenTelemetry instrumentation, interceptors, and logging. All code lives in a single package: `elastictransport`.

## Commands

```bash
# Unit tests (with race detector)
go test -v -race ./...

# Benchmarks
go test -bench=. ./...

# Integration tests (requires running Elasticsearch)
go test -v -race --tags=integration ./...

# Multi-node integration tests
go test -v --tags=integration,multinode ./...

# Lint
golangci-lint run

# Single test
go test -v -run TestFunctionName ./elastictransport/
```

## Architecture

Everything is in the `elastictransport` package. There is no cmd/, internal/, or multi-package structure.

**Client (`elastictransport.go`)** — The `Client` struct implements `Interface` (the `Perform(*http.Request)` method). It owns the retry loop, request decoration (auth, headers, compression), and delegates to a `ConnectionPool` for node selection.

**Two construction APIs:**

- `NewClient(opts ...Option)` — preferred, functional options pattern
- `New(Config{...})` — deprecated struct-based config, still fully functional. `NewClient` converts options to a `Config` and calls `New`.

**Options (`option.go`, `options.go`)** — `Option` is a self-describing value type with `Name()`, `String()` (secrets redacted), and `Describe(showSensitive)`. `Options` (slice type) supports `Validate()`, `Visit()`, and `Describe()`. `With*` constructors build options. Sensitive options (passwords, API keys, certs) use `newSensitiveOption` to separate masked/unmasked descriptions.

**Connection pools (`connection.go`)** — Multiple pool implementations: `singleConnectionPool`, `statusConnectionPool` (dead/live tracking with resurrection), `synchronizedPool`, `synchronizedUpdatablePool`. Pools implement `ConnectionPool`; discovery-aware pools implement `UpdatableConnectionPool`; closeable ones implement `CloseableConnectionPool`. The `synchronizedPool` wrapper makes non-concurrent-safe pools safe. Custom pools via `ConnectionPoolFunc` config. Connection pools are pluggable — users can implement their own pool strategy via `ConnectionPoolFunc`, so new built-in pool strategies are generally not needed.

**Discovery (`discovery.go`)** — `Discoverable` interface. Calls `/_nodes/http` to refresh the connection pool. Supports periodic scheduling via `WithDiscoverNodesInterval`. During discovery, pools implementing `UpdatableConnectionPool` get in-place `Update()` calls; others get replaced entirely.

**Interceptors (`interceptor.go`)** — `InterceptorFunc` wraps the `RoundTripFunc` chain. Multiple interceptors are merged into one at construction time via `mergeInterceptors`.

**Gzip (`gzip.go`)** — Two compressor strategies: `simpleGzipCompressor` (allocates per-request) and `pooledGzipCompressor` (reuses via `sync.Pool`). Controlled by `PoolCompressor` config / `WithPooledCompression` option.

**Instrumentation (`instrumentation.go`)** — OpenTelemetry integration via the `Instrumentation` interface. Called in the request path (`AfterResponse`).

## Hot Path

`Client.Perform` is the hot path — every Elasticsearch API call flows through it. Changes to the retry loop, connection selection, request decoration, or compression in `elastictransport.go` require careful benchmarking and review. Performance regressions here affect every user of `go-elasticsearch`. Always run `go test -bench=. ./...` and compare before/after when touching this code.

## Conventions

- All `.go` files must have the Apache 2.0 license header (17 lines, see `.github/license-header.txt`). CI checks this.
- Test files use the naming convention `*_internal_test.go` (package `elastictransport`), `*_test.go` (external), `*_benchmark_test.go`, `*_integration_test.go`.
- Integration tests use build tags: `//go:build integration` or `//go:build integration && multinode`.
- The `Config` struct is deprecated but kept until the next major version. New features must add both a `With*` option constructor and the corresponding `Config` field.
- Version is in `elastictransport/version/version.go`, managed by release-please.
- The `v8` module path is not tied to Elasticsearch 8 specifically — it supports Elasticsearch 8+ and is also used by `go-elasticsearch` v9.
- When a feature interacts with pluggable interfaces (connection pools, selectors, interceptors), document edge cases and limitations for custom implementations at the user-facing API surface: the `With*` option godoc, the corresponding `Config` field comment, and `doc.go`. Internal code comments alone are not enough, as users read godoc and option constructors, not internal types.

## Contributing

- PRs require 1 maintainer review.
- A CLA check runs automatically.
- Always include tests covering changes or new features. CI runs unit tests; integration tests do not need to pass locally.
- Contributions should focus on capabilities users cannot achieve externally. Since connection pools are pluggable, new pool strategies belong in user code, not here.
- Branch naming: prefer `feat/`, `fix/`, `chore/`, etc. prefixes.

### Commit Messages

This repo uses conventional commits for changelog generation via release-please.

| Prefix      | When to use                                    |
| ----------- | ---------------------------------------------- |
| `feat:`     | New user-facing functionality                  |
| `fix:`      | Bug fix                                        |
| `perf:`     | Performance improvement (no functional change) |
| `refactor:` | Code restructuring (no functional change)      |
| `test:`     | Adding or updating tests only                  |
| `docs:`     | Documentation changes only                     |
| `chore:`    | Maintenance (CI, dependencies, tooling)        |
| `ci:`       | CI pipeline changes                            |

### Downstream Impact

This library is used across several public Elastic Go repositories:

- **`go-elasticsearch`** — the primary consumer (direct dependency); released versions here must be manually updated via PR and backported to relevant version branches
- **`terraform-provider-elasticstack`** (direct dependency)
- **`beats`**, **`apm-server`**, **`fleet-server`**, **`go-docappender`**, **`opentelemetry-collector-components`** (indirect, via `go-elasticsearch`)
