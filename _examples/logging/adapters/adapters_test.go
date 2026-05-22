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

package adapters

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/elastic/elastic-transport-go/v8/elastictransport"
	"github.com/go-logr/logr/funcr"
	"github.com/rs/zerolog"
	"github.com/sirupsen/logrus"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestAdaptersImplementInterface(t *testing.T) {
	// Compile-time checks are in each file via var _ assertions,
	// but verify at runtime too.
	var _ elastictransport.LeveledLogger = (*ZapAdapter)(nil)
	var _ elastictransport.LeveledLogger = (*ZerologAdapter)(nil)
	var _ elastictransport.LeveledLogger = (*LogrusAdapter)(nil)
	var _ elastictransport.LeveledLogger = (*LogrAdapter)(nil)
}

func TestZapAdapter(t *testing.T) {
	var buf bytes.Buffer
	enc := zapcore.NewJSONEncoder(zapcore.EncoderConfig{
		MessageKey: "msg",
		LevelKey:   "level",
		EncodeLevel: zapcore.LowercaseLevelEncoder,
	})
	core := zapcore.NewCore(enc, zapcore.AddSync(&buf), zapcore.DebugLevel)
	logger := zap.New(core).Sugar()

	ctx := context.Background()
	adapter := &ZapAdapter{Logger: logger}
	adapter.Debug(ctx, "debug msg", "key", "val")
	adapter.Info(ctx, "info msg", "count", 42)
	adapter.Warn(ctx, "warn msg")
	adapter.Error(ctx, "error msg", "error", errors.New("boom"))
	_ = logger.Sync()

	out := buf.String()
	for _, want := range []string{"debug msg", "info msg", "warn msg", "error msg", "boom"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output:\n%s", want, out)
		}
	}
}

func TestZerologAdapter(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)

	ctx := context.Background()
	adapter := &ZerologAdapter{Logger: logger}
	adapter.Debug(ctx, "debug msg", "key", "val")
	adapter.Info(ctx, "info msg", "count", 42)
	adapter.Warn(ctx, "warn msg")
	adapter.Error(ctx, "error msg", "error", "boom")

	out := buf.String()
	for _, want := range []string{"debug msg", "info msg", "warn msg", "error msg", "boom"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output:\n%s", want, out)
		}
	}
}

func TestLogrusAdapter(t *testing.T) {
	var buf bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&buf)
	logger.SetLevel(logrus.DebugLevel)
	logger.SetFormatter(&logrus.TextFormatter{DisableTimestamp: true})

	ctx := context.Background()
	adapter := &LogrusAdapter{Logger: logger}
	adapter.Debug(ctx, "debug msg", "key", "val")
	adapter.Info(ctx, "info msg", "count", 42)
	adapter.Warn(ctx, "warn msg")
	adapter.Error(ctx, "error msg", "error", "boom")

	out := buf.String()
	for _, want := range []string{"debug msg", "info msg", "warn msg", "error msg", "boom"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output:\n%s", want, out)
		}
	}
}

func TestLogrAdapter(t *testing.T) {
	var buf bytes.Buffer
	logger := funcr.NewJSON(func(obj string) {
		buf.WriteString(obj)
		buf.WriteString("\n")
	}, funcr.Options{Verbosity: 1})

	ctx := context.Background()
	adapter := &LogrAdapter{Logger: logger}
	adapter.Debug(ctx, "debug msg", "key", "val")
	adapter.Info(ctx, "info msg", "count", 42)
	adapter.Warn(ctx, "warn msg")
	adapter.Error(ctx, "error msg", "error", errors.New("boom"))

	out := buf.String()
	for _, want := range []string{"debug msg", "info msg", "warn msg", "error msg"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output:\n%s", want, out)
		}
	}
}

type testCtxKey struct{}

func testContextAttrs(ctx context.Context) []any {
	if v, ok := ctx.Value(testCtxKey{}).(string); ok {
		return []any{"trace_id", v}
	}
	return nil
}

func TestZapAdapterContextAttrs(t *testing.T) {
	var buf bytes.Buffer
	enc := zapcore.NewJSONEncoder(zapcore.EncoderConfig{
		MessageKey:  "msg",
		LevelKey:    "level",
		EncodeLevel: zapcore.LowercaseLevelEncoder,
	})
	core := zapcore.NewCore(enc, zapcore.AddSync(&buf), zapcore.DebugLevel)
	logger := zap.New(core).Sugar()

	ctx := context.WithValue(context.Background(), testCtxKey{}, "abc123")
	adapter := &ZapAdapter{Logger: logger, ContextAttrs: testContextAttrs}
	adapter.Info(ctx, "test", "key", "val")
	_ = logger.Sync()

	if !strings.Contains(buf.String(), "abc123") {
		t.Errorf("expected trace_id in output: %s", buf.String())
	}
}

func TestZerologAdapterContextAttrs(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)

	ctx := context.WithValue(context.Background(), testCtxKey{}, "abc123")
	adapter := &ZerologAdapter{Logger: logger, ContextAttrs: testContextAttrs}
	adapter.Info(ctx, "test", "key", "val")

	if !strings.Contains(buf.String(), "abc123") {
		t.Errorf("expected trace_id in output: %s", buf.String())
	}
}

func TestLogrusAdapterContextAttrs(t *testing.T) {
	var buf bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&buf)
	logger.SetFormatter(&logrus.TextFormatter{DisableTimestamp: true})

	ctx := context.WithValue(context.Background(), testCtxKey{}, "abc123")
	adapter := &LogrusAdapter{Logger: logger, ContextAttrs: testContextAttrs}
	adapter.Info(ctx, "test", "key", "val")

	if !strings.Contains(buf.String(), "abc123") {
		t.Errorf("expected trace_id in output: %s", buf.String())
	}
}

func TestLogrAdapterContextAttrs(t *testing.T) {
	var buf bytes.Buffer
	logger := funcr.NewJSON(func(obj string) {
		buf.WriteString(obj)
	}, funcr.Options{Verbosity: 1})

	ctx := context.WithValue(context.Background(), testCtxKey{}, "abc123")
	adapter := &LogrAdapter{Logger: logger, ContextAttrs: testContextAttrs}
	adapter.Info(ctx, "test", "key", "val")

	if !strings.Contains(buf.String(), "abc123") {
		t.Errorf("expected trace_id in output: %s", buf.String())
	}
}
