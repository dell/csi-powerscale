package constants

import (
	"time"

	csmlog "github.com/Ecosystems/container-storage-modules/src/csmlog"
)

// Copyright © 2019-2026 Dell Inc. or its subsidiaries. All Rights Reserved.
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

const (
	// PluginName is the name of the CSI plug-in.
	PluginName = "csi-isilon.dellemc.com"

	// VerboseName is a longer description of the driver, used in the Application-Type HTTP header.
	VerboseName = "CSI Driver for Dell EMC PowerScale"

	// DefaultAccessZone is "System"
	DefaultAccessZone = "System"
	// ModeNode is csi driver's "mode "deployment mode
	ModeNode = "node"
	// ModeController is csi driver's "controller "deployment mode
	ModeController = "controller"

	// DefaultVolumeSizeInBytes is default volume sgolang/protobuf/blob/master/ptypesize to create on an Isilon
	// cluster when no size is given, expressed in bytes
	DefaultVolumeSizeInBytes = 3 * BytesInGiB

	// BytesInGiB is the number of bytes in a gigabyte
	BytesInGiB = 1024 * 1024 * 1024
	// TRUE constant
	TRUE = "TRUE"
	// FALSE constant
	FALSE = "FALSE"

	// DefaultPortNumber is the port number in default to set the HTTPS port number of the Isilon OneFS API server
	DefaultPortNumber = "8080"

	// DefaultIsiPath is the default isiPath which will be used if there's
	// no proper isiPath value set in neither storageclass.yaml nor values.yaml
	DefaultIsiPath = "/ifs"

	// DefaultIsiVolumePathPermissions are the default permissions for export-backed volume directory path
	DefaultIsiVolumePathPermissions = "0777"

	// DefaultDirectoryBackedVolumePermissions are the default permissions for directory-backed volume subdirectory path
	DefaultDirectoryBackedVolumePermissions = "0750"

	// MaxIsiConnRetries is the max number of retries to validate connection to PowerScale Array
	MaxIsiConnRetries = 10

	// KubeConfig of kubernetes cluster
	KubeConfig = "KUBECONFIG"

	// IsilonConfigFile isilon-creds file with credential info of isilon clusters
	IsilonConfigFile = "/isilon-configs/config"

	// DefaultLogLevel for csi logs
	DefaultLogLevel = csmlog.DebugLevel

	// ParamCSILogLevel csi driver log level
	ParamCSILogLevel = "CSI_LOG_LEVEL"

	// DefaultPodmonAPIPortNumber is the port number in default to expose internal health APIs
	DefaultPodmonAPIPortNumber = "8083"

	// DefaultPodmonPollRate is the default polling frequency to check for array connectivity
	DefaultPodmonPollRate = 60

	// ParamAZReconcileInterval interval to monitor and reconcile network interface labels on nodes
	ParamAZReconcileInterval = "AZ_RECONCILE_INTERVAL"

	// DefaultAZReconcileInterval default interval to monitor and reconcile network interface labels on nodes
	DefaultAZReconcileInterval = time.Duration(1 * time.Hour)

	// AllowedNetworksModeDefault is the default NFS network selection mode
	AllowedNetworksModeDefault = "single"

	// AllowedNetworksModeMulti selects all matching IPs for multi-NIC support
	AllowedNetworksModeMulti = "multi"

	// MaxInterfaceWarningThreshold is the threshold for warning about too many interfaces
	MaxInterfaceWarningThreshold = 32

	// DefaultMetricsPort is the default port for the driver metrics endpoint
	DefaultMetricsPort = ":8443"

	// DefaultMetricsCollectionInterval is the default collection cadence for metrics collectors
	DefaultMetricsCollectionInterval = 30 * time.Second

	// DefaultMetricsCollectionCacheTTL is the default cache TTL for metrics responses
	DefaultMetricsCollectionCacheTTL = 25 * time.Second

	// DefaultMetricsArrayRateLimit is the default OneFS metrics request budget per endpoint per minute
	DefaultMetricsArrayRateLimit = 100

	// DefaultMetricsArrayTimeout is the default timeout for OneFS metrics calls
	DefaultMetricsArrayTimeout = 30 * time.Second

	// DefaultMetricsArrayCBThreshold is the default failure threshold before opening circuit breaker
	DefaultMetricsArrayCBThreshold = 3

	// DefaultMetricsArrayCBResetTimeout is the default delay before circuit breaker half-open retry
	DefaultMetricsArrayCBResetTimeout = 30 * time.Second

	// DirectoryBackedParam is the StorageClass parameter for directory-backed provisioning mode
	DirectoryBackedParam = "DirectoryBacked"

	// SharedExportPathParam is the StorageClass parameter for shared export path
	SharedExportPathParam = "SharedExportPath"

	// NFSTransportSecurityNone indicates no transport security enforcement
	NFSTransportSecurityNone = "none"

	// NFSTransportSecurityTLS indicates TLS (server authentication only)
	NFSTransportSecurityTLS = "tls"

	// NFSTransportSecurityMTLS indicates mutual TLS (client and server authentication)
	NFSTransportSecurityMTLS = "mtls"

	// SmartConnectZoneFQDNParam is the StorageClass parameter for SmartConnect FQDN
	SmartConnectZoneFQDNParam = "SmartConnectZoneFQDN"

	// NFSTransportSecurityParam is the StorageClass parameter for transport security mode
	NFSTransportSecurityParam = "NFSTransportSecurity"

	// MTLSEventReasonIPAddressForbidden is the Kubernetes event reason for IP address rejection
	MTLSEventReasonIPAddressForbidden = "MTLSIPAddressForbidden"

	// MTLSEventReasonNoFQDNConfigured is the Kubernetes event reason for missing FQDN
	MTLSEventReasonNoFQDNConfigured = "MTLSNoFQDNConfigured"

	// TLSEventReasonHandshakeTimeout is the Kubernetes event reason for TLS handshake timeout
	TLSEventReasonHandshakeTimeout = "TLSHandshakeTimeout"

	// TLSEventReasonCertSANMismatch is the Kubernetes event reason for certificate SAN mismatch
	TLSEventReasonCertSANMismatch = "CertSANMismatch"

	// TLSEventReasonClientCertExpired is the Kubernetes event reason for expired client certificate
	TLSEventReasonClientCertExpired = "ClientCertExpired"

	// TLSEventReasonTrustAnchorFailure is the Kubernetes event reason for trust anchor validation failure
	TLSEventReasonTrustAnchorFailure = "TLSTrustAnchorFailure"

	// TLSEventReasonDaemonMissing is the Kubernetes event reason for missing tlshd daemon
	TLSEventReasonDaemonMissing = "TLSHandshakeDaemonMissing"

	// MTLSEventReasonMountFailed is the Kubernetes event reason for generic mTLS mount failure
	MTLSEventReasonMountFailed = "MTLSMountFailed"

	// TLSEventReasonKernelNotSupported is the Kubernetes event reason for missing kernel TLS support
	TLSEventReasonKernelNotSupported = "KernelTLSNotSupported"

	// TopologyKeyTLSCapable is the topology key for TLS capability
	TopologyKeyTLSCapable = "csi-isilon.dellemc.com/tls-capable"

	// DefaultTLSHandshakeTimeout is the default TLS handshake timeout in seconds
	DefaultTLSHandshakeTimeout = 30

	// KernelTLSModulePath is the path to check for kernel TLS support
	KernelTLSModulePath = "/sys/module/tls"

	// TLSHandshakeDaemonPath is the default path to the tlshd binary. The node
	// DaemonSet mounts the host's /usr/sbin directory read-only at /host/usr/sbin,
	// so the driver detects tlshd here at runtime. The host directory is mounted
	// (rather than the binary directly) so node pods start even when tlshd is not
	// installed; tlshd is only required when mTLS/TLS transport is enabled.
	TLSHandshakeDaemonPath = "/host/usr/sbin/tlshd"
)
