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
	"testing"

	"github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/common/constants"
	isi "github.com/Ecosystems/container-storage-modules/src/gopowerscale"
	isimocks "github.com/Ecosystems/container-storage-modules/src/gopowerscale/mocks"
	"github.com/Ecosystems/container-storage-modules/src/gopowerscale/openapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestResolveMountFQDN(t *testing.T) {
	tests := []struct {
		name        string
		scFQDN      string
		clusterFQDN string
		envFQDN     string
		expected    string
	}{
		{
			name:        "StorageClass FQDN takes highest precedence",
			scFQDN:      "zone1.smartconnect.example.com",
			clusterFQDN: "default.smartconnect.example.com",
			envFQDN:     "global.smartconnect.example.com",
			expected:    "zone1.smartconnect.example.com",
		},
		{
			name:        "Cluster FQDN used when StorageClass absent",
			scFQDN:      "",
			clusterFQDN: "default.smartconnect.example.com",
			envFQDN:     "global.smartconnect.example.com",
			expected:    "default.smartconnect.example.com",
		},
		{
			name:        "Environment variable FQDN used as fallback",
			scFQDN:      "",
			clusterFQDN: "",
			envFQDN:     "global.smartconnect.example.com",
			expected:    "global.smartconnect.example.com",
		},
		{
			name:        "Empty string when no FQDN configured",
			scFQDN:      "",
			clusterFQDN: "",
			envFQDN:     "",
			expected:    "",
		},
		{
			name:        "StorageClass overrides even when all are set",
			scFQDN:      "sc.example.com",
			clusterFQDN: "cluster.example.com",
			envFQDN:     "env.example.com",
			expected:    "sc.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ResolveMountFQDN(tt.scFQDN, tt.clusterFQDN, tt.envFQDN)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsIPAddress(t *testing.T) {
	tests := []struct {
		name     string
		target   string
		expected bool
	}{
		// IPv4 addresses
		{name: "IPv4 address", target: "192.168.1.100", expected: true},
		{name: "IPv4 loopback", target: "127.0.0.1", expected: true},
		{name: "IPv4 all zeros", target: "0.0.0.0", expected: true},
		{name: "IPv4 broadcast", target: "255.255.255.255", expected: true},

		// IPv6 addresses
		{name: "IPv6 full address", target: "2001:db8::1", expected: true},
		{name: "IPv6 loopback", target: "::1", expected: true},
		{name: "IPv6 with brackets", target: "[2001:db8::1]", expected: true},
		{name: "IPv6 full form", target: "2001:0db8:0000:0000:0000:0000:0000:0001", expected: true},

		// Valid FQDNs (not IP addresses)
		{name: "Simple FQDN", target: "zone1.smartconnect.example.com", expected: false},
		{name: "Short hostname", target: "powerscale.local", expected: false},
		{name: "Single label", target: "localhost", expected: false},
		{name: "Numeric hostname", target: "server123.example.com", expected: false},

		// Edge cases
		{name: "Empty string", target: "", expected: false},
		{name: "Invalid IP-like string", target: "192.168.1.999", expected: false},
		{name: "Partial IP", target: "192.168.1", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsIPAddress(tt.target)
			assert.Equal(t, tt.expected, result, "IsIPAddress(%q) should be %v", tt.target, tt.expected)
		})
	}
}

func TestValidateNFSTransportSecurity(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		expectError bool
	}{
		{name: "Empty string is valid", value: "", expectError: false},
		{name: "none is valid", value: "none", expectError: false},
		{name: "tls is valid", value: "tls", expectError: false},
		{name: "mtls is valid", value: "mtls", expectError: false},
		{name: "NONE uppercase is valid", value: "NONE", expectError: false},
		{name: "TLS uppercase is valid", value: "TLS", expectError: false},
		{name: "MTLS uppercase is valid", value: "MTLS", expectError: false},
		{name: "Mixed case is valid", value: "mTLS", expectError: false},
		{name: "Invalid value", value: "invalid", expectError: true},
		{name: "ssl is invalid", value: "ssl", expectError: true},
		{name: "true is invalid", value: "true", expectError: true},
		{name: "1 is invalid", value: "1", expectError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNFSTransportSecurity(tt.value)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIsMTLSEnabled(t *testing.T) {
	tests := []struct {
		name              string
		transportSecurity string
		expected          bool
	}{
		{name: "mtls lowercase", transportSecurity: "mtls", expected: true},
		{name: "MTLS uppercase", transportSecurity: "MTLS", expected: true},
		{name: "mTLS mixed case", transportSecurity: "mTLS", expected: true},
		{name: "tls is not mtls", transportSecurity: "tls", expected: false},
		{name: "none is not mtls", transportSecurity: "none", expected: false},
		{name: "empty is not mtls", transportSecurity: "", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsMTLSEnabled(tt.transportSecurity)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsTLSEnabled(t *testing.T) {
	tests := []struct {
		name              string
		transportSecurity string
		expected          bool
	}{
		{name: "mtls enables TLS", transportSecurity: "mtls", expected: true},
		{name: "tls enables TLS", transportSecurity: "tls", expected: true},
		{name: "none does not enable TLS", transportSecurity: "none", expected: false},
		{name: "empty does not enable TLS", transportSecurity: "", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsTLSEnabled(tt.transportSecurity)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidateMountOptionsForMTLS(t *testing.T) {
	tests := []struct {
		name              string
		mountOptions      []string
		transportSecurity string
		expectError       bool
		expectWarning     bool
	}{
		{
			name:              "Valid mTLS with xprtsec=mtls",
			mountOptions:      []string{"xprtsec=mtls", "vers=4.1", "hard"},
			transportSecurity: "mtls",
			expectError:       false,
			expectWarning:     false,
		},
		{
			name:              "mTLS with conflicting xprtsec=none",
			mountOptions:      []string{"xprtsec=none", "vers=4.1"},
			transportSecurity: "mtls",
			expectError:       true,
			expectWarning:     false,
		},
		{
			name:              "mTLS with conflicting xprtsec=tls",
			mountOptions:      []string{"xprtsec=tls", "vers=4.1"},
			transportSecurity: "mtls",
			expectError:       true,
			expectWarning:     false,
		},
		{
			name:              "mTLS without xprtsec - no warning (driver auto-injects)",
			mountOptions:      []string{"vers=4.1", "hard"},
			transportSecurity: "mtls",
			expectError:       false,
			expectWarning:     false,
		},
		{
			name:              "Non-mTLS mode ignores mount options",
			mountOptions:      []string{"xprtsec=none"},
			transportSecurity: "none",
			expectError:       false,
			expectWarning:     false,
		},
		{
			name:              "Empty transport security ignores mount options",
			mountOptions:      []string{"xprtsec=none"},
			transportSecurity: "",
			expectError:       false,
			expectWarning:     false,
		},
		{
			name:              "Empty mount options with mTLS - no warning (driver auto-injects)",
			mountOptions:      []string{},
			transportSecurity: "mtls",
			expectError:       false,
			expectWarning:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warning, err := ValidateMountOptionsForMTLS(tt.mountOptions, tt.transportSecurity)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			if tt.expectWarning {
				assert.NotEmpty(t, warning)
			} else {
				assert.Empty(t, warning)
			}
		})
	}
}

func TestValidateMTLSMountTarget(t *testing.T) {
	tests := []struct {
		name              string
		mountTarget       string
		transportSecurity string
		expectValid       bool
		expectEventReason string
	}{
		{
			name:              "Valid FQDN with mTLS",
			mountTarget:       "zone1.smartconnect.example.com",
			transportSecurity: "mtls",
			expectValid:       true,
			expectEventReason: "",
		},
		{
			name:              "IP address with mTLS (forbidden)",
			mountTarget:       "192.168.1.100",
			transportSecurity: "mtls",
			expectValid:       false,
			expectEventReason: constants.MTLSEventReasonIPAddressForbidden,
		},
		{
			name:              "IPv6 address with mTLS (forbidden)",
			mountTarget:       "2001:db8::1",
			transportSecurity: "mtls",
			expectValid:       false,
			expectEventReason: constants.MTLSEventReasonIPAddressForbidden,
		},
		{
			name:              "Empty mount target with mTLS (no FQDN)",
			mountTarget:       "",
			transportSecurity: "mtls",
			expectValid:       false,
			expectEventReason: constants.MTLSEventReasonNoFQDNConfigured,
		},
		{
			name:              "IP address without mTLS (allowed)",
			mountTarget:       "192.168.1.100",
			transportSecurity: "none",
			expectValid:       true,
			expectEventReason: "",
		},
		{
			name:              "Empty mount target without mTLS (allowed)",
			mountTarget:       "",
			transportSecurity: "",
			expectValid:       true,
			expectEventReason: "",
		},
		{
			name:              "IP address with TLS only (allowed - no FQDN check for TLS)",
			mountTarget:       "192.168.1.100",
			transportSecurity: "tls",
			expectValid:       true,
			expectEventReason: "",
		},
		{
			name:              "Empty mount target with TLS only (allowed - no FQDN check for TLS)",
			mountTarget:       "",
			transportSecurity: "tls",
			expectValid:       true,
			expectEventReason: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateMTLSMountTarget(tt.mountTarget, tt.transportSecurity)
			assert.Equal(t, tt.expectValid, result.Valid)
			assert.Equal(t, tt.expectEventReason, result.EventReason)
		})
	}
}

func TestEmitMTLSEvent(t *testing.T) {
	tests := []struct {
		name          string
		k8sclient     interface{}
		pvcNamespace  string
		pvcName       string
		reason        string
		message       string
		expectNoError bool
		expectError   bool
	}{
		{
			name:          "Nil k8s client returns no error",
			k8sclient:     nil,
			pvcNamespace:  "default",
			pvcName:       "test-pvc",
			reason:        "TestReason",
			message:       "Test message",
			expectNoError: true,
		},
		{
			name:          "Empty PVC namespace returns no error",
			k8sclient:     fake.NewSimpleClientset(),
			pvcNamespace:  "",
			pvcName:       "test-pvc",
			reason:        "TestReason",
			message:       "Test message",
			expectNoError: true,
		},
		{
			name:          "Empty PVC name returns no error",
			k8sclient:     fake.NewSimpleClientset(),
			pvcNamespace:  "default",
			pvcName:       "",
			reason:        "TestReason",
			message:       "Test message",
			expectNoError: true,
		},
		{
			name:          "Valid parameters with fake client",
			k8sclient:     fake.NewSimpleClientset(),
			pvcNamespace:  "default",
			pvcName:       "test-pvc",
			reason:        "TestReason",
			message:       "Test message",
			expectNoError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			var k8sClient kubernetes.Interface
			if tt.k8sclient != nil {
				k8sClient = tt.k8sclient.(kubernetes.Interface)
			}
			err := EmitMTLSEvent(ctx, k8sClient, tt.pvcNamespace, tt.pvcName, tt.reason, tt.message)
			if tt.expectNoError {
				assert.NoError(t, err)
			} else if tt.expectError {
				assert.Error(t, err)
			}
		})
	}
}

func TestLogMTLSMountOperation(t *testing.T) {
	tests := []struct {
		name   string
		fields MTLSLogFields
	}{
		{
			name: "Complete mTLS log fields",
			fields: MTLSLogFields{
				MountTarget:     "192.168.1.100:/ifs/data/vol1",
				MountTargetFQDN: "zone1.smartconnect.example.com",
				TLSEnabled:      true,
				AccessZone:      "System",
				MountOptions:    "xprtsec=mtls,vers=4.1",
			},
		},
		{
			name: "Minimal mTLS log fields",
			fields: MTLSLogFields{
				MountTarget:  "192.168.1.100:/ifs/data/vol1",
				TLSEnabled:   false,
				AccessZone:   "System",
				MountOptions: "vers=4.1",
			},
		},
		{
			name:   "Empty log fields",
			fields: MTLSLogFields{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(_ *testing.T) {
			ctx := context.Background()
			// This function just logs, so we're testing it doesn't panic
			LogMTLSMountOperation(ctx, tt.fields)
		})
	}
}

func TestValidateClusterTLSMode(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		requestedMode string
		apiErr        error
		settings      *openapi.V27NfsSettingsGlobal
		expectedErr   string
	}{
		{
			name:          "no validation needed for empty mode",
			requestedMode: "",
		},
		{
			name:          "no validation needed for plaintext",
			requestedMode: constants.NFSTransportSecurityNone,
		},
		{
			name:          "API failure fails closed",
			requestedMode: constants.NFSTransportSecurityMTLS,
			apiErr:        errors.New("404 Not Found"),
			expectedErr:   "cluster does not support NFS over TLS",
		},
		{
			name:          "nfs_tls_mode not configured fails closed",
			requestedMode: constants.NFSTransportSecurityMTLS,
			settings:      &openapi.V27NfsSettingsGlobal{},
			expectedErr:   "cluster nfs_tls_mode is not configured",
		},
		{
			name:          "requested mode not supported by cluster",
			requestedMode: constants.NFSTransportSecurityMTLS,
			settings:      &openapi.V27NfsSettingsGlobal{NfsTLSMode: openapi.PtrString("none:tls")},
			expectedErr:   "does not support 'mtls'",
		},
		{
			name:          "mode supported with recommended TLS 1.3",
			requestedMode: constants.NFSTransportSecurityMTLS,
			settings: &openapi.V27NfsSettingsGlobal{
				NfsTLSMode:       openapi.PtrString("none:tls:mtls"),
				NfsTLSMinVersion: openapi.PtrString("1.3"),
			},
		},
		{
			name:          "mode supported with TLS 1.2 warns but succeeds",
			requestedMode: constants.NFSTransportSecurityTLS,
			settings: &openapi.V27NfsSettingsGlobal{
				NfsTLSMode:       openapi.PtrString("tls"),
				NfsTLSMinVersion: openapi.PtrString("1.2"),
			},
		},
		{
			name:          "mode supported without min version warns but succeeds",
			requestedMode: constants.NFSTransportSecurityMTLS,
			settings:      &openapi.V27NfsSettingsGlobal{NfsTLSMode: openapi.PtrString("mtls")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := &isimocks.Client{}
			client := &isi.Client{API: mockAPI}

			if tt.apiErr != nil {
				mockAPI.On("Get", anyArgs[0:6]...).Return(tt.apiErr).Once()
			} else if tt.settings != nil {
				mockAPI.On("Get", anyArgs[0:6]...).Return(nil).Run(func(args mock.Arguments) {
					resp := args.Get(5).(*openapi.V27NfsSettingsGlobalResponse)
					*resp = openapi.V27NfsSettingsGlobalResponse{Settings: tt.settings}
				}).Once()
			}

			err := ValidateClusterTLSMode(ctx, client, tt.requestedMode)
			if tt.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.expectedErr)
			}
		})
	}
}

func TestEmitMTLSEventCreateFailure(t *testing.T) {
	client := fake.NewSimpleClientset()
	client.PrependReactor("create", "events", func(_ k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("event induced error")
	})

	err := EmitMTLSEvent(context.Background(), client, "test-ns", "test-pvc",
		constants.MTLSEventReasonIPAddressForbidden, "mount target is an IP address")
	assert.ErrorContains(t, err, "event induced error")
}

// TestEmitMTLSEventPopulatesPVCUID verifies the emitted event carries the PVC UID and
// ResourceVersion. `kubectl describe pvc` selects events by involvedObject.uid, so an
// event with an empty UID is persisted but never shown to the operator - which would
// silently defeat the mTLS diagnostics required by AC-002, AC-005 and AC-007.
func TestEmitMTLSEventPopulatesPVCUID(t *testing.T) {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test-pvc",
			Namespace:       "test-ns",
			UID:             types.UID("d3b07384-d9a0-4c9b-9f2a-1f5a1b2c3d4e"),
			ResourceVersion: "424242",
		},
	}
	client := fake.NewSimpleClientset(pvc)

	err := EmitMTLSEvent(context.Background(), client, "test-ns", "test-pvc",
		constants.MTLSEventReasonIPAddressForbidden, "mount target is an IP address")
	assert.NoError(t, err)

	events, err := client.CoreV1().Events("test-ns").List(context.Background(), metav1.ListOptions{})
	assert.NoError(t, err)
	assert.Len(t, events.Items, 1)

	got := events.Items[0]
	assert.Equal(t, pvc.UID, got.InvolvedObject.UID, "event must carry the PVC UID")
	assert.Equal(t, "424242", got.InvolvedObject.ResourceVersion)
	assert.Equal(t, "v1", got.InvolvedObject.APIVersion)
	assert.Equal(t, "PersistentVolumeClaim", got.InvolvedObject.Kind)
	assert.Equal(t, "test-pvc", got.InvolvedObject.Name)
	assert.Equal(t, "test-ns", got.InvolvedObject.Namespace)
	assert.Equal(t, constants.MTLSEventReasonIPAddressForbidden, got.Reason)
	assert.Equal(t, corev1.EventTypeWarning, got.Type)
	assert.EqualValues(t, 1, got.Count)
	assert.False(t, got.FirstTimestamp.IsZero(), "event must carry a first timestamp")
	assert.False(t, got.LastTimestamp.IsZero(), "event must carry a last timestamp")
}

// TestEmitMTLSEventPVCLookupFailureStillEmits ensures the UID lookup is best-effort:
// when the PVC cannot be read the event is still emitted rather than dropped.
func TestEmitMTLSEventPVCLookupFailureStillEmits(t *testing.T) {
	client := fake.NewSimpleClientset() // no PVC registered -> lookup returns NotFound
	client.PrependReactor("get", "persistentvolumeclaims", func(_ k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("api server unavailable")
	})

	err := EmitMTLSEvent(context.Background(), client, "test-ns", "missing-pvc",
		constants.TLSEventReasonHandshakeTimeout, "TLS handshake timed out")
	assert.NoError(t, err)

	events, err := client.CoreV1().Events("test-ns").List(context.Background(), metav1.ListOptions{})
	assert.NoError(t, err)
	assert.Len(t, events.Items, 1, "event must still be emitted when the PVC lookup fails")
	assert.Empty(t, events.Items[0].InvolvedObject.UID)
	assert.Equal(t, constants.TLSEventReasonHandshakeTimeout, events.Items[0].Reason)
}
