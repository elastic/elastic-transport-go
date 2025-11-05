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
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

func Test_mergeInterceptors(t *testing.T) {
	success := func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     req.Header.Clone(),
			Body:       io.NopCloser(strings.NewReader(`{}`)),
		}, nil
	}

	const (
		testInterceptReqHeader  = "X-TEST-INTERCEPT-REQ"
		testInterceptRespHeader = "X-TEST-INTERCEPT-RESP"
	)

	dummyInterceptorFactory := func(num int) InterceptorFunc {
		return func(next RoundTripFunc) RoundTripFunc {
			return func(r *http.Request) (*http.Response, error) {
				r.Header.Add(testInterceptReqHeader, strconv.Itoa(num))
				resp, err := next(r)
				resp.Header.Add(testInterceptRespHeader, strconv.Itoa(num))
				return resp, err
			}
		}
	}

	validateReqRespH := func(expectReq, expectResp string) func(t *testing.T, resp *http.Response) {
		return func(t *testing.T, resp *http.Response) {
			if strings.Join(resp.Header.Values(testInterceptReqHeader), ",") != expectReq {
				t.Error("expected header value", resp.Header.Values(testInterceptReqHeader))
			}
			if strings.Join(resp.Header.Values(testInterceptRespHeader), ",") != expectResp {
				t.Error("expected header value", resp.Header.Values(testInterceptRespHeader))
			}
		}
	}

	type args struct {
		interceptors []InterceptorFunc
	}
	tests := []struct {
		name             string
		args             args
		wantErr          bool
		validateResponse func(t *testing.T, resp *http.Response)
	}{
		{
			name:    "interceptor array nil",
			args:    args{interceptors: nil},
			wantErr: false,
		},
		{
			name:    "interceptor array empty",
			args:    args{interceptors: []InterceptorFunc{}},
			wantErr: false,
		},
		{
			name:             "interceptor array nil interceptor",
			args:             args{interceptors: []InterceptorFunc{nil, dummyInterceptorFactory(1)}},
			wantErr:          false,
			validateResponse: validateReqRespH("1", "1"),
		},
		{
			name:             "interceptor array 1 interceptor",
			args:             args{interceptors: []InterceptorFunc{dummyInterceptorFactory(1)}},
			wantErr:          false,
			validateResponse: validateReqRespH("1", "1"),
		},
		{
			name: "interceptor array many interceptors",
			args: args{interceptors: []InterceptorFunc{
				dummyInterceptorFactory(1), dummyInterceptorFactory(2), dummyInterceptorFactory(3)}},
			wantErr:          false,
			validateResponse: validateReqRespH("1,2,3", "3,2,1"),
		},
		{
			name: "interceptor returns error",
			args: args{interceptors: []InterceptorFunc{
				func(next RoundTripFunc) RoundTripFunc {
					return func(r *http.Request) (*http.Response, error) {
						return nil, errors.New("test error")
					}
				},
			}},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, "http://localhost", nil)
			if err != nil {
				t.Fatal(err)
			}
			got, err := mergeInterceptors(tt.args.interceptors)(success)(req)
			if (err != nil) != tt.wantErr {
				t.Errorf("mergeInterceptors() error = %v, wantErr %v", err, tt.wantErr)
			} else {
				if tt.validateResponse != nil {
					tt.validateResponse(t, got)
				}
			}

		})
	}
}
