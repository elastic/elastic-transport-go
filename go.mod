module github.com/elastic/elastic-transport-go/v8

go 1.21

toolchain go1.25.9

// OpenTelemetry is pinned to v1.29.x: v1.30+ raises the minimum Go version
// above 1.21 (see go.mod `go` directive). Bump together with the Go minimum.
require (
	go.opentelemetry.io/otel v1.29.0
	go.opentelemetry.io/otel/sdk v1.29.0
	go.opentelemetry.io/otel/trace v1.29.0
)

require (
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/stretchr/testify v1.11.1 // indirect
	go.opentelemetry.io/otel/metric v1.29.0 // indirect
	golang.org/x/sys v0.30.0 // indirect
)
