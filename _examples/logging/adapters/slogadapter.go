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

// Package adapters provides LeveledLogger implementations for popular Go
// logging libraries.
//
// Each adapter is a thin wrapper that translates the
// [elastictransport.LeveledLogger] interface into the target library's API.
// Copy the adapter you need into your project, or import this package directly.
//
// For log/slog, use [elastictransport.SlogLogger] from the main package — it
// is the canonical adapter and requires no additional dependencies:
//
//	tp, _ := elastictransport.NewClient(
//	    elastictransport.WithLeveledLogger(&elastictransport.SlogLogger{
//	        Logger: slog.Default(),
//	    }),
//	)
package adapters
