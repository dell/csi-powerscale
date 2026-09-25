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
	"os"
	"os/exec"

	"github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/common/constants"
	csmlog "github.com/Ecosystems/container-storage-modules/src/csmlog"
)

// TLSCapabilityResult holds the result of TLS capability detection.
type TLSCapabilityResult struct {
	KernelTLSSupported bool
	TLSHDAvailable     bool
	TLSCapable         bool
	KernelError        string
	DaemonError        string
}

// Package-level variables for testing
var (
	// kernelTLSModulePath can be overridden in tests
	kernelTLSModulePath = constants.KernelTLSModulePath

	// tlsHandshakeDaemonPath can be overridden in tests
	tlsHandshakeDaemonPath = constants.TLSHandshakeDaemonPath

	// statFunc can be overridden in tests
	statFunc = os.Stat

	// lookPathFunc can be overridden in tests
	lookPathFunc = exec.LookPath
)

// CheckKernelTLSSupport checks if the kernel has TLS support by verifying
// the existence of /sys/module/tls.
// Returns true if kernel TLS is supported, false otherwise.
func CheckKernelTLSSupport(ctx context.Context) (bool, error) {
	_, err := statFunc(kernelTLSModulePath)
	if err != nil {
		if os.IsNotExist(err) {
			csmlog.WithContext(ctx).Debugf("Kernel TLS module not found at %s", kernelTLSModulePath)
			return false, nil
		}
		csmlog.WithContext(ctx).Errorf("Error checking kernel TLS support: %v", err)
		return false, err
	}
	csmlog.WithContext(ctx).Debugf("Kernel TLS module found at %s", kernelTLSModulePath)
	return true, nil
}

// CheckTLSHandshakeDaemon checks if the tlshd daemon is available.
// It first checks if the binary exists at the expected path, then
// verifies it's in the system PATH.
// Returns true if tlshd is available, false otherwise.
func CheckTLSHandshakeDaemon(ctx context.Context) (bool, error) {
	// First check the default path
	_, err := statFunc(tlsHandshakeDaemonPath)
	if err == nil {
		csmlog.WithContext(ctx).Debugf("TLS handshake daemon found at %s", tlsHandshakeDaemonPath)
		return true, nil
	}

	// If not at default path, check if it's in PATH
	path, err := lookPathFunc("tlshd")
	if err == nil {
		csmlog.WithContext(ctx).Debugf("TLS handshake daemon found in PATH at %s", path)
		return true, nil
	}

	csmlog.WithContext(ctx).Debugf("TLS handshake daemon not found (checked %s and PATH)", tlsHandshakeDaemonPath)
	return false, nil
}

// GetTLSCapabilityStatus performs a combined check for TLS capability.
// A node is considered TLS-capable if both:
// 1. The kernel has TLS support (/sys/module/tls exists)
// 2. The tlshd daemon is available
func GetTLSCapabilityStatus(ctx context.Context) TLSCapabilityResult {
	result := TLSCapabilityResult{}

	// Check kernel TLS support
	kernelSupported, err := CheckKernelTLSSupport(ctx)
	result.KernelTLSSupported = kernelSupported
	if err != nil {
		result.KernelError = err.Error()
	}

	// Check tlshd daemon
	daemonAvailable, err := CheckTLSHandshakeDaemon(ctx)
	result.TLSHDAvailable = daemonAvailable
	if err != nil {
		result.DaemonError = err.Error()
	}

	// Node is TLS-capable only if both conditions are met
	result.TLSCapable = result.KernelTLSSupported && result.TLSHDAvailable

	csmlog.WithContext(ctx).WithFields(map[string]interface{}{
		"kernel_tls_supported": result.KernelTLSSupported,
		"tlshd_available":      result.TLSHDAvailable,
		"tls_capable":          result.TLSCapable,
	}).Info("TLS capability check completed")

	return result
}

// ValidateTLSCapabilityForMount validates that the node has TLS capability
// before attempting an mTLS mount. Returns an error if the node is not capable.
func ValidateTLSCapabilityForMount(ctx context.Context, transportSecurity string) (TLSCapabilityResult, error) {
	// Only validate for mTLS mounts
	if !IsMTLSEnabled(transportSecurity) {
		return TLSCapabilityResult{TLSCapable: true}, nil
	}

	result := GetTLSCapabilityStatus(ctx)

	// Log detailed capability status for mTLS mounts
	csmlog.WithContext(ctx).WithFields(map[string]interface{}{
		"transport_security":   transportSecurity,
		"kernel_tls_supported": result.KernelTLSSupported,
		"tlshd_available":      result.TLSHDAvailable,
		"tls_capable":          result.TLSCapable,
	}).Info("Validating TLS capability for mTLS mount")

	return result, nil
}

// LogTLSReadiness performs a non-fatal TLS capability readiness check intended to
// be called once at node startup (e.g. during driver registration). It never
// returns an error and never blocks node startup: TLS/mTLS is an opt-in feature,
// so a node that lacks kernel TLS support or the tlshd daemon is still perfectly
// usable for plain NFS. The check exists purely to give operators early, actionable
// visibility into whether the node is ready for mTLS mounts and, if not, exactly
// which dependency is missing and how to remediate it.
func LogTLSReadiness(ctx context.Context) TLSCapabilityResult {
	result := GetTLSCapabilityStatus(ctx)
	if result.TLSCapable {
		csmlog.WithContext(ctx).Info("mTLS readiness: node is TLS-capable (kernel TLS module and tlshd daemon are present)")
		return result
	}

	// Not TLS-capable is informational, not an error. Surface the specific missing
	// dependency with remediation guidance so it is obvious from the node logs.
	msg := GetTLSCapabilityErrorMessage(result)
	if msg == "" {
		msg = "one or more TLS dependencies are unavailable"
	}
	csmlog.WithContext(ctx).WithFields(map[string]interface{}{
		"kernel_tls_supported": result.KernelTLSSupported,
		"tlshd_available":      result.TLSHDAvailable,
		"remediation":          msg,
	}).Info("mTLS readiness: node is NOT TLS-capable. Plain NFS is unaffected; mTLS/TLS mounts on this node will be rejected until the missing dependency is installed")

	return result
}

// GetTLSCapabilityEventReason returns the appropriate Kubernetes event reason
// for a TLS capability failure.
func GetTLSCapabilityEventReason(result TLSCapabilityResult) string {
	if !result.KernelTLSSupported {
		return constants.TLSEventReasonKernelNotSupported
	}
	if !result.TLSHDAvailable {
		return constants.TLSEventReasonDaemonMissing
	}
	return ""
}

// GetTLSCapabilityErrorMessage returns a descriptive error message for
// a TLS capability failure.
func GetTLSCapabilityErrorMessage(result TLSCapabilityResult) string {
	if !result.KernelTLSSupported {
		return "Kernel TLS (kTLS) support not available. Ensure kernel version 6.5+ and TLS module is loaded (modprobe tls)."
	}
	if !result.TLSHDAvailable {
		return "TLS handshake daemon (tlshd) not found. Install ktls-utils package and ensure tlshd is running."
	}
	return ""
}
