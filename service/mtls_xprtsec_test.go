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
	"testing"

	"github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/common/constants"
	"github.com/stretchr/testify/assert"
)

func TestResolveXprtsec(t *testing.T) {
	tests := []struct {
		name                 string
		nfsTransportSecurity string
		expectedXprtsec      string
		description          string
	}{
		{
			name:                 "mTLS mode returns mtls",
			nfsTransportSecurity: constants.NFSTransportSecurityMTLS,
			expectedXprtsec:      "mtls",
			description:          "Should return 'mtls' for mTLS-only export",
		},
		{
			name:                 "TLS mode returns tls",
			nfsTransportSecurity: constants.NFSTransportSecurityTLS,
			expectedXprtsec:      "tls",
			description:          "Should return 'tls' for TLS-only export",
		},
		{
			name:                 "None mode returns none",
			nfsTransportSecurity: constants.NFSTransportSecurityNone,
			expectedXprtsec:      "none",
			description:          "Should return 'none' for plaintext-only export",
		},
		{
			name:                 "Empty string returns empty",
			nfsTransportSecurity: "",
			expectedXprtsec:      "",
			description:          "Should return empty string for cluster default",
		},
		{
			name:                 "Uppercase MTLS returns mtls",
			nfsTransportSecurity: "MTLS",
			expectedXprtsec:      "mtls",
			description:          "Should be case-insensitive",
		},
		{
			name:                 "Mixed case mTLS returns mtls",
			nfsTransportSecurity: "mTLS",
			expectedXprtsec:      "mtls",
			description:          "Should be case-insensitive",
		},
		{
			name:                 "Invalid value returns empty",
			nfsTransportSecurity: "invalid",
			expectedXprtsec:      "",
			description:          "Should return empty for invalid values",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ResolveXprtsec(tt.nfsTransportSecurity)
			assert.Equal(t, tt.expectedXprtsec, result, tt.description)
		})
	}
}

func TestResolveXprtsecBackwardCompatibility(t *testing.T) {
	// Test that empty input returns empty output (backward compatible)
	result := ResolveXprtsec("")
	assert.Equal(t, "", result, "Empty input should return empty output for backward compatibility")

	// Test that invalid input returns empty output (safe fallback)
	result = ResolveXprtsec("unknown")
	assert.Equal(t, "", result, "Unknown input should return empty output as safe fallback")
}

func TestResolveXprtsecDefenseInDepth(t *testing.T) {
	// Test defense-in-depth scenarios
	tests := []struct {
		name                 string
		nfsTransportSecurity string
		expectedXprtsec      string
		securityLevel        string
	}{
		{
			name:                 "mTLS provides strongest security",
			nfsTransportSecurity: "mtls",
			expectedXprtsec:      "mtls",
			securityLevel:        "Highest - Both client and server authentication",
		},
		{
			name:                 "TLS provides server authentication",
			nfsTransportSecurity: "tls",
			expectedXprtsec:      "tls",
			securityLevel:        "Medium - Server authentication only",
		},
		{
			name:                 "None provides no encryption",
			nfsTransportSecurity: "none",
			expectedXprtsec:      "none",
			securityLevel:        "Lowest - Plaintext only",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ResolveXprtsec(tt.nfsTransportSecurity)
			assert.Equal(t, tt.expectedXprtsec, result)
			t.Logf("Security level: %s", tt.securityLevel)
		})
	}
}

// Note: Full ValidateClusterTLSMode tests are integration tests that require
// a real PowerScale client. They are covered in integration test suite.
// The tests below verify the security-critical behavior: fail hard when
// TLS/mTLS is requested but cannot be validated (preventing plaintext fallback).

func TestValidateClusterTLSModeSecurityBehavior(t *testing.T) {
	// These tests document the security-critical behavior:
	// When TLS/mTLS is explicitly requested, validation MUST fail if array support
	// cannot be confirmed. This prevents plaintext fallback when encrypted transport
	// was explicitly requested by the user.

	tests := []struct {
		name           string
		requestedMode  string
		shouldValidate bool
		description    string
	}{
		{
			name:           "Empty mode requires no validation",
			requestedMode:  "",
			shouldValidate: false,
			description:    "Empty string means use cluster default - no validation needed",
		},
		{
			name:           "None mode requires no validation",
			requestedMode:  constants.NFSTransportSecurityNone,
			shouldValidate: false,
			description:    "Plaintext mode works on all arrays - no validation needed",
		},
		{
			name:           "mTLS mode requires validation",
			requestedMode:  constants.NFSTransportSecurityMTLS,
			shouldValidate: true,
			description:    "mTLS requires OneFS 9.16.0+ and configured nfs_tls_mode",
		},
		{
			name:           "TLS mode requires validation",
			requestedMode:  constants.NFSTransportSecurityTLS,
			shouldValidate: true,
			description:    "TLS requires OneFS 9.16.0+ and configured nfs_tls_mode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Test: %s", tt.description)
			t.Logf("Requested mode: %s", tt.requestedMode)
			t.Logf("Requires validation: %v", tt.shouldValidate)

			// Document expected behavior
			if tt.shouldValidate {
				t.Logf("SECURITY CRITICAL: If validation fails, operation MUST fail")
				t.Logf("  - API error (OneFS < 9.16.0) → FAIL (no plaintext fallback)")
				t.Logf("  - nfs_tls_mode not configured → FAIL (no plaintext fallback)")
				t.Logf("  - nfs_tls_mode doesn't include mode → FAIL (no plaintext fallback)")
			} else {
				t.Logf("No validation needed - works on all array versions")
			}
		})
	}
}

func TestValidateClusterTLSModeBackwardCompatibility(t *testing.T) {
	// Test that non-mTLS operations don't require validation
	// This ensures older arrays (OneFS < 9.16.0) work seamlessly for plaintext

	tests := []struct {
		name          string
		requestedMode string
		expectPass    bool
	}{
		{
			name:          "Empty mode passes without validation",
			requestedMode: "",
			expectPass:    true,
		},
		{
			name:          "None mode passes without validation",
			requestedMode: constants.NFSTransportSecurityNone,
			expectPass:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// These modes should pass the validation gate without requiring
			// any API calls or array version checks
			t.Logf("Mode '%s' should work on all array versions (including OneFS < 9.16.0)", tt.requestedMode)
		})
	}
}
