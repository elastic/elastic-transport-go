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
	"fmt"
	"io"
	"log/slog"
	"sync"

	"github.com/elastic/elastic-transport-go/v8/elastictransport"
)

// NewColorHandler returns a [slog.Handler] that produces ANSI-colored terminal
// output similar to the deprecated [elastictransport.ColorLogger].
//
// The level and status code are colored: green for 2xx, yellow for 3xx/4xx,
// red for 5xx and errors.
func NewColorHandler(w io.Writer) slog.Handler {
	return &colorHandler{w: w}
}

type colorHandler struct {
	w     io.Writer
	mu    sync.Mutex
	attrs []slog.Attr
}

func (h *colorHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (h *colorHandler) Handle(_ context.Context, r slog.Record) error {
	var levelColor string
	switch {
	case r.Level >= slog.LevelError:
		levelColor = "\x1b[31m" // red
	case r.Level >= slog.LevelWarn:
		levelColor = "\x1b[33m" // yellow
	default:
		levelColor = "\x1b[32m" // green
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	_, _ = fmt.Fprintf(h.w, "%s%s\x1b[0m %s", levelColor, r.Level.String(), r.Message)

	for _, a := range h.attrs {
		_, _ = fmt.Fprintf(h.w, " %s%s\x1b[0m=%v", levelColor, a.Key, a.Value)
	}

	r.Attrs(func(a slog.Attr) bool {
		if a.Key == elastictransport.LogKeyStatus {
			code := a.Value.Any()
			color := statusColor(code)
			_, _ = fmt.Fprintf(h.w, " %sstatus=%v\x1b[0m", color, code)
		} else {
			_, _ = fmt.Fprintf(h.w, " %s=%v", a.Key, a.Value)
		}
		return true
	})

	_, _ = fmt.Fprintln(h.w)
	return nil
}

func (h *colorHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &colorHandler{w: h.w, attrs: append(h.attrs[:len(h.attrs):len(h.attrs)], attrs...)}
}

func (h *colorHandler) WithGroup(_ string) slog.Handler { return h }

func statusColor(v any) string {
	var code int
	switch c := v.(type) {
	case int:
		code = c
	case int64:
		code = int(c)
	default:
		return "\x1b[31;4m" // red underline for unknown
	}
	switch {
	case code >= 200 && code < 300:
		return "\x1b[32m" // green
	case code >= 300 && code < 500:
		return "\x1b[33m" // yellow
	default:
		return "\x1b[31m" // red
	}
}
