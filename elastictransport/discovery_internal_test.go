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
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"sort"
	"testing"
	"time"
)

func TestDiscovery(t *testing.T) {
	defaultHandler := func(w http.ResponseWriter, r *http.Request) {
		f, err := os.Open("testdata/nodes.info.json")
		if err != nil {
			http.Error(w, fmt.Sprintf("Fixture error: %s", err), 500)
			return
		}
		if _, err := io.Copy(w, f); err != nil {
			http.Error(w, fmt.Sprintf("Fixture error: %s", err), 500)
		}
	}

	srv := &http.Server{Addr: "localhost:10001", Handler: http.HandlerFunc(defaultHandler)}
	srvTLS := &http.Server{Addr: "localhost:12001", Handler: http.HandlerFunc(defaultHandler)}

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Errorf("Unable to start server: %s", err)
			return
		}
	}()
	go func() {
		if err := srvTLS.ListenAndServeTLS("testdata/cert.pem", "testdata/key.pem"); err != nil && err != http.ErrServerClosed {
			t.Errorf("Unable to start server: %s", err)
			return
		}
	}()
	defer func() { _ = srv.Close() }()
	defer func() { _ = srvTLS.Close() }()

	time.Sleep(50 * time.Millisecond)

	assertNodeInfoSuccess := func(t *testing.T, nodes []nodeInfo) {
		t.Helper()
		if len(nodes) != 3 {
			t.Errorf("Unexpected number of nodes, want=3, got=%d", len(nodes))
		}

		for _, node := range nodes {
			switch node.Name {
			case "es1":
				if node.URL.String() != "http://127.0.0.1:10001" {
					t.Errorf("Unexpected URL: %s", node.URL.String())
				}
			case "es2":
				if node.URL.String() != "http://localhost:10002" {
					t.Errorf("Unexpected URL: %s", node.URL.String())
				}
			case "es3":
				if node.URL.String() != "http://127.0.0.1:10003" {
					t.Errorf("Unexpected URL: %s", node.URL.String())
				}
			}
		}
	}

	t.Run("getNodesInfo()", func(t *testing.T) {
		u, _ := url.Parse("http://" + srv.Addr)
		tp, _ := New(Config{URLs: []*url.URL{u}})

		nodes, err := tp.getNodesInfo(context.Background())
		if err != nil {
			t.Fatalf("ERROR: %s", err)
		}
		fmt.Printf("NodesInfo: %+v\n", nodes)

		assertNodeInfoSuccess(t, nodes)
	})

	t.Run("getNodesInfo() with interceptor", func(t *testing.T) {
		u, _ := url.Parse("http://" + srv.Addr)
		interceptorCalled := false
		tp, _ := New(Config{URLs: []*url.URL{u}, Interceptors: []InterceptorFunc{
			func(next RoundTripFunc) RoundTripFunc {
				return func(request *http.Request) (*http.Response, error) {
					interceptorCalled = true
					return next(request)
				}
			},
		}})

		nodes, err := tp.getNodesInfo(context.Background())
		if err != nil {
			t.Fatalf("ERROR: %s", err)
		}
		fmt.Printf("NodesInfo: %+v\n", nodes)

		if interceptorCalled != true {
			t.Errorf("Interceptor was not called")
		}

		assertNodeInfoSuccess(t, nodes)
	})

	t.Run("DiscoverNodes()", func(t *testing.T) {
		u, _ := url.Parse("http://" + srv.Addr)
		tp, _ := New(Config{URLs: []*url.URL{u}})

		_ = tp.DiscoverNodesContext(context.Background())

		pool, ok := tp.pool.(*statusConnectionPool)
		if !ok {
			t.Fatalf("Unexpected pool, want=statusConnectionPool, got=%T", tp.pool)
		}

		if len(pool.live) != 2 {
			t.Errorf("Unexpected number of nodes, want=2, got=%d", len(pool.live))
		}

		for _, conn := range pool.live {
			switch conn.Name {
			case "es1":
				if conn.URL.String() != "http://127.0.0.1:10001" {
					t.Errorf("Unexpected URL: %s", conn.URL.String())
				}
			case "es2":
				if conn.URL.String() != "http://localhost:10002" {
					t.Errorf("Unexpected URL: %s", conn.URL.String())
				}
			default:
				t.Errorf("Unexpected node: %s", conn.Name)
			}
		}
	})

	t.Run("DiscoverNodes() with SSL and authorization", func(t *testing.T) {
		u, _ := url.Parse("https://" + srvTLS.Addr)
		tp, _ := New(Config{
			URLs:     []*url.URL{u},
			Username: "foo",
			Password: "bar",
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		})

		_ = tp.DiscoverNodesContext(context.Background())

		pool, ok := tp.pool.(*statusConnectionPool)
		if !ok {
			t.Fatalf("Unexpected pool, want=statusConnectionPool, got=%T", tp.pool)
		}

		if len(pool.live) != 2 {
			t.Errorf("Unexpected number of nodes, want=2, got=%d", len(pool.live))
		}

		for _, conn := range pool.live {
			switch conn.Name {
			case "es1":
				if conn.URL.String() != "https://127.0.0.1:10001" {
					t.Errorf("Unexpected URL: %s", conn.URL.String())
				}
			case "es2":
				if conn.URL.String() != "https://localhost:10002" {
					t.Errorf("Unexpected URL: %s", conn.URL.String())
				}
			default:
				t.Errorf("Unexpected node: %s", conn.Name)
			}
		}
	})

	t.Run("scheduleDiscoverNodes()", func(t *testing.T) {
		t.Skip("Skip") // TODO(karmi): Investigate the intermittent failures of this test

		var numURLs int
		u, _ := url.Parse("http://" + srv.Addr)

		tp, _ := New(Config{URLs: []*url.URL{u}, DiscoverNodesInterval: 10 * time.Millisecond})

		tp.Lock()
		numURLs = len(tp.pool.URLs())
		tp.Unlock()
		if numURLs != 1 {
			t.Errorf("Unexpected number of nodes, want=1, got=%d", numURLs)
		}

		time.Sleep(18 * time.Millisecond) // Wait until (*Client).scheduleDiscoverNodes()
		tp.Lock()
		numURLs = len(tp.pool.URLs())
		tp.Unlock()
		if numURLs != 2 {
			t.Errorf("Unexpected number of nodes, want=2, got=%d", numURLs)
		}
	})

	t.Run("Role based nodes discovery", func(t *testing.T) {
		type Node struct {
			URL   string
			Roles []string
		}

		type fields struct {
			Nodes map[string]Node
		}
		type wants struct {
			wantErr    bool
			wantsNConn int
		}
		tests := []struct {
			name string
			args fields
			want wants
		}{
			{
				"Default roles should allow every node to be selected",
				fields{
					Nodes: map[string]Node{
						"es1": {
							URL: "es1:9200",
							Roles: []string{
								"data",
								"data_cold",
								"data_content",
								"data_frozen",
								"data_hot",
								"data_warm",
								"ingest",
								"master",
								"ml",
								"remote_cluster_client",
								"transform",
							},
						},
						"es2": {
							URL: "es2:9200",
							Roles: []string{
								"data",
								"data_cold",
								"data_content",
								"data_frozen",
								"data_hot",
								"data_warm",
								"ingest",
								"master",
								"ml",
								"remote_cluster_client",
								"transform",
							},
						},
						"es3": {
							URL: "es3:9200",
							Roles: []string{
								"data",
								"data_cold",
								"data_content",
								"data_frozen",
								"data_hot",
								"data_warm",
								"ingest",
								"master",
								"ml",
								"remote_cluster_client",
								"transform",
							},
						},
					},
				},
				wants{
					false, 3,
				},
			},
			{
				"Master only node should not be selected",
				fields{
					Nodes: map[string]Node{
						"es1": {
							URL: "es1:9200",
							Roles: []string{
								"master",
							},
						},
						"es2": {
							URL: "es2:9200",
							Roles: []string{
								"data",
								"data_cold",
								"data_content",
								"data_frozen",
								"data_hot",
								"data_warm",
								"ingest",
								"master",
								"ml",
								"remote_cluster_client",
								"transform",
							},
						},
						"es3": {
							URL: "es3:9200",
							Roles: []string{
								"data",
								"data_cold",
								"data_content",
								"data_frozen",
								"data_hot",
								"data_warm",
								"ingest",
								"master",
								"ml",
								"remote_cluster_client",
								"transform",
							},
						},
					},
				},

				wants{
					false, 2,
				},
			},
			{
				"Master and data only nodes should be selected",
				fields{
					Nodes: map[string]Node{
						"es1": {
							URL: "es1:9200",
							Roles: []string{
								"data",
								"master",
							},
						},
						"es2": {
							URL: "es2:9200",
							Roles: []string{
								"data",
								"master",
							},
						},
					},
				},

				wants{
					false, 2,
				},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				type Http struct {
					PublishAddress string `json:"publish_address"`
				}
				type nodesInfo struct {
					Roles []string `json:"roles"`
					Http  Http     `json:"http"`
				}

				var urls []*url.URL
				for _, node := range tt.args.Nodes {
					u, _ := url.Parse(node.URL)
					urls = append(urls, u)

				}

				newRoundTripper := func() http.RoundTripper {
					return &mockTransp{
						RoundTripFunc: func(req *http.Request) (*http.Response, error) {
							nodes := make(map[string]map[string]nodesInfo)
							nodes["nodes"] = make(map[string]nodesInfo)
							for name, node := range tt.args.Nodes {
								u, _ := url.Parse(node.URL)
								nodes["nodes"][name] = nodesInfo{
									Roles: node.Roles,
									Http:  Http{PublishAddress: u.String()},
								}
							}

							b, _ := json.MarshalIndent(nodes, "", "    ")

							return &http.Response{
								Status:        "200 OK",
								StatusCode:    200,
								ContentLength: int64(len(b)),
								Header:        map[string][]string{"Content-Type": {"application/json"}},
								Body:          io.NopCloser(bytes.NewReader(b)),
							}, nil
						},
					}
				}

				c, _ := New(Config{
					URLs:      urls,
					Transport: newRoundTripper(),
				})
				_ = c.DiscoverNodesContext(context.Background())

				pool, ok := c.pool.(*statusConnectionPool)
				if !ok {
					t.Fatalf("Unexpected pool, want=statusConnectionPool, got=%T", c.pool)
				}

				if len(pool.live) != tt.want.wantsNConn {
					t.Errorf("Unexpected number of nodes, want=%d, got=%d", tt.want.wantsNConn, len(pool.live))
				}

				for _, conn := range pool.live {
					if !reflect.DeepEqual(tt.args.Nodes[conn.ID].Roles, conn.Roles) {
						t.Errorf("Unexpected roles for node %s, want=%s, got=%s", conn.Name, tt.args.Nodes[conn.ID], conn.Roles)
					}
				}

				if err := c.DiscoverNodesContext(context.Background()); (err != nil) != tt.want.wantErr {
					t.Errorf("DiscoverNodes() error = %v, wantErr %v", err, tt.want.wantErr)
				}
			})
		}
	})
}

func TestGetNodeURL_IPv6(t *testing.T) {
	c := &Client{}

	tests := []struct {
		name           string
		publishAddress string
		scheme         string
		wantHost       string
	}{
		{
			name:           "bare IPv6 loopback",
			publishAddress: "[::1]:9200",
			scheme:         "http",
			wantHost:       "[::1]:9200",
		},
		{
			name:           "bare IPv6 global address",
			publishAddress: "[2001:db8::1]:9200",
			scheme:         "http",
			wantHost:       "[2001:db8::1]:9200",
		},
		{
			name:           "hostname/IPv6 address",
			publishAddress: "ip6host/[2001:db8::1]:9200",
			scheme:         "https",
			wantHost:       "ip6host:9200",
		},
		{
			name:           "IPv6 with zone ID",
			publishAddress: "[fe80::1%25eth0]:9200",
			scheme:         "http",
			wantHost:       "[fe80::1%25eth0]:9200",
		},
		{
			name:           "IPv4 still works",
			publishAddress: "127.0.0.1:9200",
			scheme:         "http",
			wantHost:       "127.0.0.1:9200",
		},
		{
			name:           "hostname/IPv4 still works",
			publishAddress: "localhost/127.0.0.1:9200",
			scheme:         "http",
			wantHost:       "localhost:9200",
		},
		{
			name:           "bare hostname:port",
			publishAddress: "es-node1:9200",
			scheme:         "http",
			wantHost:       "es-node1:9200",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := nodeInfo{}
			node.HTTP.PublishAddress = tt.publishAddress

			u := c.getNodeURL(node, tt.scheme)

			if u.Host != tt.wantHost {
				t.Errorf("getNodeURL(%q).Host = %q, want %q", tt.publishAddress, u.Host, tt.wantHost)
			}
			if u.Scheme != tt.scheme {
				t.Errorf("getNodeURL(%q).Scheme = %q, want %q", tt.publishAddress, u.Scheme, tt.scheme)
			}
		})
	}
}

func TestDiscoveryIPv6(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		f, err := os.Open("testdata/nodes.info.ipv6.json")
		if err != nil {
			http.Error(w, fmt.Sprintf("Fixture error: %s", err), 500)
			return
		}
		defer func() { _ = f.Close() }()
		if _, err := io.Copy(w, f); err != nil {
			http.Error(w, fmt.Sprintf("Fixture error: %s", err), 500)
		}
	}

	srv := &http.Server{Addr: "localhost:10011", Handler: http.HandlerFunc(handler)}
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Errorf("Unable to start server: %s", err)
		}
	}()
	defer func() { _ = srv.Close() }()
	time.Sleep(50 * time.Millisecond)

	t.Run("getNodesInfo() with IPv6 publish_address", func(t *testing.T) {
		u, _ := url.Parse("http://" + srv.Addr)
		tp, _ := New(Config{URLs: []*url.URL{u}})

		nodes, err := tp.getNodesInfo(context.Background())
		if err != nil {
			t.Fatalf("ERROR: %s", err)
		}

		if len(nodes) != 3 {
			t.Fatalf("Unexpected number of nodes, want=3, got=%d", len(nodes))
		}

		for _, node := range nodes {
			switch node.Name {
			case "es1":
				if node.URL.String() != "http://[::1]:10001" {
					t.Errorf("es1: unexpected URL %q, want %q", node.URL.String(), "http://[::1]:10001")
				}
			case "es2":
				if node.URL.String() != "http://ip6host:10002" {
					t.Errorf("es2: unexpected URL %q, want %q", node.URL.String(), "http://ip6host:10002")
				}
			case "es3":
				if node.URL.String() != "http://127.0.0.1:10003" {
					t.Errorf("es3: unexpected URL %q, want %q", node.URL.String(), "http://127.0.0.1:10003")
				}
			}
		}
	})

	t.Run("DiscoverNodes() with IPv6 publish_address", func(t *testing.T) {
		u, _ := url.Parse("http://" + srv.Addr)
		tp, _ := New(Config{URLs: []*url.URL{u}})

		if err := tp.DiscoverNodesContext(context.Background()); err != nil {
			t.Fatalf("ERROR: %s", err)
		}

		pool, ok := tp.pool.(*statusConnectionPool)
		if !ok {
			t.Fatalf("Unexpected pool type: %T", tp.pool)
		}

		// es3 is master-only, so only es1 and es2 should be in the pool.
		if len(pool.live) != 2 {
			t.Fatalf("Unexpected number of live connections, want=2, got=%d", len(pool.live))
		}

		urls := make([]string, 0, len(pool.live))
		for _, conn := range pool.live {
			urls = append(urls, conn.URL.String())
		}
		sort.Strings(urls)

		wantURLs := []string{"http://[::1]:10001", "http://ip6host:10002"}
		sort.Strings(wantURLs)

		if !reflect.DeepEqual(urls, wantURLs) {
			t.Errorf("Unexpected pool URLs:\n  got:  %v\n  want: %v", urls, wantURLs)
		}
	})
}
