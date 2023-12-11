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
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"net/http"
	"strconv"
)

const schemaUrl = "https://opentelemetry.io/schemas/1.21.0"
const tracerName = "elasticsearch-api"

// Constants for Semantic Convention
// see https://opentelemetry.io/docs/specs/semconv/database/elasticsearch/ for details.
const attrDbSystem = "db.system"
const attrDbStatement = "db.statement"
const attrDbOperation = "db.operation"
const attrDbElasticsearchClusterName = "db.elasticsearch.cluster.name"
const attrDbElasticsearchNodeName = "db.elasticsearch.node.name"
const attrHttpRequestMethod = "http.request.method"
const attrUrlFull = "url.full"
const attrServerAddress = "server.address"
const attrServerPort = "server.port"
const attrPathParts = "db.elasticsearch.path_parts."

// Instrumentation defines the interface the client uses to propagate information about the requests.
// Each method is called with the current context or request for propagation.
type Instrumentation interface {
	// Start creates the span before building the request, returned context will be propagated to the request by the client.
	Start(ctx context.Context, name string) context.Context

	// Close will be called once the client has returned.
	Close(ctx context.Context)

	// RecordError propagates an error.
	RecordError(ctx context.Context, err error)

	// RecordClusterId provides the cluster id returned by Elastic Cloud.
	RecordClusterId(ctx context.Context, id string)

	// RecordNodeName provides the node name returned by Elastic Cloud.
	RecordNodeName(ctx context.Context, name string)

	// RecordPathPart provides the path variables, called once per variable in the url.
	RecordPathPart(ctx context.Context, pathPart, value string)

	// ShouldRecordQuery determines is the endpoint is within the allow list for query recording.
	ShouldRecordQuery(endpoint string) bool

	// RecordQuery provides the endpoint name as well as the current request payload.
	RecordQuery(ctx context.Context, query string)

	// BeforeRequest provides the request and endpoint name, called before sending to the server.
	BeforeRequest(req *http.Request, endpoint string)

	// AfterRequest provides the request, system used (e.g. elasticsearch) and endpoint name.
	// Called after the request has been enhanced with the information from the transport and sent to the server.
	AfterRequest(req *http.Request, system, endpoint string)
}

type ElasticsearchOpenTelemetry struct {
	tracer      trace.Tracer
	recordQuery bool
}

// NewOtelInstrumentation returns a new instrument for Open Telemetry traces
// If no provider is passed, the instrumentation will fall back to the global otel provider.
// captureSearchBody sets the query capture behavior for search endpoints.
// version should be set to the version provided by the caller.
func NewOtelInstrumentation(provider trace.TracerProvider, captureSearchBody bool, version string) *ElasticsearchOpenTelemetry {
	if provider == nil {
		provider = otel.GetTracerProvider()
	}
	return &ElasticsearchOpenTelemetry{
		tracer: provider.Tracer(
			tracerName,
			trace.WithInstrumentationVersion(version),
			trace.WithSchemaURL(schemaUrl),
		),
		recordQuery: captureSearchBody,
	}
}

// Start begins a new span in the given context with the provided name.
// Span will always have a kind set to trace.SpanKindClient.
// The context span aware is returned for use within the client.
func (i *ElasticsearchOpenTelemetry) Start(ctx context.Context, name string) context.Context {
	newCtx, _ := i.tracer.Start(ctx, name, trace.WithSpanKind(trace.SpanKindClient))
	return newCtx
}

// Close call for the end of the span, preferably defered by the client once started.
func (i *ElasticsearchOpenTelemetry) Close(ctx context.Context) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.End()
	}
}

// ShouldRecordQuery filters for search endpoints.
func (i *ElasticsearchOpenTelemetry) ShouldRecordQuery(endpoint string) bool {
	// allow list of endpoints that will propagate query to OpenTelemetry.
	// see https://opentelemetry.io/docs/specs/semconv/database/elasticsearch/#call-level-attributes
	var searchEndpoints = map[string]struct{}{
		"search":                 {},
		"async_search.submit":    {},
		"msearch":                {},
		"eql.search":             {},
		"terms_enum":             {},
		"search_template":        {},
		"msearch_template":       {},
		"render_search_template": {},
	}

	if i.recordQuery {
		if _, ok := searchEndpoints[endpoint]; ok {
			return true
		}
	}
	return false
}

// RecordQuery add the db.statement attributes only for the search endpoints.
func (i *ElasticsearchOpenTelemetry) RecordQuery(ctx context.Context, query string) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.SetAttributes(attribute.String(attrDbStatement, query))
	}
}

// RecordError sets any provided error as an OTel error in the active span.
func (i *ElasticsearchOpenTelemetry) RecordError(ctx context.Context, err error) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.SetStatus(codes.Error, "an error happened while executing a request")
		span.RecordError(err)
	}
}

// RecordClusterId propagate the cluster ID provided by Elastic Cloud via the X-Found-Handling-Cluster header.
func (i *ElasticsearchOpenTelemetry) RecordClusterId(ctx context.Context, id string) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.SetAttributes(
			attribute.String(attrDbElasticsearchClusterName, id),
		)
	}
}

// RecordNodeName propagate the node name provided by Elastic Cloud via the X-Found-Handling-Instance header.
func (i *ElasticsearchOpenTelemetry) RecordNodeName(ctx context.Context, name string) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.SetAttributes(
			attribute.String(attrDbElasticsearchNodeName, name),
		)
	}
}

// RecordPathPart sets the couple for a specific path part.
// An index placed in the path would translate to `db.elasticsearch.path_parts.index`.
func (i *ElasticsearchOpenTelemetry) RecordPathPart(ctx context.Context, pathPart, value string) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.SetAttributes(attribute.String(attrPathParts+pathPart, value))
	}
}

// BeforeRequest noop for interface.
func (i *ElasticsearchOpenTelemetry) BeforeRequest(req *http.Request, endpoint string) {}

// AfterRequest enrich the span with the available data from the request.
func (i *ElasticsearchOpenTelemetry) AfterRequest(req *http.Request, system, endpoint string) {
	span := trace.SpanFromContext(req.Context())
	if span.IsRecording() {
		span.SetAttributes(
			attribute.String(attrDbSystem, system),
			attribute.String(attrDbOperation, endpoint),
			attribute.String(attrHttpRequestMethod, req.Method),
			attribute.String(attrUrlFull, req.URL.String()),
			attribute.String(attrServerAddress, req.URL.Hostname()),
		)
		if value, err := strconv.ParseInt(req.URL.Port(), 10, 32); err == nil {
			span.SetAttributes(attribute.Int64(attrServerPort, value))
		}
	}
}