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
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"
)

// NewJSONECSLogger creates a *slog.Logger that produces JSON output following
// the Elastic Common Schema (ECS), similar to the deprecated
// [elastictransport.JSONLogger].
//
// Legacy JSONLogger output:
//
//	{"@timestamp":"...","event":{"duration":...},"url":{...},"http":{...}}
//
// This handler restructures the flat slog attributes into nested ECS fields.
// Round-trip attributes logged by the transport (method, url, status, duration)
// are mapped to their ECS equivalents. Additional attributes are included at
// the top level.
func NewJSONECSLogger(w io.Writer) *slog.Logger {
	return slog.New(&ecsHandler{w: w})
}

type ecsHandler struct {
	w  io.Writer
	mu sync.Mutex
}

func (h *ecsHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (h *ecsHandler) Handle(_ context.Context, r slog.Record) error {
	doc := map[string]any{
		"@timestamp": r.Time.UTC().Format(time.RFC3339),
		"log.level":  r.Level.String(),
		"message":    r.Message,
	}

	var method, urlStr, reqBody, resBody string
	var status int
	var dur time.Duration
	var errMsg string
	hasStatus := false

	r.Attrs(func(a slog.Attr) bool {
		switch a.Key {
		case "method":
			method = a.Value.String()
		case "url":
			urlStr = a.Value.String()
		case "status":
			status = intFromValue(a.Value)
			hasStatus = status != 0
		case "duration":
			dur = durationFromValue(a.Value)
		case "error":
			errMsg = fmt.Sprint(a.Value.Any())
		case "request.body":
			reqBody = a.Value.String()
		case "response.body":
			resBody = a.Value.String()
		default:
			doc[a.Key] = a.Value.Any()
		}
		return true
	})

	if dur > 0 {
		doc["event"] = map[string]any{"duration": dur.Nanoseconds()}
	}

	if method != "" || urlStr != "" {
		httpReq := map[string]any{}
		if method != "" {
			httpReq["method"] = method
		}
		if reqBody != "" {
			httpReq["body"] = reqBody
		}

		httpRes := map[string]any{}
		if hasStatus {
			httpRes["status_code"] = status
		}
		if resBody != "" {
			httpRes["body"] = resBody
		}

		httpObj := map[string]any{"request": httpReq}
		if len(httpRes) > 0 {
			httpObj["response"] = httpRes
		}
		doc["http"] = httpObj
	}

	if urlStr != "" {
		doc["url"] = map[string]any{"full": urlStr}
	}

	if errMsg != "" {
		doc["error"] = map[string]any{"message": errMsg}
	}

	b, err := json.Marshal(doc)
	if err != nil {
		return err
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	_, _ = h.w.Write(b)
	_, _ = h.w.Write([]byte("\n"))
	return nil
}

func (h *ecsHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *ecsHandler) WithGroup(_ string) slog.Handler       { return h }
