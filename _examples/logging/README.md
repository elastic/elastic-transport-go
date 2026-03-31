# Logging Examples

This example demonstrates how to use `WithLeveledLogger` for all transport logging, replacing the deprecated `WithLogger`, `WithDebugLogger`, and bundled logger types.

## Running

```bash
cd _examples/logging
go run .
```

## slog Feature Parity

Each legacy logger has a drop-in replacement in the `sloghandler` sub-package
(`elastictransport/sloghandler`):

| Legacy Logger | Replacement                        | Notes                                    |
| ------------- | ---------------------------------- | ---------------------------------------- |
| `TextLogger`  | `sloghandler.NewTextHandler(w)`    | Key=value format instead of bespoke line |
| `ColorLogger` | `sloghandler.NewColorHandler(w)`   | Colors by level and status code          |
| `JSONLogger`  | `sloghandler.NewJSONECSHandler(w)` | Restructures attrs into nested ECS JSON  |
| `CurlLogger`  | `sloghandler.NewCurlHandler(w)`    | Formats round-trips as curl commands     |

### Example: TextLogger replacement

```go
import "github.com/elastic/elastic-transport-go/v8/elastictransport/sloghandler"

// Before (deprecated)
tp, _ := elastictransport.NewClient(
    elastictransport.WithURLs(u),
    elastictransport.WithLogger(&elastictransport.TextLogger{Output: os.Stdout}),
)

// After
tp, _ := elastictransport.NewClient(
    elastictransport.WithURLs(u),
    elastictransport.WithLeveledLogger(&elastictransport.SlogLogger{
        Logger: slog.New(sloghandler.NewTextHandler(os.Stdout)),
    }),
)
```

## Library Adapters

Copy-paste adapters for popular Go logging libraries are in the `adapters/` directory:

| Library | Adapter          | File                         | Import                       |
| ------- | ---------------- | ---------------------------- | ---------------------------- |
| slog    | `SlogLogger`     | (main package)               | `elastictransport`           |
| zap     | `ZapAdapter`     | `adapters/zapadapter.go`     | `go.uber.org/zap`            |
| zerolog | `ZerologAdapter` | `adapters/zerologadapter.go` | `github.com/rs/zerolog`      |
| logrus  | `LogrusAdapter`  | `adapters/logrusadapter.go`  | `github.com/sirupsen/logrus` |
| logr    | `LogrAdapter`    | `adapters/logradapter.go`    | `github.com/go-logr/logr`    |

### Example: zap

```go
logger, _ := zap.NewProduction()
tp, _ := elastictransport.NewClient(
    elastictransport.WithURLs(u),
    elastictransport.WithLeveledLogger(&adapters.ZapAdapter{
        Logger: logger.Sugar(),
    }),
)
```

### Example: zerolog

```go
logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
tp, _ := elastictransport.NewClient(
    elastictransport.WithURLs(u),
    elastictransport.WithLeveledLogger(&adapters.ZerologAdapter{
        Logger: logger,
    }),
)
```

### Example: logrus

```go
logger := logrus.New()
logger.SetFormatter(&logrus.JSONFormatter{})
tp, _ := elastictransport.NewClient(
    elastictransport.WithURLs(u),
    elastictransport.WithLeveledLogger(&adapters.LogrusAdapter{
        Logger: logger,
    }),
)
```

### Example: logr

```go
// Using logr with the standard library backend:
logger := stdr.New(log.Default())
tp, _ := elastictransport.NewClient(
    elastictransport.WithURLs(u),
    elastictransport.WithLeveledLogger(&adapters.LogrAdapter{
        Logger: logger,
    }),
)
```

## OpenTelemetry Trace Context

All adapters (including `SlogLogger`) support a `ContextAttrs` function that
extracts key-value pairs from `context.Context` and prepends them to every log
entry. This is the recommended way to add trace correlation without writing a
custom slog Handler.

Use `elastictransport.OTelContextAttrs` (provided in the main package):

```go
// SlogLogger with OTel trace IDs
tp, _ := elastictransport.NewClient(
    elastictransport.WithURLs(u),
    elastictransport.WithLeveledLogger(&elastictransport.SlogLogger{
        Logger:       slog.Default(),
        ContextAttrs: elastictransport.OTelContextAttrs,
    }),
    elastictransport.WithInterceptors(
        elastictransport.LoggingInterceptor(false, false),
    ),
)
```

```go
// Zap adapter with OTel trace IDs
tp, _ := elastictransport.NewClient(
    elastictransport.WithURLs(u),
    elastictransport.WithLeveledLogger(&adapters.ZapAdapter{
        Logger:       zapLogger.Sugar(),
        ContextAttrs: elastictransport.OTelContextAttrs,
    }),
    elastictransport.WithInterceptors(
        elastictransport.LoggingInterceptor(false, false),
    ),
)
```

The `ContextAttrs` function signature is `func(context.Context) []any` — it's
not OTel-specific. You can use it to extract any context-carried data (request
IDs, tenant IDs, etc.):

```go
ContextAttrs: func(ctx context.Context) []any {
    if reqID, ok := ctx.Value(requestIDKey{}).(string); ok {
        return []any{"request_id", reqID}
    }
    return nil
},
```

## Writing Your Own Adapter

Implement the four methods of `elastictransport.LeveledLogger`:

```go
type LeveledLogger interface {
    Debug(ctx context.Context, msg string, keysAndValues ...any)
    Info(ctx context.Context, msg string, keysAndValues ...any)
    Warn(ctx context.Context, msg string, keysAndValues ...any)
    Error(ctx context.Context, msg string, keysAndValues ...any)
}
```

The `keysAndValues` argument is a flat slice of alternating keys (strings) and values (any type), following the same convention as `log/slog`. For libraries that use a different convention:

- **Map-based** (logrus): iterate in steps of 2 to build `map[string]any`
- **Builder-chain** (zerolog): iterate in steps of 2, calling `.Interface(key, value)` per pair
- **Typed fields** (zap `Logger`): use `SugaredLogger` instead, which accepts `keysAndValues ...interface{}`
- **No warn level** (logr): map Warn to `V(0).Info()`
- **Error with `error` param** (logr): extract the `"error"` key from keysAndValues

Add a `ContextAttrs func(context.Context) []any` field and call it at the start
of each method to support context-based enrichment (see adapters for examples).
