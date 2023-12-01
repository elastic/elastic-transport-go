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
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"net/http"
	"reflect"
	"testing"
)

var spanName = "search"
var endpoint = spanName

func NewTestOpenTelemetry() (*tracetest.InMemoryExporter, *sdktrace.TracerProvider, *ElasticsearchOpenTelemetry) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exporter))
	otel.SetTracerProvider(provider)
	instrumentation := NewOtelInstrumentation(true, "8.99.0-SNAPSHOT")
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
			t.Fatalf("wrong number of spans recorded, got = %v, want %v", len(exporter.GetSpans()), 1)
		}

		span := exporter.GetSpans()[0]

		if span.Name != spanName {
			t.Errorf("Start() invalid span name, got = %v, want %v", span.Name, spanName)
		}

		if span.SpanKind != trace.SpanKindClient {
			t.Errorf("wrong Span kind, expected, got = %v, want %v", span.SpanKind, trace.SpanKindClient)
		}
	})
}

func TestElasticsearchOpenTelemetry_BeforeRequest(t *testing.T) {
	t.Run("BeforeRequest noop", func(t *testing.T) {
		_, _, instrument := NewTestOpenTelemetry()

		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
		snapshot := req.Clone(context.Background())
		if err != nil {
			t.Fatalf("error while creating request")
		}
		instrument.BeforeRequest(req, endpoint)

		if !reflect.DeepEqual(req, snapshot) {
			t.Fatalf("request should not have changed")
		}
	})
}

//func TestElasticsearchOpenTelemetry_AfterRequest(t *testing.T) {
//	type fields struct {
//		tracer      trace.Tracer
//		recordQuery bool
//	}
//	type args struct {
//		req      *http.Request
//		system   string
//		endpoint string
//	}
//	tests := []struct {
//		name   string
//		fields fields
//		args   args
//	}{
//		// TODO: Add test cases.
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			i := &ElasticsearchOpenTelemetry{
//				tracer:      tt.fields.tracer,
//				recordQuery: tt.fields.recordQuery,
//			}
//			i.AfterRequest(tt.args.req, tt.args.system, tt.args.endpoint)
//		})
//	}
//}

//func TestElasticsearchOpenTelemetry_RecordError(t *testing.T) {
//	type fields struct {
//		tracer      trace.Tracer
//		recordQuery bool
//	}
//	type args struct {
//		ctx context.Context
//		err error
//	}
//	tests := []struct {
//		name   string
//		fields fields
//		args   args
//	}{
//		// TODO: Add test cases.
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			i := &ElasticsearchOpenTelemetry{
//				tracer:      tt.fields.tracer,
//				recordQuery: tt.fields.recordQuery,
//			}
//			i.RecordError(tt.args.ctx, tt.args.err)
//		})
//	}
//}
//
//func TestElasticsearchOpenTelemetry_RecordClusterId(t *testing.T) {
//	type fields struct {
//		tracer      trace.Tracer
//		recordQuery bool
//	}
//	type args struct {
//		ctx context.Context
//		id  string
//	}
//	tests := []struct {
//		name   string
//		fields fields
//		args   args
//	}{
//		// TODO: Add test cases.
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			i := &ElasticsearchOpenTelemetry{
//				tracer:      tt.fields.tracer,
//				recordQuery: tt.fields.recordQuery,
//			}
//			i.RecordClusterId(tt.args.ctx, tt.args.id)
//		})
//	}
//}
//
//func TestElasticsearchOpenTelemetry_RecordNodeName(t *testing.T) {
//	type fields struct {
//		tracer      trace.Tracer
//		recordQuery bool
//	}
//	type args struct {
//		ctx  context.Context
//		name string
//	}
//	tests := []struct {
//		name   string
//		fields fields
//		args   args
//	}{
//		// TODO: Add test cases.
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			i := &ElasticsearchOpenTelemetry{
//				tracer:      tt.fields.tracer,
//				recordQuery: tt.fields.recordQuery,
//			}
//			i.RecordNodeName(tt.args.ctx, tt.args.name)
//		})
//	}
//}
//
//func TestElasticsearchOpenTelemetry_RecordPathPart(t *testing.T) {
//	type fields struct {
//		tracer      trace.Tracer
//		recordQuery bool
//	}
//	type args struct {
//		ctx      context.Context
//		pathPart string
//		value    string
//	}
//	tests := []struct {
//		name   string
//		fields fields
//		args   args
//	}{
//		// TODO: Add test cases.
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			i := &ElasticsearchOpenTelemetry{
//				tracer:      tt.fields.tracer,
//				recordQuery: tt.fields.recordQuery,
//			}
//			i.RecordPathPart(tt.args.ctx, tt.args.pathPart, tt.args.value)
//		})
//	}
//}
//
//func TestElasticsearchOpenTelemetry_RecordQuery(t *testing.T) {
//	type fields struct {
//		tracer      trace.Tracer
//		recordQuery bool
//	}
//	type args struct {
//		ctx      context.Context
//		endpoint string
//		query    string
//	}
//	tests := []struct {
//		name   string
//		fields fields
//		args   args
//	}{
//		// TODO: Add test cases.
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			i := &ElasticsearchOpenTelemetry{
//				tracer:      tt.fields.tracer,
//				recordQuery: tt.fields.recordQuery,
//			}
//			i.RecordQuery(tt.args.ctx, tt.args.endpoint, tt.args.query)
//		})
//	}
//}
//
//func TestNewOtelInstrumentation(t *testing.T) {
//	type args struct {
//		captureSearchBody bool
//		version           string
//	}
//	tests := []struct {
//		name string
//		args args
//		want *ElasticsearchOpenTelemetry
//	}{
//		// TODO: Add test cases.
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			if got := NewOtelInstrumentation(tt.args.captureSearchBody, tt.args.version); !reflect.DeepEqual(got, tt.want) {
//				t.Errorf("NewOtelInstrumentation() = %v, want %v", got, tt.want)
//			}
//		})
//	}
//}
