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

package elastictransport_test

import (
	"compress/gzip"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/elastic/elastic-transport-go/v8/elastictransport"
)

func ExampleNewClient() {
	u, _ := url.Parse("http://localhost:9200")

	tp, err := elastictransport.NewClient(
		elastictransport.WithURLs(u),
	)
	if err != nil {
		panic(err)
	}

	fmt.Println(tp.URLs()[0].Host)
	// Output: localhost:9200
}

func ExampleNewClient_basicAuth() {
	u, _ := url.Parse("http://localhost:9200")

	_, err := elastictransport.NewClient(
		elastictransport.WithURLs(u),
		elastictransport.WithBasicAuth("elastic", "changeme"),
	)
	if err != nil {
		panic(err)
	}

	fmt.Println("client created with basic auth")
	// Output: client created with basic auth
}

func ExampleNewClient_multipleNodes() {
	u1, _ := url.Parse("http://es01:9200")
	u2, _ := url.Parse("http://es02:9200")
	u3, _ := url.Parse("http://es03:9200")

	tp, err := elastictransport.NewClient(
		elastictransport.WithURLs(u1, u2, u3),
	)
	if err != nil {
		panic(err)
	}

	fmt.Println(len(tp.URLs()))
	// Output: 3
}

func ExampleNewClient_retries() {
	u, _ := url.Parse("http://localhost:9200")

	_, err := elastictransport.NewClient(
		elastictransport.WithURLs(u),
		elastictransport.WithRetry(5, 429, 502, 503, 504),
		elastictransport.WithRetryBackoff(func(attempt int) time.Duration {
			return time.Duration(attempt) * 100 * time.Millisecond
		}),
	)
	if err != nil {
		panic(err)
	}

	fmt.Println("client created with custom retry config")
	// Output: client created with custom retry config
}

func ExampleNewClient_compression() {
	u, _ := url.Parse("http://localhost:9200")

	_, err := elastictransport.NewClient(
		elastictransport.WithURLs(u),
		elastictransport.WithCompression(gzip.BestSpeed),
	)
	if err != nil {
		panic(err)
	}

	fmt.Println("client created with compression")
	// Output: client created with compression
}

func ExampleNewClient_globalHeaders() {
	u, _ := url.Parse("http://localhost:9200")

	hdr := http.Header{}
	hdr.Set("X-Request-Source", "my-app")

	_, err := elastictransport.NewClient(
		elastictransport.WithURLs(u),
		elastictransport.WithHeader(hdr),
	)
	if err != nil {
		panic(err)
	}

	fmt.Println("client created with global headers")
	// Output: client created with global headers
}

func ExampleNewClient_interceptors() {
	u, _ := url.Parse("http://localhost:9200")

	loggingInterceptor := func(next elastictransport.RoundTripFunc) elastictransport.RoundTripFunc {
		return func(req *http.Request) (*http.Response, error) {
			fmt.Printf("-> %s %s\n", req.Method, req.URL.Path)
			return next(req)
		}
	}

	_, err := elastictransport.NewClient(
		elastictransport.WithURLs(u),
		elastictransport.WithInterceptors(loggingInterceptor),
	)
	if err != nil {
		panic(err)
	}

	fmt.Println("client created with interceptors")
	// Output: client created with interceptors
}

func ExampleValidateOptions() {
	u, _ := url.Parse("http://localhost:9200")

	opts := elastictransport.Options{
		elastictransport.WithURLs(u),
		elastictransport.WithMaxRetries(5),
	}

	if err := opts.Validate(); err != nil {
		log.Fatal(err)
	}

	tp, err := elastictransport.NewClient(opts...)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(tp.URLs()[0].Host)
	// Output: localhost:9200
}

func ExampleValidateOptions_invalid() {
	err := elastictransport.ValidateOptions(
		elastictransport.WithMaxRetries(-1),
	)
	fmt.Println(err)
	// Output: transport options: WithMaxRetries: value must be >= 0, got -1
}

func ExampleOption_String() {
	opt := elastictransport.WithMaxRetries(5)
	fmt.Println(opt)
	// Output: WithMaxRetries(5)
}

func ExampleOption_String_sensitive() {
	opt := elastictransport.WithAPIKey("Zm9vYmFy")

	fmt.Println(opt)
	fmt.Println(opt.Describe(true))
	// Output:
	// WithAPIKey(****)
	// WithAPIKey("Zm9vYmFy")
}

func ExampleOptions_Visit() {
	u, _ := url.Parse("http://localhost:9200")

	opts := elastictransport.Options{
		elastictransport.WithURLs(u),
		elastictransport.WithMaxRetries(3),
		elastictransport.WithMetrics(),
	}

	opts.Visit(func(o elastictransport.Option) {
		fmt.Println(o.Name())
	})
	// Output:
	// WithURLs
	// WithMaxRetries
	// WithMetrics
}

func ExampleOptions_Describe() {
	opts := elastictransport.Options{
		elastictransport.WithMaxRetries(3),
		elastictransport.WithAPIKey("secret-key"),
	}

	fmt.Println("--- masked ---")
	fmt.Println(opts.Describe(false))
	fmt.Println("--- unmasked ---")
	fmt.Println(opts.Describe(true))
	// Output:
	// --- masked ---
	// WithMaxRetries(3)
	// WithAPIKey(****)
	// --- unmasked ---
	// WithMaxRetries(3)
	// WithAPIKey("secret-key")
}
