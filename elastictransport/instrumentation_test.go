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
	"context"
	"fmt"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

var spanName = "search"
var endpoint = spanName

func NewTestOpenTelemetry() (*tracetest.InMemoryExporter, *sdktrace.TracerProvider, *ElasticsearchOpenTelemetry) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exporter))
	instrumentation := NewOtelInstrumentation(provider, true, "8.99.0-SNAPSHOT")
	return exporter, provider, instrumentation
}

func TestElasticsearchOpenTelemetry_StartClose(t *testing.T) {
	t.Run("Valid Start name", func(t *testing.T) {
		exporter, provider, instrument := NewTestOpenTelemetry()

		ctx := instrument.Start(context.Background(), spanName)
		instrument.Close(ctx)
		err := provider.ForceFlush(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		if ctx == nil {
			t.Fatalf("Start() returned an empty context")
		}

		if len(exporter.GetSpans()) != 1 {
			t.Fatalf("wrong number of spans recorded, got %v, want %v", len(exporter.GetSpans()), 1)
		}

		span := exporter.GetSpans()[0]

		if span.Name != spanName {
			t.Errorf("invalid span name, got %v, want %v", span.Name, spanName)
		}

		if span.SpanKind != trace.SpanKindClient {
			t.Errorf("wrong Span kind, expected, got %v, want %v", span.SpanKind, trace.SpanKindClient)
		}
	})
}

func TestElasticsearchOpenTelemetry_BeforeRequest(t *testing.T) {
	t.Run("BeforeRequest noop", func(t *testing.T) {
		_, _, instrument := NewTestOpenTelemetry()

		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
		if err != nil {
			t.Fatalf("error while creating request")
		}
		snapshot := req.Clone(context.Background())
		instrument.BeforeRequest(req, endpoint)

		if !reflect.DeepEqual(req, snapshot) {
			t.Fatalf("request should not have changed")
		}
	})
}

func TestElasticsearchOpenTelemetry_AfterRequest(t *testing.T) {
	t.Run("AfterRequest", func(t *testing.T) {
		exporter, provider, instrument := NewTestOpenTelemetry()
		fullUrl := "http://elastic:elastic@localhost:9200/test-index/_search"

		ctx := instrument.Start(context.Background(), spanName)
		req, err := http.NewRequestWithContext(ctx, http.MethodOptions, fullUrl, nil)
		if err != nil {
			t.Fatalf("error while creating request")
		}
		instrument.AfterRequest(req, "elasticsearch", endpoint)
		instrument.Close(ctx)
		err = provider.ForceFlush(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		if len(exporter.GetSpans()) != 1 {
			t.Fatalf("wrong number of spans recorded, got %v, want %v", len(exporter.GetSpans()), 1)
		}

		span := exporter.GetSpans()[0]

		if span.Name != spanName {
			t.Errorf("invalid span name, got %v, want %v", span.Name, spanName)
		}

		for _, attribute := range span.Attributes {
			switch attribute.Key {
			case attrDbSystem:
				if !attribute.Valid() && attribute.Value.AsString() != "elasticsearch" {
					t.Errorf("invalid %v, got %v, want %v", attrDbSystem, attribute.Value.AsString(), "elasticsearch")
				}
			case attrDbOperation:
				if !attribute.Valid() && attribute.Value.AsString() != endpoint {
					t.Errorf("invalid %v, got %v, want %v", attrDbOperation, attribute.Value.AsString(), endpoint)
				}
			case attrHttpRequestMethod:
				if !attribute.Valid() && attribute.Value.AsString() != http.MethodOptions {
					t.Errorf("invalid %v, got %v, want %v", attrHttpRequestMethod, attribute.Value.AsString(), http.MethodOptions)
				}
			case attrUrlFull:
				if !attribute.Valid() && attribute.Value.AsString() != fullUrl {
					t.Errorf("invalid %v, got %v, want %v", attrUrlFull, attribute.Value.AsString(), fullUrl)
				}
			case attrServerAddress:
				if !attribute.Valid() && attribute.Value.AsString() != "localhost" {
					t.Errorf("invalid %v, got %v, want %v", attrServerAddress, attribute.Value.AsString(), "localhost")
				}
			case attrServerPort:
				if !attribute.Valid() && attribute.Value.AsInt64() != 9200 {
					t.Errorf("invalid %v, got %v, want %v", attrServerPort, attribute.Value.AsInt64(), 9200)
				}
			}
		}
	})
}

func TestElasticsearchOpenTelemetry_RecordError(t *testing.T) {
	exporter, provider, instrument := NewTestOpenTelemetry()

	ctx := instrument.Start(context.Background(), spanName)
	instrument.RecordError(ctx, fmt.Errorf("these are not the spans you are looking for"))
	instrument.Close(ctx)
	err := provider.ForceFlush(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(exporter.GetSpans()) != 1 {
		t.Fatalf("wrong number of spans recorded, got %v, want %v", len(exporter.GetSpans()), 1)
	}

	span := exporter.GetSpans()[0]

	if span.Name != spanName {
		t.Errorf("invalid span name, got %v, want %v", span.Name, spanName)
	}

	if span.Status.Code != codes.Error {
		t.Errorf("expected the span to have a status.code Error, got %v, want %v", span.Status.Code, codes.Error)
	}
}

func TestElasticsearchOpenTelemetry_RecordClusterId(t *testing.T) {
	exporter, provider, instrument := NewTestOpenTelemetry()

	ctx := instrument.Start(context.Background(), spanName)
	clusterId := "randomclusterid"
	instrument.AfterResponse(ctx, &http.Response{Header: map[string][]string{
		"X-Found-Handling-Cluster": {clusterId},
	}})
	instrument.Close(ctx)
	err := provider.ForceFlush(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(exporter.GetSpans()) != 1 {
		t.Fatalf("wrong number of spans recorded, got %v, want %v", len(exporter.GetSpans()), 1)
	}

	span := exporter.GetSpans()[0]

	if span.Name != spanName {
		t.Errorf("invalid span name, got %v, want %v", span.Name, spanName)
	}

	for _, attribute := range span.Attributes {
		switch attribute.Key {
		case attrDbElasticsearchClusterName:
			if !attribute.Valid() && attribute.Value.AsString() != clusterId {
				t.Errorf("invalid %v, got %v, want %v", attrServerAddress, attribute.Value.AsString(), clusterId)
			}
		}
	}
}

func TestElasticsearchOpenTelemetry_RecordClusterName(t *testing.T) {
	const cloudHeader = "X-Found-Handling-Cluster"
	const onPremHeader = "Elastic-Cluster-Name"

	testCases := []struct {
		name    string
		headers map[string][]string
		// want is the expected db.elasticsearch.cluster.name attribute value.
		// An empty string means the attribute must not be present.
		want string
	}{
		{
			name:    "Elastic Cloud header only",
			headers: map[string][]string{cloudHeader: {"cloud-cluster"}},
			want:    "cloud-cluster",
		},
		{
			name:    "self-managed header only",
			headers: map[string][]string{onPremHeader: {"onprem-cluster"}},
			want:    "onprem-cluster",
		},
		{
			name: "both present, Cloud takes precedence",
			headers: map[string][]string{
				cloudHeader:  {"cloud-cluster"},
				onPremHeader: {"onprem-cluster"},
			},
			want: "cloud-cluster",
		},
		{
			name: "empty Cloud header falls back to self-managed",
			headers: map[string][]string{
				cloudHeader:  {""},
				onPremHeader: {"onprem-cluster"},
			},
			want: "onprem-cluster",
		},
		{
			name:    "neither header present",
			headers: map[string][]string{},
			want:    "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			exporter, provider, instrument := NewTestOpenTelemetry()

			ctx := instrument.Start(context.Background(), spanName)
			instrument.AfterResponse(ctx, &http.Response{Header: tc.headers})
			instrument.Close(ctx)
			if err := provider.ForceFlush(context.Background()); err != nil {
				t.Fatal(err)
			}

			if len(exporter.GetSpans()) != 1 {
				t.Fatalf("wrong number of spans recorded, got %v, want %v", len(exporter.GetSpans()), 1)
			}

			span := exporter.GetSpans()[0]

			var got string
			var found bool
			for _, attr := range span.Attributes {
				if attr.Key == attrDbElasticsearchClusterName {
					got = attr.Value.AsString()
					found = true
				}
			}

			if tc.want == "" {
				if found {
					t.Errorf("expected no %v attribute, got %v", attrDbElasticsearchClusterName, got)
				}
				return
			}

			if !found {
				t.Fatalf("expected %v attribute to be set to %v, but it was missing", attrDbElasticsearchClusterName, tc.want)
			}
			if got != tc.want {
				t.Errorf("invalid %v, got %v, want %v", attrDbElasticsearchClusterName, got, tc.want)
			}
		})
	}
}

func TestElasticsearchOpenTelemetry_RecordNodeName(t *testing.T) {
	exporter, provider, instrument := NewTestOpenTelemetry()

	ctx := instrument.Start(context.Background(), spanName)
	nodeName := "randomnodename"
	instrument.AfterResponse(ctx, &http.Response{Header: map[string][]string{
		"X-Found-Handling-Instance": {nodeName},
	}})
	instrument.Close(ctx)
	err := provider.ForceFlush(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(exporter.GetSpans()) != 1 {
		t.Fatalf("wrong number of spans recorded, got %v, want %v", len(exporter.GetSpans()), 1)
	}

	span := exporter.GetSpans()[0]

	if span.Name != spanName {
		t.Errorf("invalid span name, got %v, want %v", span.Name, spanName)
	}

	for _, attribute := range span.Attributes {
		switch attribute.Key {
		case attrDbElasticsearchNodeName:
			if !attribute.Valid() && attribute.Value.AsString() != nodeName {
				t.Errorf("invalid %v, got %v, want %v", attrDbElasticsearchNodeName, attribute.Value.AsString(), nodeName)
			}
		}
	}
}

func TestElasticsearchOpenTelemetry_RecordPathPart(t *testing.T) {
	exporter, provider, instrument := NewTestOpenTelemetry()
	indexName := "test-index"
	pretty := "true"

	ctx := instrument.Start(context.Background(), spanName)
	instrument.RecordPathPart(ctx, "index", indexName)
	instrument.RecordPathPart(ctx, "pretty", pretty)
	instrument.Close(ctx)
	err := provider.ForceFlush(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(exporter.GetSpans()) != 1 {
		t.Fatalf("wrong number of spans recorded, got %v, want %v", len(exporter.GetSpans()), 1)
	}

	span := exporter.GetSpans()[0]

	if span.Name != spanName {
		t.Errorf("invalid span name, got %v, want %v", span.Name, spanName)
	}

	for _, attribute := range span.Attributes {
		switch attribute.Key {
		case attrPathParts + "index":
			if !attribute.Valid() && attribute.Value.AsString() != indexName {
				t.Errorf("invalid %v, got %v, want %v", attrPathParts+"index", attribute.Value.AsString(), indexName)
			}
		case attrPathParts + "pretty":
			if !attribute.Valid() && attribute.Value.AsString() != indexName {
				t.Errorf("invalid %v, got %v, want %v", attrPathParts+"pretty", attribute.Value.AsString(), indexName)
			}
		}
	}
}

func TestElasticsearchOpenTelemetry_RecordRequestBody(t *testing.T) {
	exporter, provider, instrument := NewTestOpenTelemetry()
	fullUrl := "http://elastic:elastic@localhost:9200/test-index/_search"
	query := `{"query": {"match_all": {}}}`

	// Won't log query
	ctx := instrument.Start(context.Background(), spanName)
	_, err := http.NewRequestWithContext(ctx, http.MethodOptions, fullUrl, strings.NewReader(query))
	if err != nil {
		t.Fatalf("error while creating request")
	}
	if reader := instrument.RecordRequestBody(ctx, "foo.endpoint", strings.NewReader(query)); reader != nil {
		t.Errorf("returned reader should be nil")
	}
	instrument.Close(ctx)

	// Will log query
	secondCtx := instrument.Start(context.Background(), spanName)
	_, err = http.NewRequestWithContext(ctx, http.MethodOptions, fullUrl, strings.NewReader(query))
	if err != nil {
		t.Fatalf("error while creating request")
	}
	if reader := instrument.RecordRequestBody(secondCtx, "search", strings.NewReader(query)); reader == nil {
		t.Errorf("returned reader should not be nil")
	}
	instrument.Close(secondCtx)

	err = provider.ForceFlush(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(exporter.GetSpans()) != 2 {
		t.Fatalf("wrong number of spans recorded, got %v, want %v", len(exporter.GetSpans()), 1)
	}

	span := exporter.GetSpans()[0]
	if span.Name != spanName {
		t.Errorf("invalid span name, got %v, want %v", span.Name, spanName)
	}

	for _, attribute := range span.Attributes {
		switch attribute.Key {
		case attrDbStatement:
			t.Errorf("span should not have a %v entry", attrDbStatement)
		}
	}

	querySpan := exporter.GetSpans()[1]
	if querySpan.Name != spanName {
		t.Errorf("invalid span name, got %v, want %v", querySpan.Name, spanName)
	}

	for _, attribute := range querySpan.Attributes {
		switch attribute.Key {
		case attrDbStatement:
			if !attribute.Valid() && attribute.Value.AsString() != query {
				t.Errorf("invalid query provided, got %v, want %v", attribute.Value.AsString(), query)
			}
		}
	}
}
