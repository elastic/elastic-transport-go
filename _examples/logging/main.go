// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

// This example demonstrates how to use WithLeveledLogger for all transport
// logging, including slog-based replacements for the deprecated TextLogger,
// ColorLogger, CurlLogger, and JSONLogger, as well as adapters for popular
// third-party logging libraries.
//
// Run: cd _examples/logging && go run .
package main

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/elastic/elastic-transport-go/v8/_examples/logging/adapters"
	"github.com/elastic/elastic-transport-go/v8/elastictransport"
	"github.com/elastic/elastic-transport-go/v8/elastictransport/sloghandler"
	"github.com/rs/zerolog"
	"github.com/sirupsen/logrus"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	u, _ := url.Parse("http://localhost:9200")

	mockTransport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			Status:     "200 OK",
			StatusCode: 200,
			Header:     http.Header{"Content-Type": {"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"name":"es","cluster_name":"docker-cluster"}`)),
		}, nil
	})

	section("slog TextLogger equivalent")
	demonstrate(u, mockTransport, &elastictransport.SlogLogger{
		Logger: slog.New(sloghandler.NewTextHandler(os.Stdout)),
	})

	section("slog ColorLogger equivalent")
	demonstrate(u, mockTransport, &elastictransport.SlogLogger{
		Logger: slog.New(sloghandler.NewColorHandler(os.Stdout)),
	})

	section("slog JSON/ECS Logger equivalent")
	demonstrate(u, mockTransport, &elastictransport.SlogLogger{
		Logger: slog.New(sloghandler.NewJSONECSHandler(os.Stdout)),
	})

	section("slog CurlLogger equivalent")
	demonstrate(u, mockTransport, &elastictransport.SlogLogger{
		Logger: slog.New(sloghandler.NewCurlHandler(os.Stdout)),
	})

	section("zap adapter")
	zapLogger := newZapLogger()
	demonstrate(u, mockTransport, &adapters.ZapAdapter{
		Logger: zapLogger,
	})
	_ = zapLogger.Sync()

	section("zerolog adapter")
	demonstrate(u, mockTransport, &adapters.ZerologAdapter{
		Logger: zerolog.New(os.Stdout).With().Timestamp().Logger().Level(zerolog.DebugLevel),
	})

	section("logrus adapter")
	logrusLogger := logrus.New()
	logrusLogger.SetOutput(os.Stdout)
	logrusLogger.SetLevel(logrus.DebugLevel)
	demonstrate(u, mockTransport, &adapters.LogrusAdapter{
		Logger: logrusLogger,
	})

	section("slog (default)")
	demonstrate(u, mockTransport, &elastictransport.SlogLogger{
		Logger: slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})),
	})

	section("slog with OTel trace context")
	demonstrate(u, mockTransport, &elastictransport.SlogLogger{
		Logger:       slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})),
		ContextAttrs: elastictransport.OTelContextAttrs,
	})

	section("zap with OTel trace context")
	demonstrate(u, mockTransport, &adapters.ZapAdapter{
		Logger:       newZapLogger(),
		ContextAttrs: elastictransport.OTelContextAttrs,
	})
}

func demonstrate(u *url.URL, rt http.RoundTripper, logger elastictransport.LeveledLogger) {
	tp, err := elastictransport.NewClient(
		elastictransport.WithURLs(u),
		elastictransport.WithTransport(rt),
		elastictransport.WithLeveledLogger(logger),
		elastictransport.WithInterceptors(
			elastictransport.LoggingInterceptor(false, false),
		),
		elastictransport.WithDisableRetry(),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating client: %s\n", err)
		return
	}

	req, _ := http.NewRequest("GET", "/", nil)
	res, err := tp.Perform(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error performing request: %s\n", err)
		return
	}
	defer func() {
		_, _ = io.Copy(io.Discard, res.Body)
		_ = res.Body.Close()
	}()
}

func section(name string) {
	fmt.Printf("\n%s\n%s\n", name, strings.Repeat("─", len(name)))
}

func newZapLogger() *zap.SugaredLogger {
	cfg := zapcore.EncoderConfig{
		MessageKey:  "msg",
		LevelKey:    "level",
		TimeKey:     "time",
		EncodeLevel: zapcore.LowercaseLevelEncoder,
		EncodeTime:  zapcore.ISO8601TimeEncoder,
	}
	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(cfg),
		zapcore.AddSync(os.Stdout),
		zapcore.DebugLevel,
	)
	return zap.New(core).Sugar()
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
