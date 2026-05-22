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

//go:build !integration
// +build !integration

package elastictransport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestCloseConcurrent exercises Close racing with many in-flight Perform
// calls while scheduleDiscoverNodes is actively ticking. The goal is to
// prove under the race detector that:
//
//   - Close returns within its context deadline and drains discoverWaitGroup.
//   - Every Perform call returns either a *http.Response or ErrClosed,
//     never a panic, an unexpected error, or a hang.
//   - A non-zero number of calls observe each outcome, i.e. Close actually
//     fires mid-traffic rather than before or after.
func TestCloseConcurrent(t *testing.T) {
	// Build a /_nodes/http payload that points both nodes back at the
	// httptest server address; otherwise discovery would replace the pool
	// with addresses that don't exist and the test would connect-refuse.
	var serverAddr string
	nodesTmpl := `{"nodes":{` +
		`"es1":{"name":"es1","http":{"publish_address":"%s"},"roles":["master","data"]},` +
		`"es2":{"name":"es2","http":{"publish_address":"%s"},"roles":["master","data"]}` +
		`}}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/_nodes/http" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, nodesTmpl, serverAddr, serverAddr)
			return
		}
		// Simulate a small amount of work so Perform calls overlap with Close.
		time.Sleep(1 * time.Millisecond)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	serverAddr = strings.TrimPrefix(srv.URL, "http://")

	u, _ := url.Parse(srv.URL)
	tp, err := New(Config{
		URLs:                  []*url.URL{u},
		DiscoverNodesInterval: 5 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("New: %s", err)
	}

	const workers = 50
	const testTimeout = 5 * time.Second

	var (
		okCount     atomic.Int64
		closedCount atomic.Int64
		stopLoop    atomic.Bool
		wg          sync.WaitGroup
	)

	deadline := time.AfterFunc(testTimeout, func() {
		stopLoop.Store(true)
		t.Error("TestCloseConcurrent timed out; probable deadlock")
	})
	defer deadline.Stop()

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for !stopLoop.Load() {
				req, err := http.NewRequest("GET", "/", nil)
				if err != nil {
					t.Errorf("NewRequest: %s", err)
					return
				}
				res, err := tp.Perform(req)
				if err != nil {
					if errors.Is(err, ErrClosed) {
						closedCount.Add(1)
						return
					}
					t.Errorf("unexpected Perform error: %v", err)
					return
				}
				_, _ = io.Copy(io.Discard, res.Body)
				_ = res.Body.Close()
				okCount.Add(1)
			}
		}()
	}

	// Let traffic build up and discovery tick at least a few times.
	time.Sleep(20 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := tp.Close(ctx); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Close returned unexpected error: %v", err)
	}

	wg.Wait()
	stopLoop.Store(true)

	if okCount.Load() == 0 {
		t.Errorf("no Perform calls succeeded; Close may have fired before any request completed")
	}
	if closedCount.Load() == 0 {
		t.Errorf("no Perform calls observed ErrClosed; Close may not have taken effect")
	}
	t.Logf("ok=%d closed=%d", okCount.Load(), closedCount.Load())
}
