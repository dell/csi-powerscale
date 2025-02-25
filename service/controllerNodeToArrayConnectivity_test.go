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
	"fmt"
	"io"
	"net/http"
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

// TODO: This test also seems to be failing.
func TestQueryArrayStatus_Mock_IoReadAll(t *testing.T) {
	tests := []struct {
		name            string
		ctx             context.Context
		url             string
		lastAttempt     int64
		ReadAllErr      error
		wantArrayStatus bool
		wantErr         bool
	}{
		{
			name:            "Unmarshal json - connectivity is broken",
			ctx:             context.Background(),
			url:             "http://example.com/api",
			lastAttempt:     1660000000,
			ReadAllErr:      nil,
			wantArrayStatus: false,
			wantErr:         false,
		},
		{
			name:            "Unmarshal json - failed to read API response",
			ctx:             context.Background(),
			url:             "http://example.com/api",
			lastAttempt:     1660000000,
			ReadAllErr:      errors.New("failed to read API response"),
			wantArrayStatus: false,
			wantErr:         true,
		},
		{
			name:            "Unmarshal json - connectivity is ok",
			ctx:             context.Background(),
			url:             "http://example.com/api",
			lastAttempt:     time.Now().Unix(),
			ReadAllErr:      nil,
			wantArrayStatus: false,
			wantErr:         false,
		},
		{
			name:            "Unmarshal json - connectivity is broken",
			ctx:             context.Background(),
			url:             "http://example.com/api",
			lastAttempt:     1560000002,
			ReadAllErr:      nil,
			wantArrayStatus: false,
			wantErr:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalGetIoReadAll := GetIoReadAll
			GetIoReadAll = func(_ io.Reader) ([]byte, error) {
				return []byte(`{
					"lastSuccess": 1560000000,
					"lastAttempt": ` + fmt.Sprint(tt.lastAttempt) + `
				}`), tt.ReadAllErr
			}
			defer func() {
				GetIoReadAll = originalGetIoReadAll
			}()

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
