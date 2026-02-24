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
	"errors"
	"io"
	"strings"
	"testing"
)

type errReader struct{ err error }

func (e *errReader) Read([]byte) (int, error) { return 0, e.err }
func (e *errReader) Close() error             { return nil }

type trackingPool struct {
	value    any
	putCalls int
}

func (p *trackingPool) Get() any {
	return p.value
}

func (p *trackingPool) Put(v any) {
	p.putCalls++
	p.value = v
}

func decompressGzip(t *testing.T, buf *bytes.Buffer) string {
	t.Helper()
	zr, err := gzip.NewReader(buf)
	if err != nil {
		t.Fatalf("failed to create gzip reader: %s", err)
	}
	defer func() {
		closeErr := zr.Close()
		if closeErr != nil {
			t.Fatalf("failed to close gzip reader: %s", closeErr)
		}
	}()
	var out bytes.Buffer
	if _, err := io.Copy(&out, zr); err != nil {
		t.Fatalf("failed to decompress gzip data: %s", err)
	}
	return out.String()
}

func TestGzipCompressor(t *testing.T) {
	compressors := []struct {
		name string
		new  func(int) gzipCompressor
	}{
		{"Simple", newSimpleGzipCompressor},
		{"Pooled", newPooledGzipCompressor},
	}

	for _, tc := range compressors {
		t.Run(tc.name, func(t *testing.T) {
			t.Run("RoundTrip", func(t *testing.T) {
				c := tc.new(gzip.DefaultCompression)
				input := "elasticsearch"

				buf, err := c.compress(io.NopCloser(strings.NewReader(input)))
				if err != nil {
					t.Fatalf("unexpected error: %s", err)
				}
				if got := decompressGzip(t, buf); got != input {
					t.Fatalf("expected %q, got %q", input, got)
				}
			})

			t.Run("InvalidLevel", func(t *testing.T) {
				c := tc.new(42)

				_, err := c.compress(io.NopCloser(strings.NewReader("data")))
				if err == nil {
					t.Fatal("expected error for invalid compression level")
				}
				if !strings.Contains(err.Error(), "failed setting up") {
					t.Fatalf("unexpected error message: %s", err)
				}
			})

			t.Run("ReadError", func(t *testing.T) {
				c := tc.new(gzip.DefaultCompression)

				_, err := c.compress(&errReader{err: errors.New("synthetic read error")})
				if err == nil {
					t.Fatal("expected error from failing reader")
				}
				if !strings.Contains(err.Error(), "failed to compress request body") {
					t.Fatalf("unexpected error message: %s", err)
				}
			})
		})
	}
}

func TestGzipPooledReuse(t *testing.T) {
	c := newPooledGzipCompressor(gzip.DefaultCompression)

	for _, input := range []string{"first payload", "second payload"} {
		buf, err := c.compress(io.NopCloser(strings.NewReader(input)))
		if err != nil {
			t.Fatalf("unexpected error on input %q: %s", input, err)
		}
		if got := decompressGzip(t, buf); got != input {
			t.Fatalf("expected %q, got %q", input, got)
		}
		c.collectBuffer(buf)
	}
}

func TestGzipPooledCollectBuffer(t *testing.T) {
	c := newPooledGzipCompressor(gzip.DefaultCompression)

	buf, err := c.compress(io.NopCloser(strings.NewReader("payload")))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	c.collectBuffer(buf)

	buf, err = c.compress(io.NopCloser(strings.NewReader("another")))
	if err != nil {
		t.Fatalf("unexpected error after buffer reuse: %s", err)
	}
	if got := decompressGzip(t, buf); got != "another" {
		t.Fatalf("expected %q, got %q", "another", got)
	}
}

func TestGzipPooledBufferReturnedOnError(t *testing.T) {
	c := newPooledGzipCompressor(gzip.DefaultCompression).(*pooledGzipCompressor)

	pool := &trackingPool{value: new(bytes.Buffer)}
	c.bufferPool = pool

	_, err := c.compress(&errReader{err: errors.New("synthetic read error")})
	if err == nil {
		t.Fatal("expected error from failing reader")
	}

	if pool.putCalls != 1 {
		t.Fatalf("expected buffer to be returned once, got %d", pool.putCalls)
	}
	if _, ok := pool.value.(*bytes.Buffer); !ok || pool.value == nil {
		t.Fatal("expected buffer pool to contain a bytes.Buffer after error")
	}
}

func TestGzipPooledReadErrorRecovery(t *testing.T) {
	c := newPooledGzipCompressor(gzip.DefaultCompression)

	_, err := c.compress(&errReader{err: errors.New("synthetic read error")})
	if err == nil {
		t.Fatal("expected error from failing reader")
	}

	input := "after error"
	buf, err := c.compress(io.NopCloser(strings.NewReader(input)))
	if err != nil {
		t.Fatalf("unexpected error after previous read failure: %s", err)
	}
	if got := decompressGzip(t, buf); got != input {
		t.Fatalf("expected %q, got %q", input, got)
	}
}
