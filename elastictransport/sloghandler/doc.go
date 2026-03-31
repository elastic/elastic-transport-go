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

// Package sloghandler provides [log/slog.Handler] implementations that
// format transport log entries in specific styles: plain text, ANSI color,
// curl commands, and ECS-structured JSON.
//
// Each handler is a drop-in replacement for the corresponding deprecated
// logger in the [elastictransport] package:
//
//   - [NewTextHandler] replaces [elastictransport.TextLogger]
//   - [NewColorHandler] replaces [elastictransport.ColorLogger]
//   - [NewCurlHandler] replaces [elastictransport.CurlLogger]
//   - [NewJSONECSHandler] replaces [elastictransport.JSONLogger]
//
// Pass a handler to [log/slog.New] and then to
// [elastictransport.SlogLogger]:
//
//	tp, _ := elastictransport.NewClient(
//	    elastictransport.WithURLs(u),
//	    elastictransport.WithLeveledLogger(&elastictransport.SlogLogger{
//	        Logger: slog.New(sloghandler.NewColorHandler(os.Stderr)),
//	    }),
//	    elastictransport.WithInterceptors(
//	        elastictransport.LoggingInterceptor(false, false),
//	    ),
//	)
package sloghandler
