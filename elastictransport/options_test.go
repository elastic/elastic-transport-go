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
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestOptionName(t *testing.T) {
	tests := []struct {
		opt  Option
		name string
	}{
		{WithMaxRetries(3), "WithMaxRetries"},
		{WithDisableRetry(), "WithDisableRetry"},
		{WithAPIKey("key"), "WithAPIKey"},
		{WithBasicAuth("u", "p"), "WithBasicAuth"},
		{WithCompression(), "WithCompression"},
		{WithMetrics(), "WithMetrics"},
		{WithInterceptors(), "WithInterceptors"},
	}
	for _, tt := range tests {
		if got := tt.opt.Name(); got != tt.name {
			t.Errorf("Name() = %q, want %q", got, tt.name)
		}
	}
}

func TestOptionStringMasksSecrets(t *testing.T) {
	tests := []struct {
		opt            Option
		wantContains   string
		wantNotContain string
	}{
		{
			opt:            WithBasicAuth("admin", "s3cret"),
			wantContains:   `"admin"`,
			wantNotContain: "s3cret",
		},
		{
			opt:            WithAPIKey("Zm9vYmFy"),
			wantContains:   "****",
			wantNotContain: "Zm9vYmFy",
		},
		{
			opt:            WithServiceToken("tok-abc-123"),
			wantContains:   "****",
			wantNotContain: "tok-abc-123",
		},
		{
			opt:            WithCACert([]byte("PEM DATA")),
			wantContains:   "****",
			wantNotContain: "PEM DATA",
		},
		{
			opt:            WithCertificateFingerprint("7A3A6031CD"),
			wantContains:   "****",
			wantNotContain: "7A3A6031CD",
		},
	}
	for _, tt := range tests {
		s := tt.opt.String()
		if !strings.Contains(s, tt.wantContains) {
			t.Errorf("String() = %q, want it to contain %q", s, tt.wantContains)
		}
		if tt.wantNotContain != "" && strings.Contains(s, tt.wantNotContain) {
			t.Errorf("String() = %q, must NOT contain secret %q", s, tt.wantNotContain)
		}
	}
}

func TestOptionDescribeShowsSensitive(t *testing.T) {
	tests := []struct {
		opt          Option
		wantContains string
	}{
		{WithBasicAuth("admin", "s3cret"), "s3cret"},
		{WithAPIKey("Zm9vYmFy"), "Zm9vYmFy"},
		{WithServiceToken("tok-abc-123"), "tok-abc-123"},
		{WithCACert([]byte("PEM DATA")), "len=8"},
		{WithCertificateFingerprint("7A3A6031CD"), "7A3A6031CD"},
	}
	for _, tt := range tests {
		s := tt.opt.Describe(true)
		if !strings.Contains(s, tt.wantContains) {
			t.Errorf("Describe(true) = %q, want it to contain %q", s, tt.wantContains)
		}
	}
}

func TestOptionStringNonSensitive(t *testing.T) {
	u, _ := url.Parse("http://localhost:9200")
	tests := []struct {
		opt  Option
		want string
	}{
		{WithMaxRetries(5), "WithMaxRetries(5)"},
		{WithDisableRetry(), "WithDisableRetry()"},
		{WithMetrics(), "WithMetrics()"},
		{WithDebugLogger(), "WithDebugLogger()"},
		{WithCompression(), "WithCompression()"},
		{WithCompression(1), "WithCompression(1)"},
		{WithRetryOnStatus(502, 503), "WithRetryOnStatus([502 503])"},
		{WithDiscoverNodesInterval(30 * time.Second), "WithDiscoverNodesInterval(30s)"},
		{WithDiscoverNodeTimeout(5 * time.Second), "WithDiscoverNodeTimeout(5s)"},
		{WithURLs(u), "WithURLs(http://localhost:9200)"},
		{WithUserAgent("my-agent"), `WithUserAgent("my-agent")`},
		{WithRetryBackoff(nil), "WithRetryBackoff(func)"},
		{WithRetryOnError(nil), "WithRetryOnError(func)"},
		{WithConnectionPoolFunc(nil), "WithConnectionPoolFunc(func)"},
		{WithInterceptors(nil, nil), "WithInterceptors(2 funcs)"},
		{WithHeader(http.Header{"X-A": {"1"}, "X-B": {"2"}}), "WithHeader(2 entries)"},
	}
	for _, tt := range tests {
		got := tt.opt.String()
		if got != tt.want {
			t.Errorf("String() = %q, want %q", got, tt.want)
		}
		if got != tt.opt.Describe(false) {
			t.Error("String() and Describe(false) should be equal")
		}
		if got != tt.opt.Describe(true) {
			t.Error("non-sensitive option: Describe(true) should equal String()")
		}
	}
}

func TestValidateOptions(t *testing.T) {
	t.Run("valid options", func(t *testing.T) {
		u, _ := url.Parse("http://localhost:9200")
		err := ValidateOptions(
			WithURLs(u),
			WithMaxRetries(3),
			WithCompression(),
		)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
	})

	t.Run("single invalid option", func(t *testing.T) {
		err := ValidateOptions(WithMaxRetries(-1))
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "WithMaxRetries") {
			t.Errorf("error should mention WithMaxRetries: %s", err)
		}
	})

	t.Run("multiple invalid options", func(t *testing.T) {
		err := ValidateOptions(
			WithMaxRetries(-1),
			WithCompression(99),
		)
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "WithMaxRetries") {
			t.Errorf("error should mention WithMaxRetries: %s", err)
		}
		if !strings.Contains(err.Error(), "WithCompression") {
			t.Errorf("error should mention WithCompression: %s", err)
		}
	})

	t.Run("empty options", func(t *testing.T) {
		err := ValidateOptions()
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
	})

	t.Run("zero-value option is safe", func(t *testing.T) {
		err := ValidateOptions(Option{})
		if err != nil {
			t.Fatalf("zero-value Option should not cause error: %s", err)
		}
	})
}

func TestOptionsValidate(t *testing.T) {
	opts := Options{
		WithMaxRetries(-1),
		WithCompression(99),
	}
	err := opts.Validate()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "WithMaxRetries") {
		t.Errorf("error should mention WithMaxRetries: %s", err)
	}
	if !strings.Contains(err.Error(), "WithCompression") {
		t.Errorf("error should mention WithCompression: %s", err)
	}
}

func TestOptionsVisit(t *testing.T) {
	opts := Options{
		WithMaxRetries(3),
		WithDisableRetry(),
		WithMetrics(),
	}
	var names []string
	opts.Visit(func(o Option) {
		names = append(names, o.Name())
	})
	want := []string{"WithMaxRetries", "WithDisableRetry", "WithMetrics"}
	if len(names) != len(want) {
		t.Fatalf("visited %d options, want %d", len(names), len(want))
	}
	for i, n := range names {
		if n != want[i] {
			t.Errorf("names[%d] = %q, want %q", i, n, want[i])
		}
	}
}

func TestOptionsString(t *testing.T) {
	opts := Options{
		WithMaxRetries(3),
		WithAPIKey("secret"),
	}
	s := opts.String()
	if !strings.Contains(s, "WithMaxRetries(3)") {
		t.Errorf("String() should contain WithMaxRetries(3): %s", s)
	}
	if !strings.Contains(s, "****") {
		t.Errorf("String() should redact API key: %s", s)
	}
	if strings.Contains(s, "secret") {
		t.Errorf("String() should NOT contain secret value: %s", s)
	}
	lines := strings.Split(s, "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d: %q", len(lines), s)
	}
}

func TestOptionsDescribe(t *testing.T) {
	opts := Options{
		WithMaxRetries(3),
		WithAPIKey("mykey"),
	}

	masked := opts.Describe(false)
	if strings.Contains(masked, "mykey") {
		t.Errorf("Describe(false) should NOT contain secret: %s", masked)
	}

	unmasked := opts.Describe(true)
	if !strings.Contains(unmasked, "mykey") {
		t.Errorf("Describe(true) should contain secret: %s", unmasked)
	}
}

func TestOptionsStringImplementsStringer(t *testing.T) {
	opts := Options{WithMetrics()}
	s := opts.String()
	if s != "WithMetrics()" {
		t.Errorf("unexpected Stringer output: %q", s)
	}
}

func TestOptionStringImplementsStringer(t *testing.T) {
	opt := WithMaxRetries(5)
	s := opt.String()
	if s != "WithMaxRetries(5)" {
		t.Errorf("unexpected Stringer output: %q", s)
	}
}
