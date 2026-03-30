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

package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"
)

// NewCurlLogger creates a *slog.Logger that formats round-trip log entries
// as runnable curl commands, similar to the deprecated
// [elastictransport.CurlLogger].
//
// Legacy CurlLogger output:
//
//	curl -X GET 'http://localhost:9200/?pretty'
//	# => 2024-01-15T10:30:00Z [200 OK] 5ms
//
// This handler produces equivalent output from the structured attributes
// logged by the transport's round-trip adapter.
func NewCurlLogger(w io.Writer) *slog.Logger {
	return slog.New(&curlHandler{w: w})
}

type curlHandler struct {
	w  io.Writer
	mu sync.Mutex
}

func (h *curlHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (h *curlHandler) Handle(_ context.Context, r slog.Record) error {
	var method, urlStr string
	var status int
	var dur time.Duration
	var errMsg string

	r.Attrs(func(a slog.Attr) bool {
		switch a.Key {
		case "method":
			method = a.Value.String()
		case "url":
			urlStr = a.Value.String()
		case "status":
			status = intFromValue(a.Value)
		case "duration":
			dur = durationFromValue(a.Value)
		case "error":
			errMsg = fmt.Sprint(a.Value.Any())
		}
		return true
	})

	// Only format as curl for round-trip messages
	if method == "" && urlStr == "" {
		// Non-round-trip message: fall back to simple format
		h.mu.Lock()
		defer h.mu.Unlock()
		_, _ = fmt.Fprintf(h.w, "# %s: %s\n", r.Level.String(), r.Message)
		r.Attrs(func(a slog.Attr) bool {
			_, _ = fmt.Fprintf(h.w, "#   %s=%v\n", a.Key, a.Value)
			return true
		})
		return nil
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if method == "HEAD" {
		_, _ = fmt.Fprintf(h.w, "curl --head '%s?pretty'\n", urlStr)
	} else {
		_, _ = fmt.Fprintf(h.w, "curl -X %s '%s?pretty'\n", method, urlStr)
	}

	ts := r.Time.UTC().Format(time.RFC3339)
	if errMsg != "" {
		_, _ = fmt.Fprintf(h.w, "# => %s [ERROR: %s] %s\n", ts, errMsg, dur.Truncate(time.Millisecond))
	} else {
		_, _ = fmt.Fprintf(h.w, "# => %s [%d] %s\n", ts, status, dur.Truncate(time.Millisecond))
	}
	_, _ = fmt.Fprintln(h.w)

	return nil
}

func (h *curlHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *curlHandler) WithGroup(_ string) slog.Handler       { return h }
