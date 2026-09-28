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

package constants

const (

	// EnvCSIEndpoint is the name of the unix domain socket that the csi driver is listening on
	EnvCSIEndpoint = "CSI_ENDPOINT"

	// EnvPort is the name of the enviroment variable used to set the
	// HTTPS port number of the Isilon OneFS API server
	EnvPort = "X_CSI_ISI_PORT"

	// EnvSkipCertificateValidation is the name of the enviroment variable used to specify
	// that the Isilon OneFS API server's certificate chain and host name should not
	// be verified
	EnvSkipCertificateValidation = "X_CSI_ISI_SKIP_CERTIFICATE_VALIDATION"

	// EnvIsiAuthType sets the default Authentication method as Basic Authentication
	EnvIsiAuthType = "X_CSI_ISI_AUTH_TYPE"

	// EnvPath is the root path under which all the volumes (directories) will be provisioned, e.g. /ifs/engineering
	EnvPath = "X_CSI_ISI_PATH"

	// EnvIsiVolumePathPermissions is the default permissions for volume directory path, e.g. 0777
	EnvIsiVolumePathPermissions = "X_CSI_ISI_VOLUME_PATH_PERMISSIONS"

	// EnvIgnoreUnresolvableHosts exhibits default OneFS behavior of failing to add export if any of existing hosts from
	// same export is unresolvable
	EnvIgnoreUnresolvableHosts = "X_CSI_ISI_IGNORE_UNRESOLVABLE_HOSTS"

	// EnvAutoProbe is the name of the environment variable used to specify
	// that the controller service should automatically probe itself if it
	// receives incoming requests before having been probed, in direct
	// violation of the CSI spec
	EnvAutoProbe = "X_CSI_ISI_AUTOPROBE"

	// EnvGOCSIDebug indicates whether to print REQUESTs and RESPONSEs of all CSI method calls(from gocsi)
	EnvGOCSIDebug = "X_CSI_DEBUG"

	// EnvVerbose indicates whether the driver should log OneFS REST API response body content
	EnvVerbose = "X_CSI_VERBOSE"

	// EnvQuotaEnabled is the boolean flag that indicates whether the provisioner should attempt to set (later unset) quota on a newly provisioned volume
	EnvQuotaEnabled = "X_CSI_ISI_QUOTA_ENABLED"

	// EnvAccessZone is the name of the access zone a volume can be created in, e.g. "System"
	EnvAccessZone = "X_CSI_ISI_ACCESS_ZONE"

	// EnvNoProbeOnStart indicates whether a probe should be attempted upon start
	EnvNoProbeOnStart = "X_CSI_ISI_NO_PROBE_ON_START"

	// EnvNodeName is the name of a k8s node
	EnvNodeName = "X_CSI_NODE_NAME"

	// EnvNodeIP is the ip address of a k8s node
	EnvNodeIP = "X_CSI_NODE_IP"

	// EnvCustomTopologyEnabled indicates if custom topology has to be used by CSI Driver
	EnvCustomTopologyEnabled = "X_CSI_CUSTOM_TOPOLOGY_ENABLED"

	// EnvKubeConfigPath indicates kubernetes configuration that has to be used by CSI Driver
	EnvKubeConfigPath = "KUBECONFIG"

	// EnvAllowedNetworks indicates list of networks on which NFS traffic is allowed
	EnvAllowedNetworks = "X_CSI_ALLOWED_NETWORKS"

	// EnvAllowedNetworksMode indicates the NFS network selection mode (single or multi)
	EnvAllowedNetworksMode = "X_CSI_ALLOWED_NETWORKS_MODE"

	// EnvIsilonConfigFile specifies the filepath containing Isilon cluster's config details
	EnvIsilonConfigFile = "X_CSI_ISI_CONFIG_PATH"

	// EnvMaxVolumesPerNode specifies maximum number of volumes that controller can publish to the node.
	EnvMaxVolumesPerNode = "X_CSI_MAX_VOLUMES_PER_NODE"

	// EnvIsHealthMonitorEnabled specifies if health monitor is enabled.
	EnvIsHealthMonitorEnabled = "X_CSI_HEALTH_MONITOR_ENABLED"

	// EnvReplicationContextPrefix enables sidecars to read required information from volume context
	EnvReplicationContextPrefix = "X_CSI_REPLICATION_CONTEXT_PREFIX"

	// EnvReplicationPrefix is used as a prefix to find out if replication is enabled
	EnvReplicationPrefix = "X_CSI_REPLICATION_PREFIX" // #nosec G101

	// EnvPodmonEnabled indicates that podmon is enabled
	EnvPodmonEnabled = "X_CSI_PODMON_ENABLED"

	// EnvPodmonAPIPORT indicates the port to be used for exposing podmon API health
	EnvPodmonAPIPORT = "X_CSI_PODMON_API_PORT"

	// EnvPodmonArrayConnectivityPollRate indicates the polling frequency to check array connectivity
	EnvPodmonArrayConnectivityPollRate = "X_CSI_PODMON_ARRAY_CONNECTIVITY_POLL_RATE"

	// EnvPodmonAPIToken is the shared secret token used to authenticate requests
	// between the CSI controller and node podmon API endpoints.
	// When set, both the node HTTP server and the controller HTTP client
	// will use Bearer token authentication. If unset, authentication is skipped
	// for backward compatibility.
	EnvPodmonAPIToken = "X_CSI_PODMON_API_TOKEN" // #nosec G101

	// EnvMetadataRetrieverEndpoint specifies the endpoint address for csi-metadata-retriever sidecar
	EnvMetadataRetrieverEndpoint = "CSI_RETRIEVER_ENDPOINT"

	// EnvMetricsEnabled enables the Prometheus metrics HTTP server on the driver pods
	EnvMetricsEnabled = "X_CSI_METRICS_ENABLED"

	// EnvMetricsPort is the port on which the Prometheus metrics HTTP server listens
	EnvMetricsPort = "X_CSI_METRICS_PORT"

	// EnvMetricsTLSCertFile is the TLS certificate file for the metrics endpoint
	EnvMetricsTLSCertFile = "X_CSI_METRICS_TLS_CERT_FILE"

	// EnvMetricsTLSKeyFile is the TLS private key file for the metrics endpoint
	EnvMetricsTLSKeyFile = "X_CSI_METRICS_TLS_KEY_FILE"

	// EnvMetricsCollectionInterval is the background metrics collection interval
	EnvMetricsCollectionInterval = "X_CSI_METRICS_COLLECTION_INTERVAL"

	// EnvMetricsCollectionCacheTTL is the cache TTL for metrics collection responses
	EnvMetricsCollectionCacheTTL = "X_CSI_METRICS_COLLECTION_CACHE_TTL"

	// EnvMetricsArrayRateLimit is the OneFS metrics request budget per minute per endpoint
	EnvMetricsArrayRateLimit = "X_CSI_METRICS_ARRAY_RATE_LIMIT"

	// EnvMetricsArrayTimeout is the timeout for OneFS metrics calls
	EnvMetricsArrayTimeout = "X_CSI_METRICS_ARRAY_TIMEOUT"

	// EnvMetricsArrayCBThreshold is the circuit breaker failure threshold for OneFS metrics calls
	EnvMetricsArrayCBThreshold = "X_CSI_METRICS_ARRAY_CB_THRESHOLD"

	// EnvMetricsArrayCBResetTimeout is the circuit breaker reset timeout for OneFS metrics calls
	EnvMetricsArrayCBResetTimeout = "X_CSI_METRICS_ARRAY_CB_RESET_TIMEOUT"

	// EnvMetricsLeaderElectionEnabled enables leader election for metrics collection in multi-controller deployments
	EnvMetricsLeaderElectionEnabled = "X_CSI_METRICS_LEADER_ELECTION_ENABLED"

	// EnvMetricsLeaderElectionLeaseDuration is the lease duration for metrics leader election
	EnvMetricsLeaderElectionLeaseDuration = "X_CSI_METRICS_LEADER_ELECTION_LEASE_DURATION"

	// EnvMetricsLeaderElectionRenewDeadline is the renew deadline for metrics leader election
	EnvMetricsLeaderElectionRenewDeadline = "X_CSI_METRICS_LEADER_ELECTION_RENEW_DEADLINE"

	// EnvMetricsLeaderElectionRetryPeriod is the retry period for metrics leader election
	EnvMetricsLeaderElectionRetryPeriod = "X_CSI_METRICS_LEADER_ELECTION_RETRY_PERIOD"

	// EnvPodName is the name of the pod where the driver is running
	EnvPodName = "POD_NAME"

	// EnvCSIMode is the mode of the CSI driver (controller or node)
	EnvCSIMode = "X_CSI_MODE"

	// EnvDriverNamespace is the namespace where the PowerScale driver is deployed
	EnvDriverNamespace = "X_CSI_DRIVER_NAMESPACE"

	// EnvEnableDriverFSGroupChown enables driver-side recursive fsGroup chown for export-backed volumes.
	// When disabled, the driver still advertises VOLUME_MOUNT_GROUP (required for directory-backed volumes),
	// but export-backed volumes fall back to a sequential recursive chown within the configured timeout.
	EnvEnableDriverFSGroupChown = "X_CSI_ISILON_ENABLE_DRIVER_FSGROUP_CHOWN"

	// EnvChownWorkers sets the number of parallel os.Chown workers for recursive fsGroup application.
	EnvChownWorkers = "X_CSI_ISILON_CHOWN_WORKERS"

	// EnvChownWriteBatch sets how many completed file paths are buffered before the resumable state file is written.
	EnvChownWriteBatch = "X_CSI_ISILON_CHOWN_WRITEBATCH"

	// EnvChownTimeoutSeconds sets the per-NodeStageVolume/NodePublishVolume timeout for recursive chown.
	EnvChownTimeoutSeconds = "X_CSI_ISILON_CHOWN_TIMEOUT_SECONDS"

	// EnvNFSMountFQDN is the global default FQDN for NFS mounts (lowest precedence)
	EnvNFSMountFQDN = "X_CSI_ISI_NFS_MOUNT_FQDN"

	// EnvTLSHandshakeTimeoutSeconds is the TLS handshake timeout in seconds
	EnvTLSHandshakeTimeoutSeconds = "X_CSI_ISI_TLS_HANDSHAKE_TIMEOUT_SECONDS"
)
