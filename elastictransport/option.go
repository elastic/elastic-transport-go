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

package elastictransport

import (
	"compress/gzip"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// Option configures a Client. Use the With* functions to obtain Option values.
type Option struct {
	apply func(*Config) error
}

// WithURLs sets the Elasticsearch node URLs the transport will connect to.
// Multiple URLs enable round-robin load balancing and failover.
func WithURLs(urls ...*url.URL) Option {
	return Option{apply: func(c *Config) error {
		c.URLs = urls
		return nil
	}}
}

// WithBasicAuth configures HTTP Basic Authentication with the given username
// and password.
func WithBasicAuth(username, password string) Option {
	return Option{apply: func(c *Config) error {
		c.Username = username
		c.Password = password
		return nil
	}}
}

// WithAPIKey configures API Key authentication. The key should be the
// base64-encoded value returned by the Elasticsearch create API key endpoint.
func WithAPIKey(apiKey string) Option {
	return Option{apply: func(c *Config) error {
		c.APIKey = apiKey
		return nil
	}}
}

// WithServiceToken configures service token authentication using the given
// bearer token.
func WithServiceToken(token string) Option {
	return Option{apply: func(c *Config) error {
		c.ServiceToken = token
		return nil
	}}
}

// WithHeader sets global HTTP headers that are added to every request.
// Per-request headers set on the *http.Request take precedence.
func WithHeader(header http.Header) Option {
	return Option{apply: func(c *Config) error {
		c.Header = header
		return nil
	}}
}

// WithCACert sets the PEM-encoded CA certificate used to verify the server's
// TLS certificate. This requires the underlying transport to be an
// *http.Transport.
func WithCACert(cert []byte) Option {
	return Option{apply: func(c *Config) error {
		c.CACert = cert
		return nil
	}}
}

// WithCertificateFingerprint configures SHA-256 certificate fingerprint
// verification. When set, the transport verifies that at least one certificate
// in the chain matches the hex-encoded fingerprint, independent of CA trust.
func WithCertificateFingerprint(fingerprint string) Option {
	return Option{apply: func(c *Config) error {
		c.CertificateFingerprint = fingerprint
		return nil
	}}
}

// WithUserAgent sets the User-Agent header value sent with every request.
func WithUserAgent(ua string) Option {
	return Option{apply: func(c *Config) error {
		c.UserAgent = ua
		return nil
	}}
}

// WithTransport sets the http.RoundTripper used for HTTP requests. If not set,
// a clone of http.DefaultTransport is used.
func WithTransport(rt http.RoundTripper) Option {
	return Option{apply: func(c *Config) error {
		c.Transport = rt
		return nil
	}}
}

// WithLogger sets the Logger used to log request and response information.
func WithLogger(l Logger) Option {
	return Option{apply: func(c *Config) error {
		c.Logger = l
		return nil
	}}
}

// WithSelector sets the Selector used to pick connections from the pool.
func WithSelector(s Selector) Option {
	return Option{apply: func(c *Config) error {
		c.Selector = s
		return nil
	}}
}

// WithConnectionPoolFunc sets a factory function for creating a custom
// connection pool. Pools are synchronised by default; implement
// ConcurrentSafeConnectionPool to opt out when your pool is already safe for
// concurrent use.
//
// During discovery, if the current pool implements UpdatableConnectionPool,
// discovery prefers in-place Update() and this function is only called when
// Update() is not available.
func WithConnectionPoolFunc(f func([]*Connection, Selector) ConnectionPool) Option {
	return Option{apply: func(c *Config) error {
		c.ConnectionPoolFunc = f
		return nil
	}}
}

// WithDisableRetry disables automatic request retries. When set,
// RetryOnStatus, RetryOnError, MaxRetries, and RetryBackoff are ignored.
func WithDisableRetry() Option {
	return Option{apply: func(c *Config) error {
		c.DisableRetry = true
		return nil
	}}
}

// WithRetry configures retry behaviour with the given maximum number of
// retries and optional HTTP status codes that trigger a retry. If no status
// codes are provided, the defaults (502, 503, 504) are used.
//
// maxRetries must be non-negative.
func WithRetry(maxRetries int, onStatus ...int) Option {
	return Option{apply: func(c *Config) error {
		if maxRetries < 0 {
			return fmt.Errorf("WithRetry: maxRetries must be >= 0, got %d", maxRetries)
		}
		c.MaxRetries = maxRetries
		if len(onStatus) > 0 {
			c.RetryOnStatus = onStatus
		}
		return nil
	}}
}

// WithRetryOnStatus sets the HTTP status codes that trigger a retry.
// The defaults are 502, 503, and 504.
func WithRetryOnStatus(statuses ...int) Option {
	return Option{apply: func(c *Config) error {
		c.RetryOnStatus = statuses
		return nil
	}}
}

// WithRetryOnError sets a function that decides whether a transport-level error
// should trigger a retry. The function receives the original request and the
// error. Returning true allows the retry.
func WithRetryOnError(fn func(*http.Request, error) bool) Option {
	return Option{apply: func(c *Config) error {
		c.RetryOnError = fn
		return nil
	}}
}

// WithMaxRetries sets the maximum number of retries for a failed request.
// The default is 3. n must be non-negative.
func WithMaxRetries(n int) Option {
	return Option{apply: func(c *Config) error {
		if n < 0 {
			return fmt.Errorf("WithMaxRetries: value must be >= 0, got %d", n)
		}
		c.MaxRetries = n
		return nil
	}}
}

// WithRetryBackoff sets a backoff function called between retries. The function
// receives the retry attempt number (starting at 1) and returns the duration to
// wait before the next attempt.
func WithRetryBackoff(fn func(attempt int) time.Duration) Option {
	return Option{apply: func(c *Config) error {
		c.RetryBackoff = fn
		return nil
	}}
}

// WithCompression enables gzip compression for request bodies using a pooled
// gzip writer. An optional compression level may be provided (see the
// compress/gzip constants, e.g. gzip.BestSpeed). When omitted, gzip.DefaultCompression
// is used.
func WithCompression(level ...int) Option {
	return Option{apply: func(c *Config) error {
		c.CompressRequestBody = true
		c.PoolCompressor = true
		if len(level) > 0 {
			l := level[0]
			if l < gzip.HuffmanOnly || l > gzip.BestCompression {
				return fmt.Errorf("WithCompression: invalid gzip level %d (valid range: %d to %d)",
					l, gzip.HuffmanOnly, gzip.BestCompression)
			}
			c.CompressRequestBodyLevel = l
		}
		return nil
	}}
}

// WithMetrics enables internal transport metrics collection.
func WithMetrics() Option {
	return Option{apply: func(c *Config) error {
		c.EnableMetrics = true
		return nil
	}}
}

// WithDebugLogger enables a debug logger that writes connection-management
// information to os.Stdout.
func WithDebugLogger() Option {
	return Option{apply: func(c *Config) error {
		c.EnableDebugLogger = true
		return nil
	}}
}

// WithInstrumentation sets the Instrumentation used for tracing and metrics
// propagation (e.g. OpenTelemetry).
func WithInstrumentation(i Instrumentation) Option {
	return Option{apply: func(c *Config) error {
		c.Instrumentation = i
		return nil
	}}
}

// WithDiscoverNodesInterval sets the interval at which the transport
// automatically discovers cluster nodes. A zero value disables periodic
// discovery.
func WithDiscoverNodesInterval(d time.Duration) Option {
	return Option{apply: func(c *Config) error {
		c.DiscoverNodesInterval = d
		return nil
	}}
}

// WithDiscoverNodeTimeout sets the per-request timeout for node discovery
// calls. If not set, discovery uses the default transport timeout.
func WithDiscoverNodeTimeout(d time.Duration) Option {
	return Option{apply: func(c *Config) error {
		c.DiscoverNodeTimeout = &d
		return nil
	}}
}

// WithInterceptors sets the request/response interceptors applied on every
// round trip. Interceptors are composed in order and cannot be changed after
// transport creation.
func WithInterceptors(interceptors ...InterceptorFunc) Option {
	return Option{apply: func(c *Config) error {
		c.Interceptors = interceptors
		return nil
	}}
}

