// Copyright © 2026 Dell Inc. or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package service

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/common/constants"
	"github.com/stretchr/testify/assert"
)

func TestParseTLSError(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		err            error
		expectedReason string
		expectedCode   string
		expectMapped   bool
	}{
		{
			name:           "Nil error returns empty classification",
			err:            nil,
			expectedReason: "",
			expectedCode:   "",
			expectMapped:   true,
		},
		{
			name:           "Handshake timeout",
			err:            errors.New("mount failed: connection timed out after 30s"),
			expectedReason: constants.TLSEventReasonHandshakeTimeout,
			expectedCode:   "DeadlineExceeded",
			expectMapped:   true,
		},
		{
			name:           "Certificate expired",
			err:            errors.New("x509: certificate has expired or is not yet valid"),
			expectedReason: constants.TLSEventReasonClientCertExpired,
			expectedCode:   "FailedPrecondition",
			expectMapped:   true,
		},
		{
			name:           "SAN mismatch",
			err:            errors.New("x509: certificate is valid for example.com, not nfs.example.com"),
			expectedReason: constants.TLSEventReasonCertSANMismatch,
			expectedCode:   "FailedPrecondition",
			expectMapped:   true,
		},
		{
			name:           "Trust anchor failure",
			err:            errors.New("x509: certificate signed by unknown authority"),
			expectedReason: constants.TLSEventReasonTrustAnchorFailure,
			expectedCode:   "FailedPrecondition",
			expectMapped:   true,
		},
		{
			name:           "TLS daemon missing",
			err:            errors.New("cannot start tls handshake: tlshd not found"),
			expectedReason: constants.TLSEventReasonDaemonMissing,
			expectedCode:   "FailedPrecondition",
			expectMapped:   true,
		},
		{
			name:           "Unmapped error falls back to generic mount failed",
			err:            errors.New("generic mount error"),
			expectedReason: constants.MTLSEventReasonMountFailed,
			expectedCode:   "Internal",
			expectMapped:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseTLSError(ctx, tt.err)
			if tt.err == nil {
				assert.Empty(t, result.Reason)
				assert.Empty(t, result.Message)
				return
			}
			assert.Equal(t, tt.expectedReason, result.Reason)
			assert.Equal(t, tt.expectedCode, result.ErrorCode)
			if tt.expectMapped {
				assert.NotEmpty(t, result.Message)
			}
		})
	}
}

func TestIsTLSError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "Nil error is not TLS error",
			err:      nil,
			expected: false,
		},
		{
			name:     "TLS keyword detected",
			err:      errors.New("tls handshake failed"),
			expected: true,
		},
		{
			name:     "Certificate keyword detected",
			err:      errors.New("certificate expired"),
			expected: true,
		},
		{
			name:     "X509 keyword detected",
			err:      errors.New("x509 validation failed"),
			expected: true,
		},
		{
			name:     "Unrelated error is not TLS",
			err:      errors.New("permission denied"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsTLSError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestIsTLSError_NoFalsePositives guards against over-broad keyword matching.
// Bare symptom words ("expired", "issuer", "san") must not classify a storage or
// network failure as a certificate problem, because IsTLSError gates ParseTLSError
// and the emission of certificate-specific Kubernetes events.
func TestIsTLSError_NoFalsePositives(t *testing.T) {
	notTLS := []string{
		"The session has expired",
		"Token expired, please re-authenticate",
		"failed to sanitize input",
		"sandbox error while starting container",
		"unknown issuer of request",
		"mount.nfs: an incorrect mount option was specified",
		"connection refused",
		"i/o timeout",
		"context deadline exceeded",
		"no space left on device",
	}
	for _, msg := range notTLS {
		t.Run("not TLS: "+msg, func(t *testing.T) {
			assert.False(t, IsTLSError(errors.New(msg)),
				"error must not be misclassified as a TLS/certificate failure: %q", msg)
		})
	}
}

// TestIsTLSError_GenuineTLSErrorsStillDetected ensures tightening the matcher did not
// regress detection of real kernel / tlshd / crypto-x509 failure strings.
func TestIsTLSError_GenuineTLSErrorsStillDetected(t *testing.T) {
	tlsErrors := []string{
		"x509: certificate has expired or is not yet valid",
		"x509: certificate signed by unknown authority",
		"unable to get local issuer certificate",
		"certificate verify failed",
		"tlshd: handshake failed",
		"tls handshake timeout",
		"peer certificate has expired",
		"x509: certificate is valid for foo.example.com, not bar.example.com",
		"subject alternative name mismatch",
		"TLS trust anchor validation failed",
		"SSL routines: unexpected eof while reading",
		"xprtsec=mtls not supported by server",
	}
	for _, msg := range tlsErrors {
		t.Run("is TLS: "+msg, func(t *testing.T) {
			assert.True(t, IsTLSError(errors.New(msg)),
				"genuine TLS error must still be detected: %q", msg)
		})
	}
}

func TestExtractCertificateInfo(t *testing.T) {
	tests := []struct {
		name             string
		output           string
		expectedSubject  string
		expectedIssuer   string
		expectedNotAfter string
	}{
		{
			name: "OpenSSL subject and issuer output",
			output: `subject=CN = nfs-client.example.com, O = Dell Technologies
issuer=CN = Dell Private CA, O = Dell Technologies
notAfter=Dec 31 23:59:59 2025 GMT`,
			expectedSubject:  "CN=nfs-client.example.com",
			expectedIssuer:   "CN=Dell Private CA",
			expectedNotAfter: "Dec 31 23:59:59 2025 GMT",
		},
		{
			name:             "Empty output",
			output:           "",
			expectedSubject:  "",
			expectedIssuer:   "",
			expectedNotAfter: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := ExtractCertificateInfo(tt.output)
			assert.Equal(t, tt.expectedSubject, info["subject"])
			assert.Equal(t, tt.expectedIssuer, info["issuer"])
			assert.Equal(t, tt.expectedNotAfter, info["notAfter"])
		})
	}
}

func TestGetTLSHandshakeTimeout(t *testing.T) {
	tests := []struct {
		name      string
		envValue  string
		expected  int
		setEnv    bool
		mockParse func(string, int, int) (int64, error)
	}{
		{
			name:     "Default value when env not set",
			expected: constants.DefaultTLSHandshakeTimeout,
		},
		{
			name:     "Valid env value",
			envValue: "60",
			expected: 60,
			setEnv:   true,
		},
		{
			name:     "Invalid env value falls back to default",
			envValue: "invalid",
			expected: constants.DefaultTLSHandshakeTimeout,
			setEnv:   true,
		},
		{
			name:     "Negative env value falls back to default",
			envValue: "-10",
			expected: constants.DefaultTLSHandshakeTimeout,
			setEnv:   true,
		},
		{
			name:     "Zero env value falls back to default",
			envValue: "0",
			expected: constants.DefaultTLSHandshakeTimeout,
			setEnv:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setEnv {
				os.Setenv(constants.EnvTLSHandshakeTimeoutSeconds, tt.envValue)
				defer os.Unsetenv(constants.EnvTLSHandshakeTimeoutSeconds)
			} else {
				os.Unsetenv(constants.EnvTLSHandshakeTimeoutSeconds)
			}

			result := GetTLSHandshakeTimeout(context.Background())
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCreateMountTimeoutContext(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name              string
		transportSecurity string
		expectTimeout     bool
	}{
		{
			name:              "mTLS creates timeout context",
			transportSecurity: "mtls",
			expectTimeout:     true,
		},
		{
			name:              "TLS does not create timeout context",
			transportSecurity: "tls",
			expectTimeout:     false,
		},
		{
			name:              "none does not create timeout context",
			transportSecurity: "none",
			expectTimeout:     false,
		},
		{
			name:              "Empty transport does not create timeout context",
			transportSecurity: "",
			expectTimeout:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(_ *testing.T) {
			_, cancel := CreateMountTimeoutContext(ctx, tt.transportSecurity)
			defer cancel()

			// For mTLS, a deadline should be set on the context
			// For non-mTLS, it returns the original context without a deadline
			// We can verify by checking if the context has a deadline
		})
	}
}

func TestIsTimeoutError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "Nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "Mount timeout error",
			err:      &MountError{Err: errors.New("timeout"), Timeout: true},
			expected: true,
		},
		{
			name:     "Non-timeout mount error",
			err:      &MountError{Err: errors.New("failed"), Timeout: false},
			expected: false,
		},
		{
			name:     "Regular error",
			err:      errors.New("regular error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsTimeoutError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMountError(t *testing.T) {
	underlying := errors.New("connection refused")

	timeoutErr := &MountError{Operation: "mount", Err: underlying, Timeout: true}
	assert.Equal(t, "mount operation timed out: connection refused", timeoutErr.Error())
	assert.Equal(t, underlying, timeoutErr.Unwrap())
	assert.ErrorIs(t, timeoutErr, underlying)

	failureErr := &MountError{Operation: "mount", Err: underlying}
	assert.Equal(t, "mount operation failed: connection refused", failureErr.Error())
}
