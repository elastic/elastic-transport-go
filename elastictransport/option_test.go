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
	"compress/gzip"
	"context"
	"net/http"
	"net/url"
	"reflect"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	t.Run("Zero options uses defaults", func(t *testing.T) {
		tp, err := NewClient()
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}
		if tp.transport == nil {
			t.Error("Expected the transport to not be nil")
		}
		if tp.maxRetries != defaultMaxRetries {
			t.Errorf("Expected default maxRetries=%d, got=%d", defaultMaxRetries, tp.maxRetries)
		}
		if !reflect.DeepEqual(tp.retryOnStatus, defaultRetryOnStatus[:]) {
			t.Errorf("Expected default retryOnStatus, got=%v", tp.retryOnStatus)
		}
	})

	t.Run("Parity with New(Config{})", func(t *testing.T) {
		cfgClient, err := New(Config{})
		if err != nil {
			t.Fatalf("New(Config{}) error: %s", err)
		}
		optClient, err := NewClient()
		if err != nil {
			t.Fatalf("NewClient() error: %s", err)
		}

		if cfgClient.maxRetries != optClient.maxRetries {
			t.Errorf("maxRetries: Config=%d, Option=%d", cfgClient.maxRetries, optClient.maxRetries)
		}
		if !reflect.DeepEqual(cfgClient.retryOnStatus, optClient.retryOnStatus) {
			t.Errorf("retryOnStatus: Config=%v, Option=%v", cfgClient.retryOnStatus, optClient.retryOnStatus)
		}
		if cfgClient.disableRetry != optClient.disableRetry {
			t.Errorf("disableRetry: Config=%v, Option=%v", cfgClient.disableRetry, optClient.disableRetry)
		}
		if cfgClient.compressRequestBody != optClient.compressRequestBody {
			t.Errorf("compressRequestBody: Config=%v, Option=%v", cfgClient.compressRequestBody, optClient.compressRequestBody)
		}
	})
}

func TestWithURLs(t *testing.T) {
	u1, _ := url.Parse("http://localhost:9200")
	u2, _ := url.Parse("http://localhost:9201")

	tp, err := NewClient(WithURLs(u1, u2))
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	urls := tp.URLs()
	if len(urls) != 2 {
		t.Fatalf("Expected 2 URLs, got %d", len(urls))
	}
	if urls[0].Host != "localhost:9200" {
		t.Errorf("Unexpected first URL host: %s", urls[0].Host)
	}
	if urls[1].Host != "localhost:9201" {
		t.Errorf("Unexpected second URL host: %s", urls[1].Host)
	}
}

func TestWithBasicAuth(t *testing.T) {
	u, _ := url.Parse("http://example.com")
	tp, err := NewClient(WithURLs(u), WithBasicAuth("foo", "bar"))
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	req, _ := http.NewRequest("GET", "/", nil)
	tp.setReqAuth(u, req)

	username, password, ok := req.BasicAuth()
	if !ok {
		t.Fatal("Expected Basic Auth to be set")
	}
	if username != "foo" || password != "bar" {
		t.Errorf("Unexpected credentials: %s:%s", username, password)
	}
}

func TestWithAPIKey(t *testing.T) {
	u, _ := url.Parse("http://example.com")
	tp, err := NewClient(WithURLs(u), WithAPIKey("Zm9vYmFy"))
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	req, _ := http.NewRequest("GET", "/", nil)
	tp.setReqAuth(u, req)

	value := req.Header.Get("Authorization")
	if value != "APIKey Zm9vYmFy" {
		t.Errorf("Unexpected Authorization header: %s", value)
	}
}

func TestWithServiceToken(t *testing.T) {
	u, _ := url.Parse("http://example.com")
	tp, err := NewClient(WithURLs(u), WithServiceToken("AAEAAWVs"))
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	req, _ := http.NewRequest("GET", "/", nil)
	tp.setReqAuth(u, req)

	value := req.Header.Get("Authorization")
	if value != "Bearer AAEAAWVs" {
		t.Errorf("Unexpected Authorization header: %s", value)
	}
}

func TestWithHeader(t *testing.T) {
	hdr := http.Header{}
	hdr.Set("X-Custom", "test-value")

	tp, err := NewClient(WithHeader(hdr))
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	req, _ := http.NewRequest("GET", "/", nil)
	tp.setReqGlobalHeader(req)

	if req.Header.Get("X-Custom") != "test-value" {
		t.Errorf("Unexpected header value: %s", req.Header.Get("X-Custom"))
	}
}

func TestWithUserAgent(t *testing.T) {
	u, _ := url.Parse("http://example.com")
	tp, err := NewClient(WithURLs(u), WithUserAgent("my-agent/1.0"))
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	req, _ := http.NewRequest("GET", "/", nil)
	tp.setReqUserAgent(req)

	if req.UserAgent() != "my-agent/1.0" {
		t.Errorf("Unexpected User-Agent: %s", req.UserAgent())
	}
}

func TestWithTransport(t *testing.T) {
	mock := &mockTransp{
		RoundTripFunc: func(req *http.Request) (*http.Response, error) {
			return &http.Response{Status: "MOCK"}, nil
		},
	}

	tp, err := NewClient(WithURLs(&url.URL{}), WithTransport(mock))
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	res, err := tp.roundTrip(&http.Request{URL: &url.URL{}})
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	if res.Status != "MOCK" {
		t.Errorf("Unexpected response: %+v", res)
	}
}

func TestWithDisableRetry(t *testing.T) {
	tp, err := NewClient(WithDisableRetry(true))
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	if !tp.disableRetry {
		t.Error("Expected disableRetry to be true")
	}
}

func TestWithRetryOnStatus(t *testing.T) {
	tp, err := NewClient(WithRetryOnStatus(404, 429))
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	if !reflect.DeepEqual(tp.retryOnStatus, []int{404, 429}) {
		t.Errorf("Unexpected retryOnStatus: %v", tp.retryOnStatus)
	}
}

func TestWithMaxRetries(t *testing.T) {
	tp, err := NewClient(WithMaxRetries(10))
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	if tp.maxRetries != 10 {
		t.Errorf("Expected maxRetries=10, got=%d", tp.maxRetries)
	}
}

func TestWithRetryBackoff(t *testing.T) {
	fn := func(attempt int) time.Duration { return time.Duration(attempt) * time.Second }
	tp, err := NewClient(WithRetryBackoff(fn))
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	if tp.retryBackoff == nil {
		t.Fatal("Expected retryBackoff to be set")
	}
	if tp.retryBackoff(2) != 2*time.Second {
		t.Errorf("Unexpected backoff duration: %s", tp.retryBackoff(2))
	}
}

func TestWithRetryOnError(t *testing.T) {
	fn := func(req *http.Request, err error) bool { return false }
	tp, err := NewClient(WithRetryOnError(fn))
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	if tp.retryOnError == nil {
		t.Fatal("Expected retryOnError to be set")
	}
}

func TestWithCompressRequestBody(t *testing.T) {
	tp, err := NewClient(WithCompressRequestBody(true))
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	if !tp.compressRequestBody {
		t.Error("Expected compressRequestBody to be true")
	}
}

func TestWithCompressRequestBodyLevel(t *testing.T) {
	tp, err := NewClient(
		WithCompressRequestBody(true),
		WithCompressRequestBodyLevel(gzip.BestSpeed),
	)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	if tp.compressRequestBodyLevel != gzip.BestSpeed {
		t.Errorf("Expected compressRequestBodyLevel=%d, got=%d", gzip.BestSpeed, tp.compressRequestBodyLevel)
	}
}

func TestWithMetrics(t *testing.T) {
	tp, err := NewClient(WithMetrics(true))
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	if tp.metrics == nil {
		t.Error("Expected metrics to be enabled")
	}
}

func TestWithCertificateFingerprint(t *testing.T) {
	tp, err := NewClient(
		WithCertificateFingerprint("7A3A6031CD097DA0EE84D65137912A84576B50194045B41F4F4B8AC1A98116BE"),
	)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	if tp.transport == nil {
		t.Fatal("Expected transport to be set")
	}
	httpTP, ok := tp.transport.(*http.Transport)
	if !ok {
		t.Fatalf("Expected *http.Transport, got %T", tp.transport)
	}
	if httpTP.DialTLSContext == nil {
		t.Error("Expected DialTLSContext to be set for fingerprint verification")
	}
}

func TestWithDiscoverNodesInterval(t *testing.T) {
	u, _ := url.Parse("http://localhost:9200")
	tp, err := NewClient(
		WithURLs(u),
		WithDiscoverNodesInterval(30*time.Second),
		WithTransport(&mockTransp{
			RoundTripFunc: func(req *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 200, Body: http.NoBody}, nil
			},
		}),
	)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	defer tp.Close(context.Background()) //nolint:errcheck
	if tp.discoverNodesInterval != 30*time.Second {
		t.Errorf("Expected 30s, got %s", tp.discoverNodesInterval)
	}
}

func TestWithDiscoverNodeTimeout(t *testing.T) {
	tp, err := NewClient(WithDiscoverNodeTimeout(5 * time.Second))
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	if tp.discoverNodeTimeout == nil {
		t.Fatal("Expected discoverNodeTimeout to be set")
	}
	if *tp.discoverNodeTimeout != 5*time.Second {
		t.Errorf("Expected 5s, got %s", *tp.discoverNodeTimeout)
	}
}

func TestWithSelector(t *testing.T) {
	sel := newRoundRobinSelector()
	u, _ := url.Parse("http://localhost:9200")
	tp, err := NewClient(WithURLs(u), WithSelector(sel))
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	if tp.selector != sel {
		t.Error("Expected selector to match the provided one")
	}
}

func TestWithConnectionPoolFunc(t *testing.T) {
	u, _ := url.Parse("http://localhost:9200")
	tp, err := NewClient(
		WithURLs(u),
		WithConnectionPoolFunc(func(conns []*Connection, selector Selector) ConnectionPool {
			return &CustomConnectionPool{
				urls: []*url.URL{{Scheme: "http", Host: "custom1"}},
			}
		}),
	)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	sp, ok := tp.pool.(*synchronizedPool)
	if !ok {
		t.Fatalf("Expected *synchronizedPool, got %T", tp.pool)
	}
	if _, ok := sp.pool.(*CustomConnectionPool); !ok {
		t.Fatalf("Expected inner pool to be *CustomConnectionPool, got %T", sp.pool)
	}
}

func TestWithInterceptors(t *testing.T) {
	interceptor := func(next RoundTripFunc) RoundTripFunc {
		return func(req *http.Request) (*http.Response, error) {
			req.Header.Add("X-Test", "true")
			return next(req)
		}
	}

	u, _ := url.Parse("http://example.com")
	tp, err := NewClient(
		WithURLs(u),
		WithInterceptors(interceptor),
		WithTransport(&mockTransp{
			RoundTripFunc: func(req *http.Request) (*http.Response, error) {
				return &http.Response{Status: "MOCK"}, nil
			},
		}),
	)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	if tp.interceptor == nil {
		t.Fatal("Expected interceptor to be set")
	}

	req, _ := http.NewRequest("GET", "/", nil)
	res, err := tp.roundTrip(req)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	if res.Status != "MOCK" {
		t.Errorf("Unexpected response: %+v", res)
	}
	if req.Header.Get("X-Test") != "true" {
		t.Error("Expected interceptor to set X-Test header")
	}
}

func TestWithPoolCompressor(t *testing.T) {
	tp, err := NewClient(
		WithCompressRequestBody(true),
		WithPoolCompressor(true),
	)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	if tp.gzipCompressor == nil {
		t.Fatal("Expected gzipCompressor to be set")
	}
}

func TestOptionLastValueWins(t *testing.T) {
	tp, err := NewClient(
		WithMaxRetries(1),
		WithMaxRetries(7),
	)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	if tp.maxRetries != 7 {
		t.Errorf("Expected last value (7) to win, got=%d", tp.maxRetries)
	}
}

func TestOptionParityWithConfig(t *testing.T) {
	u, _ := url.Parse("http://example.com")
	mock := &mockTransp{
		RoundTripFunc: func(req *http.Request) (*http.Response, error) {
			return &http.Response{Status: "MOCK"}, nil
		},
	}
	hdr := http.Header{}
	hdr.Set("X-Custom", "value")

	cfgClient, err := New(Config{
		URLs:                []*url.URL{u},
		Username:            "user",
		Password:            "pass",
		APIKey:              "key123",
		ServiceToken:        "tok456",
		Header:              hdr,
		UserAgent:           "test-agent",
		Transport:           mock,
		DisableRetry:        true,
		RetryOnStatus:       []int{429},
		MaxRetries:          5,
		CompressRequestBody: true,
		EnableMetrics:       true,
	})
	if err != nil {
		t.Fatalf("New error: %s", err)
	}

	optClient, err := NewClient(
		WithURLs(u),
		WithBasicAuth("user", "pass"),
		WithAPIKey("key123"),
		WithServiceToken("tok456"),
		WithHeader(hdr),
		WithUserAgent("test-agent"),
		WithTransport(mock),
		WithDisableRetry(true),
		WithRetryOnStatus(429),
		WithMaxRetries(5),
		WithCompressRequestBody(true),
		WithMetrics(true),
	)
	if err != nil {
		t.Fatalf("NewClient error: %s", err)
	}

	if cfgClient.userAgent != optClient.userAgent {
		t.Errorf("userAgent mismatch: %q vs %q", cfgClient.userAgent, optClient.userAgent)
	}
	if cfgClient.username != optClient.username {
		t.Errorf("username mismatch: %q vs %q", cfgClient.username, optClient.username)
	}
	if cfgClient.password != optClient.password {
		t.Errorf("password mismatch: %q vs %q", cfgClient.password, optClient.password)
	}
	if cfgClient.apikey != optClient.apikey {
		t.Errorf("apikey mismatch: %q vs %q", cfgClient.apikey, optClient.apikey)
	}
	if cfgClient.servicetoken != optClient.servicetoken {
		t.Errorf("servicetoken mismatch: %q vs %q", cfgClient.servicetoken, optClient.servicetoken)
	}
	if cfgClient.disableRetry != optClient.disableRetry {
		t.Errorf("disableRetry mismatch: %v vs %v", cfgClient.disableRetry, optClient.disableRetry)
	}
	if !reflect.DeepEqual(cfgClient.retryOnStatus, optClient.retryOnStatus) {
		t.Errorf("retryOnStatus mismatch: %v vs %v", cfgClient.retryOnStatus, optClient.retryOnStatus)
	}
	if cfgClient.maxRetries != optClient.maxRetries {
		t.Errorf("maxRetries mismatch: %d vs %d", cfgClient.maxRetries, optClient.maxRetries)
	}
	if cfgClient.compressRequestBody != optClient.compressRequestBody {
		t.Errorf("compressRequestBody mismatch: %v vs %v", cfgClient.compressRequestBody, optClient.compressRequestBody)
	}
	if (cfgClient.metrics == nil) != (optClient.metrics == nil) {
		t.Errorf("metrics mismatch: cfg=%v, opt=%v", cfgClient.metrics != nil, optClient.metrics != nil)
	}
}
