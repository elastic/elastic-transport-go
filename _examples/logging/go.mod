module github.com/elastic/elastic-transport-go/v8/_examples/logging

go 1.21

require (
	github.com/elastic/elastic-transport-go/v8 v8.0.0
	github.com/go-logr/logr v1.4.3
	github.com/rs/zerolog v1.33.0
	github.com/sirupsen/logrus v1.9.3
	go.uber.org/zap v1.27.0
)

require (
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.19 // indirect
	go.opentelemetry.io/otel v1.29.0 // indirect
	go.opentelemetry.io/otel/metric v1.29.0 // indirect
	go.opentelemetry.io/otel/trace v1.29.0 // indirect
	go.uber.org/multierr v1.10.0 // indirect
	golang.org/x/sys v0.24.0 // indirect
)

replace github.com/elastic/elastic-transport-go/v8 => ../..
