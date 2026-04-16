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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/elastic/elastic-transport-go/v8/elastictransport"
)

// NewJSONECSHandler returns a [slog.Handler] that produces JSON output
// following the Elastic Common Schema (ECS), similar to the deprecated
// [elastictransport.JSONLogger].
//
// Round-trip attributes (method, url, status, duration) are mapped to nested
// ECS fields. Additional attributes are included at the top level.
//
// Output format:
//
//	{"@timestamp":"...","event":{"duration":...},"url":{"full":"..."},"http":{"request":{...},"response":{...}}}
func NewJSONECSHandler(w io.Writer) slog.Handler {
	return &ecsHandler{w: w}
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
		case elastictransport.LogKeyMethod:
			method = a.Value.String()
		case elastictransport.LogKeyURL:
			urlStr = a.Value.String()
		case elastictransport.LogKeyStatus:
			status = intFromValue(a.Value)
			hasStatus = status != 0
		case elastictransport.LogKeyDuration:
			dur = durationFromValue(a.Value)
		case elastictransport.LogKeyError:
			errMsg = fmt.Sprint(a.Value.Any())
		case elastictransport.LogKeyRequestBody:
			reqBody = a.Value.String()
		case elastictransport.LogKeyResponseBody:
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
