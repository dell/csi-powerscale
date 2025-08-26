/*
 *
 * Copyright © 2021-2024 Dell Inc. or its subsidiaries. All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

/*
 Copyright (c) 2025 Dell Inc, or its subsidiaries.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

      http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package service

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	_ "net/http/pprof" // #nosec G108
	"testing"
	"time"

	"golang.org/x/net/context"
)

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
