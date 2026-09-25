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
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/common/constants"
	csmlog "github.com/Ecosystems/container-storage-modules/src/csmlog"
)

// TLSErrorClassification holds the result of TLS error classification.
type TLSErrorClassification struct {
	Reason      string
	Message     string
	ErrorCode   string
	Recoverable bool
	SubjectDN   string
	IssuerDN    string
	NotAfter    string
}

// Common TLS error patterns and their classification rules
var (
	// Error patterns are matched in order; first match wins
	tlsErrorPatterns = []tlsErrorPattern{
		{
			reason:      constants.TLSEventReasonHandshakeTimeout,
			patterns:    []string{"connection timed out", "handshake timeout", "tls handshake timeout", "i/o timeout", "context deadline exceeded"},
			message:     "TLS handshake timed out. Verify network connectivity and TLS daemon (tlshd) are operational.",
			errorCode:   "DeadlineExceeded",
			recoverable: true,
		},
		{
			reason:      constants.TLSEventReasonClientCertExpired,
			patterns:    []string{"certificate has expired", "certificate expired", "x509: certificate has expired or is not yet valid"},
			message:     "Client or server certificate has expired. Renew the certificate and update /etc/tlshd.conf.",
			errorCode:   "FailedPrecondition",
			recoverable: false,
		},
		{
			reason:      constants.TLSEventReasonCertSANMismatch,
			patterns:    []string{"x509: certificate is valid for", "name does not match", "san mismatch", "subject alternative name"},
			message:     "Certificate SAN does not match the mount target FQDN. Verify the SmartConnect FQDN and certificate SAN.",
			errorCode:   "FailedPrecondition",
			recoverable: false,
		},
		{
			reason:      constants.TLSEventReasonTrustAnchorFailure,
			patterns:    []string{"x509: certificate signed by unknown authority", "unable to get local issuer certificate", "certificate verify failed", "trust anchor", "ca certificate"},
			message:     "TLS trust anchor validation failed. Verify the CA certificate chain in /etc/tlshd.conf.",
			errorCode:   "FailedPrecondition",
			recoverable: false,
		},
		{
			reason:      constants.TLSEventReasonDaemonMissing,
			patterns:    []string{"tlshd", "tls handshake daemon", "no tls support"},
			message:     "TLS handshake daemon (tlshd) is unavailable. Install ktls-utils and start tlshd.",
			errorCode:   "FailedPrecondition",
			recoverable: false,
		},
	}

	// certInfoRegexp extracts certificate subject/issuer info from OpenSSL-style output
	certSubjectRegexp = regexp.MustCompile(`subject=\s*([A-Z]+)\s*=\s*([^,\n]+)`)
	certIssuerRegexp  = regexp.MustCompile(`issuer=\s*([A-Z]+)\s*=\s*([^,\n]+)`)
	certNotAfter      = regexp.MustCompile(`notAfter=\s*([A-Za-z0-9:\s]+)`)
)

type tlsErrorPattern struct {
	reason      string
	patterns    []string
	message     string
	errorCode   string
	recoverable bool
}

// ParseTLSError classifies a TLS-related error string into a specific event reason.
// It returns a TLSErrorClassification with the detected reason and a prescriptive message.
func ParseTLSError(ctx context.Context, err error) TLSErrorClassification {
	if err == nil {
		return TLSErrorClassification{}
	}

	errStr := strings.ToLower(err.Error())

	for _, p := range tlsErrorPatterns {
		for _, pattern := range p.patterns {
			if strings.Contains(errStr, strings.ToLower(pattern)) {
				csmlog.WithContext(ctx).WithFields(map[string]interface{}{
					"tls_error_reason": p.reason,
					"error":            err.Error(),
				}).Debug("TLS error classified")
				return TLSErrorClassification{
					Reason:      p.reason,
					Message:     p.message,
					ErrorCode:   p.errorCode,
					Recoverable: p.recoverable,
				}
			}
		}
	}

	// Default classification for unmapped TLS errors
	return TLSErrorClassification{
		Reason:      constants.MTLSEventReasonMountFailed,
		Message:     "mTLS mount failed: " + err.Error(),
		ErrorCode:   "Internal",
		Recoverable: false,
	}
}

// IsTLSError determines whether an error is TLS-related.
//
// Matching is anchored on TLS-domain terms only. Bare symptom words such as
// "expired", "issuer" or "san" are deliberately NOT sufficient on their own: they
// are substrings of unrelated messages ("the session has expired", "unknown issuer
// of request", "sanitize input", "sandbox error"). Because IsTLSError gates
// ParseTLSError and the emission of certificate-specific Kubernetes events, a false
// positive would mislabel a storage or network failure as a certificate problem and
// hand the operator a misleading diagnostic.
//
// Dropping the standalone symptom words loses no real coverage: every TLS failure
// surfaced by the kernel, tlshd or crypto/x509 names its domain ("x509: certificate
// has expired", "unable to get local issuer certificate", "tlshd: handshake failed"),
// so the anchor is always present. Genuine timeouts that carry no TLS anchor are
// handled by the separate IsTimeoutError path in NodePublishVolume.
func IsTLSError(err error) bool {
	if err == nil {
		return false
	}
	return containsTLSDomainAnchor(strings.ToLower(err.Error()))
}

// tlsDomainAnchors are terms that unambiguously identify the TLS/certificate domain.
var tlsDomainAnchors = []string{
	"tls", "x509", "certificate", "handshake", "tlshd",
	"ssl", "xprtsec", "trust anchor", "subject alternative name",
}

// containsTLSDomainAnchor reports whether the (lower-cased) error string contains a
// term that unambiguously places the error in the TLS/certificate domain.
func containsTLSDomainAnchor(errStr string) bool {
	for _, anchor := range tlsDomainAnchors {
		if strings.Contains(errStr, anchor) {
			return true
		}
	}
	return false
}

// ExtractCertificateInfo parses OpenSSL-style certificate subject/issuer information
// from a string (e.g., openssl x509 -in cert -noout -subject -issuer).
func ExtractCertificateInfo(output string) map[string]string {
	info := map[string]string{
		"subject":  "",
		"issuer":   "",
		"notAfter": "",
	}

	if matches := certSubjectRegexp.FindAllStringSubmatch(output, -1); len(matches) > 0 {
		subjects := make([]string, 0, len(matches))
		for _, m := range matches {
			if len(m) >= 3 {
				subjects = append(subjects, m[1]+"="+m[2])
			}
		}
		info["subject"] = strings.Join(subjects, ", ")
	}

	if matches := certIssuerRegexp.FindAllStringSubmatch(output, -1); len(matches) > 0 {
		issuers := make([]string, 0, len(matches))
		for _, m := range matches {
			if len(m) >= 3 {
				issuers = append(issuers, m[1]+"="+m[2])
			}
		}
		info["issuer"] = strings.Join(issuers, ", ")
	}

	if match := certNotAfter.FindStringSubmatch(output); len(match) >= 2 {
		info["notAfter"] = strings.TrimSpace(match[1])
	}

	return info
}

// Mockable functions for testing
var (
	execCommandFunc = exec.Command
	fileExistsFunc  = os.Stat
)

// GetTLSHandshakeTimeout returns the TLS handshake timeout in seconds from
// environment variable or default.
func GetTLSHandshakeTimeout(ctx context.Context) int {
	timeoutStr := os.Getenv(constants.EnvTLSHandshakeTimeoutSeconds)
	if timeoutStr == "" {
		return constants.DefaultTLSHandshakeTimeout
	}

	// parse timeout, default to 30 on error
	parsedTimeout, err := parseInt(timeoutStr, 10, 64)
	if err != nil {
		csmlog.WithContext(ctx).Warnf("Invalid value '%s' for %s, using default %d seconds", timeoutStr, constants.EnvTLSHandshakeTimeoutSeconds, constants.DefaultTLSHandshakeTimeout)
		return constants.DefaultTLSHandshakeTimeout
	}
	if parsedTimeout <= 0 {
		csmlog.WithContext(ctx).Warnf("Non-positive value '%s' for %s, using default %d seconds", timeoutStr, constants.EnvTLSHandshakeTimeoutSeconds, constants.DefaultTLSHandshakeTimeout)
		return constants.DefaultTLSHandshakeTimeout
	}
	return int(parsedTimeout)
}

// parseInt is a wrapper for strconv.ParseInt, mockable for tests
var parseInt = func(s string, base, bitSize int) (int64, error) {
	return strconv.ParseInt(s, base, bitSize)
}

// CreateMountTimeoutContext creates a context with the configured TLS handshake timeout.
// If transportSecurity is not mTLS, the original context is returned unchanged.
func CreateMountTimeoutContext(ctx context.Context, transportSecurity string) (context.Context, context.CancelFunc) {
	if !IsMTLSEnabled(transportSecurity) {
		return ctx, func() {}
	}

	timeout := GetTLSHandshakeTimeout(ctx)
	csmlog.WithContext(ctx).Debugf("Using TLS handshake timeout of %d seconds", timeout)
	return context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
}

// MountError wraps an error from a mount operation and provides additional context.
type MountError struct {
	Operation string
	Err       error
	Timeout   bool
}

func (e *MountError) Error() string {
	if e.Timeout {
		return "mount operation timed out: " + e.Err.Error()
	}
	return "mount operation failed: " + e.Err.Error()
}

// Unwrap returns the underlying error for errors.Is/As support.
func (e *MountError) Unwrap() error {
	return e.Err
}

// IsTimeoutError checks if an error indicates a timeout.
func IsTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	var mountErr *MountError
	if errors.As(err, &mountErr) {
		return mountErr.Timeout
	}
	return errors.Is(err, context.DeadlineExceeded) || os.IsTimeout(err)
}
