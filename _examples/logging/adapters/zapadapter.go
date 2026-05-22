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
	"context"

	"github.com/elastic/elastic-transport-go/v8/elastictransport"
	"go.uber.org/zap"
)

// ZapAdapter implements [elastictransport.LeveledLogger] using a
// [*zap.SugaredLogger]. The SugaredLogger's Debugw/Infow/Warnw/Errorw
// methods accept the same (msg, keysAndValues...) signature, making this
// a direct pass-through.
//
// Example:
//
//	logger, _ := zap.NewProduction()
//	tp, _ := elastictransport.NewClient(
//	    elastictransport.WithLeveledLogger(&adapters.ZapAdapter{
//	        Logger: logger.Sugar(),
//	    }),
//	)
type ZapAdapter struct {
	Logger       *zap.SugaredLogger
	ContextAttrs func(context.Context) []any
}

var _ elastictransport.LeveledLogger = (*ZapAdapter)(nil)

func (a *ZapAdapter) Debug(ctx context.Context, msg string, keysAndValues ...any) {
	a.Logger.Debugw(msg, a.enrich(ctx, keysAndValues)...)
}

func (a *ZapAdapter) Info(ctx context.Context, msg string, keysAndValues ...any) {
	a.Logger.Infow(msg, a.enrich(ctx, keysAndValues)...)
}

func (a *ZapAdapter) Warn(ctx context.Context, msg string, keysAndValues ...any) {
	a.Logger.Warnw(msg, a.enrich(ctx, keysAndValues)...)
}

func (a *ZapAdapter) Error(ctx context.Context, msg string, keysAndValues ...any) {
	a.Logger.Errorw(msg, a.enrich(ctx, keysAndValues)...)
}

func (a *ZapAdapter) enrich(ctx context.Context, kv []any) []any {
	if a.ContextAttrs == nil {
		return kv
	}
	if extra := a.ContextAttrs(ctx); len(extra) > 0 {
		return append(extra, kv...)
	}
	return kv
}
