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
	"github.com/go-logr/logr"
)

// LogrAdapter implements [elastictransport.LeveledLogger] using a
// [logr.Logger]. Since logr does not have a Warn level, warnings are
// logged at V(0) (the default info verbosity). Debug uses V(1).
//
// The Error method extracts any value keyed "error" from keysAndValues
// and passes it as the error argument to [logr.Logger.Error]. If no
// "error" key is found, a nil error is passed.
//
// Example:
//
//	// Using logr with the standard library backend:
//	import "github.com/go-logr/stdr"
//	logger := stdr.New(log.Default())
//	tp, _ := elastictransport.NewClient(
//	    elastictransport.WithLeveledLogger(&adapters.LogrAdapter{
//	        Logger: logger,
//	    }),
//	)
type LogrAdapter struct {
	Logger logr.Logger
}

var _ elastictransport.LeveledLogger = (*LogrAdapter)(nil)

func (a *LogrAdapter) Debug(msg string, keysAndValues ...any) {
	a.Logger.V(1).Info(msg, keysAndValues...)
}

func (a *LogrAdapter) Info(msg string, keysAndValues ...any) {
	a.Logger.Info(msg, keysAndValues...)
}

func (a *LogrAdapter) Warn(msg string, keysAndValues ...any) {
	a.Logger.V(0).Info(msg, keysAndValues...)
}

func (a *LogrAdapter) Error(msg string, keysAndValues ...any) {
	var err error
	remaining := make([]any, 0, len(keysAndValues))
	for i := 0; i+1 < len(keysAndValues); i += 2 {
		if fmt.Sprint(keysAndValues[i]) == "error" {
			if e, ok := keysAndValues[i+1].(error); ok {
				err = e
				continue
			}
		}
		remaining = append(remaining, keysAndValues[i], keysAndValues[i+1])
	}
	a.Logger.Error(err, msg, remaining...)
}
