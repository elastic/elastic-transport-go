# elastic-transport-go

This library was lifted from elasticsearch-net and then transformed to be used across all Elastic services rather than
only Elasticsearch.

It provides the Transport interface used by `go-elasticsearch`, connection pool, cluster discovery, and multiple loggers.

## Installation

Add the package to your go.mod file:

`require github.com/elastic/elastic-transport-go/v8 main`

## Usage

### Transport

The transport provides the basic layer to access Elasticsearch APIs. Create a
client with `NewClient` and functional options:

```go
package main

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/elastic/elastic-transport-go/v8/elastictransport"
)

func main() {
	u, _ := url.Parse("http://127.0.0.1:9200")

	transport, err := elastictransport.NewClient(
		elastictransport.WithURLs(u),
	)
	if err != nil {
		log.Fatalln(err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = transport.Close(ctx)
	}()

	req, _ := http.NewRequest("GET", "/", nil)

	res, err := transport.Perform(req)
	if err != nil {
		log.Fatalln(err)
	}
	defer res.Body.Close()

	log.Println(res)
}
```

Options are applied in order; when the same setting is specified more than once
the last value wins. See the `With*` functions in the
[package documentation](https://pkg.go.dev/github.com/elastic/elastic-transport-go/v8/elastictransport)
for the full list of available options.

Common examples:

```go
// Multiple nodes with basic auth, custom retries, and compression
transport, err := elastictransport.NewClient(
    elastictransport.WithURLs(u1, u2, u3),
    elastictransport.WithBasicAuth("elastic", "changeme"),
    elastictransport.WithRetry(5, 429, 502, 503, 504),
    elastictransport.WithRetryBackoff(func(attempt int) time.Duration {
        return time.Duration(attempt) * 100 * time.Millisecond
    }),
    elastictransport.WithCompression(gzip.BestSpeed),
)
```

> **Note:** The older `New(Config{...})` API is deprecated but remains fully
> functional for backwards compatibility.

> **Note:** It is _critical_ to both close the response body _and_ to consume it,
> in order to re-use persistent TCP connections in the default HTTP transport. If
> you're not interested in the response body, call `io.Copy(io.Discard, res.Body)`.

### Discovery

Discovery module calls the cluster to retrieve its complete list of nodes.

Once your transport has been set up, you can easily trigger this behavior like so:

```go
err := transport.DiscoverNodes()
```

Or configure automatic periodic discovery when creating the client:

```go
transport, err := elastictransport.NewClient(
    elastictransport.WithURLs(u),
    elastictransport.WithDiscoverNodesInterval(5 * time.Minute),
)
```

### Metrics

Allows you to retrieve metrics directly from the transport. Enable metrics when
creating the client:

```go
transport, err := elastictransport.NewClient(
    elastictransport.WithURLs(u),
    elastictransport.WithMetrics(),
)
```

### Leveled Logging

`WithLeveledLogger` is the recommended way to enable **all** logging — it covers
both request/response round-trips and transport-internal events (connection
management, node discovery). The `LeveledLogger` interface uses the same
`(msg, keysAndValues...)` convention as `log/slog`:

```go
type LeveledLogger interface {
    Debug(msg string, keysAndValues ...any)
    Info(msg string, keysAndValues ...any)
    Warn(msg string, keysAndValues ...any)
    Error(msg string, keysAndValues ...any)
}
```

A ready-made `SlogLogger` adapter wraps any `*slog.Logger`:

```go
transport, err := elastictransport.NewClient(
    elastictransport.WithURLs(u),
    elastictransport.WithLeveledLogger(&elastictransport.SlogLogger{
        Logger: slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
            Level: slog.LevelDebug,
        })),
    }),
)
```

Successful round-trips are logged at **Info** level; errors at **Error** level;
connection-management events at **Debug**, **Warn**, or **Error** depending on
severity.

#### Body Logging

Request and response body capture is off by default. Opt in with
`WithLeveledLoggerBodyLogging`:

```go
transport, err := elastictransport.NewClient(
    elastictransport.WithURLs(u),
    elastictransport.WithLeveledLogger(&elastictransport.SlogLogger{
        Logger: slog.Default(),
    }),
    elastictransport.WithLeveledLoggerBodyLogging(true, true),
)
```

#### Custom Logger Implementations

Ready-made adapters for zap, zerolog, logrus, and logr — plus slog-based
replacements for every deprecated logger — are in
[`_examples/logging/`](./_examples/logging/).

To use a different logging library, implement the four methods on a thin wrapper:

```go
type ZapLeveledLogger struct{ Logger *zap.SugaredLogger }

func (l *ZapLeveledLogger) Debug(msg string, kv ...any) { l.Logger.Debugw(msg, kv...) }
func (l *ZapLeveledLogger) Info(msg string, kv ...any)  { l.Logger.Infow(msg, kv...) }
func (l *ZapLeveledLogger) Warn(msg string, kv ...any)  { l.Logger.Warnw(msg, kv...) }
func (l *ZapLeveledLogger) Error(msg string, kv ...any) { l.Logger.Errorw(msg, kv...) }
```

#### Interaction with WithLogger

When `WithLogger` is explicitly set alongside `WithLeveledLogger`, the
`Logger` handles round-trip logging and the `LeveledLogger` handles only
connection-management events. This preserves backward compatibility for users
migrating incrementally.

#### Migrating from WithDebugLogger / WithLogger

`WithDebugLogger()` and `WithLogger()` still work but are deprecated. Under the
hood `WithDebugLogger` now creates a `SlogLogger` wrapping `slog.Default()`.

|                       | `WithDebugLogger()` | `WithLogger()`  | `WithLeveledLogger()` |
| --------------------- | ------------------- | --------------- | --------------------- |
| Round-trip logging    | No                  | Yes             | Yes                   |
| Connection events     | Yes (Debug only)    | No              | Yes                   |
| Output destination    | stdout              | User-controlled | User-controlled       |
| Log levels            | Debug only          | None            | Debug/Info/Warn/Error |
| Structured data       | No                  | No              | Yes (key-value pairs) |
| Custom logger support | No                  | Yes             | Yes                   |
| Per-client isolation  | Yes                 | Yes             | Yes                   |

Replace either legacy option:

```go
// Before (WithDebugLogger)
transport, err := elastictransport.NewClient(
    elastictransport.WithURLs(u),
    elastictransport.WithDebugLogger(),
)

// Before (WithLogger)
transport, err := elastictransport.NewClient(
    elastictransport.WithURLs(u),
    elastictransport.WithLogger(&elastictransport.TextLogger{Output: os.Stdout}),
)

// After (single option for all logging)
transport, err := elastictransport.NewClient(
    elastictransport.WithURLs(u),
    elastictransport.WithLeveledLogger(&elastictransport.SlogLogger{
        Logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
            Level: slog.LevelDebug,
        })),
    }),
)
```

### Request/Response Loggers (Deprecated)

> **Deprecated:** Use `WithLeveledLogger` instead. The loggers below remain
> functional for backward compatibility.

A logger can be provided via the `WithLogger` option. Several bundled loggers
are available:

#### TextLogger

config:

```go
transport, err := elastictransport.NewClient(
    elastictransport.WithURLs(u),
    elastictransport.WithLogger(&elastictransport.TextLogger{Output: os.Stdout, EnableRequestBody: true, EnableResponseBody: true}),
)
```

output:

```
< {
<   "name" : "es",
<   "cluster_name" : "elasticsearch",
<   "cluster_uuid" : "RxB1iqTNT9q3LlIkTsmWRA",
<   "version" : {
<     "number" : "8.0.0-SNAPSHOT",
<     "build_flavor" : "default",
<     "build_type" : "docker",
<     "build_hash" : "0564e027dc6c69236937b1edcc04c207b4cd8128",
<     "build_date" : "2021-11-25T00:23:33.139514432Z",
<     "build_snapshot" : true,
<     "lucene_version" : "9.0.0",
<     "minimum_wire_compatibility_version" : "7.16.0",
<     "minimum_index_compatibility_version" : "7.0.0"
<   },
<   "tagline" : "You Know, for Search"
< }
```

#### JSONLogger

config:

```go
transport, err := elastictransport.NewClient(
    elastictransport.WithURLs(u),
    elastictransport.WithLogger(&elastictransport.JSONLogger{Output: os.Stdout, EnableRequestBody: true, EnableResponseBody: true}),
)
```

output:

```json
{
  "@timestamp": "2021-11-25T16:33:51Z",
  "event": {
    "duration": 2892269
  },
  "url": {
    "scheme": "http",
    "domain": "127.0.0.1",
    "port": 9200,
    "path": "/",
    "query": ""
  },
  "http": {
    "request": {
      "method": "GET"
    },
    "response": {
      "status_code": 200,
      "body": "{\n  \"name\" : \"es1\",\n  \"cluster_name\" : \"go-elasticsearch\",\n  \"cluster_uuid\" : \"RxB1iqTNT9q3LlIkTsmWRA\",\n  \"version\" : {\n    \"number\" : \"8.0.0-SNAPSHOT\",\n    \"build_flavor\" : \"default\",\n    \"build_type\" : \"docker\",\n    \"build_hash\" : \"0564e027dc6c69236937b1edcc04c207b4cd8128\",\n    \"build_date\" : \"2021-11-25T00:23:33.139514432Z\",\n    \"build_snapshot\" : true,\n    \"lucene_version\" : \"9.0.0\",\n    \"minimum_wire_compatibility_version\" : \"8.0.0\",\n    \"minimum_index_compatibility_version\" : \"7.0.0\"\n  },\n  \"tagline\" : \"You Know, for Search\"\n}\n"
    }
  }
}
```

#### ColorLogger

config:

```go
transport, err := elastictransport.NewClient(
    elastictransport.WithURLs(u),
    elastictransport.WithLogger(&elastictransport.ColorLogger{Output: os.Stdout, EnableRequestBody: true, EnableResponseBody: true}),
)
```

output:

```
GET http://127.0.0.1:9200/ 200 OK 2ms
« {
«   "name" : "es1",
«   "cluster_name" : "go-elasticsearch",
«   "cluster_uuid" : "RxB1iqTNT9q3LlIkTsmWRA",
«   "version" : {
«     "number" : "8.0.0-SNAPSHOT",
«     "build_flavor" : "default",
«     "build_type" : "docker",
«     "build_hash" : "0564e027dc6c69236937b1edcc04c207b4cd8128",
«     "build_date" : "2021-11-25T00:23:33.139514432Z",
«     "build_snapshot" : true,
«     "lucene_version" : "9.0.0",
«     "minimum_wire_compatibility_version" : "7.16.0",
«     "minimum_index_compatibility_version" : "7.0.0"
«   },
«   "tagline" : "You Know, for Search"
« }
────────────────────────────────────────────────────────────────────────────────
```

#### CurlLogger

config:

```go
transport, err := elastictransport.NewClient(
    elastictransport.WithURLs(u),
    elastictransport.WithLogger(&elastictransport.CurlLogger{Output: os.Stdout, EnableRequestBody: true, EnableResponseBody: true}),
)
```

output:

```shell
curl -X GET 'http://localhost:9200/?pretty'
# => 2021-11-25T16:40:11Z [200 OK] 3ms
# {
#  "name": "es1",
#  "cluster_name": "go-elasticsearch",
#  "cluster_uuid": "RxB1iqTNT9q3LlIkTsmWRA",
#  "version": {
#   "number": "8.0.0-SNAPSHOT",
#   "build_flavor": "default",
#   "build_type": "docker",
#   "build_hash": "0564e027dc6c69236937b1edcc04c207b4cd8128",
#   "build_date": "2021-11-25T00:23:33.139514432Z",
#   "build_snapshot": true,
#   "lucene_version": "9.0.0",
#   "minimum_wire_compatibility_version": "7.16.0",
#   "minimum_index_compatibility_version": "7.0.0"
#  },
#  "tagline": "You Know, for Search"
# }
```

# License

Licensed under the Apache License, Version 2.0.
