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
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
)

// ConfigureTLS applies transport TLS settings in-place.
//
// This helper is useful when you want to configure a *http.Transport first and
// then wrap it in custom middleware that implements http.RoundTripper.
//
// When certificateFingerprint is set, certificate verification is performed by
// comparing SHA-256 digests of peer certificates against the fingerprint. In
// this mode caCert is ignored because the custom DialTLSContext bypasses
// standard CA verification. If you only need CA-based verification, leave
// certificateFingerprint empty.
func ConfigureTLS(transport *http.Transport, caCert []byte, certificateFingerprint string) error {
	if transport == nil {
		return errors.New("transport cannot be nil")
	}

	if certificateFingerprint != "" {
		fingerprint, err := hex.DecodeString(certificateFingerprint)
		if err != nil {
			return fmt.Errorf("invalid certificate fingerprint %q: %w", certificateFingerprint, err)
		}

		transport.DialTLSContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			d := tls.Dialer{Config: &tls.Config{InsecureSkipVerify: true}}
			conn, err := d.DialContext(ctx, network, addr)
			if err != nil {
				return nil, err
			}

			tlsConn := conn.(*tls.Conn)
			for _, cert := range tlsConn.ConnectionState().PeerCertificates {
				digest := sha256.Sum256(cert.Raw)
				if bytes.Equal(digest[:], fingerprint) {
					return tlsConn, nil
				}
			}
			_ = tlsConn.Close()
			return nil, fmt.Errorf("fingerprint mismatch, provided: %s", certificateFingerprint)
		}
	}

	if caCert != nil && certificateFingerprint == "" {
		if transport.TLSClientConfig == nil {
			transport.TLSClientConfig = &tls.Config{}
		}

		transport.TLSClientConfig.RootCAs = x509.NewCertPool()
		if ok := transport.TLSClientConfig.RootCAs.AppendCertsFromPEM(caCert); !ok {
			return errors.New("unable to add CA certificate")
		}
	}

	return nil
}
