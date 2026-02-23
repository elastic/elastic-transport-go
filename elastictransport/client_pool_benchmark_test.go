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
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
)

// benchTransport returns a minimal response without hitting the network.
// Each call returns a fresh *http.Response so parallel goroutines don't share
// mutable state.
type benchTransport struct{}

func (t *benchTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode:    200,
		Body:          io.NopCloser(strings.NewReader("")),
		ContentLength: 0,
		Header:        http.Header{},
	}, nil
}

// benchmarkPerform exercises Client.Perform at varying parallelism levels.
// Perform's hot path acquires the client-level pool lock for Next, OnSuccess,
// and OnFailure -- the lock that changed from Mutex to RWMutex in this branch.
func benchmarkPerform(b *testing.B, name string, client *Client) {
	b.Helper()
	b.Run(name, func(b *testing.B) {
		for _, p := range []int{1, 10, 100, 1000} {
			b.Run(fmt.Sprintf("goroutines-%d", p), func(b *testing.B) {
				b.SetParallelism(p)
				b.RunParallel(func(pb *testing.PB) {
					for pb.Next() {
						req, _ := http.NewRequest("GET", "/", nil)
						res, err := client.Perform(req)
						if err != nil {
							b.Fatalf("Perform: %s", err)
						}
						res.Body.Close()
					}
				})
			})
		}
	})
}

func BenchmarkClientPoolAccess(b *testing.B) {
	b.ReportAllocs()

	b.Run("SingleConnectionPool", func(b *testing.B) {
		u, _ := url.Parse("http://localhost:9200")
		tp, err := New(Config{
			URLs:         []*url.URL{u},
			Transport:    &benchTransport{},
			DisableRetry: true,
		})
		if err != nil {
			b.Fatal(err)
		}
		benchmarkPerform(b, "Perform", tp)
	})

	b.Run("StatusConnectionPool", func(b *testing.B) {
		var urls []*url.URL
		for i := 0; i < 10; i++ {
			u, _ := url.Parse(fmt.Sprintf("http://node%d:9200", i))
			urls = append(urls, u)
		}
		tp, err := New(Config{
			URLs:         urls,
			Transport:    &benchTransport{},
			DisableRetry: true,
		})
		if err != nil {
			b.Fatal(err)
		}
		benchmarkPerform(b, "Perform", tp)
	})

	b.Run("CustomPool/SynchronizedWrapper", func(b *testing.B) {
		var urls []*url.URL
		for i := 0; i < 10; i++ {
			u, _ := url.Parse(fmt.Sprintf("http://node%d:9200", i))
			urls = append(urls, u)
		}
		tp, err := New(Config{
			URLs: urls,
			ConnectionPoolFunc: func(conns []*Connection, sel Selector) ConnectionPool {
				return &benchCustomPool{conns: conns}
			},
			Transport:    &benchTransport{},
			DisableRetry: true,
		})
		if err != nil {
			b.Fatal(err)
		}
		benchmarkPerform(b, "Perform", tp)
	})

	b.Run("CustomPool/ConcurrentSafeOptIn", func(b *testing.B) {
		var urls []*url.URL
		for i := 0; i < 10; i++ {
			u, _ := url.Parse(fmt.Sprintf("http://node%d:9200", i))
			urls = append(urls, u)
		}
		tp, err := New(Config{
			URLs: urls,
			ConnectionPoolFunc: func(conns []*Connection, sel Selector) ConnectionPool {
				return &benchConcurrentSafeCustomPool{
					benchCustomPool: benchCustomPool{conns: conns},
				}
			},
			Transport:    &benchTransport{},
			DisableRetry: true,
		})
		if err != nil {
			b.Fatal(err)
		}
		benchmarkPerform(b, "Perform", tp)
	})
}

type benchCustomPool struct {
	mu    sync.Mutex
	conns []*Connection
	curr  int
}

func (p *benchCustomPool) Next() (*Connection, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	c := p.conns[p.curr%len(p.conns)]
	p.curr++
	return c, nil
}

func (p *benchCustomPool) OnSuccess(*Connection) error { return nil }
func (p *benchCustomPool) OnFailure(*Connection) error { return nil }
func (p *benchCustomPool) URLs() []*url.URL {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]*url.URL, len(p.conns))
	for i, c := range p.conns {
		out[i] = c.URL
	}
	return out
}

type benchConcurrentSafeCustomPool struct {
	benchCustomPool
}

func (p *benchConcurrentSafeCustomPool) ConcurrentSafe() {}
