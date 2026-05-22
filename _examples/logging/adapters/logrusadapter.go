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
	"fmt"

	"github.com/elastic/elastic-transport-go/v8/elastictransport"
	"github.com/sirupsen/logrus"
)

// LogrusAdapter implements [elastictransport.LeveledLogger] using a
// [*logrus.Logger]. Key-value pairs are converted to [logrus.Fields].
//
// Example:
//
//	logger := logrus.New()
//	logger.SetFormatter(&logrus.JSONFormatter{})
//	tp, _ := elastictransport.NewClient(
//	    elastictransport.WithLeveledLogger(&adapters.LogrusAdapter{
//	        Logger: logger,
//	    }),
//	)
type LogrusAdapter struct {
	Logger       *logrus.Logger
	ContextAttrs func(context.Context) []any
}

var _ elastictransport.LeveledLogger = (*LogrusAdapter)(nil)

func (a *LogrusAdapter) Debug(ctx context.Context, msg string, keysAndValues ...any) {
	a.Logger.WithContext(ctx).WithFields(kvToFields(a.enrich(ctx, keysAndValues))).Debug(msg)
}

func (a *LogrusAdapter) Info(ctx context.Context, msg string, keysAndValues ...any) {
	a.Logger.WithContext(ctx).WithFields(kvToFields(a.enrich(ctx, keysAndValues))).Info(msg)
}

func (a *LogrusAdapter) Warn(ctx context.Context, msg string, keysAndValues ...any) {
	a.Logger.WithContext(ctx).WithFields(kvToFields(a.enrich(ctx, keysAndValues))).Warn(msg)
}

func (a *LogrusAdapter) Error(ctx context.Context, msg string, keysAndValues ...any) {
	a.Logger.WithContext(ctx).WithFields(kvToFields(a.enrich(ctx, keysAndValues))).Error(msg)
}

func (a *LogrusAdapter) enrich(ctx context.Context, kv []any) []any {
	if a.ContextAttrs == nil {
		return kv
	}
	if extra := a.ContextAttrs(ctx); len(extra) > 0 {
		return append(extra, kv...)
	}
	return kv
}

func kvToFields(kv []any) logrus.Fields {
	fields := make(logrus.Fields, len(kv)/2)
	for i := 0; i+1 < len(kv); i += 2 {
		fields[fmt.Sprint(kv[i])] = kv[i+1]
	}
	return fields
}
