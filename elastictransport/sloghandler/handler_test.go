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

package sloghandler

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/elastic/elastic-transport-go/v8/elastictransport"
)

func roundTripRecord() slog.Record {
	r := slog.NewRecord(time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC), slog.LevelInfo, "request", 0)
	r.AddAttrs(
		slog.String(elastictransport.LogKeyMethod, "GET"),
		slog.String(elastictransport.LogKeyURL, "http://localhost:9200/"),
		slog.Int(elastictransport.LogKeyStatus, 200),
		slog.Duration(elastictransport.LogKeyDuration, 5*time.Millisecond),
	)
	return r
}

func TestNewTextHandler(t *testing.T) {
	var buf bytes.Buffer
	h := NewTextHandler(&buf)

	if err := h.Handle(context.Background(), roundTripRecord()); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	for _, want := range []string{"level=INFO", "msg=request", "method=GET", "status=200"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output:\n%s", want, out)
		}
	}
}

func TestNewColorHandler(t *testing.T) {
	var buf bytes.Buffer
	h := NewColorHandler(&buf)

	if err := h.Handle(context.Background(), roundTripRecord()); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	for _, want := range []string{"INFO", "request", "method=GET", "status=200"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output:\n%s", want, out)
		}
	}
	// Should contain ANSI escape codes
	if !strings.Contains(out, "\x1b[") {
		t.Error("expected ANSI escape codes in color output")
	}
}

func TestNewColorHandlerErrorLevel(t *testing.T) {
	var buf bytes.Buffer
	h := NewColorHandler(&buf)

	r := slog.NewRecord(time.Now(), slog.LevelError, "request failed", 0)
	r.AddAttrs(slog.String(elastictransport.LogKeyError, "connection refused"))

	if err := h.Handle(context.Background(), r); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	if !strings.Contains(out, "\x1b[31m") { // red
		t.Error("expected red ANSI code for error level")
	}
}

func TestNewCurlHandler(t *testing.T) {
	var buf bytes.Buffer
	h := NewCurlHandler(&buf)

	if err := h.Handle(context.Background(), roundTripRecord()); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	if !strings.Contains(out, "curl -X GET 'http://localhost:9200/?pretty'") {
		t.Errorf("expected curl command in output:\n%s", out)
	}
	if !strings.Contains(out, "[200]") {
		t.Errorf("expected status code in output:\n%s", out)
	}
}

func TestNewCurlHandlerHead(t *testing.T) {
	var buf bytes.Buffer
	h := NewCurlHandler(&buf)

	r := slog.NewRecord(time.Now(), slog.LevelInfo, "request", 0)
	r.AddAttrs(
		slog.String(elastictransport.LogKeyMethod, "HEAD"),
		slog.String(elastictransport.LogKeyURL, "http://localhost:9200/"),
		slog.Int(elastictransport.LogKeyStatus, 200),
	)

	if err := h.Handle(context.Background(), r); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(buf.String(), "curl --head") {
		t.Errorf("expected 'curl --head' for HEAD method:\n%s", buf.String())
	}
}

func TestNewCurlHandlerNonRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	h := NewCurlHandler(&buf)

	r := slog.NewRecord(time.Now(), slog.LevelDebug, "removing connection", 0)
	r.AddAttrs(slog.String("node", "http://localhost:9200"))

	if err := h.Handle(context.Background(), r); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(buf.String(), "# DEBUG: removing connection") {
		t.Errorf("expected comment-style fallback:\n%s", buf.String())
	}
}

func TestNewJSONECSHandler(t *testing.T) {
	var buf bytes.Buffer
	h := NewJSONECSHandler(&buf)

	if err := h.Handle(context.Background(), roundTripRecord()); err != nil {
		t.Fatal(err)
	}

	var doc map[string]any
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("invalid JSON: %s\noutput: %s", err, buf.String())
	}

	if doc["@timestamp"] != "2024-01-15T10:30:00Z" {
		t.Errorf("unexpected @timestamp: %v", doc["@timestamp"])
	}
	if doc["message"] != "request" {
		t.Errorf("unexpected message: %v", doc["message"])
	}

	httpObj, ok := doc["http"].(map[string]any)
	if !ok {
		t.Fatal("expected http object")
	}
	req, ok := httpObj["request"].(map[string]any)
	if !ok {
		t.Fatal("expected http.request object")
	}
	if req["method"] != "GET" {
		t.Errorf("unexpected method: %v", req["method"])
	}

	res, ok := httpObj["response"].(map[string]any)
	if !ok {
		t.Fatal("expected http.response object")
	}
	if res["status_code"] != float64(200) {
		t.Errorf("unexpected status_code: %v", res["status_code"])
	}

	urlObj, ok := doc["url"].(map[string]any)
	if !ok {
		t.Fatal("expected url object")
	}
	if urlObj["full"] != "http://localhost:9200/" {
		t.Errorf("unexpected url.full: %v", urlObj["full"])
	}
}

func TestNewJSONECSHandlerError(t *testing.T) {
	var buf bytes.Buffer
	h := NewJSONECSHandler(&buf)

	r := slog.NewRecord(time.Now(), slog.LevelError, "request failed", 0)
	r.AddAttrs(
		slog.String(elastictransport.LogKeyMethod, "GET"),
		slog.String(elastictransport.LogKeyURL, "http://localhost:9200/"),
		slog.String(elastictransport.LogKeyError, "connection refused"),
	)

	if err := h.Handle(context.Background(), r); err != nil {
		t.Fatal(err)
	}

	var doc map[string]any
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("invalid JSON: %s", err)
	}

	errObj, ok := doc["error"].(map[string]any)
	if !ok {
		t.Fatal("expected error object")
	}
	if errObj["message"] != "connection refused" {
		t.Errorf("unexpected error.message: %v", errObj["message"])
	}
}

func TestColorHandlerWithAttrs(t *testing.T) {
	var buf bytes.Buffer
	h := NewColorHandler(&buf)
	h2 := h.WithAttrs([]slog.Attr{slog.String("component", "test")})

	r := slog.NewRecord(time.Now(), slog.LevelInfo, "hello", 0)
	if err := h2.Handle(context.Background(), r); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(buf.String(), "component") {
		t.Errorf("expected pre-set attr in output:\n%s", buf.String())
	}
}
