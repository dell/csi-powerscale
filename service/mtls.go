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
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/common/constants"
	csmlog "github.com/Ecosystems/container-storage-modules/src/csmlog"
	isi "github.com/Ecosystems/container-storage-modules/src/gopowerscale"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ResolveMountFQDN resolves the NFS mount target FQDN using a three-layer precedence chain.
// Precedence (highest to lowest):
// 1. StorageClass parameter SmartConnectZoneFQDN
// 2. Cluster configuration field nfsMountFQDN
// 3. Environment variable X_CSI_ISI_NFS_MOUNT_FQDN
// Returns the resolved FQDN or empty string if none configured.
func ResolveMountFQDN(scFQDN, clusterFQDN, envFQDN string) string {
	// Layer 1: StorageClass parameter (highest precedence)
	if scFQDN != "" {
		return scFQDN
	}
	// Layer 2: Cluster configuration field
	if clusterFQDN != "" {
		return clusterFQDN
	}
	// Layer 3: Environment variable (lowest precedence)
	return envFQDN
}

// IsIPAddress checks if the given target is an IPv4 or IPv6 address literal.
// Returns true if target is an IP address, false if it's a hostname/FQDN.
func IsIPAddress(target string) bool {
	if target == "" {
		return false
	}
	// net.ParseIP handles both IPv4 and IPv6 addresses
	// It also handles IPv6 addresses in brackets like [::1]
	cleanTarget := strings.TrimPrefix(strings.TrimSuffix(target, "]"), "[")
	return net.ParseIP(cleanTarget) != nil
}

// ValidateNFSTransportSecurity validates the NFSTransportSecurity parameter value.
// Valid values are: "" (empty), "none", "tls", "mtls"
// Returns nil if valid, error if invalid.
func ValidateNFSTransportSecurity(value string) error {
	switch strings.ToLower(value) {
	case "", constants.NFSTransportSecurityNone, constants.NFSTransportSecurityTLS, constants.NFSTransportSecurityMTLS:
		return nil
	default:
		return fmt.Errorf("invalid NFSTransportSecurity value '%s': must be one of '', 'none', 'tls', or 'mtls'", value)
	}
}

// IsMTLSEnabled checks if mTLS is enabled based on the transport security setting.
func IsMTLSEnabled(transportSecurity string) bool {
	return strings.ToLower(transportSecurity) == constants.NFSTransportSecurityMTLS
}

// IsTLSEnabled checks if TLS (server-only) or mTLS is enabled.
func IsTLSEnabled(transportSecurity string) bool {
	lower := strings.ToLower(transportSecurity)
	return lower == constants.NFSTransportSecurityTLS || lower == constants.NFSTransportSecurityMTLS
}

// ValidateMountOptionsForMTLS validates that mount options do not conflict with the transport security setting.
// When mTLS is enabled:
// - Rejects if mountOptions contains "xprtsec=none" (direct conflict)
// - Rejects if mountOptions contains "xprtsec=tls" (mTLS requires mutual auth, not server-only TLS)
// Note: The driver automatically injects "xprtsec=mtls" at mount time (see mount.go publishVolumeFunc),
// so users do not need to specify it manually in the StorageClass mountOptions.
// Returns (warning, error) where error is non-nil for conflicts, warning is informational only.
func ValidateMountOptionsForMTLS(mountOptions []string, transportSecurity string) (string, error) {
	if !IsMTLSEnabled(transportSecurity) {
		return "", nil
	}

	for _, opt := range mountOptions {
		optLower := strings.ToLower(opt)
		// Check for conflicting xprtsec values
		if optLower == "xprtsec=none" {
			return "", fmt.Errorf("mount option 'xprtsec=none' cannot be used with NFSTransportSecurity: mtls")
		}
		if optLower == "xprtsec=tls" {
			return "", fmt.Errorf("mount option 'xprtsec=tls' cannot be used with NFSTransportSecurity: mtls; mTLS requires mutual authentication")
		}
	}

	return "", nil
}

// MTLSValidationResult holds the result of mTLS validation for a mount operation.
type MTLSValidationResult struct {
	Valid        bool
	ErrorCode    string
	ErrorMessage string
	EventReason  string
}

// ValidateMTLSMountTarget validates the mount target for mTLS requirements.
// When mTLS is enabled:
// - Returns error if no FQDN is configured
// - Returns error if target is an IP address
// Returns MTLSValidationResult with validation outcome.
func ValidateMTLSMountTarget(mountTarget, transportSecurity string) MTLSValidationResult {
	if !IsMTLSEnabled(transportSecurity) {
		return MTLSValidationResult{Valid: true}
	}

	// Check if FQDN is configured
	if mountTarget == "" {
		return MTLSValidationResult{
			Valid:        false,
			ErrorCode:    "InvalidArgument",
			ErrorMessage: "No FQDN configured for mTLS mount. Configure SmartConnectZoneFQDN, cluster nfsMountFQDN, or X_CSI_ISI_NFS_MOUNT_FQDN environment variable.",
			EventReason:  constants.MTLSEventReasonNoFQDNConfigured,
		}
	}

	// Check if target is an IP address
	if IsIPAddress(mountTarget) {
		return MTLSValidationResult{
			Valid:        false,
			ErrorCode:    "InvalidArgument",
			ErrorMessage: fmt.Sprintf("mTLS mount target '%s' is an IP address. Set StorageClass parameter 'SmartConnectZoneFQDN' to the OneFS SmartConnect zone FQDN.", mountTarget),
			EventReason:  constants.MTLSEventReasonIPAddressForbidden,
		}
	}

	return MTLSValidationResult{Valid: true}
}

// EmitMTLSEvent emits a Kubernetes event for mTLS-related issues.
// The event is attached to the PVC object if pvcNamespace and pvcName are provided.
//
// The PVC UID and ResourceVersion are resolved before the event is created because
// `kubectl describe pvc` searches events with a field selector that includes
// involvedObject.uid. An event with an empty UID is persisted but never surfaces in
// the PVC description, which would silently defeat the operator-facing diagnostics
// required by AC-002, AC-005 and AC-007. The lookup is best-effort: if it fails the
// event is still emitted (degraded visibility is better than no event at all).
func EmitMTLSEvent(ctx context.Context, k8sclient kubernetes.Interface, pvcNamespace, pvcName, reason, message string) error {
	if k8sclient == nil {
		csmlog.WithContext(ctx).Warnf("Cannot emit mTLS event: Kubernetes client is nil")
		return nil
	}

	if pvcNamespace == "" || pvcName == "" {
		csmlog.WithContext(ctx).Warnf("Cannot emit mTLS event: PVC namespace or name is empty")
		return nil
	}

	involvedObject := corev1.ObjectReference{
		APIVersion: "v1",
		Kind:       "PersistentVolumeClaim",
		Namespace:  pvcNamespace,
		Name:       pvcName,
	}

	lookupCtx, cancel := context.WithTimeout(ctx, pvcLookupTimeout)
	defer cancel()
	if pvc, err := k8sclient.CoreV1().PersistentVolumeClaims(pvcNamespace).Get(lookupCtx, pvcName, metav1.GetOptions{}); err == nil {
		involvedObject.UID = pvc.UID
		involvedObject.ResourceVersion = pvc.ResourceVersion
	} else {
		csmlog.WithContext(ctx).Debugf("Failed to fetch PVC %s/%s for mTLS event UID: %v", pvcNamespace, pvcName, err)
	}

	now := metav1.NewTime(time.Now())
	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "mtls-",
			Namespace:    pvcNamespace,
		},
		InvolvedObject: involvedObject,
		Reason:         reason,
		Message:        message,
		Type:           corev1.EventTypeWarning,
		Source: corev1.EventSource{
			Component: constants.PluginName,
		},
		FirstTimestamp: now,
		LastTimestamp:  now,
		Count:          1,
	}

	_, err := k8sclient.CoreV1().Events(pvcNamespace).Create(ctx, event, metav1.CreateOptions{})
	if err != nil {
		csmlog.WithContext(ctx).Errorf("Failed to emit mTLS event: %v", err)
		return err
	}

	csmlog.WithContext(ctx).Infof("Emitted mTLS event: reason=%s, message=%s", reason, message)
	return nil
}

// LogMTLSMountOperation logs a structured mTLS mount operation with all relevant fields.
func LogMTLSMountOperation(ctx context.Context, fields MTLSLogFields) {
	log := csmlog.WithContext(ctx)
	log.WithFields(map[string]interface{}{
		"mount_target":      fields.MountTarget,
		"mount_target_fqdn": fields.MountTargetFQDN,
		"tls_enabled":       fields.TLSEnabled,
		"access_zone":       fields.AccessZone,
		"mount_options":     fields.MountOptions,
		"outcome":           fields.Outcome,
		"error_code":        fields.ErrorCode,
		"error_message":     fields.ErrorMessage,
		"node_id":           fields.NodeID,
	}).Info("mTLS mount operation")
}

// MTLSLogFields contains the structured logging fields for mTLS operations.
type MTLSLogFields struct {
	MountTarget     string
	MountTargetFQDN string
	TLSEnabled      bool
	AccessZone      string
	MountOptions    string
	Outcome         string
	ErrorCode       string
	ErrorMessage    string
	NodeID          string
}

// ResolveXprtsec determines the xprtsec value for export creation based on NFSTransportSecurity.
// Returns empty string if no specific xprtsec should be set (use cluster default).
//
// Mapping:
//   - "mtls" → "mtls" (mTLS-only export)
//   - "tls" → "tls" (TLS-only export, no plaintext, no mTLS)
//   - "none" → "none" (Plaintext-only export)
//   - "" (empty) → "" (Use cluster default, usually "none:tls:mtls")
//
// This function enables defense-in-depth security by enforcing transport security
// at the PowerScale export level in addition to client-side mount options.
func ResolveXprtsec(nfsTransportSecurity string) string {
	switch strings.ToLower(nfsTransportSecurity) {
	case constants.NFSTransportSecurityMTLS:
		return "mtls" // mTLS-only export
	case constants.NFSTransportSecurityTLS:
		return "tls" // TLS-only export (no plaintext, no mTLS)
	case constants.NFSTransportSecurityNone:
		return "none" // Plaintext-only export
	default:
		return "" // Use cluster default (usually "none:tls:mtls")
	}
}

// ValidateClusterTLSMode validates that the cluster supports the requested transport security mode.
// Returns error if cluster nfs_tls_mode does not include the required mode.
//
// SECURITY CRITICAL: When TLS/mTLS is explicitly requested, this function MUST fail if the array
// does not support it. Allowing fallback to plaintext would create a security vulnerability.
//
// Parameters:
//   - ctx: Context for logging and cancellation
//   - client: gopowerscale client for API calls
//   - requestedMode: Transport security mode from StorageClass (e.g., "mtls", "tls", "none")
//
// Returns:
//   - nil if cluster supports the requested mode (or no validation needed for plaintext/empty)
//   - error if cluster does not support the requested mode OR validation cannot be performed
func ValidateClusterTLSMode(ctx context.Context, client *isi.Client, requestedMode string) error {
	if requestedMode == "" || requestedMode == constants.NFSTransportSecurityNone {
		return nil // No validation needed for plaintext or default
	}

	settings, err := client.GetNfsSettingsGlobal(ctx)
	if err != nil {
		// SECURITY CRITICAL: If TLS/mTLS is explicitly requested but we cannot validate array support,
		// we MUST fail. Allowing the operation to proceed would risk creating plaintext exports
		// when the user explicitly requested encrypted transport.
		csmlog.WithContext(ctx).Errorf("Cannot validate cluster TLS mode for requested mode '%s': %v. "+
			"This may indicate OneFS < 9.16.0 (which does not support NFS over TLS) or insufficient API privileges. "+
			"Failing operation to prevent plaintext fallback when TLS/mTLS was explicitly requested.", requestedMode, err)
		return fmt.Errorf("cluster does not support NFS over TLS (OneFS 9.16.0+ required for '%s' mode): %w", requestedMode, err)
	}

	if settings.NfsTLSMode == nil {
		// SECURITY CRITICAL: nfs_tls_mode not configured means TLS is not enabled on the array.
		// We MUST fail when TLS/mTLS is explicitly requested.
		csmlog.WithContext(ctx).Errorf("Cluster nfs_tls_mode is not configured, but '%s' mode was explicitly requested. "+
			"Configure NFS over TLS on the PowerScale array before using NFSTransportSecurity parameter. "+
			"Failing operation to prevent plaintext fallback.", requestedMode)
		return fmt.Errorf("cluster nfs_tls_mode is not configured; cannot create '%s' export (configure NFS over TLS on PowerScale array)", requestedMode)
	}

	// Check if cluster mode includes requested mode
	modeFound := false
	clusterModes := strings.Split(*settings.NfsTLSMode, ":")
	for _, mode := range clusterModes {
		if strings.EqualFold(strings.TrimSpace(mode), requestedMode) {
			modeFound = true
			break
		}
	}
	if !modeFound {
		csmlog.WithContext(ctx).Errorf("Cluster nfs_tls_mode '%s' does not include requested mode '%s'. "+
			"Update PowerScale cluster TLS configuration to include '%s' in nfs_tls_mode. "+
			"Failing operation to prevent plaintext fallback.", *settings.NfsTLSMode, requestedMode, requestedMode)
		return fmt.Errorf("cluster nfs_tls_mode '%s' does not support '%s'; update cluster TLS configuration to include '%s'", *settings.NfsTLSMode, requestedMode, requestedMode)
	}

	// Validate minimum TLS version for mutual authentication.
	// TLS 1.3 is the recommended minimum (RFC 9289 baseline); TLS 1.2 is accepted
	// but logged as a warning because 1.3 offers stronger key exchange and removes
	// legacy cipher suites susceptible to downgrade attacks.
	if settings.NfsTLSMinVersion != nil {
		minVer := *settings.NfsTLSMinVersion
		if minVer == "1.3" {
			csmlog.WithContext(ctx).Infof("Cluster TLS validation passed: nfs_tls_mode includes '%s', nfs_tls_min_version='1.3' (recommended)", requestedMode)
		} else {
			csmlog.WithContext(ctx).Warnf("Cluster nfs_tls_min_version is '%s'; TLS 1.3 is recommended for '%s' mode. "+
				"Consider setting nfs_tls_min_version to '1.3' for strongest mutual authentication security.", minVer, requestedMode)
		}
	} else {
		csmlog.WithContext(ctx).Warnf("Cluster nfs_tls_min_version is not configured. "+
			"TLS 1.3 is recommended for '%s' mode. "+
			"Consider setting nfs_tls_min_version to '1.3' for strongest mutual authentication security.", requestedMode)
	}

	return nil
}
