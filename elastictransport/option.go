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
	"net/http"
	"net/url"
	"time"
)

// Option configures a Client. Use the With* functions to obtain Option values.
type Option struct {
	apply func(*Config)
}

// WithURLs sets the Elasticsearch node URLs the transport will connect to.
// Multiple URLs enable round-robin load balancing and failover.
func WithURLs(urls ...*url.URL) Option {
	return Option{apply: func(c *Config) {
		c.URLs = urls
	}}
}

// WithBasicAuth configures HTTP Basic Authentication with the given username
// and password.
func WithBasicAuth(username, password string) Option {
	return Option{apply: func(c *Config) {
		c.Username = username
		c.Password = password
	}}
}

// WithAPIKey configures API Key authentication. The key should be the
// base64-encoded value returned by the Elasticsearch create API key endpoint.
func WithAPIKey(apiKey string) Option {
	return Option{apply: func(c *Config) {
		c.APIKey = apiKey
	}}
}

// WithServiceToken configures service token authentication using the given
// bearer token.
func WithServiceToken(token string) Option {
	return Option{apply: func(c *Config) {
		c.ServiceToken = token
	}}
}

// WithHeader sets global HTTP headers that are added to every request.
// Per-request headers set on the *http.Request take precedence.
func WithHeader(header http.Header) Option {
	return Option{apply: func(c *Config) {
		c.Header = header
	}}
}

// WithCACert sets the PEM-encoded CA certificate used to verify the server's
// TLS certificate. This requires the underlying transport to be an
// *http.Transport.
func WithCACert(cert []byte) Option {
	return Option{apply: func(c *Config) {
		c.CACert = cert
	}}
}

// WithCertificateFingerprint configures SHA-256 certificate fingerprint
// verification. When set, the transport verifies that at least one certificate
// in the chain matches the hex-encoded fingerprint, independent of CA trust.
func WithCertificateFingerprint(fingerprint string) Option {
	return Option{apply: func(c *Config) {
		c.CertificateFingerprint = fingerprint
	}}
}

// WithUserAgent sets the User-Agent header value sent with every request.
func WithUserAgent(ua string) Option {
	return Option{apply: func(c *Config) {
		c.UserAgent = ua
	}}
}

// WithTransport sets the http.RoundTripper used for HTTP requests. If not set,
// a clone of http.DefaultTransport is used.
func WithTransport(rt http.RoundTripper) Option {
	return Option{apply: func(c *Config) {
		c.Transport = rt
	}}
}

// WithLogger sets the Logger used to log request and response information.
func WithLogger(l Logger) Option {
	return Option{apply: func(c *Config) {
		c.Logger = l
	}}
}

// WithSelector sets the Selector used to pick connections from the pool.
func WithSelector(s Selector) Option {
	return Option{apply: func(c *Config) {
		c.Selector = s
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
	return Option{apply: func(c *Config) {
		c.ConnectionPoolFunc = f
	}}
}

// WithDisableRetry disables automatic request retries. When true,
// RetryOnStatus, RetryOnError, MaxRetries, and RetryBackoff are ignored.
func WithDisableRetry(disable bool) Option {
	return Option{apply: func(c *Config) {
		c.DisableRetry = disable
	}}
}

// WithRetryOnStatus sets the HTTP status codes that trigger a retry.
// The defaults are 502, 503, and 504.
func WithRetryOnStatus(statuses ...int) Option {
	return Option{apply: func(c *Config) {
		c.RetryOnStatus = statuses
	}}
}

// WithRetryOnError sets a function that decides whether a transport-level error
// should trigger a retry. The function receives the original request and the
// error. Returning true allows the retry.
func WithRetryOnError(fn func(*http.Request, error) bool) Option {
	return Option{apply: func(c *Config) {
		c.RetryOnError = fn
	}}
}

// WithMaxRetries sets the maximum number of retries for a failed request.
// The default is 3.
func WithMaxRetries(n int) Option {
	return Option{apply: func(c *Config) {
		c.MaxRetries = n
	}}
}

// WithRetryBackoff sets a backoff function called between retries. The function
// receives the retry attempt number (starting at 1) and returns the duration to
// wait before the next attempt.
func WithRetryBackoff(fn func(attempt int) time.Duration) Option {
	return Option{apply: func(c *Config) {
		c.RetryBackoff = fn
	}}
}

// WithCompressRequestBody enables or disables gzip compression for request
// bodies.
func WithCompressRequestBody(compress bool) Option {
	return Option{apply: func(c *Config) {
		c.CompressRequestBody = compress
	}}
}

// WithCompressRequestBodyLevel sets the gzip compression level used when
// CompressRequestBody is enabled. See the compress/gzip constants for valid
// values (e.g. gzip.BestSpeed, gzip.BestCompression). The default is
// gzip.DefaultCompression.
func WithCompressRequestBodyLevel(level int) Option {
	return Option{apply: func(c *Config) {
		c.CompressRequestBodyLevel = level
	}}
}

// WithPoolCompressor enables a sync.Pool based gzip writer, reducing
// allocations under high throughput. It should be combined with
// WithCompressRequestBody(true).
func WithPoolCompressor(enabled bool) Option {
	return Option{apply: func(c *Config) {
		c.PoolCompressor = enabled
	}}
}

// WithMetrics enables or disables internal transport metrics collection.
func WithMetrics(enabled bool) Option {
	return Option{apply: func(c *Config) {
		c.EnableMetrics = enabled
	}}
}

// WithDebugLogger enables a debug logger that writes connection-management
// information to os.Stdout.
func WithDebugLogger(enabled bool) Option {
	return Option{apply: func(c *Config) {
		c.EnableDebugLogger = enabled
	}}
}

// WithInstrumentation sets the Instrumentation used for tracing and metrics
// propagation (e.g. OpenTelemetry).
func WithInstrumentation(i Instrumentation) Option {
	return Option{apply: func(c *Config) {
		c.Instrumentation = i
	}}
}

// WithDiscoverNodesInterval sets the interval at which the transport
// automatically discovers cluster nodes. A zero value disables periodic
// discovery.
func WithDiscoverNodesInterval(d time.Duration) Option {
	return Option{apply: func(c *Config) {
		c.DiscoverNodesInterval = d
	}}
}

// WithDiscoverNodeTimeout sets the per-request timeout for node discovery
// calls. If not set, discovery uses the default transport timeout.
func WithDiscoverNodeTimeout(d time.Duration) Option {
	return Option{apply: func(c *Config) {
		c.DiscoverNodeTimeout = &d
	}}
}

// WithInterceptors sets the request/response interceptors applied on every
// round trip. Interceptors are composed in order and cannot be changed after
// transport creation.
func WithInterceptors(interceptors ...InterceptorFunc) Option {
	return Option{apply: func(c *Config) {
		c.Interceptors = interceptors
	}}
}
