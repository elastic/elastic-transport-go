# Logging Examples

This example demonstrates how to use `WithLeveledLogger` for all transport logging, replacing the deprecated `WithLogger`, `WithDebugLogger`, and bundled logger types.

## Running

```bash
cd _examples/logging
go run .
```

## slog Feature Parity

Each legacy logger can be replaced with a configured `*slog.Logger` passed to `&elastictransport.SlogLogger{Logger: ...}`:

| Legacy Logger | Replacement                         | File            | Notes                                    |
| ------------- | ----------------------------------- | --------------- | ---------------------------------------- |
| `TextLogger`  | `slog.NewTextHandler`               | `slog_text.go`  | Key=value format instead of bespoke line |
| `ColorLogger` | Custom `slog.Handler` with ANSI     | `slog_color.go` | Colors by level and status code          |
| `JSONLogger`  | Custom `slog.Handler` with ECS keys | `slog_json.go`  | Restructures attrs into nested ECS JSON  |
| `CurlLogger`  | Custom `slog.Handler`               | `slog_curl.go`  | Formats round-trips as curl commands     |

### Example: TextLogger replacement

```go
// Before (deprecated)
tp, _ := elastictransport.NewClient(
    elastictransport.WithURLs(u),
    elastictransport.WithLogger(&elastictransport.TextLogger{Output: os.Stdout}),
)

// After
tp, _ := elastictransport.NewClient(
    elastictransport.WithURLs(u),
    elastictransport.WithLeveledLogger(&elastictransport.SlogLogger{
        Logger: slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
            Level: slog.LevelDebug,
        })),
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

## Writing Your Own Adapter

Implement the four methods of `elastictransport.LeveledLogger`:

```go
type LeveledLogger interface {
    Debug(msg string, keysAndValues ...any)
    Info(msg string, keysAndValues ...any)
    Warn(msg string, keysAndValues ...any)
    Error(msg string, keysAndValues ...any)
}
```

The `keysAndValues` argument is a flat slice of alternating keys (strings) and values (any type), following the same convention as `log/slog`. For libraries that use a different convention:

- **Map-based** (logrus): iterate in steps of 2 to build `map[string]any`
- **Builder-chain** (zerolog): iterate in steps of 2, calling `.Interface(key, value)` per pair
- **Typed fields** (zap `Logger`): use `SugaredLogger` instead, which accepts `keysAndValues ...interface{}`
- **No warn level** (logr): map Warn to `V(0).Info()`
- **Error with `error` param** (logr): extract the `"error"` key from keysAndValues
