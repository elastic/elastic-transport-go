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
	"io"
	"log/slog"
)

// NewTextLogger creates a *slog.Logger that produces plain-text output similar
// to the deprecated [elastictransport.TextLogger].
//
// Legacy TextLogger output:
//
//	2024-01-15T10:30:00Z GET http://localhost:9200/ [status:200 request:5ms]
//
// slog equivalent output:
//
//	time=2024-01-15T10:30:00.000Z level=INFO msg=request method=GET url=http://localhost:9200/ status=200 duration=5ms
//
// The structured key=value format differs from the legacy bespoke format, but
// carries the same information and is more machine-parseable.
func NewTextLogger(w io.Writer) *slog.Logger {
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
}
