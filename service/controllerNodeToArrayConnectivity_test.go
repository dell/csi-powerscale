// Copyright © 2025-2026 Dell Inc. or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//      http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	_ "net/http/pprof" // #nosec G108
	"testing"
	"time"
)

func TestQueryArrayStatus_UnsupportedScheme(t *testing.T) {
	s := &service{}
	got, err := s.queryArrayStatus(context.Background(), "ftp://example.com/api")
	if err == nil {
		t.Errorf("expected error for unsupported scheme, got nil")
	}
	if got {
		t.Errorf("expected false for unsupported scheme")
	}
}

func TestQueryArrayStatus_ConnectivityBroken_NotStale(t *testing.T) {
	originalGetIoReadAll := GetIoReadAll
	originalGetTimeNow := getTimeNow
	originalGetPollingFrequency := getPollingFrequency
	defer func() {
		GetIoReadAll = originalGetIoReadAll
		getTimeNow = originalGetTimeNow
		getPollingFrequency = originalGetPollingFrequency
	}()

	body := `{"lastSuccess": 1560000000, "lastAttempt": 1560000015}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(body))
	}))
	defer server.Close()

	GetIoReadAll = func(_ io.Reader) ([]byte, error) {
		return []byte(body), nil
	}
	getTimeNow = func() time.Time {
		return time.Unix(1560000016, 0)
	}
	getPollingFrequency = func(_ context.Context) int64 {
		return 10
	}

	s := &service{}
	got, err := s.queryArrayStatus(context.Background(), server.URL)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if got {
		t.Errorf("expected false for broken connectivity")
	}
}

func TestQueryArrayStatus(t *testing.T) {
	tests := []struct {
		name            string
		ctx             context.Context
		url             string
		wantArrayStatus bool
		wantErr         bool
	}{
		{
			name:            "Failed to unmarshal json",
			ctx:             context.Background(),
			url:             "http://example.com/api",
			wantArrayStatus: false,
			wantErr:         true,
		},
		{
			name: "Context cancelled",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			}(),
			url:             "http://example.com/api",
			wantArrayStatus: false,
			wantErr:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &service{}
			got, err := s.queryArrayStatus(tt.ctx, tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("queryArrayStatus() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.wantArrayStatus {
				t.Errorf("queryArrayStatus() = %v, want %v", got, tt.wantArrayStatus)
			}
		})
	}
}

func TestQueryArrayStatus_HttpRequest_Error(t *testing.T) {
	originalGetHTTPNewRequestWithContext := GetHTTPNewRequestWithContext
	GetHTTPNewRequestWithContext = func(_ context.Context, _, _ string, _ io.Reader) (*http.Request, error) {
		return nil, errors.New("failed to create request")
	}
	defer func() {
		GetHTTPNewRequestWithContext = originalGetHTTPNewRequestWithContext
	}()

	tests := []struct {
		name            string
		ctx             context.Context
		url             string
		wantArrayStatus bool
		wantErr         bool
	}{
		{
			name:            "Failed to create request for API",
			ctx:             context.Background(),
			url:             "",
			wantArrayStatus: false,
			wantErr:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &service{}
			got, err := s.queryArrayStatus(tt.ctx, tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("queryArrayStatus() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.wantArrayStatus {
				t.Errorf("queryArrayStatus() = %v, want %v", got, tt.wantArrayStatus)
			}
		})
	}
}

func TestQueryArrayStatus_Invoke_Panic(t *testing.T) {
	originalGetHTTPNewRequestWithContext := GetHTTPNewRequestWithContext
	GetHTTPNewRequestWithContext = func(_ context.Context, _, _ string, _ io.Reader) (*http.Request, error) {
		panic("test panic")
	}
	defer func() {
		GetHTTPNewRequestWithContext = originalGetHTTPNewRequestWithContext
	}()

	tests := []struct {
		name            string
		ctx             context.Context
		url             string
		wantArrayStatus bool
		wantErr         bool
	}{
		{
			name:            "Test panic",
			ctx:             context.Background(),
			url:             "",
			wantArrayStatus: false,
			wantErr:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &service{}
			got, err := s.queryArrayStatus(tt.ctx, tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("queryArrayStatus() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.wantArrayStatus {
				t.Errorf("queryArrayStatus() = %v, want %v", got, tt.wantArrayStatus)
			}
		})
	}
}

func TestQueryArrayStatus_Mock_IoReadAll(t *testing.T) {
	originalGetIoReadAll := GetIoReadAll
	originalGetTimeNow := getTimeNow
	originalSetPollingFrequency := getPollingFrequency
	after := func() {
		GetIoReadAll = originalGetIoReadAll
		getTimeNow = originalGetTimeNow
		getPollingFrequency = originalSetPollingFrequency
	}

	tests := []struct {
		name            string
		body            string
		readErr         error
		unmarshalErr    bool
		lastAttempt     int64
		lastSuccess     int64
		currentTime     int64
		tolerance       int64
		wantArrayStatus bool
		wantErr         bool
	}{
		{
			name: "Connectivity is ok",
			body: `{
				"lastSuccess": 1560000000,
				"lastAttempt": 1560000002
			}`,
			readErr:         nil,
			unmarshalErr:    false,
			currentTime:     1560000003,
			tolerance:       10,
			wantArrayStatus: true,
			wantErr:         false,
		},
		{
			name: "Connectivity is broken due to stale attempt",
			body: `{
				"lastSuccess": 1560000000,
				"lastAttempt": 1560000002
			}`,
			readErr:         nil,
			unmarshalErr:    false,
			currentTime:     1560000030,
			tolerance:       10,
			wantArrayStatus: false,
			wantErr:         false,
		},
		{
			name:            "Unmarshal error",
			body:            `invalid-json`,
			readErr:         nil,
			unmarshalErr:    true,
			currentTime:     1560000003,
			tolerance:       10,
			wantArrayStatus: false,
			wantErr:         true,
		},
		{
			name:            "Read error",
			body:            ``,
			readErr:         errors.New("read error"),
			unmarshalErr:    false,
			currentTime:     1560000003,
			tolerance:       10,
			wantArrayStatus: false,
			wantErr:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			GetIoReadAll = originalGetIoReadAll
			getTimeNow = originalGetTimeNow
			getPollingFrequency = originalSetPollingFrequency

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			defer after()
			GetIoReadAll = func(_ io.Reader) ([]byte, error) {
				return []byte(tt.body), tt.readErr
			}
			getTimeNow = func() time.Time {
				return time.Unix(tt.currentTime, 0)
			}
			getPollingFrequency = func(_ context.Context) int64 {
				return tt.tolerance
			}

			s := &service{}
			got, err := s.queryArrayStatus(context.Background(), server.URL)
			if (err != nil) != tt.wantErr {
				t.Errorf("queryArrayStatus() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.wantArrayStatus {
				t.Errorf("queryArrayStatus() = %v, want %v", got, tt.wantArrayStatus)
			}
		})
	}
}

// TestQueryArrayStatus_AuthorizationHeader verifies bearer token propagation to the podmon API.
func TestQueryArrayStatus_AuthorizationHeader(t *testing.T) {
	originalGetTimeNow := getTimeNow
	originalSetPollingFrequency := getPollingFrequency
	originalPodmonToken := PodmonAPIToken
	defer func() {
		getTimeNow = originalGetTimeNow
		getPollingFrequency = originalSetPollingFrequency
		PodmonAPIToken = originalPodmonToken
	}()

	getTimeNow = func() time.Time {
		return time.Unix(1000, 0)
	}
	getPollingFrequency = func(_ context.Context) int64 {
		return 10
	}

	tests := []struct {
		name       string
		token      string
		wantHeader string
	}{
		{
			name:       "token configured",
			token:      "test-token",
			wantHeader: "Bearer test-token",
		},
		{
			name:       "token not configured",
			token:      "",
			wantHeader: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotAuthHeader string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotAuthHeader = r.Header.Get("Authorization")
				_, _ = w.Write([]byte(`{"lastSuccess":995,"lastAttempt":1000}`))
			}))
			defer server.Close()

			PodmonAPIToken = tt.token
			s := &service{}
			connected, err := s.queryArrayStatus(context.Background(), server.URL)
			if err != nil {
				t.Fatalf("queryArrayStatus() unexpected error: %v", err)
			}
			if !connected {
				t.Fatalf("queryArrayStatus() connected = false, want true")
			}
			if gotAuthHeader != tt.wantHeader {
				t.Fatalf("Authorization header = %q, want %q", gotAuthHeader, tt.wantHeader)
			}
		})
	}
}

func TestQueryArrayStatus_ReadBodyError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	originalGetIoReadAll := GetIoReadAll
	GetIoReadAll = func(_ io.Reader) ([]byte, error) {
		return nil, errors.New("failed to read body")
	}
	defer func() { GetIoReadAll = originalGetIoReadAll }()

	s := &service{}
	got, err := s.queryArrayStatus(context.Background(), server.URL)
	if err == nil || got {
		t.Fatalf("queryArrayStatus() = %v, %v; want false and a read error", got, err)
	}
}

func TestQueryArrayStatus_StaleConnectivity(t *testing.T) {
	now := time.Now().Unix()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Last attempt is recent, but the last success is older than the tolerance.
		_, _ = w.Write([]byte(fmt.Sprintf(`{"lastSuccess":%d,"lastAttempt":%d}`, now-3600, now)))
	}))
	defer server.Close()

	s := &service{}
	got, err := s.queryArrayStatus(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("queryArrayStatus() unexpected error: %v", err)
	}
	if got {
		t.Error("queryArrayStatus() = true; want false when the last success is outside the tolerance")
	}
}
