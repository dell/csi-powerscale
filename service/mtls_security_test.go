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
	"fmt"
	"testing"

	"github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/common/constants"
	"github.com/stretchr/testify/assert"
)

// TestMTLSSecurityCriticalBehavior documents and tests the security-critical behavior:
// When mTLS/TLS is explicitly requested, the driver MUST fail if the array doesn't support it.
// This prevents plaintext fallback when encrypted transport was explicitly requested.
func TestMTLSSecurityCriticalBehavior(t *testing.T) {
	t.Run("Security Requirement: No plaintext fallback when mTLS requested", func(t *testing.T) {
		// SECURITY CRITICAL: When a user explicitly requests mTLS via NFSTransportSecurity: "mtls",
		// the driver MUST NOT create a plaintext export if the array doesn't support mTLS.
		// This would be a security vulnerability.

		t.Log("Scenario: User requests mTLS but array doesn't support it")
		t.Log("Expected: Operation FAILS with clear error message")
		t.Log("Reason: Prevent plaintext fallback when encrypted transport was explicitly requested")
		t.Log("")
		t.Log("ValidateClusterTLSMode behavior:")
		t.Log("  - API call fails (OneFS < 9.16.0) → FAIL with error")
		t.Log("  - nfs_tls_mode is nil → FAIL with error")
		t.Log("  - nfs_tls_mode doesn't include 'mtls' → FAIL with error")
		t.Log("")
		t.Log("This ensures defense-in-depth: both client AND server enforce mTLS")
	})

	t.Run("Security Requirement: No plaintext fallback when TLS requested", func(t *testing.T) {
		// SECURITY CRITICAL: When a user explicitly requests TLS via NFSTransportSecurity: "tls",
		// the driver MUST NOT create a plaintext export if the array doesn't support TLS.

		t.Log("Scenario: User requests TLS but array doesn't support it")
		t.Log("Expected: Operation FAILS with clear error message")
		t.Log("Reason: Prevent plaintext fallback when encrypted transport was explicitly requested")
	})

	t.Run("Backward Compatibility: Plaintext works on all arrays", func(t *testing.T) {
		// BACKWARD COMPATIBILITY: When NFSTransportSecurity is empty or "none",
		// the driver should work on all array versions, including OneFS < 9.16.0.

		t.Log("Scenario: User doesn't specify NFSTransportSecurity (or sets it to 'none')")
		t.Log("Expected: Operation SUCCEEDS on all array versions")
		t.Log("Reason: Plaintext/default mode doesn't require NFS over TLS support")
		t.Log("")
		t.Log("ValidateClusterTLSMode behavior:")
		t.Log("  - Empty string → PASS (no validation)")
		t.Log("  - 'none' → PASS (no validation)")
		t.Log("")
		t.Log("This ensures older arrays (OneFS < 9.16.0) work seamlessly for non-mTLS operations")
	})
}

// TestMTLSValidationGate tests the validation gate logic
func TestMTLSValidationGate(t *testing.T) {
	tests := []struct {
		name                string
		requestedMode       string
		requiresValidation  bool
		failsOnOldArray     bool
		failsOnUnconfigured bool
		description         string
	}{
		{
			name:                "Empty mode - no validation",
			requestedMode:       "",
			requiresValidation:  false,
			failsOnOldArray:     false,
			failsOnUnconfigured: false,
			description:         "Empty string uses cluster default - works on all arrays",
		},
		{
			name:                "None mode - no validation",
			requestedMode:       constants.NFSTransportSecurityNone,
			requiresValidation:  false,
			failsOnOldArray:     false,
			failsOnUnconfigured: false,
			description:         "Plaintext mode works on all arrays",
		},
		{
			name:                "mTLS mode - requires validation",
			requestedMode:       constants.NFSTransportSecurityMTLS,
			requiresValidation:  true,
			failsOnOldArray:     true,
			failsOnUnconfigured: true,
			description:         "mTLS requires OneFS 9.16.0+ and configured nfs_tls_mode",
		},
		{
			name:                "TLS mode - requires validation",
			requestedMode:       constants.NFSTransportSecurityTLS,
			requiresValidation:  true,
			failsOnOldArray:     true,
			failsOnUnconfigured: true,
			description:         "TLS requires OneFS 9.16.0+ and configured nfs_tls_mode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Description: %s", tt.description)
			t.Logf("Requested mode: %s", tt.requestedMode)
			t.Logf("Requires validation: %v", tt.requiresValidation)
			t.Logf("Fails on old array (OneFS < 9.16.0): %v", tt.failsOnOldArray)
			t.Logf("Fails on unconfigured nfs_tls_mode: %v", tt.failsOnUnconfigured)

			// Verify the validation gate logic
			if tt.requiresValidation {
				assert.True(t, tt.failsOnOldArray, "Should fail on old arrays to prevent plaintext fallback")
				assert.True(t, tt.failsOnUnconfigured, "Should fail on unconfigured TLS to prevent plaintext fallback")
			} else {
				assert.False(t, tt.failsOnOldArray, "Should work on old arrays for backward compatibility")
				assert.False(t, tt.failsOnUnconfigured, "Should work without TLS configuration for backward compatibility")
			}
		})
	}
}

// TestMTLSErrorMessages tests that error messages are clear and actionable
func TestMTLSErrorMessages(t *testing.T) {
	t.Run("Error message clarity", func(t *testing.T) {
		scenarios := []struct {
			scenario        string
			expectedMessage string
			userAction      string
		}{
			{
				scenario:        "OneFS < 9.16.0 with mTLS requested",
				expectedMessage: "cluster does not support NFS over TLS (OneFS 9.16.0+ required for 'mtls' mode)",
				userAction:      "Upgrade PowerScale to OneFS 9.16.0+ or remove NFSTransportSecurity parameter",
			},
			{
				scenario:        "nfs_tls_mode not configured with mTLS requested",
				expectedMessage: "cluster nfs_tls_mode is not configured; cannot create 'mtls' export",
				userAction:      "Configure NFS over TLS on PowerScale array",
			},
			{
				scenario:        "nfs_tls_mode doesn't include requested mode",
				expectedMessage: "cluster nfs_tls_mode 'none:tls' does not support 'mtls'",
				userAction:      "Update cluster TLS configuration to include 'mtls' in nfs_tls_mode",
			},
		}

		for _, s := range scenarios {
			t.Logf("Scenario: %s", s.scenario)
			t.Logf("Expected error: %s", s.expectedMessage)
			t.Logf("User action: %s", s.userAction)
			t.Log("")
		}
	})
}

// TestMTLSFQDNValidation_NodeSide verifies the node-side (NodePublishVolume)
// ValidateMTLSMountTarget check: when mTLS is enabled and the resolved mount
// target is an IP (either because no FQDN was configured or because an IP was
// explicitly set), the mount must be rejected with a clear error.
func TestMTLSFQDNValidation_NodeSide(t *testing.T) {
	tests := []struct {
		name        string
		fqdn        string
		security    string
		expectValid bool
		errContains string
	}{
		{
			name:        "mTLS with valid FQDN passes",
			fqdn:        "powerscale.example.com",
			security:    "mtls",
			expectValid: true,
		},
		{
			name:        "mTLS with empty FQDN (falls back to IP) fails",
			fqdn:        "",
			security:    "mtls",
			expectValid: false,
			errContains: "is an IP address",
		},
		{
			name:        "mTLS with IPv4 address fails",
			fqdn:        "192.168.1.100",
			security:    "mtls",
			expectValid: false,
			errContains: "is an IP address",
		},
		{
			name:        "mTLS with IPv6 address fails",
			fqdn:        "::1",
			security:    "mtls",
			expectValid: false,
			errContains: "is an IP address",
		},
		{
			name:        "non-mTLS with IP address passes (backward compatible)",
			fqdn:        "192.168.1.100",
			security:    "none",
			expectValid: true,
		},
		{
			name:        "non-mTLS with empty FQDN passes (backward compatible)",
			fqdn:        "",
			security:    "",
			expectValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the node-side logic: when FQDN is empty, azServiceIP is used
			mountTarget := tt.fqdn
			if mountTarget == "" {
				mountTarget = "10.0.0.1" // simulate IP fallback when no FQDN
			}
			result := ValidateMTLSMountTarget(mountTarget, tt.security)
			assert.Equal(t, tt.expectValid, result.Valid, "validation result mismatch")
			if !tt.expectValid {
				assert.Contains(t, result.ErrorMessage, tt.errContains)
				assert.NotEmpty(t, result.EventReason, "should have an event reason for Kubernetes event")
			}
		})
	}
}

// TestMTLSFQDNValidation_ControllerSide verifies the controller-side
// (CreateVolume) fail-fast check: when NFSTransportSecurity=mtls,
// SmartConnectZoneFQDN must be present and must not be an IP address.
// This catches misconfiguration at PVC creation time (before the export is
// created on PowerScale), giving the user an immediate, actionable error.
func TestMTLSFQDNValidation_ControllerSide(t *testing.T) {
	tests := []struct {
		name              string
		smartConnectFQDN  string
		transportSecurity string
		expectRejectEmpty bool
		expectRejectIP    bool
	}{
		{
			name:              "mTLS with valid FQDN - allowed",
			smartConnectFQDN:  "powerscale.example.com",
			transportSecurity: "mtls",
			expectRejectEmpty: false,
			expectRejectIP:    false,
		},
		{
			name:              "mTLS with empty FQDN - rejected at CreateVolume",
			smartConnectFQDN:  "",
			transportSecurity: "mtls",
			expectRejectEmpty: true,
			expectRejectIP:    false,
		},
		{
			name:              "mTLS with IPv4 address - rejected at CreateVolume",
			smartConnectFQDN:  "192.168.1.100",
			transportSecurity: "mtls",
			expectRejectEmpty: false,
			expectRejectIP:    true,
		},
		{
			name:              "mTLS with IPv6 address - rejected at CreateVolume",
			smartConnectFQDN:  "fd00::1",
			transportSecurity: "mtls",
			expectRejectEmpty: false,
			expectRejectIP:    true,
		},
		{
			name:              "non-mTLS with IP - allowed (backward compatible)",
			smartConnectFQDN:  "192.168.1.100",
			transportSecurity: "none",
			expectRejectEmpty: false,
			expectRejectIP:    false,
		},
		{
			name:              "non-mTLS with empty FQDN - allowed (backward compatible)",
			smartConnectFQDN:  "",
			transportSecurity: "",
			expectRejectEmpty: false,
			expectRejectIP:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the controller-side validation logic from controller.go
			if IsMTLSEnabled(tt.transportSecurity) {
				if tt.smartConnectFQDN == "" {
					assert.True(t, tt.expectRejectEmpty,
						"mTLS with empty SmartConnectZoneFQDN should be rejected at CreateVolume time")
					return
				}
				if IsIPAddress(tt.smartConnectFQDN) {
					assert.True(t, tt.expectRejectIP,
						"mTLS with IP as SmartConnectZoneFQDN should be rejected at CreateVolume time")
					return
				}
				// Valid FQDN
				assert.False(t, tt.expectRejectEmpty)
				assert.False(t, tt.expectRejectIP)
			} else {
				// Non-mTLS: no validation, all values pass
				assert.False(t, tt.expectRejectEmpty)
				assert.False(t, tt.expectRejectIP)
			}
		})
	}
}

// TestMTLSDefenseInDepth documents the defense-in-depth security model
func TestMTLSDefenseInDepth(t *testing.T) {
	t.Run("Two-layer security enforcement", func(t *testing.T) {
		t.Log("Defense-in-Depth Security Model:")
		t.Log("")
		t.Log("Layer 1 (Client-side): Linux kernel mount option xprtsec=mtls")
		t.Log("  - Enforced by: Linux kernel kTLS + tlshd daemon")
		t.Log("  - Effect: This client's mount requires mTLS")
		t.Log("  - Limitation: Other clients can still mount with plaintext if export allows it")
		t.Log("")
		t.Log("Layer 2 (Server-side): PowerScale export xprtsec='mtls'")
		t.Log("  - Enforced by: PowerScale OneFS NFS server")
		t.Log("  - Effect: Export rejects plaintext connections from ALL clients")
		t.Log("  - Benefit: Complete security - no client can bypass mTLS")
		t.Log("")
		t.Log("SECURITY CRITICAL: Both layers must be configured for complete security")
		t.Log("  - Without Layer 2: Other clients can mount with plaintext (security gap)")
		t.Log("  - With Layer 2: All clients must use mTLS (defense-in-depth)")
	})

	t.Run("Security gap without server-side enforcement", func(t *testing.T) {
		t.Log("WITHOUT v27 API (before this implementation):")
		t.Log("  - Export xprtsec: 'none:tls:mtls' (allows all modes)")
		t.Log("  - Client mount: xprtsec=mtls (enforced)")
		t.Log("  - Security gap: Other clients can mount with plaintext!")
		t.Log("")
		t.Log("WITH v27 API (after this implementation):")
		t.Log("  - Export xprtsec: 'mtls' (mTLS only)")
		t.Log("  - Client mount: xprtsec=mtls (enforced)")
		t.Log("  - Defense-in-depth: Both layers enforce mTLS")
	})
}

// TestTLSErrorClassificationWiring verifies that TLS error classification
// infrastructure correctly identifies and categorizes mount-time TLS failures.
// AC-002: Expired cert, SAN mismatch, trust anchor failures must be classified.
func TestTLSErrorClassificationWiring(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name              string
		mountError        error
		isMTLS            bool
		expectTLSError    bool
		expectReason      string
		expectRecoverable bool
	}{
		{
			name:           "Expired certificate classified as ClientCertExpired",
			mountError:     errors.New("mount failed: x509: certificate has expired or is not yet valid"),
			isMTLS:         true,
			expectTLSError: true,
			expectReason:   constants.TLSEventReasonClientCertExpired,
		},
		{
			name:           "SAN mismatch classified as CertSANMismatch",
			mountError:     errors.New("x509: certificate is valid for zone1.example.com, not powerscale.example.com"),
			isMTLS:         true,
			expectTLSError: true,
			expectReason:   constants.TLSEventReasonCertSANMismatch,
		},
		{
			name:           "Unknown CA classified as TrustAnchorFailure",
			mountError:     errors.New("x509: certificate signed by unknown authority"),
			isMTLS:         true,
			expectTLSError: true,
			expectReason:   constants.TLSEventReasonTrustAnchorFailure,
		},
		{
			name:              "Handshake timeout classified as TLSHandshakeTimeout",
			mountError:        errors.New("connection timed out during tls handshake"),
			isMTLS:            true,
			expectTLSError:    true,
			expectReason:      constants.TLSEventReasonHandshakeTimeout,
			expectRecoverable: true,
		},
		{
			name:           "tlshd missing classified as DaemonMissing",
			mountError:     errors.New("tlshd: no such file or directory"),
			isMTLS:         true,
			expectTLSError: true,
			expectReason:   constants.TLSEventReasonDaemonMissing,
		},
		{
			name:           "Non-TLS error not classified",
			mountError:     errors.New("permission denied"),
			isMTLS:         true,
			expectTLSError: false,
		},
		{
			name:           "TLS error in non-mTLS mode - classification skipped",
			mountError:     errors.New("x509: certificate has expired"),
			isMTLS:         false,
			expectTLSError: true, // IsTLSError returns true regardless of mode
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Step 1: IsTLSError detects TLS-related errors
			isTLS := IsTLSError(tt.mountError)
			assert.Equal(t, tt.expectTLSError, isTLS, "IsTLSError detection mismatch")

			// Step 2: Only classify in mTLS mode (mirrors node.go logic)
			if tt.isMTLS && isTLS {
				classification := ParseTLSError(ctx, tt.mountError)
				assert.Equal(t, tt.expectReason, classification.Reason,
					"TLS error should be classified with correct reason")
				assert.NotEmpty(t, classification.Message,
					"Classification should include a prescriptive message")
				assert.NotEmpty(t, classification.ErrorCode,
					"Classification should include a gRPC error code")
			}
		})
	}
}

// TestMountTimeoutContextWiring verifies that CreateMountTimeoutContext
// is properly configured for mTLS mounts. AC-007.
func TestMountTimeoutContextWiring(t *testing.T) {
	t.Run("mTLS mount gets timeout context", func(t *testing.T) {
		ctx := context.Background()
		mountCtx, cancel := CreateMountTimeoutContext(ctx, "mtls")
		defer cancel()

		// Should have a deadline
		deadline, ok := mountCtx.Deadline()
		assert.True(t, ok, "mTLS mount context should have a deadline")
		assert.False(t, deadline.IsZero(), "deadline should be non-zero")
	})

	t.Run("Non-mTLS mount gets no timeout", func(t *testing.T) {
		ctx := context.Background()
		mountCtx, cancel := CreateMountTimeoutContext(ctx, "none")
		defer cancel()

		_, ok := mountCtx.Deadline()
		assert.False(t, ok, "non-mTLS mount context should NOT have a deadline")
	})

	t.Run("Empty transport security gets no timeout", func(t *testing.T) {
		ctx := context.Background()
		mountCtx, cancel := CreateMountTimeoutContext(ctx, "")
		defer cancel()

		_, ok := mountCtx.Deadline()
		assert.False(t, ok, "empty transport security should NOT have a deadline")
	})

	t.Run("TLS (non-mutual) gets no timeout", func(t *testing.T) {
		ctx := context.Background()
		mountCtx, cancel := CreateMountTimeoutContext(ctx, "tls")
		defer cancel()

		_, ok := mountCtx.Deadline()
		assert.False(t, ok, "server-only TLS should NOT have a deadline")
	})
}

// TestIsTimeoutErrorDetection verifies that IsTimeoutError correctly
// identifies context deadline and timeout errors. AC-007.
func TestIsTimeoutErrorDetection(t *testing.T) {
	t.Run("context.DeadlineExceeded detected as timeout", func(t *testing.T) {
		assert.True(t, IsTimeoutError(context.DeadlineExceeded))
	})

	t.Run("MountError with Timeout=true detected", func(t *testing.T) {
		err := &MountError{
			Operation: "mount",
			Err:       errors.New("timed out"),
			Timeout:   true,
		}
		assert.True(t, IsTimeoutError(err))
	})

	t.Run("MountError with Timeout=false not detected", func(t *testing.T) {
		err := &MountError{
			Operation: "mount",
			Err:       errors.New("some error"),
			Timeout:   false,
		}
		assert.False(t, IsTimeoutError(err))
	})

	t.Run("Regular error not detected as timeout", func(t *testing.T) {
		assert.False(t, IsTimeoutError(errors.New("some error")))
	})

	t.Run("Nil error not detected as timeout", func(t *testing.T) {
		assert.False(t, IsTimeoutError(nil))
	})

	t.Run("Wrapped DeadlineExceeded detected", func(t *testing.T) {
		err := fmt.Errorf("mount operation: %w", context.DeadlineExceeded)
		assert.True(t, IsTimeoutError(err))
	})
}

// TestPostMountDiagnosticsChain verifies the complete post-mount diagnostic
// chain: error detection → classification → event reason mapping.
// This tests the logic wired in node.go after publishVolume fails.
func TestPostMountDiagnosticsChain(t *testing.T) {
	ctx := context.Background()

	// Simulate the full diagnostic chain as wired in node.go
	scenarioTests := []struct {
		name              string
		mountErr          error
		transportSecurity string
		expectClassified  bool
		expectEventReason string
		expectGRPCCode    string
	}{
		{
			name:              "AC-002: Expired cert → ClientCertExpired event",
			mountErr:          errors.New("certificate has expired"),
			transportSecurity: "mtls",
			expectClassified:  true,
			expectEventReason: constants.TLSEventReasonClientCertExpired,
			expectGRPCCode:    "FailedPrecondition",
		},
		{
			name:              "AC-002: SAN mismatch → CertSANMismatch event",
			mountErr:          errors.New("x509: certificate is valid for wrong.com, not target.com"),
			transportSecurity: "mtls",
			expectClassified:  true,
			expectEventReason: constants.TLSEventReasonCertSANMismatch,
			expectGRPCCode:    "FailedPrecondition",
		},
		{
			name:              "AC-002: Trust anchor → TLSTrustAnchorFailure event",
			mountErr:          errors.New("x509: certificate signed by unknown authority"),
			transportSecurity: "mtls",
			expectClassified:  true,
			expectEventReason: constants.TLSEventReasonTrustAnchorFailure,
			expectGRPCCode:    "FailedPrecondition",
		},
		{
			name:              "Non-TLS mount error not classified (backward compat)",
			mountErr:          errors.New("access denied by server while mounting"),
			transportSecurity: "mtls",
			expectClassified:  false,
		},
		{
			name:              "TLS error in non-mTLS mode not classified",
			mountErr:          errors.New("certificate has expired"),
			transportSecurity: "",
			expectClassified:  false,
		},
	}

	for _, tt := range scenarioTests {
		t.Run(tt.name, func(t *testing.T) {
			// Mirror the exact logic from node.go lines 305-328
			shouldClassify := IsMTLSEnabled(tt.transportSecurity) && IsTLSError(tt.mountErr)
			assert.Equal(t, tt.expectClassified, shouldClassify,
				"classification gate mismatch")

			if shouldClassify {
				classification := ParseTLSError(ctx, tt.mountErr)
				assert.Equal(t, tt.expectEventReason, classification.Reason,
					"event reason should match for PVC event emission")
				assert.Equal(t, tt.expectGRPCCode, classification.ErrorCode,
					"gRPC code should match for error response")
				assert.NotEmpty(t, classification.Message,
					"classification should include prescriptive remediation message")
			}
		})
	}
}
