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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestMetrics(t *testing.T) {
	t.Run("Metrics()", func(t *testing.T) {
		tp, _ := New(
			Config{
				URLs: []*url.URL{
					{Scheme: "http", Host: "foo1"},
					{Scheme: "http", Host: "foo2"},
					{Scheme: "http", Host: "foo3"},
				},
				DisableRetry:  true,
				EnableMetrics: true,
			},
		)

		tp.metrics.requests.Store(3)
		tp.metrics.failures.Store(4)
		tp.metrics.responses[200] = 1
		tp.metrics.responses[404] = 2

		req, _ := http.NewRequest("HEAD", "/", nil)
		_, _ = tp.Perform(req)

		m, err := tp.Metrics()
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		fmt.Println(m)

		if m.Requests != 4 {
			t.Errorf("Unexpected output, want=4, got=%d", m.Requests)
		}
		if m.Failures != 5 {
			t.Errorf("Unexpected output, want=5, got=%d", m.Failures)
		}
		if len(m.Responses) != 2 {
			t.Errorf("Unexpected output: %+v", m.Responses)
		}
		if len(m.Connections) != 3 {
			t.Errorf("Unexpected output: %+v", m.Connections)
		}
	})

	t.Run("Metrics() when not enabled", func(t *testing.T) {
		tp, _ := New(Config{})

		_, err := tp.Metrics()
		if err == nil {
			t.Fatalf("Expected error, got: %v", err)
		}
	})

	t.Run("String()", func(t *testing.T) {
		var m ConnectionMetric

		m = ConnectionMetric{URL: "http://foo1"}

		if m.String() != "{http://foo1}" {
			t.Errorf("Unexpected output: %s", m)
		}

		tt, _ := time.Parse(time.RFC3339, "2010-11-11T11:00:00Z")
		m = ConnectionMetric{
			URL:       "http://foo2",
			IsDead:    true,
			Failures:  123,
			DeadSince: &tt,
		}

		match, err := regexp.MatchString(
			`{http://foo2 dead=true failures=123 dead_since=Nov 11 \d+:00:00}`,
			m.String(),
		)
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		if !match {
			t.Errorf("Unexpected output: %s", m)
		}
	})
}

func TestTransportPerformAndReadMetricsResponses(t *testing.T) {
	t.Run("Read Metrics.Responses", func(t *testing.T) {
		u, _ := url.Parse("https://foo.com/bar")
		tp, _ := New(Config{
			EnableMetrics: true,
			URLs:          []*url.URL{u},
			Transport: &mockTransp{
				RoundTripFunc: func(req *http.Request) (*http.Response, error) { return &http.Response{Status: "MOCK"}, nil },
			}})

		ch := make(chan struct{})
		go func() {
			for {
				select {
				case <-ch:
					return
				default:
					metrics, _ := tp.Metrics()
					for range metrics.Responses {
					}
				}
			}
		}()

		for i := 0; i < 100000; i++ {
			req, _ := http.NewRequest("GET", "/abc", nil)
			_, _ = tp.Perform(req)
		}

		ch <- struct{}{}
		close(ch)
	})
}

func TestMetricsConcurrentWithDiscoverNodes(t *testing.T) {
	makeResponse := func(body []byte) *http.Response {
		return &http.Response{
			Status:        "200 OK",
			StatusCode:    200,
			ContentLength: int64(len(body)),
			Header:        map[string][]string{"Content-Type": {"application/json"}},
			Body:          io.NopCloser(bytes.NewReader(body)),
		}
	}

	buildNodesPayload := func(nodes map[string]string) []byte {
		type nodeHTTP struct {
			PublishAddress string `json:"publish_address"`
		}
		type node struct {
			Roles []string `json:"roles"`
			HTTP  nodeHTTP `json:"http"`
		}
		payload := map[string]map[string]node{"nodes": {}}
		for id, addr := range nodes {
			payload["nodes"][id] = node{
				Roles: []string{"data", "master"},
				HTTP:  nodeHTTP{PublishAddress: addr},
			}
		}
		b, _ := json.Marshal(payload)
		return b
	}

	payloads := [][]byte{
		buildNodesPayload(map[string]string{
			"es1": "es1:9200",
			"es2": "es2:9200",
		}),
		buildNodesPayload(map[string]string{
			"es1": "es1:9200",
			"es2": "es2:9200",
			"es3": "es3:9200",
		}),
	}

	var n uint64
	u1, _ := url.Parse("http://seed1:9200")
	u2, _ := url.Parse("http://seed2:9200")
	tp, _ := New(Config{
		EnableMetrics: true,
		URLs:          []*url.URL{u1, u2},
		Transport: &mockTransp{
			RoundTripFunc: func(req *http.Request) (*http.Response, error) {
				idx := atomic.AddUint64(&n, 1) % uint64(len(payloads))
				return makeResponse(payloads[idx]), nil
			},
		},
	})

	errCh := make(chan error, 2)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 400; i++ {
			if err := tp.DiscoverNodesContext(context.Background()); err != nil {
				errCh <- fmt.Errorf("discover nodes failed: %w", err)
				return
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 3000; i++ {
			m, err := tp.Metrics()
			if err != nil {
				errCh <- fmt.Errorf("read metrics failed: %w", err)
				return
			}
			_ = len(m.Connections)
		}
	}()

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Fatal(err)
	}
}
