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
	"fmt"
	"net/http"
	"net/url"
	"regexp"
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

		tp.metrics.requests = 3
		tp.metrics.failures = 4
		tp.metrics.responses[200] = 1
		tp.metrics.responses[404] = 2

		req, _ := http.NewRequest("HEAD", "/", nil)
		tp.Perform(req)

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

	t.Run("Retry metrics tracking", func(t *testing.T) {
		var attemptCount int
		expectedRetries := 2

		u, _ := url.Parse("http://foo.bar")
		tp, _ := New(Config{
			URLs: []*url.URL{u},
			Transport: &mockTransp{
				RoundTripFunc: func(req *http.Request) (*http.Response, error) {
					attemptCount++
					fmt.Printf("Attempt #%d", attemptCount)
					if attemptCount <= expectedRetries {
						fmt.Print(": ERR\n")
						return nil, &mockNetError{error: fmt.Errorf("Mock network error (%d)", attemptCount)}
					}
					fmt.Print(": OK\n")
					return &http.Response{Status: "200 OK", StatusCode: 200}, nil
				},
			},
			EnableMetrics: true,
		})

		req, _ := http.NewRequest("GET", "/test", nil)
		res, err := tp.Perform(req)

		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		if res.StatusCode != 200 {
			t.Errorf("Unexpected response status: %d", res.StatusCode)
		}

		// Verify metrics
		metrics, err := tp.Metrics()
		if err != nil {
			t.Fatalf("Failed to get metrics: %s", err)
		}

		if metrics.Requests != 1 {
			t.Errorf("Expected 1 request, got %d", metrics.Requests)
		}

		if metrics.Retries != expectedRetries {
			t.Errorf("Expected %d retries, got %d", expectedRetries, metrics.Retries)
		}

		if metrics.Failures != expectedRetries {
			t.Errorf("Expected %d failures, got %d", expectedRetries, metrics.Failures)
		}

		// Verify the string representation includes retries
		metricsStr := metrics.String()
		expectedSubstring := fmt.Sprintf("Retries:%d", expectedRetries)
		if !regexp.MustCompile(expectedSubstring).MatchString(metricsStr) {
			t.Errorf("Expected metrics string to contain '%s', got: %s", expectedSubstring, metricsStr)
		}

		fmt.Printf("Final metrics: %s\n", metricsStr)
	})

	t.Run("No retry metrics when retries disabled", func(t *testing.T) {
		var attemptCount int

		u, _ := url.Parse("http://foo.bar")
		tp, _ := New(Config{
			URLs: []*url.URL{u},
			Transport: &mockTransp{
				RoundTripFunc: func(req *http.Request) (*http.Response, error) {
					attemptCount++
					fmt.Printf("Attempt #%d", attemptCount)
					if attemptCount == 1 {
						fmt.Print(": ERR\n")
						return nil, &mockNetError{error: fmt.Errorf("Mock network error (%d)", attemptCount)}
					}
					fmt.Print(": OK\n")
					return &http.Response{Status: "200 OK", StatusCode: 200}, nil
				},
			},
			EnableMetrics: true,
			DisableRetry:  true, // Retries disabled
		})

		req, _ := http.NewRequest("GET", "/test", nil)
		_, err := tp.Perform(req)

		// Should fail since retries are disabled
		if err == nil {
			t.Fatalf("Expected error due to disabled retries")
		}

		// Verify metrics - should show 1 failure but 0 retries
		metrics, err := tp.Metrics()
		if err != nil {
			t.Fatalf("Failed to get metrics: %s", err)
		}

		if metrics.Requests != 1 {
			t.Errorf("Expected 1 request, got %d", metrics.Requests)
		}

		if metrics.Retries != 0 {
			t.Errorf("Expected 0 retries when disabled, got %d", metrics.Retries)
		}

		if metrics.Failures != 1 {
			t.Errorf("Expected 1 failure, got %d", metrics.Failures)
		}

		fmt.Printf("Final metrics (retries disabled): %s\n", metrics.String())
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
					break
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
