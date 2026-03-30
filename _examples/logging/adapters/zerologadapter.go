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
	"fmt"

	"github.com/elastic/elastic-transport-go/v8/elastictransport"
	"github.com/rs/zerolog"
)

// ZerologAdapter implements [elastictransport.LeveledLogger] using a
// [zerolog.Logger]. Key-value pairs are added to the event via
// [zerolog.Event.Interface].
//
// Example:
//
//	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
//	tp, _ := elastictransport.NewClient(
//	    elastictransport.WithLeveledLogger(&adapters.ZerologAdapter{
//	        Logger: logger,
//	    }),
//	)
type ZerologAdapter struct {
	Logger zerolog.Logger
}

var _ elastictransport.LeveledLogger = (*ZerologAdapter)(nil)

func (a *ZerologAdapter) Debug(msg string, keysAndValues ...any) {
	addKeysAndValues(a.Logger.Debug(), keysAndValues).Msg(msg)
}

func (a *ZerologAdapter) Info(msg string, keysAndValues ...any) {
	addKeysAndValues(a.Logger.Info(), keysAndValues).Msg(msg)
}

func (a *ZerologAdapter) Warn(msg string, keysAndValues ...any) {
	addKeysAndValues(a.Logger.Warn(), keysAndValues).Msg(msg)
}

func (a *ZerologAdapter) Error(msg string, keysAndValues ...any) {
	addKeysAndValues(a.Logger.Error(), keysAndValues).Msg(msg)
}

func addKeysAndValues(e *zerolog.Event, kv []any) *zerolog.Event {
	for i := 0; i+1 < len(kv); i += 2 {
		key := fmt.Sprint(kv[i])
		e = e.Interface(key, kv[i+1])
	}
	return e
}
