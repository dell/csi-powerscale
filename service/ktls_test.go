// Copyright © 2026 Dell Inc. or its subsidiaries. All Rights Reserved.
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
	"os"
	"testing"

	"github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/common/constants"
	"github.com/stretchr/testify/assert"
)

func TestCheckKernelTLSSupport(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		statFunc       func(string) (os.FileInfo, error)
		expectedResult bool
		expectError    bool
	}{
		{
			name: "Kernel TLS module exists",
			statFunc: func(_ string) (os.FileInfo, error) {
				return nil, nil // File exists
			},
			expectedResult: true,
			expectError:    false,
		},
		{
			name: "Kernel TLS module does not exist",
			statFunc: func(_ string) (os.FileInfo, error) {
				return nil, os.ErrNotExist
			},
			expectedResult: false,
			expectError:    false,
		},
		{
			name: "Error checking kernel TLS module",
			statFunc: func(_ string) (os.FileInfo, error) {
				return nil, errors.New("permission denied")
			},
			expectedResult: false,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original and restore after test
			originalStatFunc := statFunc
			defer func() { statFunc = originalStatFunc }()

			statFunc = tt.statFunc

			result, err := CheckKernelTLSSupport(ctx)
			assert.Equal(t, tt.expectedResult, result)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCheckTLSHandshakeDaemon(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		statFunc       func(string) (os.FileInfo, error)
		lookPathFunc   func(string) (string, error)
		expectedResult bool
		expectError    bool
	}{
		{
			name: "tlshd found at default path",
			statFunc: func(_ string) (os.FileInfo, error) {
				return nil, nil // File exists
			},
			lookPathFunc: func(_ string) (string, error) {
				return "", errors.New("not found")
			},
			expectedResult: true,
			expectError:    false,
		},
		{
			name: "tlshd found in PATH",
			statFunc: func(_ string) (os.FileInfo, error) {
				return nil, os.ErrNotExist
			},
			lookPathFunc: func(_ string) (string, error) {
				return "/usr/local/bin/tlshd", nil
			},
			expectedResult: true,
			expectError:    false,
		},
		{
			name: "tlshd not found anywhere",
			statFunc: func(_ string) (os.FileInfo, error) {
				return nil, os.ErrNotExist
			},
			lookPathFunc: func(_ string) (string, error) {
				return "", errors.New("executable file not found in $PATH")
			},
			expectedResult: false,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save originals and restore after test
			originalStatFunc := statFunc
			originalLookPathFunc := lookPathFunc
			defer func() {
				statFunc = originalStatFunc
				lookPathFunc = originalLookPathFunc
			}()

			statFunc = tt.statFunc
			lookPathFunc = tt.lookPathFunc

			result, err := CheckTLSHandshakeDaemon(ctx)
			assert.Equal(t, tt.expectedResult, result)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetTLSCapabilityStatus(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name               string
		statFunc           func(string) (os.FileInfo, error)
		lookPathFunc       func(string) (string, error)
		expectedKernelTLS  bool
		expectedTLSHD      bool
		expectedTLSCapable bool
	}{
		{
			name: "Both kernel TLS and tlshd available",
			statFunc: func(_ string) (os.FileInfo, error) {
				return nil, nil // Both paths exist
			},
			lookPathFunc: func(_ string) (string, error) {
				return "/usr/sbin/tlshd", nil
			},
			expectedKernelTLS:  true,
			expectedTLSHD:      true,
			expectedTLSCapable: true,
		},
		{
			name: "Kernel TLS available but tlshd missing",
			statFunc: func(path string) (os.FileInfo, error) {
				if path == constants.KernelTLSModulePath {
					return nil, nil // Kernel module exists
				}
				return nil, os.ErrNotExist // tlshd not at default path
			},
			lookPathFunc: func(_ string) (string, error) {
				return "", errors.New("not found")
			},
			expectedKernelTLS:  true,
			expectedTLSHD:      false,
			expectedTLSCapable: false,
		},
		{
			name: "Kernel TLS missing but tlshd available",
			statFunc: func(path string) (os.FileInfo, error) {
				if path == constants.KernelTLSModulePath {
					return nil, os.ErrNotExist // Kernel module missing
				}
				return nil, nil // tlshd exists
			},
			lookPathFunc: func(_ string) (string, error) {
				return "/usr/sbin/tlshd", nil
			},
			expectedKernelTLS:  false,
			expectedTLSHD:      true,
			expectedTLSCapable: false,
		},
		{
			name: "Neither kernel TLS nor tlshd available",
			statFunc: func(_ string) (os.FileInfo, error) {
				return nil, os.ErrNotExist
			},
			lookPathFunc: func(_ string) (string, error) {
				return "", errors.New("not found")
			},
			expectedKernelTLS:  false,
			expectedTLSHD:      false,
			expectedTLSCapable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save originals and restore after test
			originalStatFunc := statFunc
			originalLookPathFunc := lookPathFunc
			defer func() {
				statFunc = originalStatFunc
				lookPathFunc = originalLookPathFunc
			}()

			statFunc = tt.statFunc
			lookPathFunc = tt.lookPathFunc

			result := GetTLSCapabilityStatus(ctx)
			assert.Equal(t, tt.expectedKernelTLS, result.KernelTLSSupported)
			assert.Equal(t, tt.expectedTLSHD, result.TLSHDAvailable)
			assert.Equal(t, tt.expectedTLSCapable, result.TLSCapable)
		})
	}
}

func TestGetTLSCapabilityEventReason(t *testing.T) {
	tests := []struct {
		name           string
		result         TLSCapabilityResult
		expectedReason string
	}{
		{
			name: "Kernel TLS not supported",
			result: TLSCapabilityResult{
				KernelTLSSupported: false,
				TLSHDAvailable:     true,
				TLSCapable:         false,
			},
			expectedReason: constants.TLSEventReasonKernelNotSupported,
		},
		{
			name: "tlshd daemon missing",
			result: TLSCapabilityResult{
				KernelTLSSupported: true,
				TLSHDAvailable:     false,
				TLSCapable:         false,
			},
			expectedReason: constants.TLSEventReasonDaemonMissing,
		},
		{
			name: "Both available (no error)",
			result: TLSCapabilityResult{
				KernelTLSSupported: true,
				TLSHDAvailable:     true,
				TLSCapable:         true,
			},
			expectedReason: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reason := GetTLSCapabilityEventReason(tt.result)
			assert.Equal(t, tt.expectedReason, reason)
		})
	}
}

func TestGetTLSCapabilityErrorMessage(t *testing.T) {
	tests := []struct {
		name           string
		result         TLSCapabilityResult
		expectNonEmpty bool
	}{
		{
			name: "Kernel TLS not supported",
			result: TLSCapabilityResult{
				KernelTLSSupported: false,
				TLSHDAvailable:     true,
			},
			expectNonEmpty: true,
		},
		{
			name: "tlshd daemon missing",
			result: TLSCapabilityResult{
				KernelTLSSupported: true,
				TLSHDAvailable:     false,
			},
			expectNonEmpty: true,
		},
		{
			name: "Both available",
			result: TLSCapabilityResult{
				KernelTLSSupported: true,
				TLSHDAvailable:     true,
			},
			expectNonEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := GetTLSCapabilityErrorMessage(tt.result)
			if tt.expectNonEmpty {
				assert.NotEmpty(t, msg)
			} else {
				assert.Empty(t, msg)
			}
		})
	}
}

func TestValidateTLSCapabilityForMount(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name              string
		transportSecurity string
		statFunc          func(string) (os.FileInfo, error)
		lookPathFunc      func(string) (string, error)
		expectTLSCapable  bool
	}{
		{
			name:              "mTLS with TLS capable node",
			transportSecurity: "mtls",
			statFunc: func(_ string) (os.FileInfo, error) {
				return nil, nil
			},
			lookPathFunc: func(_ string) (string, error) {
				return "/usr/sbin/tlshd", nil
			},
			expectTLSCapable: true,
		},
		{
			name:              "mTLS with non-TLS capable node",
			transportSecurity: "mtls",
			statFunc: func(_ string) (os.FileInfo, error) {
				return nil, os.ErrNotExist
			},
			lookPathFunc: func(_ string) (string, error) {
				return "", errors.New("not found")
			},
			expectTLSCapable: false,
		},
		{
			name:              "Non-mTLS skips validation",
			transportSecurity: "none",
			statFunc: func(_ string) (os.FileInfo, error) {
				return nil, os.ErrNotExist // Would fail if checked
			},
			lookPathFunc: func(_ string) (string, error) {
				return "", errors.New("not found")
			},
			expectTLSCapable: true, // Skipped, returns true
		},
		{
			name:              "Empty transport security skips validation",
			transportSecurity: "",
			statFunc: func(_ string) (os.FileInfo, error) {
				return nil, os.ErrNotExist
			},
			lookPathFunc: func(_ string) (string, error) {
				return "", errors.New("not found")
			},
			expectTLSCapable: true, // Skipped, returns true
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save originals and restore after test
			originalStatFunc := statFunc
			originalLookPathFunc := lookPathFunc
			defer func() {
				statFunc = originalStatFunc
				lookPathFunc = originalLookPathFunc
			}()

			statFunc = tt.statFunc
			lookPathFunc = tt.lookPathFunc

			result, err := ValidateTLSCapabilityForMount(ctx, tt.transportSecurity)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectTLSCapable, result.TLSCapable)
		})
	}
}

func TestGetTLSCapabilityStatusRecordsKernelError(t *testing.T) {
	originalStat := statFunc
	originalLookPath := lookPathFunc
	defer func() {
		statFunc = originalStat
		lookPathFunc = originalLookPath
	}()

	statFunc = func(_ string) (os.FileInfo, error) {
		return nil, errors.New("permission denied")
	}
	lookPathFunc = func(_ string) (string, error) {
		return "", errors.New("not found")
	}

	result := GetTLSCapabilityStatus(context.Background())
	assert.False(t, result.KernelTLSSupported)
	assert.False(t, result.TLSCapable)
	assert.Equal(t, "permission denied", result.KernelError)
}

func TestLogTLSReadiness(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name             string
		statFunc         func(string) (os.FileInfo, error)
		lookPathFunc     func(string) (string, error)
		expectTLSCapable bool
	}{
		{
			name: "TLS capable node logs success",
			statFunc: func(_ string) (os.FileInfo, error) {
				return nil, nil
			},
			lookPathFunc: func(_ string) (string, error) {
				return "/usr/sbin/tlshd", nil
			},
			expectTLSCapable: true,
		},
		{
			name: "Non-TLS capable node logs info (not error)",
			statFunc: func(_ string) (os.FileInfo, error) {
				return nil, os.ErrNotExist
			},
			lookPathFunc: func(_ string) (string, error) {
				return "", errors.New("not found")
			},
			expectTLSCapable: false,
		},
		{
			name: "Missing kernel TLS module logs info",
			statFunc: func(_ string) (os.FileInfo, error) {
				return nil, os.ErrNotExist
			},
			lookPathFunc: func(_ string) (string, error) {
				return "/usr/sbin/tlshd", nil
			},
			expectTLSCapable: false,
		},
		{
			name: "Missing tlshd daemon logs info",
			statFunc: func(path string) (os.FileInfo, error) {
				// Return error for the tlshd path to simulate missing daemon
				if path == "/host/usr/sbin/tlshd" {
					return nil, os.ErrNotExist
				}
				return nil, nil
			},
			lookPathFunc: func(_ string) (string, error) {
				// Return error for PATH check as well
				return "", errors.New("not found")
			},
			expectTLSCapable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save originals and restore after test
			originalStatFunc := statFunc
			originalLookPathFunc := lookPathFunc
			defer func() {
				statFunc = originalStatFunc
				lookPathFunc = originalLookPathFunc
			}()

			statFunc = tt.statFunc
			lookPathFunc = tt.lookPathFunc

			// LogTLSReadiness should never return an error or panic
			result := LogTLSReadiness(ctx)
			assert.Equal(t, tt.expectTLSCapable, result.TLSCapable)

			// Verify the function is non-fatal: it returns a result even when not TLS-capable
			assert.NotNil(t, result)
		})
	}
}
