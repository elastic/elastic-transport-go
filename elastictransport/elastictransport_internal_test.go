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
	"compress/gzip"
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

var (
	_     = fmt.Print
	lrand = rand.New(rand.NewSource(time.Now().Unix()))
)

type mockTransp struct {
	RoundTripFunc func(req *http.Request) (*http.Response, error)
}

func (t *mockTransp) RoundTrip(req *http.Request) (*http.Response, error) {
	return t.RoundTripFunc(req)
}

type mockNetError struct{ error }

func (e *mockNetError) Timeout() bool   { return false }
func (e *mockNetError) Temporary() bool { return false }

func TestConfigureTLS(t *testing.T) {
	t.Run("Returns error when transport is nil", func(t *testing.T) {
		if err := ConfigureTLS(nil, nil, ""); err == nil {
			t.Fatal("Expected error, got nil")
		}
	})

	t.Run("Returns error on invalid CA cert", func(t *testing.T) {
		transport := &http.Transport{}
		if err := ConfigureTLS(transport, []byte("invalid"), ""); err == nil {
			t.Fatal("Expected error, got nil")
		}
	})

	t.Run("Initializes TLS config and RootCAs from PEM", func(t *testing.T) {
		caCert, err := os.ReadFile("testdata/cert.pem")
		if err != nil {
			t.Fatalf("Unexpected error reading cert: %s", err)
		}

		transport := &http.Transport{}
		if err := ConfigureTLS(transport, caCert, ""); err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		if transport.TLSClientConfig == nil {
			t.Fatal("Expected TLSClientConfig to be initialized")
		}
		if transport.TLSClientConfig.RootCAs == nil {
			t.Fatal("Expected RootCAs to be initialized")
		}
	})

	t.Run("Returns error on invalid fingerprint hex", func(t *testing.T) {
		transport := &http.Transport{}
		err := ConfigureTLS(transport, nil, "not-valid-hex")
		if err == nil {
			t.Fatal("Expected error, got nil")
		}
		if !strings.Contains(err.Error(), "invalid certificate fingerprint") {
			t.Fatalf("Unexpected error message: %s", err)
		}
	})

	t.Run("Sets DialTLSContext when fingerprint is configured", func(t *testing.T) {
		transport := &http.Transport{}
		if err := ConfigureTLS(transport, nil, "A98116BE"); err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		if transport.DialTLSContext == nil {
			t.Fatal("Expected DialTLSContext to be configured")
		}
	})

	t.Run("Fingerprint takes precedence over CACert", func(t *testing.T) {
		caCert, err := os.ReadFile("testdata/cert.pem")
		if err != nil {
			t.Fatalf("Unexpected error reading cert: %s", err)
		}

		transport := &http.Transport{}
		if err := ConfigureTLS(transport, caCert, "A98116BE"); err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		if transport.DialTLSContext == nil {
			t.Fatal("Expected DialTLSContext to be configured")
		}
		if transport.TLSClientConfig != nil {
			t.Fatal("Expected TLSClientConfig to remain nil when fingerprint is set")
		}
	})
}

func TestTransport(t *testing.T) {
	t.Run("Interface", func(t *testing.T) {
		tp, _ := New(Config{})
		var _ Interface = tp
		var _ = tp.transport
	})

	t.Run("Default", func(t *testing.T) {
		tp, _ := New(Config{})
		if tp.transport == nil {
			t.Error("Expected the transport to not be nil")
		}
		if _, ok := tp.transport.(*http.Transport); !ok {
			t.Errorf("Expected the transport to be http.DefaultTransport, got: %T", tp.transport)
		}
	})

	t.Run("Custom", func(t *testing.T) {
		tp, _ := New(Config{
			URLs: []*url.URL{{}},
			Transport: &mockTransp{
				RoundTripFunc: func(req *http.Request) (*http.Response, error) { return &http.Response{Status: "MOCK"}, nil },
			},
		})

		res, err := tp.roundTrip(&http.Request{URL: &url.URL{}})
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		if res.Status != "MOCK" {
			t.Errorf("Unexpected response from transport: %+v", res)
		}
	})
}

func TestTransportConfig(t *testing.T) {
	t.Run("Defaults", func(t *testing.T) {
		tp, _ := New(Config{})

		if !reflect.DeepEqual(tp.retryOnStatus, []int{502, 503, 504}) {
			t.Errorf("Unexpected retryOnStatus: %v", tp.retryOnStatus)
		}

		if tp.disableRetry {
			t.Errorf("Unexpected disableRetry: %v", tp.disableRetry)
		}

		if tp.maxRetries != 3 {
			t.Errorf("Unexpected maxRetries: %v", tp.maxRetries)
		}

		if tp.compressRequestBody {
			t.Errorf("Unexpected compressRequestBody: %v", tp.compressRequestBody)
		}
	})

	t.Run("Custom", func(t *testing.T) {
		tp, _ := New(Config{
			RetryOnStatus:          []int{404, 408},
			DisableRetry:           true,
			MaxRetries:             5,
			CompressRequestBody:    true,
			CertificateFingerprint: "7A3A6031CD097DA0EE84D65137912A84576B50194045B41F4F4B8AC1A98116BE",
		})

		if !reflect.DeepEqual(tp.retryOnStatus, []int{404, 408}) {
			t.Errorf("Unexpected retryOnStatus: %v", tp.retryOnStatus)
		}

		if !tp.disableRetry {
			t.Errorf("Unexpected disableRetry: %v", tp.disableRetry)
		}

		if tp.maxRetries != 5 {
			t.Errorf("Unexpected maxRetries: %v", tp.maxRetries)
		}

		if !tp.compressRequestBody {
			t.Errorf("Unexpected compressRequestBody: %v", tp.compressRequestBody)
		}

		if tp.transport.(*http.Transport).DialTLSContext != nil && http.DefaultTransport.(*http.Transport).DialTLSContext != nil {
			t.Errorf("DefaultTransportContext should have been cloned.")
		}
	})

	t.Run("CACert on default transport initializes TLS client config", func(t *testing.T) {
		caCert, err := os.ReadFile("testdata/cert.pem")
		if err != nil {
			t.Fatalf("Unexpected error reading cert: %s", err)
		}

		tp, err := New(Config{CACert: caCert})
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		httpTransport, ok := tp.transport.(*http.Transport)
		if !ok {
			t.Fatalf("Expected *http.Transport, got: %T", tp.transport)
		}

		if httpTransport.TLSClientConfig == nil {
			t.Fatal("Expected TLSClientConfig to be initialized")
		}
		if httpTransport.TLSClientConfig.RootCAs == nil {
			t.Fatal("Expected RootCAs to be initialized")
		}
	})

	t.Run("TLS options on non-http.Transport returns error", func(t *testing.T) {
		caCert, err := os.ReadFile("testdata/cert.pem")
		if err != nil {
			t.Fatalf("Unexpected error reading cert: %s", err)
		}

		_, err = New(Config{
			CACert: caCert,
			Transport: &mockTransp{
				RoundTripFunc: func(req *http.Request) (*http.Response, error) {
					return &http.Response{Status: "MOCK"}, nil
				},
			},
		})
		if err == nil {
			t.Fatal("Expected error, got nil")
		}

		expected := "unable to configure TLS for transport of type *elastictransport.mockTransp"
		if err.Error() != expected {
			t.Fatalf("Unexpected error, want=%q got=%q", expected, err.Error())
		}
	})
}

func TestTransportConnectionPool(t *testing.T) {
	t.Run("Single URL", func(t *testing.T) {
		tp, _ := New(Config{URLs: []*url.URL{{Scheme: "http", Host: "foo1"}}})

		if _, ok := tp.pool.(*singleConnectionPool); !ok {
			t.Errorf("Expected connection to be singleConnectionPool, got: %T", tp)
		}

		conn, err := tp.pool.Next()
		if err != nil {
			t.Errorf("Unexpected error: %s", err)
		}

		if conn.URL.String() != "http://foo1" {
			t.Errorf("Unexpected URL, want=http://foo1, got=%s", conn.URL)
		}
	})

	t.Run("Two URLs", func(t *testing.T) {
		var (
			conn *Connection
			err  error
		)

		tp, _ := New(Config{URLs: []*url.URL{
			{Scheme: "http", Host: "foo1"},
			{Scheme: "http", Host: "foo2"},
		}})

		if _, ok := tp.pool.(*statusConnectionPool); !ok {
			t.Errorf("Expected connection to be statusConnectionPool, got: %T", tp)
		}

		conn, err = tp.pool.Next()
		if err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		if conn.URL.String() != "http://foo1" {
			t.Errorf("Unexpected URL, want=foo1, got=%s", conn.URL)
		}

		conn, err = tp.pool.Next()
		if err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		if conn.URL.String() != "http://foo2" {
			t.Errorf("Unexpected URL, want=http://foo2, got=%s", conn.URL)
		}

		conn, err = tp.pool.Next()
		if err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		if conn.URL.String() != "http://foo1" {
			t.Errorf("Unexpected URL, want=http://foo1, got=%s", conn.URL)
		}
	})
}

type CustomConnectionPool struct {
	mu   sync.Mutex
	urls []*url.URL
}

// Next returns a random connection.
func (cp *CustomConnectionPool) Next() (*Connection, error) {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	u := cp.urls[lrand.Intn(len(cp.urls))]
	return &Connection{URL: u}, nil
}

func (cp *CustomConnectionPool) OnFailure(c *Connection) error {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	var index = -1
	for i, u := range cp.urls {
		if u == c.URL {
			index = i
		}
	}
	if index > -1 {
		cp.urls = append(cp.urls[:index], cp.urls[index+1:]...)
		return nil
	}
	return fmt.Errorf("connection not found")
}
func (cp *CustomConnectionPool) OnSuccess(c *Connection) error { return nil }
func (cp *CustomConnectionPool) URLs() []*url.URL {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	return cp.urls
}

type ConcurrentSafeCustomConnectionPool struct {
	*CustomConnectionPool
}

func (cp *ConcurrentSafeCustomConnectionPool) ConcurrentSafe() {}

type ConcurrentSafeUpdatableCustomConnectionPool struct {
	mu    sync.Mutex
	conns []*Connection
	curr  int
}

func (cp *ConcurrentSafeUpdatableCustomConnectionPool) ConcurrentSafe() {}

func (cp *ConcurrentSafeUpdatableCustomConnectionPool) Next() (*Connection, error) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if len(cp.conns) == 0 {
		return nil, fmt.Errorf("no connection available")
	}

	conn := cp.conns[cp.curr%len(cp.conns)]
	cp.curr++
	return conn, nil
}

func (cp *ConcurrentSafeUpdatableCustomConnectionPool) OnSuccess(*Connection) error { return nil }
func (cp *ConcurrentSafeUpdatableCustomConnectionPool) OnFailure(*Connection) error { return nil }

func (cp *ConcurrentSafeUpdatableCustomConnectionPool) URLs() []*url.URL {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	urls := make([]*url.URL, 0, len(cp.conns))
	for _, conn := range cp.conns {
		urls = append(urls, conn.URL)
	}

	return urls
}

func (cp *ConcurrentSafeUpdatableCustomConnectionPool) Update(conns []*Connection) error {
	if len(conns) == 0 {
		return fmt.Errorf("no connection available")
	}

	cp.mu.Lock()
	cp.conns = append(make([]*Connection, 0, len(conns)), conns...)
	if cp.curr >= len(cp.conns) {
		cp.curr = 0
	}
	cp.mu.Unlock()

	return nil
}

func (cp *ConcurrentSafeUpdatableCustomConnectionPool) connections() []*Connection {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	return append(make([]*Connection, 0, len(cp.conns)), cp.conns...)
}

func TestTransportCustomConnectionPool(t *testing.T) {
	t.Run("Run", func(t *testing.T) {
		tp, _ := New(Config{
			ConnectionPoolFunc: func(conns []*Connection, selector Selector) ConnectionPool {
				return &CustomConnectionPool{
					urls: []*url.URL{
						{Scheme: "http", Host: "custom1"},
						{Scheme: "http", Host: "custom2"},
					},
				}
			},
		})

		sp, ok := tp.pool.(*synchronizedPool)
		if !ok {
			t.Fatalf("Unexpected connection pool, want=*synchronizedPool, got=%T", tp.pool)
		}
		if _, ok := sp.pool.(*CustomConnectionPool); !ok {
			t.Fatalf("Unexpected inner pool, want=*CustomConnectionPool, got=%T", sp.pool)
		}

		conn, err := tp.pool.Next()
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}
		if conn.URL == nil {
			t.Errorf("Empty connection URL: %+v", conn)
		}
		if err := tp.pool.OnFailure(conn); err != nil {
			t.Errorf("Error removing the %q connection: %s", conn.URL, err)
		}
		if len(tp.pool.URLs()) != 1 {
			t.Errorf("Unexpected number of connections in pool: %q", tp.pool)
		}
	})
}

func TestTransportConcurrentSafeCustomConnectionPool(t *testing.T) {
	t.Run("Run", func(t *testing.T) {
		tp, _ := New(Config{
			ConnectionPoolFunc: func(conns []*Connection, selector Selector) ConnectionPool {
				return &ConcurrentSafeCustomConnectionPool{
					CustomConnectionPool: &CustomConnectionPool{
						urls: []*url.URL{
							{Scheme: "http", Host: "custom1"},
							{Scheme: "http", Host: "custom2"},
						},
					},
				}
			},
		})

		if _, ok := tp.pool.(*synchronizedPool); ok {
			t.Fatalf("Unexpected connection pool wrapper for concurrent-safe pool, got=%T", tp.pool)
		}
		if _, ok := tp.pool.(*ConcurrentSafeCustomConnectionPool); !ok {
			t.Fatalf("Unexpected connection pool, want=*ConcurrentSafeCustomConnectionPool, got=%T", tp.pool)
		}

		conn, err := tp.pool.Next()
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}
		if conn.URL == nil {
			t.Errorf("Empty connection URL: %+v", conn)
		}
		if err := tp.pool.OnFailure(conn); err != nil {
			t.Errorf("Error removing the %q connection: %s", conn.URL, err)
		}
		if len(tp.pool.URLs()) != 1 {
			t.Errorf("Unexpected number of connections in pool: %q", tp.pool)
		}
	})
}

func TestTransportConcurrentSafeCustomPoolConcurrentUsage(t *testing.T) {
	makeResponse := func(statusCode int, body string) *http.Response {
		return &http.Response{
			Status:        fmt.Sprintf("%d MOCK", statusCode),
			StatusCode:    statusCode,
			ContentLength: int64(len(body)),
			Header:        http.Header{"Content-Type": {"application/json"}},
			Body:          io.NopCloser(strings.NewReader(body)),
		}
	}

	payloads := []string{
		`{"nodes":{"es1":{"roles":["master","data"],"http":{"publish_address":"es1:9200"}},"es2":{"roles":["master","data"],"http":{"publish_address":"es2:9200"}}}}`,
		`{"nodes":{"es1":{"roles":["master","data"],"http":{"publish_address":"es1:9200"}},"es2":{"roles":["master","data"],"http":{"publish_address":"es2:9200"}},"es3":{"roles":["master","data"],"http":{"publish_address":"es3:9200"}}}}`,
	}

	var payloadMu sync.Mutex
	payloadIdx := 0

	u1, _ := url.Parse("http://seed1:9200")
	u2, _ := url.Parse("http://seed2:9200")
	tp, _ := New(Config{
		EnableMetrics: true,
		URLs:          []*url.URL{u1, u2},
		ConnectionPoolFunc: func(conns []*Connection, selector Selector) ConnectionPool {
			return &ConcurrentSafeUpdatableCustomConnectionPool{
				conns: append(make([]*Connection, 0, len(conns)), conns...),
			}
		},
		Transport: &mockTransp{
			RoundTripFunc: func(req *http.Request) (*http.Response, error) {
				if req.URL.Path == "/_nodes/http" {
					payloadMu.Lock()
					body := payloads[payloadIdx%len(payloads)]
					payloadIdx++
					payloadMu.Unlock()
					return makeResponse(http.StatusOK, body), nil
				}
				return makeResponse(http.StatusOK, ""), nil
			},
		},
		DisableRetry: true,
	})

	if _, ok := tp.pool.(*synchronizedPool); ok {
		t.Fatalf("Unexpected connection pool wrapper for concurrent-safe pool, got=%T", tp.pool)
	}
	if _, ok := tp.pool.(*ConcurrentSafeUpdatableCustomConnectionPool); !ok {
		t.Fatalf("Unexpected connection pool, got=%T", tp.pool)
	}

	errCh := make(chan error, 3)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 600; i++ {
			req, _ := http.NewRequest("GET", "/", nil)
			res, err := tp.Perform(req)
			if err != nil {
				errCh <- fmt.Errorf("perform failed: %w", err)
				return
			}
			_ = res.Body.Close()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			if err := tp.DiscoverNodesContext(context.Background()); err != nil {
				errCh <- fmt.Errorf("discover nodes failed: %w", err)
				return
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			_, err := tp.Metrics()
			if err != nil {
				errCh <- fmt.Errorf("metrics failed: %w", err)
				return
			}
		}
	}()

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Fatal(err)
	}
}

func TestTransportPerform(t *testing.T) {
	t.Run("Executes", func(t *testing.T) {
		u, _ := url.Parse("https://foo.com/bar")
		tp, _ := New(Config{
			URLs: []*url.URL{u},
			Transport: &mockTransp{
				RoundTripFunc: func(req *http.Request) (*http.Response, error) { return &http.Response{Status: "MOCK"}, nil },
			}})

		req, _ := http.NewRequest("GET", "/abc", nil)

		res, err := tp.Perform(req)
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		if res.Status != "MOCK" {
			t.Errorf("Unexpected response: %+v", res)
		}
	})

	t.Run("Sets URL", func(t *testing.T) {
		u, _ := url.Parse("https://foo.com/bar")
		tp, _ := New(Config{URLs: []*url.URL{u}})

		req, _ := http.NewRequest("GET", "/abc", nil)
		tp.setReqURL(u, req)

		expected := "https://foo.com/bar/abc"

		if req.URL.String() != expected {
			t.Errorf("req.URL: got=%s, want=%s", req.URL, expected)
		}
	})

	t.Run("Sets HTTP Basic Auth from URL", func(t *testing.T) {
		u, _ := url.Parse("https://foo:bar@example.com")
		tp, _ := New(Config{URLs: []*url.URL{u}})

		req, _ := http.NewRequest("GET", "/", nil)
		tp.setReqAuth(u, req)

		username, password, ok := req.BasicAuth()
		if !ok {
			t.Error("Expected the request to have Basic Auth set")
		}

		if username != "foo" || password != "bar" {
			t.Errorf("Unexpected values for username and password: %s:%s", username, password)
		}
	})

	t.Run("Sets HTTP Basic Auth from configuration", func(t *testing.T) {
		u, _ := url.Parse("http://example.com")
		tp, _ := New(Config{URLs: []*url.URL{u}, Username: "foo", Password: "bar"})

		req, _ := http.NewRequest("GET", "/", nil)
		tp.setReqAuth(u, req)

		username, password, ok := req.BasicAuth()
		if !ok {
			t.Errorf("Expected the request to have Basic Auth set")
		}

		if username != "foo" || password != "bar" {
			t.Errorf("Unexpected values for username and password: %s:%s", username, password)
		}
	})

	t.Run("Sets APIKey Authentication from configuration", func(t *testing.T) {
		u, _ := url.Parse("http://example.com")
		tp, _ := New(Config{URLs: []*url.URL{u}, APIKey: "Zm9vYmFy"}) // foobar

		req, _ := http.NewRequest("GET", "/", nil)
		tp.setReqAuth(u, req)

		value := req.Header.Get("Authorization")
		if value == "" {
			t.Errorf("Expected the request to have the Authorization header set")
		}

		if value != "APIKey Zm9vYmFy" {
			t.Errorf(`Unexpected value for Authorization: want="APIKey Zm9vYmFy", got="%s"`, value)
		}
	})

	t.Run("Sets APIKey Authentication over ServiceToken", func(t *testing.T) {
		u, _ := url.Parse("http://example.com")
		tp, _ := New(Config{URLs: []*url.URL{u}, APIKey: "Zm9vYmFy", ServiceToken: "AAEAAWVs"}) // foobar

		req, _ := http.NewRequest("GET", "/", nil)
		tp.setReqAuth(u, req)

		value := req.Header.Get("Authorization")
		if value == "" {
			t.Errorf("Expected the request to have the Authorization header set")
		}

		if value != "APIKey Zm9vYmFy" {
			t.Errorf(`Unexpected value for Authorization: want="APIKey Zm9vYmFy", got="%s"`, value)
		}
	})

	t.Run("Sets ServiceToken Authentication from configuration", func(t *testing.T) {
		u, _ := url.Parse("http://example.com")
		tp, _ := New(Config{URLs: []*url.URL{u}, ServiceToken: "AAEAAWVs"})

		req, _ := http.NewRequest("GET", "/", nil)
		tp.setReqAuth(u, req)

		value := req.Header.Get("Authorization")
		if value == "" {
			t.Errorf("Expected the request to have the Authorization header set")
		}

		if value != "Bearer AAEAAWVs" {
			t.Errorf(`Unexpected value for Authorization: want="Bearer AAEAAWVs", got="%s"`, value)
		}
	})

	t.Run("Sets UserAgent", func(t *testing.T) {
		u, _ := url.Parse("http://example.com")
		tp, _ := New(Config{UserAgent: "elastic-transport-go", URLs: []*url.URL{u}})

		req, _ := http.NewRequest("GET", "/abc", nil)
		tp.setReqUserAgent(req)

		if !strings.HasPrefix(req.UserAgent(), "elastic-transport-go") {
			t.Errorf("Unexpected user agent: %s", req.UserAgent())
		}
	})

	t.Run("Overwrites UserAgent", func(t *testing.T) {
		u, _ := url.Parse("http://example.com")

		tp, _ := New(Config{URLs: []*url.URL{u}, Header: http.Header{
			userAgentHeader: []string{"Elastic-Fleet-Server/7.11.1 (darwin; amd64; Go 1.16.6)"},
		}})

		req, _ := http.NewRequest("GET", "/abc", nil)
		tp.setReqUserAgent(req)

		if !strings.HasPrefix(req.UserAgent(), "Elastic-Fleet-Server") {
			t.Errorf("Unexpected user agent: %s", req.UserAgent())
		}
	})

	t.Run("Sets global HTTP request headers", func(t *testing.T) {
		hdr := http.Header{}
		hdr.Set("X-Foo", "bar")

		tp, _ := New(Config{Header: hdr})

		{
			// Set the global HTTP header
			req, _ := http.NewRequest("GET", "/abc", nil)
			tp.setReqGlobalHeader(req)

			if req.Header.Get("X-Foo") != "bar" {
				t.Errorf("Unexpected global HTTP request header value: %s", req.Header.Get("X-Foo"))
			}
		}

		{
			// Do NOT overwrite an existing request header
			req, _ := http.NewRequest("GET", "/abc", nil)
			req.Header.Set("X-Foo", "baz")
			tp.setReqGlobalHeader(req)

			if req.Header.Get("X-Foo") != "baz" {
				t.Errorf("Unexpected global HTTP request header value: %s", req.Header.Get("X-Foo"))
			}
			if n := len(req.Header.Values("X-Foo")); n != 1 {
				t.Errorf("Expected 1 header value, got %d", n)
			}
		}
	})

	t.Run("Error No URL", func(t *testing.T) {
		tp, _ := New(Config{
			URLs: []*url.URL{},
			Transport: &mockTransp{
				RoundTripFunc: func(req *http.Request) (*http.Response, error) { return &http.Response{Status: "MOCK"}, nil },
			}})

		req, _ := http.NewRequest("GET", "/abc", nil)

		_, err := tp.Perform(req)
		if err.Error() != `cannot get connection: no connection available` {
			t.Fatalf("Expected error `cannot get URL`: but got error %q", err)
		}
	})
}

func TestTransportPerformRetries(t *testing.T) {
	t.Run("Retry request on network error and return the response", func(t *testing.T) {
		var (
			i       int
			numReqs = 2
		)

		u, _ := url.Parse("http://foo.bar")
		tp, _ := New(Config{
			URLs: []*url.URL{u, u, u},
			Transport: &mockTransp{
				RoundTripFunc: func(req *http.Request) (*http.Response, error) {
					i++
					fmt.Printf("Request #%d", i)
					if i == numReqs {
						fmt.Print(": OK\n")
						return &http.Response{Status: "OK"}, nil
					}
					fmt.Print(": ERR\n")
					return nil, &mockNetError{error: fmt.Errorf("Mock network error (%d)", i)}
				},
			}})

		req, _ := http.NewRequest("GET", "/abc", nil)

		res, err := tp.Perform(req)

		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		if res.Status != "OK" {
			t.Errorf("Unexpected response: %+v", res)
		}

		if i != numReqs {
			t.Errorf("Unexpected number of requests, want=%d, got=%d", numReqs, i)
		}
	})

	t.Run("Retry request on EOF error and return the response", func(t *testing.T) {
		var (
			i       int
			numReqs = 2
		)

		u, _ := url.Parse("http://foo.bar")
		tp, _ := New(Config{
			URLs: []*url.URL{u, u, u},
			Transport: &mockTransp{
				RoundTripFunc: func(req *http.Request) (*http.Response, error) {
					i++
					fmt.Printf("Request #%d", i)
					if i == numReqs {
						fmt.Print(": OK\n")
						return &http.Response{Status: "OK"}, nil
					}
					fmt.Print(": ERR\n")
					return nil, io.EOF
				},
			}})

		req, _ := http.NewRequest("GET", "/abc", nil)

		res, err := tp.Perform(req)

		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		if res.Status != "OK" {
			t.Errorf("Unexpected response: %+v", res)
		}

		if i != numReqs {
			t.Errorf("Unexpected number of requests, want=%d, got=%d", numReqs, i)
		}
	})

	t.Run("Retry request on 5xx response and return new response", func(t *testing.T) {
		var (
			i       int
			numReqs = 2
		)

		u, _ := url.Parse("http://foo.bar")
		tp, _ := New(Config{
			URLs: []*url.URL{u, u, u},
			Transport: &mockTransp{
				RoundTripFunc: func(req *http.Request) (*http.Response, error) {
					i++
					fmt.Printf("Request #%d", i)
					if i == numReqs {
						fmt.Print(": 200\n")
						return &http.Response{StatusCode: 200}, nil
					}
					fmt.Print(": 502\n")
					return &http.Response{StatusCode: 502}, nil
				},
			}})

		req, _ := http.NewRequest("GET", "/abc", nil)

		res, err := tp.Perform(req)

		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		if res.StatusCode != 200 {
			t.Errorf("Unexpected response: %+v", res)
		}

		if i != numReqs {
			t.Errorf("Unexpected number of requests, want=%d, got=%d", numReqs, i)
		}
	})

	t.Run("Close response body for a 5xx response", func(t *testing.T) {
		var (
			i       int
			numReqs = 5
		)

		u, _ := url.Parse("http://foo.bar")
		tp, _ := New(Config{
			URLs:       []*url.URL{u, u, u},
			MaxRetries: numReqs,
			Transport: &mockTransp{
				RoundTripFunc: func(req *http.Request) (*http.Response, error) {
					i++
					fmt.Printf("Request #%d", i)
					fmt.Print(": 502\n")
					body := io.NopCloser(strings.NewReader(`MOCK`))
					return &http.Response{StatusCode: 502, Body: body}, nil
				},
			}})

		req, _ := http.NewRequest("GET", "/", nil)

		res, err := tp.Perform(req)

		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		if i != numReqs+1 {
			t.Errorf("Unexpected number of requests, want=%d, got=%d", numReqs, i)
		}

		if res.StatusCode != 502 {
			t.Errorf("Unexpected response: %+v", res)
		}

		resBody, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()

		if string(resBody) != "MOCK" {
			t.Errorf("Unexpected body, want=MOCK, got=%s", resBody)
		}
	})

	t.Run("Retry request and return error when max retries exhausted", func(t *testing.T) {
		var (
			i       int
			numReqs = 3
		)

		u, _ := url.Parse("http://foo.bar")
		tp, _ := New(Config{
			URLs: []*url.URL{u, u, u},
			Transport: &mockTransp{
				RoundTripFunc: func(req *http.Request) (*http.Response, error) {
					i++
					fmt.Printf("Request #%d", i)
					fmt.Print(": ERR\n")
					return nil, &mockNetError{error: fmt.Errorf("mock network error (%d)", i)}
				},
			}})

		req, _ := http.NewRequest("GET", "/abc", nil)

		res, err := tp.Perform(req)

		if err == nil {
			t.Fatalf("Expected error, got: %v", err)
		}

		if res != nil {
			t.Errorf("Unexpected response: %+v", res)
		}

		// Should be initial HTTP request + 3 retries
		if i != numReqs+1 {
			t.Errorf("Unexpected number of requests, want=%d, got=%d", numReqs, i)
		}
	})

	t.Run("Reset request body during retry", func(t *testing.T) {
		var bodies []string
		esURL := "https://foo.com/bar"
		endpoint := "/abc"

		u, _ := url.Parse(esURL)
		tp, _ := New(Config{
			URLs: []*url.URL{u},
			Transport: &mockTransp{
				RoundTripFunc: func(req *http.Request) (*http.Response, error) {
					body, err := io.ReadAll(req.Body)
					if err != nil {
						panic(err)
					}
					expectedURL := strings.Join([]string{esURL, endpoint}, "")
					if !strings.EqualFold(req.URL.String(), expectedURL) {
						t.Fatalf("expected request url to be %s, got: %s", expectedURL, req.URL.String())
					}
					bodies = append(bodies, string(body))
					return &http.Response{Status: "MOCK", StatusCode: 502}, nil
				},
			}},
		)

		req, _ := http.NewRequest("POST", endpoint, strings.NewReader("FOOBAR"))
		res, err := tp.Perform(req)
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}
		_ = res

		if n := len(bodies); n != 4 {
			t.Fatalf("expected 4 requests, got %d", n)
		}
		for i, body := range bodies {
			if body != "FOOBAR" {
				t.Fatalf("request %d body: expected %q, got %q", i, "FOOBAR", body)
			}
		}
	})

	t.Run("Retry request on regular error", func(t *testing.T) {
		var i int

		u, _ := url.Parse("http://foo.bar")
		tp, _ := New(Config{
			URLs: []*url.URL{u, u, u},
			Transport: &mockTransp{
				RoundTripFunc: func(req *http.Request) (*http.Response, error) {
					i++
					fmt.Printf("Request #%d", i)
					fmt.Print(": ERR\n")
					return nil, fmt.Errorf("Mock regular error (%d)", i)
				},
			}})

		req, _ := http.NewRequest("GET", "/abc", nil)

		res, err := tp.Perform(req)

		if err == nil {
			t.Fatalf("Expected error, got: %v", err)
		}

		if res != nil {
			t.Errorf("Unexpected response: %+v", res)
		}

		if i != 4 {
			t.Errorf("Unexpected number of requests, want=%d, got=%d", 4, i)
		}
	})

	t.Run("Don't retry request when retries are disabled", func(t *testing.T) {
		var i int

		u, _ := url.Parse("http://foo.bar")
		tp, _ := New(Config{
			URLs: []*url.URL{u, u, u},
			Transport: &mockTransp{
				RoundTripFunc: func(req *http.Request) (*http.Response, error) {
					i++
					fmt.Printf("Request #%d", i)
					fmt.Print(": ERR\n")
					return nil, &mockNetError{error: fmt.Errorf("mock network error (%d)", i)}
				},
			},
			DisableRetry: true,
		})

		req, _ := http.NewRequest("GET", "/abc", nil)
		_, _ = tp.Perform(req)

		if i != 1 {
			t.Errorf("Unexpected number of requests, want=%d, got=%d", 1, i)
		}
	})

	t.Run("Delay the retry with a backoff function", func(t *testing.T) {
		var (
			i                int
			numReqs          = 4
			start            = time.Now()
			expectedDuration = time.Duration((numReqs-1)*100) * time.Millisecond
		)

		u, _ := url.Parse("http://foo.bar")
		tp, _ := New(Config{
			MaxRetries: numReqs,
			URLs:       []*url.URL{u, u, u},
			Transport: &mockTransp{
				RoundTripFunc: func(req *http.Request) (*http.Response, error) {
					i++
					fmt.Printf("Request #%d", i)
					if i == numReqs {
						fmt.Print(": OK\n")
						return &http.Response{Status: "OK"}, nil
					}
					fmt.Print(": ERR\n")
					return nil, &mockNetError{error: fmt.Errorf("Mock network error (%d)", i)}
				},
			},

			// A simple incremental backoff function
			//
			RetryBackoff: func(i int) time.Duration {
				d := time.Duration(i) * 100 * time.Millisecond
				fmt.Printf("Attempt: %d | Sleeping for %s...\n", i, d)
				return d
			},
		})

		req, _ := http.NewRequest("GET", "/abc", nil)

		res, err := tp.Perform(req)
		end := time.Since(start)

		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		if res.Status != "OK" {
			t.Errorf("Unexpected response: %+v", res)
		}

		if i != numReqs {
			t.Errorf("Unexpected number of requests, want=%d, got=%d", numReqs, i)
		}

		if end < expectedDuration {
			t.Errorf("Unexpected duration, want=>%s, got=%s", expectedDuration, end)
		}
	})

	t.Run("Delay the retry with retry on timeout and context deadline", func(t *testing.T) {
		var i int
		u, _ := url.Parse("http://foo.bar")
		tp, _ := New(Config{
			MaxRetries:   100,
			RetryBackoff: func(i int) time.Duration { return time.Hour },
			URLs:         []*url.URL{u},
			Transport: &mockTransp{
				RoundTripFunc: func(req *http.Request) (*http.Response, error) {
					i++
					<-req.Context().Done()
					return nil, req.Context().Err()
				},
			},
		})

		req, _ := http.NewRequest("GET", "/abc", nil)
		ctx, cancel := context.WithTimeout(req.Context(), 50*time.Millisecond)
		defer cancel()
		req = req.WithContext(ctx)

		_, err := tp.Perform(req)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected context.DeadlineExceeded, got %s", err)
		}
		if i != 1 {
			t.Fatalf("unexpected number of requests: expected 1, got got %d", i)
		}
	})

	t.Run("Retry request on next node on TLS failure", func(t *testing.T) {
		var i int

		u, _ := url.Parse("http://foo.bar")
		tp, _ := New(Config{
			URLs: []*url.URL{u, u, u},
			Transport: &mockTransp{
				RoundTripFunc: func(req *http.Request) (*http.Response, error) {
					i++
					if i == 3 {
						return nil, x509.CertificateInvalidError{Reason: x509.Expired}
					}

					fmt.Printf("Request #%d", i)
					fmt.Print(": ERR\n")
					return nil, fmt.Errorf("mock regular error (%d)", i)
				},
			}})

		req, _ := http.NewRequest("GET", "/abc", nil)

		res, err := tp.Perform(req)

		if err == nil {
			t.Fatalf("Expected error, got: %v", err)
		}

		if res != nil {
			t.Errorf("Unexpected response: %+v", res)
		}

		if i != 4 {
			t.Errorf("Unexpected number of requests, want=%d, got=%d", 4, i)
		}
	})
	t.Run("Do not retry on EOF", func(t *testing.T) {
		var i int

		u, _ := url.Parse("http://foo.bar")
		tp, _ := New(Config{
			URLs: []*url.URL{u, u, u},
			Transport: &mockTransp{
				RoundTripFunc: func(req *http.Request) (*http.Response, error) {
					switch i {
					case 0:
						i++
						return nil, io.EOF
					case 1:
						i++
						return nil, errors.New("simple custom error")
					case 2:
						i++
						return &http.Response{Status: "200 OK", StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("{}"))}, nil
					}

					return nil, nil
				},
			},
			RetryOnError: func(request *http.Request, err error) bool {
				return !errors.Is(err, io.EOF)
			},
		})

		req, _ := http.NewRequest("GET", "/", nil)
		_, err := tp.Perform(req)
		if err == nil {
			t.Fatalf("Expected error, %s", err)
		}

		res, err := tp.Perform(req)
		if err != nil {
			t.Fatalf("Expected error, %s", err)
		}

		if i != 3 {
			t.Fatalf("Unexpected req count, wanted 3, got %d", i)
		}
		if res.StatusCode != http.StatusOK {
			t.Fatalf("Unexpected status code, wanted 200, got %d", res.StatusCode)
		}
	})
}

func TestURLs(t *testing.T) {
	t.Run("Returns URLs", func(t *testing.T) {
		tp, _ := New(Config{URLs: []*url.URL{
			{Scheme: "http", Host: "localhost:9200"},
			{Scheme: "http", Host: "localhost:9201"},
		}})
		urls := tp.URLs()
		if len(urls) != 2 {
			t.Errorf("Expected get 2 urls, but got: %d", len(urls))
		}
		if urls[0].Host != "localhost:9200" {
			t.Errorf("Unexpected URL, want=localhost:9200, got=%s", urls[0].Host)
		}
	})
}

func TestMaxRetries(t *testing.T) {
	tests := []struct {
		name              string
		maxRetries        int
		disableRetry      bool
		expectedCallCount int
	}{
		{
			name:              "MaxRetries Active set to default",
			disableRetry:      false,
			expectedCallCount: 4,
		},
		{
			name:              "MaxRetries Active set to 1",
			maxRetries:        1,
			disableRetry:      false,
			expectedCallCount: 2,
		},
		{
			name:              "Max Retries Active set to 2",
			maxRetries:        2,
			disableRetry:      false,
			expectedCallCount: 3,
		},
		{
			name:              "Max Retries Active set to 3",
			maxRetries:        3,
			disableRetry:      false,
			expectedCallCount: 4,
		},
		{
			name:              "MaxRetries Inactive set to 0",
			maxRetries:        0,
			disableRetry:      true,
			expectedCallCount: 1,
		},
		{
			name:              "MaxRetries Inactive set to 3",
			maxRetries:        3,
			disableRetry:      true,
			expectedCallCount: 1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var callCount int
			c, _ := New(Config{
				URLs: []*url.URL{{}},
				Transport: &mockTransp{
					RoundTripFunc: func(req *http.Request) (*http.Response, error) {
						callCount++
						return &http.Response{
							StatusCode: http.StatusBadGateway,
							Status:     "MOCK",
						}, nil
					},
				},
				MaxRetries:   test.maxRetries,
				DisableRetry: test.disableRetry,
			})

			_, _ = c.Perform(&http.Request{URL: &url.URL{}, Header: make(http.Header)}) // errcheck ignore

			if test.expectedCallCount != callCount {
				t.Errorf("Bad retry call count, got : %d, want : %d", callCount, test.expectedCallCount)
			}
		})
	}
}

func TestRequestCompression(t *testing.T) {

	tests := []struct {
		name             string
		compressionFlag  bool
		compressionLevel int
		poolCompressor   bool
		inputBody        string
	}{
		{
			name:            "Uncompressed",
			compressionFlag: false,
			inputBody:       "elasticsearch",
		},
		{
			name:            "CompressedDefault",
			compressionFlag: true,
			inputBody:       "elasticsearch",
		},
		{
			name:             "CompressedBestSpeed",
			compressionFlag:  true,
			compressionLevel: gzip.BestSpeed,
			inputBody:        "elasticsearch",
		},
		{
			name:            "CompressedDefaultPooled",
			compressionFlag: true,
			poolCompressor:  true,
			inputBody:       "elasticsearch",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tp, _ := New(Config{
				URLs:                     []*url.URL{{}},
				CompressRequestBody:      test.compressionFlag,
				CompressRequestBodyLevel: test.compressionLevel,
				PoolCompressor:           test.poolCompressor,
				Transport: &mockTransp{
					RoundTripFunc: func(req *http.Request) (*http.Response, error) {
						if req.Body == nil || req.Body == http.NoBody {
							return nil, fmt.Errorf("unexpected body: %v", req.Body)
						}

						var buf bytes.Buffer
						_, _ = buf.ReadFrom(req.Body)

						if req.ContentLength != int64(buf.Len()) {
							return nil, fmt.Errorf("mismatched Content-Length: %d vs actual %d", req.ContentLength, buf.Len())
						}

						if test.compressionFlag {
							var unBuf bytes.Buffer
							zr, err := gzip.NewReader(&buf)
							if err != nil {
								return nil, fmt.Errorf("decompression error: %v", err)
							}
							_, _ = unBuf.ReadFrom(zr)
							buf = unBuf
						}

						if buf.String() != test.inputBody {
							return nil, fmt.Errorf("unexpected body: %s", buf.String())
						}

						return &http.Response{Status: "MOCK"}, nil
					},
				},
			})

			req, _ := http.NewRequest("POST", "/abc", bytes.NewBufferString(test.inputBody))

			res, err := tp.Perform(req)
			if err != nil {
				t.Fatalf("Unexpected error: %s", err)
			}

			if res.Status != "MOCK" {
				t.Errorf("Unexpected response: %+v", res)
			}
		})
	}
}

func TestInterceptors(t *testing.T) {
	mockInterceptor := func(next RoundTripFunc) RoundTripFunc {
		return func(req *http.Request) (*http.Response, error) {
			req.Header.Add("X-Intercept", "true")
			return next(req)
		}
	}

	tests := []struct {
		name         string
		interceptors []InterceptorFunc
		transportErr error
		assertFunc   func(t *testing.T, req *http.Request, resp *http.Response, err error)
	}{
		{
			name: "No Interceptors",
			assertFunc: func(t *testing.T, req *http.Request, resp *http.Response, err error) {
				if err != nil {
					t.Fatalf("Unexpected error: %s", err)
				}

				if resp.Status != "MOCK" {
					t.Errorf("Unexpected response: %+v", resp)
				}
			},
		},
		{
			name:         "One Interceptor",
			interceptors: []InterceptorFunc{mockInterceptor},
			assertFunc: func(t *testing.T, req *http.Request, resp *http.Response, err error) {
				if err != nil {
					t.Fatalf("Unexpected error: %s", err)
				}
				if resp.Status != "MOCK" {
					t.Errorf("Unexpected response: %+v", resp)
				}
				if req.Header.Get("X-Intercept") != "true" {
					t.Errorf("Unexpected X-Intercept header: %+v", req.Header.Get("X-Intercept"))
				}
			},
		},
		{
			name:         "One Interceptor Error",
			interceptors: []InterceptorFunc{mockInterceptor},
			transportErr: fmt.Errorf("error"),
			assertFunc: func(t *testing.T, req *http.Request, resp *http.Response, err error) {
				if err == nil {
					t.Fatal("Unexpected success")
				}
				if resp.Status != "MOCK" {
					t.Errorf("Unexpected response: %+v", resp)
				}
				if req.Header.Get("X-Intercept") != "true" {
					t.Errorf("Unexpected X-Intercept header: %+v", req.Header.Get("X-Intercept"))
				}
			},
		},
		{
			name: "Multiple Interceptors",
			interceptors: []InterceptorFunc{
				mockInterceptor,
				mockInterceptor,
				mockInterceptor,
			},
			assertFunc: func(t *testing.T, req *http.Request, resp *http.Response, err error) {
				if err != nil {
					t.Fatalf("Unexpected error: %s", err)
				}
				if resp.Status != "MOCK" {
					t.Errorf("Unexpected response: %+v", resp)
				}
				if len(req.Header.Values("X-Intercept")) != 3 {
					t.Errorf("Unexpected X-Intercept header: %+v", req.Header.Values("X-Intercept"))
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			u, _ := url.Parse("https://foo.com/bar")
			tp, _ := New(Config{
				URLs:         []*url.URL{u},
				Interceptors: test.interceptors,
				Transport: &mockTransp{
					RoundTripFunc: func(req *http.Request) (*http.Response, error) {
						return &http.Response{Status: "MOCK"}, test.transportErr
					},
				}})

			req, _ := http.NewRequest("GET", "/abc", nil)

			res, err := tp.Perform(req)
			test.assertFunc(t, req, res, err)
		})
	}
}

func TestClose(t *testing.T) {
	t.Run("Close", func(t *testing.T) {
		u, _ := url.Parse("http://foo.bar")
		tp, _ := New(Config{
			URLs: []*url.URL{u},
			Transport: &mockTransp{
				RoundTripFunc: func(req *http.Request) (*http.Response, error) {
					return &http.Response{Status: "MOCK"}, nil
				},
			}})
		if err := tp.Close(context.Background()); err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}
		// Closing a second time returns ErrAlreadyClosed.
		if err := tp.Close(context.Background()); err == nil {
			t.Fatal("Expected ErrAlreadyClosed")
		} else if !errors.Is(err, ErrAlreadyClosed) {
			t.Fatalf("Unexpected error: %s", err)
		}
	})

	t.Run("Close with nil context", func(t *testing.T) {
		u, _ := url.Parse("http://foo.bar")
		tp, _ := New(Config{
			URLs: []*url.URL{u},
			Transport: &mockTransp{
				RoundTripFunc: func(req *http.Request) (*http.Response, error) {
					return nil, fmt.Errorf("error")
				},
			}})
		if err := tp.Close(nil); err != nil { //nolint:staticcheck
			t.Fatalf("Unexpected error: %s", err)
		}
	})

	t.Run("Close should timeout", func(t *testing.T) {
		u, _ := url.Parse("http://foo.bar")
		tp, _ := New(Config{
			URLs: []*url.URL{u},
			Transport: &mockTransp{
				RoundTripFunc: func(req *http.Request) (*http.Response, error) {
					return &http.Response{Status: "MOCK"}, nil
				},
			}})

		tp.discoverWaitGroup.Add(1)

		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
		defer cancel()
		if err := tp.Close(ctx); err == nil {
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("Expected context.DeadlineExceeded, got %s", err)
			}
		}
	})

	t.Run("Close cancels future scheduleDiscoverNodes", func(t *testing.T) {
		u, _ := url.Parse("http://foo.bar")
		tp, _ := New(Config{
			URLs: []*url.URL{u},
			Transport: &mockTransp{
				RoundTripFunc: func(req *http.Request) (*http.Response, error) {
					<-req.Context().Done()
					return nil, req.Context().Err()
				},
			}})

		tp.scheduleDiscoverNodes(1 * time.Hour)
		if err := tp.Close(context.Background()); err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		tp.discoverWaitGroup.Wait()
	})

	t.Run("Close cancels running scheduleDiscoverNodes", func(t *testing.T) {
		u, _ := url.Parse("http://foo.bar")
		running := make(chan struct{})
		tp, _ := New(Config{
			URLs: []*url.URL{u},
			Transport: &mockTransp{
				RoundTripFunc: func(req *http.Request) (*http.Response, error) {
					close(running)
					<-req.Context().Done()
					return nil, req.Context().Err()
				},
			}})

		tp.scheduleDiscoverNodes(10 * time.Millisecond)
		<-running
		if err := tp.Close(context.Background()); err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		tp.discoverWaitGroup.Wait()
	})

	t.Run("Close cancels running scheduleDiscoverNodes unresponsive", func(t *testing.T) {
		u, _ := url.Parse("http://foo.bar")
		running := make(chan struct{})
		continueRunning := make(chan struct{})
		tp, _ := New(Config{
			URLs: []*url.URL{u},
			Transport: &mockTransp{
				RoundTripFunc: func(req *http.Request) (*http.Response, error) {
					close(running)
					<-continueRunning
					<-req.Context().Done()
					return nil, req.Context().Err()
				},
			}})

		tp.scheduleDiscoverNodes(10 * time.Millisecond)
		<-running

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()
		if err := tp.Close(ctx); err == nil {
			t.Fatalf("Expected error, got %s", err)
		} else if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Expected context.DeadlineExceeded, got %s", err)
		}

		close(continueRunning)
		tp.discoverWaitGroup.Wait()
	})

	t.Run("Perform should return error if transport is closed", func(t *testing.T) {
		u, _ := url.Parse("http://foo.bar")
		tp, _ := New(Config{
			URLs: []*url.URL{u},
			Transport: &mockTransp{
				RoundTripFunc: func(req *http.Request) (*http.Response, error) {
					return nil, fmt.Errorf("error")
				},
			}})
		_ = tp.Close(context.Background())
		req, _ := http.NewRequest("GET", "/abc", nil)

		_, err := tp.Perform(req)
		if err == nil {
			t.Fatalf("Expected error, got nil")
		} else if !errors.Is(err, ErrClosed) {
			t.Fatalf("Expected ErrClosed, got %s", err)
		}
	})

	t.Run("Close should close CloseableConnectionPool", func(t *testing.T) {
		u, _ := url.Parse("http://foo.bar")
		calledCloseConnectionPool := false
		tp, _ := New(Config{
			URLs: []*url.URL{u},
			Transport: &mockTransp{
				RoundTripFunc: func(req *http.Request) (*http.Response, error) {
					return nil, fmt.Errorf("error")
				},
			},
			ConnectionPoolFunc: func(connections []*Connection, selector Selector) ConnectionPool {
				return &mockConnectionPool{func(_ context.Context) error {
					calledCloseConnectionPool = true
					return nil
				}}
			},
		})
		err := tp.Close(context.Background())
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}
		if !calledCloseConnectionPool {
			t.Fatalf("Expected Close to be called on CloseableConnectionPool")
		}
	})

	t.Run("Close should error if CloseableConnectionPool fails to close", func(t *testing.T) {
		u, _ := url.Parse("http://foo.bar")
		tp, _ := New(Config{
			URLs: []*url.URL{u},
			Transport: &mockTransp{
				RoundTripFunc: func(req *http.Request) (*http.Response, error) {
					return nil, fmt.Errorf("error")
				},
			},
			ConnectionPoolFunc: func(connections []*Connection, selector Selector) ConnectionPool {
				return &mockConnectionPool{func(_ context.Context) error {
					return fmt.Errorf("mock error")
				}}
			},
		})
		err := tp.Close(context.Background())
		if err == nil {
			t.Fatalf("Expected error, got nil")
		} else if !strings.Contains(err.Error(), "mock error") {
			t.Fatalf("Expected mock error, got %s", err)
		}
	})
}

func TestDrainErrChan(t *testing.T) {
	t.Run("closed empty channel returns nil", func(t *testing.T) {
		ch := make(chan error, 1)
		close(ch)
		if err := drainErrChan(ch); err != nil {
			t.Fatalf("Expected nil, got %s", err)
		}
	})

	t.Run("closed channel with one error", func(t *testing.T) {
		ch := make(chan error, 1)
		ch <- fmt.Errorf("pool error")
		close(ch)
		err := drainErrChan(ch)
		if err == nil {
			t.Fatal("Expected error, got nil")
		}
		if !strings.Contains(err.Error(), "pool error") {
			t.Fatalf("Expected 'pool error', got %s", err)
		}
	})

	t.Run("closed channel with multiple errors", func(t *testing.T) {
		ch := make(chan error, 3)
		ch <- fmt.Errorf("error one")
		ch <- fmt.Errorf("error two")
		ch <- fmt.Errorf("error three")
		close(ch)
		err := drainErrChan(ch)
		if err == nil {
			t.Fatal("Expected error, got nil")
		}
		for _, msg := range []string{"error one", "error two", "error three"} {
			if !strings.Contains(err.Error(), msg) {
				t.Fatalf("Expected error to contain %q, got %s", msg, err)
			}
		}
	})
}

func TestCloseTimeoutWithPoolError(t *testing.T) {
	t.Run("Close should return pool error on context timeout", func(t *testing.T) {
		u, _ := url.Parse("http://foo.bar")
		tp, _ := New(Config{
			URLs: []*url.URL{u},
			Transport: &mockTransp{
				RoundTripFunc: func(req *http.Request) (*http.Response, error) {
					return nil, fmt.Errorf("error")
				},
			},
			ConnectionPoolFunc: func(connections []*Connection, selector Selector) ConnectionPool {
				return &mockConnectionPool{func(ctx context.Context) error {
					return fmt.Errorf("pool close error")
				}}
			},
		})

		// Block the done channel so the ctx.Done() path is taken.
		tp.discoverWaitGroup.Add(1)
		defer tp.discoverWaitGroup.Done()

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		err := tp.Close(ctx)
		if err == nil {
			t.Fatal("Expected error, got nil")
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Expected context.DeadlineExceeded, got %s", err)
		}
		if !strings.Contains(err.Error(), "pool close error") {
			t.Fatalf("Expected error to contain 'pool close error', got %s", err)
		}
	})

	t.Run("Close should return only context error when pool has not errored yet", func(t *testing.T) {
		u, _ := url.Parse("http://foo.bar")
		blocker := make(chan struct{})
		tp, _ := New(Config{
			URLs: []*url.URL{u},
			Transport: &mockTransp{
				RoundTripFunc: func(req *http.Request) (*http.Response, error) {
					return nil, fmt.Errorf("error")
				},
			},
			ConnectionPoolFunc: func(connections []*Connection, selector Selector) ConnectionPool {
				return &mockConnectionPool{func(ctx context.Context) error {
					<-blocker
					return fmt.Errorf("late pool error")
				}}
			},
		})
		defer close(blocker)

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()
		err := tp.Close(ctx)
		if err == nil {
			t.Fatal("Expected error, got nil")
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Expected context.DeadlineExceeded, got %s", err)
		}
		if strings.Contains(err.Error(), "late pool error") {
			t.Fatal("Did not expect pool error to be included when pool hasn't returned yet")
		}
	})
}

type mockConnectionPool struct {
	CloseFunc func(context.Context) error
}

func (m *mockConnectionPool) Next() (*Connection, error) {
	return nil, nil
}

func (m *mockConnectionPool) OnSuccess(*Connection) error {
	return nil
}

func (m *mockConnectionPool) OnFailure(*Connection) error {
	return nil
}

func (m *mockConnectionPool) URLs() []*url.URL {
	return nil
}

func (m *mockConnectionPool) Close(ctx context.Context) error {
	return m.CloseFunc(ctx)
}

// ---------------------------------------------------------------------------
// NewClient (options API) variants of key tests above.
// ---------------------------------------------------------------------------

func TestNewClientTransport(t *testing.T) {
	t.Run("Interface", func(t *testing.T) {
		tp, _ := NewClient()
		var _ Interface = tp
		var _ = tp.transport
	})

	t.Run("Default", func(t *testing.T) {
		tp, _ := NewClient()
		if tp.transport == nil {
			t.Error("Expected the transport to not be nil")
		}
		if _, ok := tp.transport.(*http.Transport); !ok {
			t.Errorf("Expected the transport to be *http.Transport, got: %T", tp.transport)
		}
	})

	t.Run("Custom", func(t *testing.T) {
		tp, _ := NewClient(
			WithURLs(&url.URL{}),
			WithTransport(&mockTransp{
				RoundTripFunc: func(req *http.Request) (*http.Response, error) { return &http.Response{Status: "MOCK"}, nil },
			}),
		)

		res, err := tp.roundTrip(&http.Request{URL: &url.URL{}})
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}
		if res.Status != "MOCK" {
			t.Errorf("Unexpected response from transport: %+v", res)
		}
	})
}

func TestNewClientTransportConfig(t *testing.T) {
	t.Run("Defaults", func(t *testing.T) {
		tp, _ := NewClient()

		if !reflect.DeepEqual(tp.retryOnStatus, []int{502, 503, 504}) {
			t.Errorf("Unexpected retryOnStatus: %v", tp.retryOnStatus)
		}
		if tp.disableRetry {
			t.Errorf("Unexpected disableRetry: %v", tp.disableRetry)
		}
		if tp.maxRetries != 3 {
			t.Errorf("Unexpected maxRetries: %v", tp.maxRetries)
		}
		if tp.compressRequestBody {
			t.Errorf("Unexpected compressRequestBody: %v", tp.compressRequestBody)
		}
	})

	t.Run("Custom", func(t *testing.T) {
		tp, _ := NewClient(
			WithRetryOnStatus(404, 408),
			WithDisableRetry(),
			WithMaxRetries(5),
			WithCompression(),
			WithCertificateFingerprint("7A3A6031CD097DA0EE84D65137912A84576B50194045B41F4F4B8AC1A98116BE"),
		)

		if !reflect.DeepEqual(tp.retryOnStatus, []int{404, 408}) {
			t.Errorf("Unexpected retryOnStatus: %v", tp.retryOnStatus)
		}
		if !tp.disableRetry {
			t.Errorf("Unexpected disableRetry: %v", tp.disableRetry)
		}
		if tp.maxRetries != 5 {
			t.Errorf("Unexpected maxRetries: %v", tp.maxRetries)
		}
		if !tp.compressRequestBody {
			t.Errorf("Unexpected compressRequestBody: %v", tp.compressRequestBody)
		}
	})
}

func TestNewClientConnectionPool(t *testing.T) {
	t.Run("Single URL", func(t *testing.T) {
		tp, _ := NewClient(WithURLs(&url.URL{Scheme: "http", Host: "foo1"}))

		if _, ok := tp.pool.(*singleConnectionPool); !ok {
			t.Errorf("Expected singleConnectionPool, got: %T", tp.pool)
		}

		conn, err := tp.pool.Next()
		if err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		if conn.URL.String() != "http://foo1" {
			t.Errorf("Unexpected URL, want=http://foo1, got=%s", conn.URL)
		}
	})

	t.Run("Two URLs", func(t *testing.T) {
		tp, _ := NewClient(WithURLs(
			&url.URL{Scheme: "http", Host: "foo1"},
			&url.URL{Scheme: "http", Host: "foo2"},
		))

		if _, ok := tp.pool.(*statusConnectionPool); !ok {
			t.Errorf("Expected statusConnectionPool, got: %T", tp.pool)
		}

		conn, err := tp.pool.Next()
		if err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		if conn.URL.String() != "http://foo1" {
			t.Errorf("Unexpected URL, want=http://foo1, got=%s", conn.URL)
		}
	})
}

func TestNewClientPerform(t *testing.T) {
	t.Run("Executes", func(t *testing.T) {
		u, _ := url.Parse("https://foo.com/bar")
		tp, _ := NewClient(
			WithURLs(u),
			WithTransport(&mockTransp{
				RoundTripFunc: func(req *http.Request) (*http.Response, error) { return &http.Response{Status: "MOCK"}, nil },
			}),
		)

		req, _ := http.NewRequest("GET", "/abc", nil)
		res, err := tp.Perform(req)
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}
		if res.Status != "MOCK" {
			t.Errorf("Unexpected response: %+v", res)
		}
	})

	t.Run("Retries on 5xx", func(t *testing.T) {
		var i int
		u, _ := url.Parse("http://foo.bar")
		tp, _ := NewClient(
			WithURLs(u, u, u),
			WithTransport(&mockTransp{
				RoundTripFunc: func(req *http.Request) (*http.Response, error) {
					i++
					if i == 2 {
						return &http.Response{StatusCode: 200}, nil
					}
					return &http.Response{StatusCode: 502}, nil
				},
			}),
		)

		req, _ := http.NewRequest("GET", "/abc", nil)
		res, err := tp.Perform(req)
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}
		if res.StatusCode != 200 {
			t.Errorf("Unexpected status code: %d", res.StatusCode)
		}
		if i != 2 {
			t.Errorf("Unexpected number of requests, want=2, got=%d", i)
		}
	})

	t.Run("Disable retry", func(t *testing.T) {
		var i int
		u, _ := url.Parse("http://foo.bar")
		tp, _ := NewClient(
			WithURLs(u, u, u),
			WithDisableRetry(),
			WithTransport(&mockTransp{
				RoundTripFunc: func(req *http.Request) (*http.Response, error) {
					i++
					return nil, &mockNetError{error: fmt.Errorf("mock error (%d)", i)}
				},
			}),
		)

		req, _ := http.NewRequest("GET", "/abc", nil)
		_, _ = tp.Perform(req)

		if i != 1 {
			t.Errorf("Unexpected number of requests, want=1, got=%d", i)
		}
	})
}

func TestNewClientClose(t *testing.T) {
	t.Run("Close", func(t *testing.T) {
		u, _ := url.Parse("http://foo.bar")
		tp, _ := NewClient(
			WithURLs(u),
			WithTransport(&mockTransp{
				RoundTripFunc: func(req *http.Request) (*http.Response, error) {
					return &http.Response{Status: "MOCK"}, nil
				},
			}),
		)
		if err := tp.Close(context.Background()); err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}
		if err := tp.Close(context.Background()); err == nil {
			t.Fatal("Expected ErrAlreadyClosed")
		} else if !errors.Is(err, ErrAlreadyClosed) {
			t.Fatalf("Unexpected error: %s", err)
		}
	})

	t.Run("Perform after close returns ErrClosed", func(t *testing.T) {
		u, _ := url.Parse("http://foo.bar")
		tp, _ := NewClient(
			WithURLs(u),
			WithTransport(&mockTransp{
				RoundTripFunc: func(req *http.Request) (*http.Response, error) {
					return nil, fmt.Errorf("error")
				},
			}),
		)
		_ = tp.Close(context.Background())
		req, _ := http.NewRequest("GET", "/abc", nil)

		_, err := tp.Perform(req)
		if !errors.Is(err, ErrClosed) {
			t.Fatalf("Expected ErrClosed, got %s", err)
		}
	})
}

func TestNewClientInterceptors(t *testing.T) {
	mockInterceptor := func(next RoundTripFunc) RoundTripFunc {
		return func(req *http.Request) (*http.Response, error) {
			req.Header.Add("X-Intercept", "true")
			return next(req)
		}
	}

	t.Run("Multiple Interceptors", func(t *testing.T) {
		u, _ := url.Parse("https://foo.com/bar")
		tp, _ := NewClient(
			WithURLs(u),
			WithInterceptors(mockInterceptor, mockInterceptor, mockInterceptor),
			WithTransport(&mockTransp{
				RoundTripFunc: func(req *http.Request) (*http.Response, error) {
					return &http.Response{Status: "MOCK"}, nil
				},
			}),
		)

		req, _ := http.NewRequest("GET", "/abc", nil)
		res, err := tp.Perform(req)
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}
		if res.Status != "MOCK" {
			t.Errorf("Unexpected response: %+v", res)
		}
		if len(req.Header.Values("X-Intercept")) != 3 {
			t.Errorf("Expected 3 X-Intercept headers, got %d", len(req.Header.Values("X-Intercept")))
		}
	})
}
