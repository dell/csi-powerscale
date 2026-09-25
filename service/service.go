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

package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/common/k8sutils"
	isilonfs "github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/common/utils/powerscale-fs"
	"github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/service/collectors"
	"github.com/prometheus/client_golang/prometheus"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"gopkg.in/yaml.v3"

	"github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/common/constants"
	fromctx "github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/common/utils/fromcontext"
	id "github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/common/utils/identifiers"

	"github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/core"
	commonext "github.com/Ecosystems/container-storage-modules/src/dell-csi-extensions/common"
	podmon "github.com/Ecosystems/container-storage-modules/src/dell-csi-extensions/podmon"
	csiext "github.com/Ecosystems/container-storage-modules/src/dell-csi-extensions/replication"
	"github.com/Ecosystems/container-storage-modules/src/gocsi"
	csictx "github.com/Ecosystems/container-storage-modules/src/gocsi/context"
	isi "github.com/Ecosystems/container-storage-modules/src/gopowerscale"
	"github.com/Ecosystems/container-storage-modules/src/gopowerscale/api"
	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/fsnotify/fsnotify"

	csmlog "github.com/Ecosystems/container-storage-modules/src/csmlog"
	"github.com/spf13/viper"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
)

var (
	// To maintain runid for Non debug mode. Note: CSI will not generate runid if CSI_DEBUG=false
	isilonConfigFile string
	// DriverConfigParamsFile is the name of the input driver config params file
	DriverConfigParamsFile string

	// Update when the manifest version changes.
	ManifestSemver string

	Manifest = map[string]string{
		"semver": ManifestSemver,
		"formed": core.CommitTime.Format(time.RFC1123),
	}

	noProbeOnStart atomic.Bool

	newIsiClientWithArgsFunc = isi.NewClientWithArgs
)

// PodmonAPIToken is the shared secret token for authenticating podmon API requests.
// This variable is package-scoped; each driver binary maintains its own instance.
var PodmonAPIToken string

// PowerScaleAPIObserver implements the api.RequestObserver interface to track PowerScale API calls
type PowerScaleAPIObserver struct {
	clusterName string
	apiRequests *prometheus.CounterVec
}

// ObservePowerScaleRequest implements the api.RequestObserver interface
func (o *PowerScaleAPIObserver) ObservePowerScaleRequest(obs api.RequestObservation) {
	if o == nil || o.apiRequests == nil {
		return
	}

	status := "failure"
	if obs.Err == nil && obs.StatusCode < 400 {
		status = "success"
	}

	o.apiRequests.WithLabelValues(o.clusterName, status).Inc()
}

// Service is the CSI service provider.
type Service interface {
	csi.ControllerServer
	csi.IdentityServer
	csi.NodeServer
	BeforeServe(context.Context, *gocsi.StoragePlugin, net.Listener) error
	RegisterAdditionalServers(server *grpc.Server)
}

type azNetworkLabels interface {
	setAzReconcileInterval(ctx context.Context, v *viper.Viper)
	getReconcileInterval() time.Duration
	getUpdateIntervalChannel() <-chan time.Duration
	ReconcileNodeAzLabels(ctx context.Context) error
}

type azReconcile interface {
	reconcileNodeAzLabels(ctx context.Context) error
}

// Opts defines service configuration options.
type Opts struct {
	Port                      string
	AccessZone                string
	Path                      string
	IsiVolumePathPermissions  string
	SkipCertificateValidation bool
	AutoProbe                 bool
	QuotaEnabled              bool
	Verbose                   uint
	CustomTopologyEnabled     bool
	KubeConfigPath            string
	allowedNetworks           []string
	allowedNetworksMode       string
	MaxVolumesPerNode         int64
	isiAuthType               uint8
	IsHealthMonitorEnabled    bool
	IgnoreUnresolvableHosts   bool
	replicationContextPrefix  string
	replicationPrefix         string

	MetricsEnabled             bool
	MetricsPort                string
	MetricsTLSCertFile         string
	MetricsTLSKeyFile          string
	MetricsCollectionInterval  time.Duration
	MetricsCollectionCacheTTL  time.Duration
	MetricsArrayRateLimit      int
	MetricsArrayTimeout        time.Duration
	MetricsArrayCBThreshold    int
	MetricsArrayCBResetTimeout time.Duration

	EnableDriverFSGroupChown bool
	ChownWorkers             int
	ChownWriteBatch          int
	ChownTimeoutSeconds      int
}

type service struct {
	// satisfies the Service interface and provided unimplemented defaults to functions not implemented
	csi.UnimplementedControllerServer
	csi.UnimplementedIdentityServer
	csi.UnimplementedNodeServer
	csiext.UnimplementedReplicationServer

	opts                        Opts
	mode                        string
	nodeID                      string
	nodeIP                      string
	statisticsCounter           int
	isiClusters                 *sync.Map
	defaultIsiClusterName       string
	azReconcileInterval         time.Duration
	updateAZReconcileIntervalCh chan time.Duration
	reconcile                   azReconcile
	k8sclient                   kubernetes.Interface
	metricsRegistry             *prometheus.Registry
	metricsStaleReporter        func(clusterName string, stale bool)
	metricsCollectorManager     *collectors.CollectorManager
	driverHealthCollector       *collectors.PSCDriverHealthCollector
	accessControlCollector      *collectors.AccessControlCollector
	directoryExportMu           sync.Map
	eventRecorder               record.EventRecorder
	eventBroadcaster            record.EventBroadcaster
}

type reconciler struct {
	service azNetworkLabels
}

// IsilonClusters To unmarshal secret.yaml file
type IsilonClusters struct {
	IsilonClusters []IsilonClusterConfig `yaml:"isilonClusters"`
}

// IsilonClusterConfig To hold config details of a isilon cluster
type IsilonClusterConfig struct {
	ClusterName               string `yaml:"clusterName"`
	Endpoint                  string `yaml:"endpoint"`
	EndpointPort              string `yaml:"endpointPort,omitempty"`
	MountEndpoint             string `yaml:"mountEndpoint,omitempty"` // This field is used to retain the orignal Endpoint after CSM-Authorization has been injected
	EndpointURL               string
	accessZone                string `yaml:"accessZone,omitempty"`
	User                      string `yaml:"username"`
	Password                  string `yaml:"password"`
	SkipCertificateValidation *bool  `yaml:"skipCertificateValidation,omitempty"`
	IsiPath                   string `yaml:"isiPath,omitempty"`
	IsiVolumePathPermissions  string `yaml:"isiVolumePathPermissions,omitempty"`
	IsDefault                 *bool  `yaml:"isDefault,omitempty"`
	ReplicationCertificateID  string `yaml:"replicationCertificateID,omitempty"`
	IgnoreUnresolvableHosts   *bool  `yaml:"ignoreUnresolvableHosts,omitempty"`
	NFSMountFQDN              string `yaml:"nfsMountFQDN,omitempty"`
	isiSvc                    *isiService
}

// To display the IsilonClusterConfig of a cluster
func (s IsilonClusterConfig) String() string {
	return fmt.Sprintf("ClusterName: %s, Endpoint: %s, EndpointPort: %s, EndpointURL: %s, User: %s, SkipCertificateValidation: %v, IsiPath: %s, IsiVolumePathPermissions: %s, IsDefault: %v, IgnoreUnresolvableHosts: %v, AccessZone: %s, NFSMountFQDN: %s, isiSvc: %v",
		s.ClusterName, s.Endpoint, s.EndpointPort, s.EndpointURL, s.User, *s.SkipCertificateValidation, s.IsiPath, s.IsiVolumePathPermissions, *s.IsDefault, *s.IgnoreUnresolvableHosts, s.accessZone, s.NFSMountFQDN, s.isiSvc)
}

// New returns a new Service.
func New() Service {
	svc := &service{}
	if enabled, _ := strconv.ParseBool(os.Getenv(constants.EnvMetricsEnabled)); enabled {
		svc.metricsRegistry = prometheus.NewRegistry()
	}
	return svc
}

// MetricsRegistry returns the Prometheus registry used for driver metrics,
// or nil when metrics are disabled.
func (s *service) MetricsRegistry() *prometheus.Registry {
	return s.metricsRegistry
}

// startMetricsCollectors instantiates all per-cluster metric collectors and
// starts a background goroutine that calls CollectAll every 30 seconds.
// It is a no-op when metricsRegistry is nil.
func (s *service) startMetricsCollectors(ctx context.Context) {
	if s.metricsRegistry == nil {
		csmlog.WithContext(ctx).Infof("Metrics registry is nil, skipping metrics collection (mode=%s, MetricsEnabled=%v)", s.mode, s.opts.MetricsEnabled)
		return
	}
	csmlog.WithContext(ctx).Infof("Starting metrics collectors (mode=%s, MetricsEnabled=%v)", s.mode, s.opts.MetricsEnabled)

	// Skip all array-specific metrics that require PowerScale API access
	if strings.EqualFold(s.mode, constants.ModeNode) {
		csmlog.WithContext(ctx).Infof("Node mode detected, registering driver health collector")
		if s.metricsCollectorManager != nil {
			s.metricsCollectorManager.Stop()
			s.metricsCollectorManager = nil
		}

		mgr := collectors.NewCollectorManager()

		// Get cluster name for node pods
		var clusterName string
		s.isiClusters.Range(func(_, value interface{}) bool {
			if cfg, ok := value.(*IsilonClusterConfig); ok && cfg != nil {
				clusterName = cfg.ClusterName
				return false // stop iteration after first cluster
			}
			return true
		})
		if clusterName == "" {
			clusterName = "default" // fallback cluster name
		}
		csmlog.WithContext(ctx).Infof("Node mode: clusterName=%s, k8sclient=%v", clusterName, s.k8sclient != nil)

		// Register driver health collector for node pods (includes API calls tracking)
		healthCollector := collectors.NewPSCDriverHealthCollector(s.metricsRegistry, clusterName, s.k8sclient)
		mgr.Register(healthCollector)
		csmlog.WithContext(ctx).Infof("Registered PSCDriverHealthCollector")

		s.metricsCollectorManager = mgr
		s.driverHealthCollector = healthCollector

		// Use default interval if not set (same as controller mode)
		interval := s.opts.MetricsCollectionInterval
		if interval <= 0 {
			interval = constants.DefaultMetricsCollectionInterval
		}
		csmlog.WithContext(ctx).Infof("Starting node mode metrics collector with interval=%v", interval)
		mgr.Start(ctx, interval)
		csmlog.WithContext(ctx).Infof("Node mode metrics collector started")
		return
	}

	// Controller mode: register all collectors including array-specific ones
	if s.metricsCollectorManager != nil {
		s.metricsCollectorManager.Stop()
		s.metricsCollectorManager = nil
	}

	mgr := collectors.NewCollectorManager()

	// Register driver health collector once for controller mode
	var clusterName string
	s.isiClusters.Range(func(_, value interface{}) bool {
		if cfg, ok := value.(*IsilonClusterConfig); ok && cfg != nil {
			clusterName = cfg.ClusterName
			return false
		}
		return true
	})
	if clusterName == "" {
		csmlog.WithContext(ctx).Warn("No cluster config available yet; skipping controller metrics collector start to avoid default cluster labels")
		return
	}
	healthCollector := collectors.NewPSCDriverHealthCollector(s.metricsRegistry, clusterName, s.k8sclient)
	mgr.Register(healthCollector)
	s.driverHealthCollector = healthCollector
	csmlog.WithContext(ctx).Infof("Initialized driverHealthCollector for cluster %s", clusterName)

	// Use a closure that captures s by pointer so metricsStaleReporter is
	// resolved at call time rather than at MetricsRuntime creation time.
	// This is necessary because startMetricsCollectors is called (via
	// updateDriverConfigParams → reconcileMetricsCollectors) before
	// startMetricsServer has had a chance to set s.metricsStaleReporter,
	// which would leave the MetricsRuntime with a permanently-nil reporter.
	runtimeCfg := collectors.RuntimeConfig{
		Timeout:        s.opts.MetricsArrayTimeout,
		CacheTTL:       s.opts.MetricsCollectionCacheTTL,
		RateLimit:      s.opts.MetricsArrayRateLimit,
		CBThreshold:    s.opts.MetricsArrayCBThreshold,
		CBResetTimeout: s.opts.MetricsArrayCBResetTimeout,
		StaleReporter: func(clusterName string, stale bool) {
			if s.metricsStaleReporter != nil {
				s.metricsStaleReporter(clusterName, stale)
			}
		},
	}

	// Check if leader election is enabled for metrics
	leaderElectionEnabled, _ := strconv.ParseBool(os.Getenv(constants.EnvMetricsLeaderElectionEnabled))

	if leaderElectionEnabled {
		s.startMetricsWithLeaderElection(ctx, mgr, runtimeCfg)
	} else {
		s.startMetricsWithoutLeaderElection(ctx, mgr, runtimeCfg)
	}
}

// startMetricsWithoutLeaderElection starts all collectors without leader election (current behavior)
func (s *service) startMetricsWithoutLeaderElection(ctx context.Context, mgr *collectors.CollectorManager, runtimeCfg collectors.RuntimeConfig) {
	// Register collectors for all clusters
	s.isiClusters.Range(func(_, value interface{}) bool {
		cfg, ok := value.(*IsilonClusterConfig)
		if !ok || cfg == nil {
			return true
		}

		clusterName := cfg.ClusterName
		isiPath := cfg.IsiPath
		if isiPath == "" {
			isiPath = s.opts.Path
		}
		rt := collectors.NewMetricsRuntime(clusterName, runtimeCfg)

		// Note: Driver health collector is registered once above, not per cluster

		// AccessControl collector - register only when isiSvc is initialized.
		// The collector will skip data collection if the client becomes unavailable,
		// but the metrics will be present as long as the collector is registered.
		if cfg.isiSvc != nil {
			accessControlCollector := collectors.NewAccessControlCollector(
				collectors.NewAccessControlAdapterWithRuntime(cfg.isiSvc.client, rt), s.metricsRegistry, clusterName,
			)
			mgr.Register(accessControlCollector)
			if s.accessControlCollector == nil {
				s.accessControlCollector = accessControlCollector
			}
		} else {
			// Register with nil client - metrics will be present but collection will fail gracefully
			csmlog.WithContext(ctx).Warnf("isiSvc not available for cluster %s, registering AccessControlCollector with nil client", clusterName)
			accessControlCollector := collectors.NewAccessControlCollector(
				collectors.NewAccessControlAdapterWithRuntime(nil, rt), s.metricsRegistry, clusterName,
			)
			mgr.Register(accessControlCollector)
			if s.accessControlCollector == nil {
				s.accessControlCollector = accessControlCollector
			}
		}

		// Remaining collectors require an active isiSvc
		if cfg.isiSvc == nil {
			csmlog.WithContext(ctx).Warnf("Skipping quota/nodepool/NFS collectors for cluster %s (isiSvc unavailable)", clusterName)
			return true
		}

		client := cfg.isiSvc.client

		// Create quota adapter with hybrid validation: path-based filtering + optional K8s validation
		var quotaAdapter *collectors.QuotaAdapter
		if s.k8sclient != nil {
			// Enable K8s validation for additional accuracy
			k8sValidator := collectors.NewK8sMetadataChecker(s.k8sclient, constants.PluginName)
			validator := collectors.NewHybridVolumeValidatorWithK8s(isiPath, k8sValidator)
			quotaAdapter = collectors.NewQuotaAdapterWithValidator(client, validator)
		} else {
			// Fall back to path-based filtering only
			quotaAdapter = collectors.NewQuotaAdapterWithPath(client, isiPath)
		}
		// Wrap with runtime for timeout, rate limiting, circuit breaking, and caching
		quotaAdapter.SetRuntime(rt)

		mgr.Register(collectors.NewQuotaCollector(
			quotaAdapter, s.metricsRegistry, clusterName,
		))
		mgr.Register(collectors.NewNodePoolCollector(
			collectors.NewNodePoolAdapterWithRuntime(client, rt), s.metricsRegistry, clusterName,
		))
		nfsAdapter := collectors.NewNFSAdapter(client)
		nfsAdapter.SetRuntime(rt)
		mgr.Register(collectors.NewNFSPerformanceCollector(
			nfsAdapter, s.metricsRegistry, clusterName,
		))

		csmlog.WithContext(ctx).Infof("Registered metrics collectors for cluster %s", clusterName)
		return true
	})

	interval := s.opts.MetricsCollectionInterval
	if interval <= 0 {
		interval = constants.DefaultMetricsCollectionInterval
	}
	csmlog.WithContext(ctx).Infof("Starting metrics collector with interval=%v", interval)
	mgr.Start(ctx, interval)
	s.metricsCollectorManager = mgr
}

// leaderElectionForMetricsFunc runs leader election for array-level metrics collection.
// It is a package-level variable so it can be overridden in unit tests.
var leaderElectionForMetricsFunc = k8sutils.LeaderElectionForMetrics

// startMetricsWithLeaderElection starts metrics with leader election for array-level collectors
func (s *service) startMetricsWithLeaderElection(ctx context.Context, mgr *collectors.CollectorManager, runtimeCfg collectors.RuntimeConfig) {
	// Get leader election configuration
	namespace := os.Getenv(constants.EnvDriverNamespace)
	if namespace == "" {
		namespace = "default"
	}

	leaseDuration := parseDuration(os.Getenv(constants.EnvMetricsLeaderElectionLeaseDuration), 15*time.Second)
	renewDeadline := parseDuration(os.Getenv(constants.EnvMetricsLeaderElectionRenewDeadline), 10*time.Second)
	retryPeriod := parseDuration(os.Getenv(constants.EnvMetricsLeaderElectionRetryPeriod), 5*time.Second)

	// Reuse the service client if possible so leader election follows the same
	// Kubernetes config as the rest of the driver.
	var k8sclientset kubernetes.Interface
	if s.k8sclient != nil {
		k8sclientset = s.k8sclient
	} else {
		kubeConfigPath, _ := csictx.LookupEnv(ctx, constants.EnvKubeConfigPath)
		var err error
		k8sclientset, err = k8sutils.CreateKubeClientSet(kubeConfigPath)
		if err != nil {
			csmlog.WithContext(ctx).Errorf("failed to create kubernetes clientset for metrics leader election: %v", err)
			csmlog.WithContext(ctx).Warn("Falling back to starting metrics without leader election")
			s.startMetricsWithoutLeaderElection(ctx, mgr, runtimeCfg)
			return
		}
	}

	// Register pod-level collectors immediately (run on all pods)
	csmlog.WithContext(ctx).Info("Registering pod-level collectors (run on all pods)")
	for _, collector := range s.getPodLevelCollectors(runtimeCfg) {
		mgr.Register(collector)
	}

	// Leader election run function for array-level collectors
	arrayLevelRunFunc := func(ctx context.Context) {
		csmlog.WithContext(ctx).Info("Became leader for metrics collection - starting array-level collectors")

		arrayMgr := collectors.NewCollectorManager()

		// Register array-level collectors when leader
		for _, collector := range s.getArrayLevelCollectors(runtimeCfg) {
			arrayMgr.Register(collector)
		}

		interval := s.opts.MetricsCollectionInterval
		if interval <= 0 {
			interval = constants.DefaultMetricsCollectionInterval
		}
		arrayMgr.Start(ctx, interval)
		csmlog.WithContext(ctx).Infof("Started array-level metrics collection with leader election")
	}

	// Start leader election for array-level collectors
	lockName := "csi-powerscale-metrics"
	csmlog.WithContext(ctx).Infof("Starting leader election for array-level metrics with lock name: %s, namespace: %s", lockName, namespace)

	go func() {
		leaderElectionForMetricsFunc(
			ctx,
			k8sclientset,
			lockName,
			namespace,
			renewDeadline,
			leaseDuration,
			retryPeriod,
			arrayLevelRunFunc,
		)
	}()

	// Start pod-level collectors without leader election
	interval := s.opts.MetricsCollectionInterval
	if interval <= 0 {
		interval = constants.DefaultMetricsCollectionInterval
	}
	mgr.Start(ctx, interval)
	s.metricsCollectorManager = mgr
	csmlog.WithContext(ctx).Infof("Started pod-level metrics collection")
}

// getPodLevelCollectors returns collectors that should run on all pods (no leader election needed)
func (s *service) getPodLevelCollectors(runtimeCfg collectors.RuntimeConfig) []collectors.Collector {
	var podCollectors []collectors.Collector

	s.isiClusters.Range(func(_, value interface{}) bool {
		cfg, ok := value.(*IsilonClusterConfig)
		if !ok || cfg == nil {
			return true
		}

		clusterName := cfg.ClusterName
		rt := collectors.NewMetricsRuntime(clusterName, runtimeCfg)

		// AccessControl collector - register unconditionally so metrics always appear in /metrics
		// even when isiSvc is not yet initialized. The collector will skip data collection
		// if the client is unavailable, but the metrics will be present.
		var client *isi.Client
		if cfg.isiSvc != nil {
			client = cfg.isiSvc.client
		}
		accessControlCollector := collectors.NewAccessControlCollector(
			collectors.NewAccessControlAdapterWithRuntime(client, rt), s.metricsRegistry, clusterName,
		)
		podCollectors = append(podCollectors, accessControlCollector)
		// Store reference to first AccessControlCollector for auth/permission tracking
		if s.accessControlCollector == nil {
			s.accessControlCollector = accessControlCollector
		}
		return true
	})

	return podCollectors
}

// getArrayLevelCollectors returns collectors that should run only on leader (leader election needed)
// NodePoolCollector and QuotaCollector are leader-elected to save API calls
// AccessControlCollector is on pod-level to ensure permission denials are recorded on all pods
func (s *service) getArrayLevelCollectors(runtimeCfg collectors.RuntimeConfig) []collectors.Collector {
	var arrayCollectors []collectors.Collector

	s.isiClusters.Range(func(_, value interface{}) bool {
		cfg, ok := value.(*IsilonClusterConfig)
		if !ok || cfg == nil || cfg.isiSvc == nil {
			return true
		}

		client := cfg.isiSvc.client
		clusterName := cfg.ClusterName
		isiPath := cfg.IsiPath
		if isiPath == "" {
			isiPath = s.opts.Path
		}
		rt := collectors.NewMetricsRuntime(clusterName, runtimeCfg)

		// Quota collector - only leader should run this to avoid duplicate API calls
		// This covers: Volume Count, Volume Size Distribution, Quota Utilization, Quota Remaining, Hard Quota Breaches
		var quotaAdapter *collectors.QuotaAdapter
		if s.k8sclient != nil {
			k8sValidator := collectors.NewK8sMetadataChecker(s.k8sclient, constants.PluginName)
			validator := collectors.NewHybridVolumeValidatorWithK8s(isiPath, k8sValidator)
			quotaAdapter = collectors.NewQuotaAdapterWithValidator(client, validator)
		} else {
			quotaAdapter = collectors.NewQuotaAdapterWithPath(client, isiPath)
		}
		quotaAdapter.SetRuntime(rt)
		arrayCollectors = append(arrayCollectors, collectors.NewQuotaCollector(
			quotaAdapter, s.metricsRegistry, clusterName,
		))

		// NodePool collector - only leader should run this to avoid duplicate API calls
		// This covers: Node Pool Capacity, Node Pool Utilization, Data Protection Status, Tiering Status
		arrayCollectors = append(
			arrayCollectors,
			collectors.NewNodePoolCollector(
				collectors.NewNodePoolAdapterWithRuntime(client, rt), s.metricsRegistry, clusterName,
			),
		)

		// NFS Performance collector - only leader should run this to avoid duplicate API calls
		// This covers: NFS read/write throughput, NFS request count (Jira requirement for NFS performance monitoring)
		nfsAdapter := collectors.NewNFSAdapter(client)
		nfsAdapter.SetRuntime(rt)
		arrayCollectors = append(
			arrayCollectors,
			collectors.NewNFSPerformanceCollector(
				nfsAdapter, s.metricsRegistry, clusterName,
			),
		)

		return true
	})

	return arrayCollectors
}

// parseDuration parses a duration string and returns the duration, or default if parsing fails
func parseDuration(durationStr string, defaultDuration time.Duration) time.Duration {
	if durationStr == "" {
		return defaultDuration
	}
	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		return defaultDuration
	}
	return duration
}

// reconcileMetricsCollectors stops the current collector manager and rebuilds
// it from the current isiClusters snapshot. It is a no-op when metrics are
// disabled. Prometheus re-registration is safe because all collector
// constructors use registerOrGetGaugeVec which reuses already-registered
// GaugeVec instances.
func (s *service) reconcileMetricsCollectors(ctx context.Context) {
	if s.metricsRegistry == nil {
		return
	}
	if strings.EqualFold(s.mode, constants.ModeNode) {
		return
	}
	csmlog.WithContext(ctx).Info("Reconciling metrics collectors after cluster config reload")
	s.startMetricsCollectors(ctx)

	// Note: Metrics are now handled by observer in gopowerscale library
	// No need to update isiService instances after driverHealthCollector initialization
}

func (s *service) initializeServiceOpts(ctx context.Context) error {
	// Get the SP's operating mode.
	s.mode = csictx.Getenv(ctx, gocsi.EnvVarMode)

	opts := Opts{}

	if port, ok := csictx.LookupEnv(ctx, constants.EnvPort); ok {
		opts.Port = port
	} else {
		// If the port number cannot be fetched, set it to default
		opts.Port = constants.DefaultPortNumber
	}

	if path, ok := csictx.LookupEnv(ctx, constants.EnvPath); ok {
		if path == "" {
			path = constants.DefaultIsiPath
		}
		opts.Path = path
	} else {
		opts.Path = constants.DefaultIsiPath
	}

	if isiVolumePathPermissions, ok := csictx.LookupEnv(ctx, constants.EnvIsiVolumePathPermissions); ok {
		if isiVolumePathPermissions == "" {
			isiVolumePathPermissions = constants.DefaultIsiVolumePathPermissions
		}
		opts.IsiVolumePathPermissions = isiVolumePathPermissions
	} else {
		opts.IsiVolumePathPermissions = constants.DefaultIsiVolumePathPermissions
	}

	if accessZone, ok := csictx.LookupEnv(ctx, constants.EnvAccessZone); ok {
		if accessZone == "" {
			accessZone = constants.DefaultAccessZone
		}
		opts.AccessZone = accessZone
	} else {
		opts.AccessZone = constants.DefaultAccessZone
	}

	if nodeNameEnv, ok := csictx.LookupEnv(ctx, constants.EnvNodeName); ok {
		s.nodeID = nodeNameEnv
	}

	if nodeIPEnv, ok := csictx.LookupEnv(ctx, constants.EnvNodeIP); ok {
		s.nodeIP = nodeIPEnv
	}

	if kubeConfigPath, ok := csictx.LookupEnv(ctx, constants.EnvKubeConfigPath); ok {
		opts.KubeConfigPath = kubeConfigPath
	}

	if cfgFile, ok := csictx.LookupEnv(ctx, constants.EnvIsilonConfigFile); ok {
		isilonConfigFile = cfgFile
	} else {
		isilonConfigFile = constants.IsilonConfigFile
	}
	if replicationContextPrefix, ok := csictx.LookupEnv(ctx, constants.EnvReplicationContextPrefix); ok {
		opts.replicationContextPrefix = replicationContextPrefix + "/"
	}
	if replicationPrefix, ok := csictx.LookupEnv(ctx, constants.EnvReplicationPrefix); ok {
		opts.replicationPrefix = replicationPrefix
	}
	if MaxVolumesPerNode, err := fromctx.GetInt64(ctx, constants.EnvMaxVolumesPerNode); err != nil {
		csmlog.WithContext(ctx).Warnf("error while parsing env variable '%s', %s, defaulting to 0", constants.EnvMaxVolumesPerNode, err)
		opts.MaxVolumesPerNode = 0
	} else {
		opts.MaxVolumesPerNode = MaxVolumesPerNode
	}

	allowedNetworks, err := fromctx.GetArray(ctx, constants.EnvAllowedNetworks)
	if err != nil {
		csmlog.WithContext(ctx).Errorf("error while parsing allowedNetworks, %v", err)
		return err
	}
	opts.allowedNetworks = allowedNetworks

	opts.allowedNetworksMode = constants.AllowedNetworksModeDefault
	if mode, ok := csictx.LookupEnv(ctx, constants.EnvAllowedNetworksMode); ok && mode != "" {
		opts.allowedNetworksMode = mode
	}
	if opts.allowedNetworksMode != constants.AllowedNetworksModeDefault &&
		opts.allowedNetworksMode != constants.AllowedNetworksModeMulti {
		csmlog.WithContext(ctx).WithFields(csmlog.Fields{
			"component":             "service-initialization",
			"allowed_networks_mode": opts.allowedNetworksMode,
			"valid_modes":           []string{constants.AllowedNetworksModeDefault, constants.AllowedNetworksModeMulti},
			"success":               false,
		}).Errorf("invalid %s value: %s (valid values: %s, %s)",
			constants.EnvAllowedNetworksMode, opts.allowedNetworksMode,
			constants.AllowedNetworksModeDefault, constants.AllowedNetworksModeMulti)
		return fmt.Errorf("invalid %s value: %s (valid values: %s, %s)",
			constants.EnvAllowedNetworksMode, opts.allowedNetworksMode,
			constants.AllowedNetworksModeDefault, constants.AllowedNetworksModeMulti)
	}
	if opts.allowedNetworksMode == constants.AllowedNetworksModeMulti && len(opts.allowedNetworks) == 0 {
		csmlog.WithContext(ctx).WithFields(csmlog.Fields{
			"component":              "service-initialization",
			"allowed_networks_mode":  opts.allowedNetworksMode,
			"allowed_networks_count": len(opts.allowedNetworks),
			"success":                false,
		}).Errorf("%s=%s requires %s to be set",
			constants.EnvAllowedNetworksMode, constants.AllowedNetworksModeMulti,
			constants.EnvAllowedNetworks)
		return fmt.Errorf("%s=%s requires %s to be set",
			constants.EnvAllowedNetworksMode, constants.AllowedNetworksModeMulti,
			constants.EnvAllowedNetworks)
	}
	csmlog.WithContext(ctx).WithFields(csmlog.Fields{
		"component":              "service-initialization",
		"allowed_networks_mode":  opts.allowedNetworksMode,
		"allowed_networks_count": len(opts.allowedNetworks),
		"allowed_networks":       opts.allowedNetworks,
		"success":                true,
	}).Infof("Multi-NIC mode initialized: %s with %d network(s)",
		opts.allowedNetworksMode, len(opts.allowedNetworks))

	opts.QuotaEnabled = fromctx.GetBoolean(ctx, constants.EnvQuotaEnabled)
	opts.SkipCertificateValidation = fromctx.GetBoolean(ctx, constants.EnvSkipCertificateValidation)
	opts.isiAuthType = uint8(fromctx.GetUint(ctx, constants.EnvIsiAuthType)) // #nosec G115 -- This is a false positive
	opts.AutoProbe = fromctx.GetBoolean(ctx, constants.EnvAutoProbe)
	opts.Verbose = fromctx.GetUint(ctx, constants.EnvVerbose)
	opts.CustomTopologyEnabled = fromctx.GetBoolean(ctx, constants.EnvCustomTopologyEnabled)
	opts.IsHealthMonitorEnabled = fromctx.GetBoolean(ctx, constants.EnvIsHealthMonitorEnabled)
	opts.IgnoreUnresolvableHosts = fromctx.GetBoolean(ctx, constants.EnvIgnoreUnresolvableHosts)

	opts.MetricsEnabled = parseMetricsBool(ctx, constants.EnvMetricsEnabled, false)
	opts.MetricsPort = formatMetricsAddr(csictx.Getenv(ctx, constants.EnvMetricsPort))
	opts.MetricsTLSCertFile = strings.TrimSpace(csictx.Getenv(ctx, constants.EnvMetricsTLSCertFile))
	opts.MetricsTLSKeyFile = strings.TrimSpace(csictx.Getenv(ctx, constants.EnvMetricsTLSKeyFile))
	opts.MetricsCollectionInterval = parseMetricsDuration(ctx, constants.EnvMetricsCollectionInterval, constants.DefaultMetricsCollectionInterval)
	opts.MetricsCollectionCacheTTL = parseMetricsDuration(ctx, constants.EnvMetricsCollectionCacheTTL, constants.DefaultMetricsCollectionCacheTTL)
	opts.MetricsArrayRateLimit = parseMetricsInt(ctx, constants.EnvMetricsArrayRateLimit, constants.DefaultMetricsArrayRateLimit)
	opts.MetricsArrayTimeout = parseMetricsDuration(ctx, constants.EnvMetricsArrayTimeout, constants.DefaultMetricsArrayTimeout)
	opts.MetricsArrayCBThreshold = parseMetricsInt(ctx, constants.EnvMetricsArrayCBThreshold, constants.DefaultMetricsArrayCBThreshold)
	opts.MetricsArrayCBResetTimeout = parseMetricsDuration(ctx, constants.EnvMetricsArrayCBResetTimeout, constants.DefaultMetricsArrayCBResetTimeout)

	opts.EnableDriverFSGroupChown = parseChownBool(ctx, constants.EnvEnableDriverFSGroupChown, true)
	opts.ChownWorkers = parseChownInt(ctx, constants.EnvChownWorkers, 8, 1)
	opts.ChownWriteBatch = parseChownInt(ctx, constants.EnvChownWriteBatch, 1000, 1)
	opts.ChownTimeoutSeconds = parseChownInt(ctx, constants.EnvChownTimeoutSeconds, 30, 1)
	isPodmonEnabled := fromctx.GetBoolean(ctx, constants.EnvPodmonEnabled)
	if podmonAPIToken, ok := csictx.LookupEnv(ctx, constants.EnvPodmonAPIToken); ok && strings.TrimSpace(podmonAPIToken) != "" {
		PodmonAPIToken = strings.TrimSpace(podmonAPIToken)
	} else if isPodmonEnabled {
		csmlog.WithContext(ctx).Warnf("%s is not set; podmon API endpoints will not require authentication", constants.EnvPodmonAPIToken)
	}

	s.opts = opts

	if c, err := k8sutils.CreateKubeClientSet(s.opts.KubeConfigPath); err == nil {
		s.k8sclient = c
	}

	return nil
}

// ValidateCreateVolumeRequest validates the CreateVolumeRequest parameter for a CreateVolume operation
func (s *service) ValidateCreateVolumeRequest(
	req *csi.CreateVolumeRequest,
) (int64, error) {
	cr := req.GetCapacityRange()
	sizeInBytes, err := validateVolSize(cr)
	if err != nil {
		return 0, err
	}

	volumeName := req.GetName()
	if volumeName == "" {
		return 0, status.Error(codes.InvalidArgument,
			"name cannot be empty")
	}

	vcs := req.GetVolumeCapabilities()
	isBlock := isVolumeTypeBlock(vcs)
	if isBlock {
		return 0, errors.New("raw block requested from NFS Volume")
	}

	return sizeInBytes, nil
}

func isVolumeTypeBlock(vcs []*csi.VolumeCapability) bool {
	for _, vc := range vcs {
		if at := vc.GetBlock(); at != nil {
			return true
		}
	}
	return false
}

// ValidateDeleteVolumeRequest validates the DeleteVolumeRequest parameter for a DeleteVolume operation
func (s *service) ValidateDeleteVolumeRequest(ctx context.Context,
	req *csi.DeleteVolumeRequest,
) error {
	if req.GetVolumeId() == "" {
		return status.Error(codes.InvalidArgument,
			"no volume id is provided by the DeleteVolumeRequest instance")
	}

	_, _, _, _, err := id.ParseNormalizedVolumeID(ctx, req.GetVolumeId())
	if err != nil {
		return status.Error(codes.InvalidArgument, fmt.Sprintf("failed to parse volume ID '%s', error : '%v'", req.GetVolumeId(), err))
	}

	return nil
}

func (s *service) probeAllClusters(ctx context.Context) error {
	isilonClusters := s.getIsilonClusters()

	probeSuccessCount := 0
	for i := range isilonClusters {
		err := s.probe(ctx, isilonClusters[i])
		if err == nil {
			probeSuccessCount++
		} else {
			csmlog.WithContext(ctx).Debugf("Probe failed for isilon cluster '%s' error:'%s'", isilonClusters[i].ClusterName, err)
		}
	}

	if probeSuccessCount == 0 {
		return fmt.Errorf("probe of all isilon clusters failed")
	}

	return nil
}

func (s *service) probe(ctx context.Context, clusterConfig *IsilonClusterConfig) error {
	csmlog.WithContext(ctx).Debugf("calling probe for cluster '%s'", clusterConfig.ClusterName)
	// Do a controller probe
	if strings.EqualFold(s.mode, constants.ModeController) {
		if err := s.controllerProbe(ctx, clusterConfig); err != nil {
			return err
		}
	} else if strings.EqualFold(s.mode, constants.ModeNode) {
		if err := s.nodeProbe(ctx, clusterConfig); err != nil {
			return err
		}
	} else if strings.EqualFold(s.mode, "") {
		csmlog.WithContext(ctx).Warn("Service mode not set, attempting both controller and node probe")
		controllerErr := s.controllerProbe(ctx, clusterConfig)
		if controllerErr != nil {
			return fmt.Errorf("probe failed")
		}

		nodeProbeErr := s.nodeProbe(ctx, clusterConfig)
		if nodeProbeErr != nil {
			return fmt.Errorf("probe failed")
		}
	} else {
		return status.Error(codes.FailedPrecondition,
			"Invalid mode")
	}

	return nil
}

func (s *service) probeOnStart(ctx context.Context) error {
	if noProbeOnStart.Load() {
		csmlog.WithContext(ctx).Debugf("noProbeOnStart is true , skip probe")
		return nil
	}

	return s.probeAllClusters(ctx)
}

func (s *service) setNoProbeOnStart(ctx context.Context) {
	if fromctx.GetBoolean(ctx, constants.EnvNoProbeOnStart) {
		csmlog.WithContext(ctx).Debug("X_CSI_ISI_NO_PROBE_ON_START is true, set noProbeOnStart to true")
		noProbeOnStart.Store(true)
		return
	}
	csmlog.WithContext(ctx).Debug("X_CSI_ISI_NO_PROBE_ON_START is false, set noProbeOnStart to false ")
	noProbeOnStart.Store(false)
}

func (s *service) autoProbe(ctx context.Context, isiConfig *IsilonClusterConfig) error {
	if isiConfig.isiSvc != nil {
		csmlog.WithContext(ctx).Debug("isiSvc already initialized, skip probing")
		return nil
	}

	if !s.opts.AutoProbe {
		return status.Error(codes.FailedPrecondition,
			"isiSvc not initialized, but auto probe is not enabled")
	}

	csmlog.WithContext(ctx).Debug("start auto-probing")
	return s.probe(ctx, isiConfig)
}

func (s *service) GetIsiClient(clientCtx context.Context, isiConfig *IsilonClusterConfig) (*isi.Client, error) {
	// First we fetch node labels using kubernetes API and check, if label
	// <provisionerName>.dellemc.com/<powerscalefqdnorip>: <provisionerName>
	// exists on node, if exists we use corresponding PowerScale FQDN or IP for creating connection
	// to PowerScale Array or else we fallback to using endpoint

	customTopologyFound := false
	if s.opts.CustomTopologyEnabled {
		labels, err := s.GetNodeLabels()
		if err != nil {
			return nil, err
		}

		// Iterate node labels and check if required label is available
		for lkey, lval := range labels {
			csmlog.WithContext(clientCtx).Infof("Label is: %s:%s\n", lkey, lval)
			if strings.HasPrefix(lkey, constants.PluginName+"/") && lval == constants.PluginName {
				csmlog.WithContext(clientCtx).Infof("Topology label %s:%s available on node", lkey, lval)
				tList := strings.SplitAfter(lkey, "/")
				if len(tList) != 0 {
					isiConfig.Endpoint = tList[1]
					isiConfig.EndpointURL = fmt.Sprintf("https://%s:%s", isiConfig.Endpoint, isiConfig.EndpointPort)
					customTopologyFound = true
				} else {
					csmlog.WithContext(clientCtx).Errorf("Fetching PowerScale FQDN/IP from topology label %s:%s failed, using endpoint "+
						"%s as PowerScale FQDN/IP", lkey, lval, isiConfig.Endpoint)
				}
				break
			}
		}
	}

	if s.opts.CustomTopologyEnabled && !customTopologyFound {
		csmlog.WithContext(clientCtx).Errorf("init client failed for custom topology")
		return nil, errors.New("init client failed for custom topology")
	}
	client, err := newIsiClientWithArgsFunc(
		clientCtx,
		isiConfig.EndpointURL,
		*isiConfig.SkipCertificateValidation,
		s.opts.Verbose,
		isiConfig.User,
		"",
		isiConfig.Password,
		isiConfig.IsiPath,
		isiConfig.IsiVolumePathPermissions,
		*isiConfig.IgnoreUnresolvableHosts,
		s.opts.isiAuthType,
	)
	if err != nil {
		csmlog.WithContext(clientCtx).Errorf("init client failed for isilon cluster '%s': '%s'", isiConfig.ClusterName, err.Error())
		return nil, err
	}

	client.SetCustomHTTPHeaders(http.Header{
		"Application-Type": {fmt.Sprintf("%s/%s", constants.VerboseName, ManifestSemver)},
	})

	// Attach observer if metrics are enabled using SetRequestObserver
	// Get APICallsTotal metric directly from registry to avoid timing issues with collector initialization
	csmlog.WithContext(clientCtx).Debugf("Observer attachment check: metricsRegistry=%v, cluster=%s",
		s.metricsRegistry != nil, isiConfig.ClusterName)
	if s.metricsRegistry != nil {
		// Get or create the APICallsTotal metric directly
		apiCallsMetric := collectors.RegisterOrGetCounterVec(s.metricsRegistry, prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "dell_powerscale_api_calls_total",
			Help: "Total OneFS REST API calls by status (success/failure).",
		}, []string{"cluster_name", "status"}))

		observer := &PowerScaleAPIObserver{
			clusterName: isiConfig.ClusterName,
			apiRequests: apiCallsMetric,
		}
		if client.API != nil {
			client.API.SetRequestObserver(observer)
			csmlog.WithContext(clientCtx).Infof("Metrics enabled for cluster %s - PowerScale API observer activated", isiConfig.ClusterName)
		} else {
			csmlog.WithContext(clientCtx).Errorf("client.API is nil for cluster %s; metrics observer not attached", isiConfig.ClusterName)
		}
	} else {
		csmlog.WithContext(clientCtx).Debugf("Metrics disabled; observer not attached for cluster %s", isiConfig.ClusterName)
	}

	return client, nil
}

func (s *service) GetIsiService(clientCtx context.Context, isiConfig *IsilonClusterConfig, _ csmlog.Level) (*isiService, error) {
	var isiClient *isi.Client
	var err error
	if isiClient, err = s.GetIsiClient(clientCtx, isiConfig); err != nil {
		return nil, err
	}

	// Create isiService - metrics are now captured via observer in gopowerscale library
	isiSvc := &isiService{
		endpoint: isiConfig.Endpoint,
		client:   isiClient,
	}

	csmlog.WithContext(clientCtx).Infof("Creating isiService for cluster %s (metrics handled by gopowerscale observer)", isiConfig.ClusterName)
	return isiSvc, nil
}

func (s *service) validateOptsParameters(clusterConfig *IsilonClusterConfig) error {
	if clusterConfig.User == "" || clusterConfig.Password == "" || clusterConfig.Endpoint == "" {
		return fmt.Errorf("invalid isi service parameters, at least one of endpoint, username and password is empty. endpoint : endpoint '%s', username : '%s'", clusterConfig.Endpoint, clusterConfig.User)
	}

	return nil
}

func (s *service) logServiceStats() {
	fields := map[string]interface{}{
		"path":                      s.opts.Path,
		"skipCertificateValidation": s.opts.SkipCertificateValidation,
		"autoprobe":                 s.opts.AutoProbe,
		"accesspoint":               s.opts.AccessZone,
		"quotaenabled":              s.opts.QuotaEnabled,
		"mode":                      s.mode,
	}
	csmlog.WithFields(fields).Infof("Configured '%s'", constants.PluginName)
}

func (s *service) BeforeServe(
	ctx context.Context, _ *gocsi.StoragePlugin, _ net.Listener,
) error {
	if err := s.initializeServiceOpts(ctx); err != nil {
		return err
	}

	s.logServiceStats()

	// Create the metrics registry early so that cluster clients created during
	// the initial syncIsilonConfigs call below are wrapped with auth/permission
	// metrics. The HTTP server is started later, after cluster configs are loaded.
	if s.opts.MetricsEnabled && s.metricsRegistry == nil {
		s.metricsRegistry = prometheus.NewRegistry()
		csmlog.WithContext(ctx).Infof("Created metrics registry for mode=%s", s.mode)
	}

	// Update the storage array list
	s.isiClusters = new(sync.Map)
	s.setNoProbeOnStart(ctx)

	// Update config params
	vc := viper.New()
	vc.AutomaticEnv()
	vc.SetConfigFile(DriverConfigParamsFile)
	if err := vc.ReadInConfig(); err != nil {
		csmlog.WithContext(ctx).Warnf("unable to read driver config params from file '%s'. Using defaults.", DriverConfigParamsFile)
	}
	if err := s.updateDriverConfigParams(ctx, vc); err != nil {
		return err
	}

	// Watch for changes to driver config params file
	vc.WatchConfig()
	vc.OnConfigChange(func(_ fsnotify.Event) {
		csmlog.WithContext(ctx).Infof("Driver config params file changed")
		if err := s.updateDriverConfigParams(ctx, vc); err != nil {
			csmlog.WithContext(ctx).Warn(err.Error())
		}
	})

	// Initialize metrics collectors BEFORE loading configs (which creates isiService)
	// This ensures driverHealthCollector is available when PowerScale client is wrapped
	if s.opts.MetricsEnabled {
		csmlog.WithContext(ctx).Infof("Initializing metrics collectors...")
		s.startMetricsCollectors(ctx)
		csmlog.WithContext(ctx).Infof("Metrics collectors initialized, driverHealthCollector=%v", s.driverHealthCollector != nil)

		// Set auth failure recording function for mount operations
		if s.accessControlCollector != nil {
			recordAuthFailureFunc = s.RecordAuthFailure
			recordPermissionDenialFunc = s.RecordPermissionDenial
			metricsEnabled = true
			csmlog.WithContext(ctx).Infof("Auth failure and permission denial recording enabled")
		}
	}

	// Load config in goroutine (dynamic loading behavior)
	// The recorder will be set AFTER the isiClusters map is replaced in loadIsilonConfigs
	go s.loadIsilonConfigs(ctx, isilonConfigFile)

	go s.startAPIService(ctx)

	if s.opts.MetricsEnabled {
		if err := s.startMetricsServer(ctx); err != nil {
			csmlog.WithContext(ctx).Warnf("metrics server startup skipped: %v", err)
		}
	}

	// Watch for changes to access zone network node labels
	if strings.EqualFold(s.mode, constants.ModeNode) {
		s.reconcile = &reconciler{
			service: s,
		}
		s.updateAZReconcileIntervalCh = make(chan time.Duration)
		go s.reconcile.reconcileNodeAzLabels(ctx)
	}

	// Watch for Node deletions and clean up stale IPs from shared exports
	if strings.EqualFold(s.mode, constants.ModeController) {
		go s.watchNodeDeletions(ctx)
	}

	// Initialize event recorder for controller mode to emit Kubernetes events
	// for provisioning failures (e.g., SharedExportNotFound)
	if strings.EqualFold(s.mode, constants.ModeController) {
		s.initEventRecorder(ctx)
		// When the service context is cancelled (driver shutdown), this goroutine
		// calls shutdownEventRecorder to stop the event broadcaster and its
		// background goroutines cleanly.
		if s.eventBroadcaster != nil {
			go func() {
				<-ctx.Done()
				s.shutdownEventRecorder()
			}()
		}
	}

	if err := s.probeOnStart(ctx); err != nil {
		return err
	}

	return nil
}

// RegisterAdditionalServers registers any additional grpc services that use the CSI socket.
func (s *service) RegisterAdditionalServers(server *grpc.Server) {
	csmlog.Info("Registering additional GRPC servers")
	csiext.RegisterReplicationServer(server, s)
	podmon.RegisterPodmonServer(server, s)
}

func (s *service) loadIsilonConfigs(ctx context.Context, configFile string) error {
	csmlog.WithContext(ctx).Info("Updating cluster config details")
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	parentFolder, _ := filepath.Abs(filepath.Dir(configFile))
	csmlog.WithContext(ctx).Debugf("Config folder: %v", parentFolder)
	err = watcher.Add(parentFolder)
	if err != nil {
		csmlog.WithContext(ctx).Errorf("Unable to add file watcher for folder %v", parentFolder)
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if event.Has(fsnotify.Create) && event.Name == parentFolder+"/..data" {
				csmlog.WithContext(ctx).Infof("**************** Cluster config file modified. Updating cluster config details: %s****************", event.Name)
				// set noProbeOnStart to false so subsequent calls can lead to probe
				noProbeOnStart.Store(false)
				err := s.syncIsilonConfigs(ctx)
				if err != nil {
					csmlog.WithContext(ctx).Debugf("Cluster configuration array length: %v", s.getIsilonClusterLength())
					csmlog.WithContext(ctx).Errorf("Invalid configuration in secret.yaml. Error: %v", err)
				} else {
					s.reconcileMetricsCollectors(ctx)
				}
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			csmlog.WithContext(ctx).Errorf("cluster config file load error: %v", err)
		}
	}
}

// getReconcileInterval returns the access zone reconcile interval
func (s *service) getReconcileInterval() time.Duration {
	syncMutex.Lock()
	defer syncMutex.Unlock()
	return s.azReconcileInterval
}

// getUpdateIntervalChannel returns the updated access zone reconcile interval
func (s *service) getUpdateIntervalChannel() <-chan time.Duration {
	return s.updateAZReconcileIntervalCh
}

// reconcileNodeAzLabels reconciles the node access zone labels
func (r *reconciler) reconcileNodeAzLabels(ctx context.Context) error {
	azReconcileInterval := r.service.getReconcileInterval()

	if azReconcileInterval == 0 {
		csmlog.WithContext(ctx).Info("Reconcile is invalid value of 0. Must be greater than 0 to enable label reconciler.")
		return nil
	}

	// On first iteration, reconcile labels then start the ticker
	if azReconcileInterval > 0 {
		err := r.service.ReconcileNodeAzLabels(ctx)
		if err != nil {
			csmlog.WithContext(ctx).Errorf("node label reconciliation failed: %v", err)
		}
	}

	go func() {
		ticker := time.NewTicker(r.service.getReconcileInterval())
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				err := r.service.ReconcileNodeAzLabels(ctx)
				if err != nil {
					csmlog.WithContext(ctx).Errorf("node label reconciliation failed: %v", err)
				}
			case newInterval := <-r.service.getUpdateIntervalChannel():
				ticker.Stop()
				ticker = time.NewTicker(r.service.getReconcileInterval())
				csmlog.WithContext(ctx).Infof("access zone reconcile interval changed to %s", newInterval)
			}
		}
	}()
	return nil
}

// Returns the size of arrays
func (s *service) getIsilonClusterLength() (length int) {
	length = 0
	s.isiClusters.Range(func(_, _ interface{}) bool {
		length++
		return true
	})
	return length
}

var syncMutex sync.Mutex

// Reads the credentials from secrets and initialize all arrays.
func (s *service) syncIsilonConfigs(ctx context.Context) error {
	csmlog.WithContext(ctx).Info("************* Synchronizing Isilon Clusters' config **************")
	syncMutex.Lock()
	defer syncMutex.Unlock()

	configBytes, err := os.ReadFile(filepath.Clean(isilonConfigFile))
	if err != nil {
		return fmt.Errorf("file ('%s') error: %v", isilonConfigFile, err)
	}

	if string(configBytes) != "" {
		csmlog.WithContext(ctx).Debugf("Current isilon configs:")
		s.isiClusters.Range(handler)
		newIsilonConfigs, defaultClusterName, err := s.getNewIsilonConfigs(ctx, configBytes)
		if err != nil {
			return err
		}

		// Update the isiClusters sync.Map
		s.isiClusters.Range(func(key interface{}, _ interface{}) bool {
			s.isiClusters.Delete(key)
			return true
		})

		for k, v := range newIsilonConfigs {
			s.isiClusters.Store(k, v)
		}
		csmlog.WithContext(ctx).Debugf("New isilon configs:")
		s.isiClusters.Range(handler)

		s.defaultIsiClusterName = defaultClusterName
		if s.defaultIsiClusterName == "" {
			csmlog.WithContext(ctx).Errorf("no default cluster name/config available")
		}

		// Set X_CSI_CLUSTER_NAME environment variable with the actual PowerScale cluster name
		// This ensures CSI operation metrics use the same cluster name as PowerScale metrics
		if defaultClusterName != "" {
			if err := os.Setenv("X_CSI_CLUSTER_NAME", defaultClusterName); err != nil {
				csmlog.WithContext(ctx).Warnf("Failed to set X_CSI_CLUSTER_NAME environment variable: %v", err)
			} else {
				csmlog.WithContext(ctx).Infof("Set X_CSI_CLUSTER_NAME to %s for CSI operation metrics", defaultClusterName)
			}
		}
	} else {
		return errors.New("isilon cluster details are not provided in isilon-creds secret")
	}
	return nil
}

func unmarshalYAMLContent(configBytes []byte) (*IsilonClusters, error) {
	yamlConfig := new(IsilonClusters)
	err := yaml.Unmarshal(configBytes, yamlConfig)
	return yamlConfig, err
}

func (s *service) getNewIsilonConfigs(ctx context.Context, configBytes []byte) (map[interface{}]interface{}, string, error) {
	var noOfDefaultClusters int
	var defaultIsiClusterName string

	var inputConfigs *IsilonClusters
	var yamlErr error
	var err error

	csmlog.WithContext(ctx).Info("reading secret file to validate cluster config details")
	inputConfigs, yamlErr = unmarshalYAMLContent(configBytes)
	if yamlErr != nil {
		csmlog.WithContext(ctx).Errorf("failed to parse isilon clusters' config details as yaml data, error: %v", yamlErr)
		return nil, defaultIsiClusterName, fmt.Errorf("failed to parse isilon clusters' config details as yaml data")
	}

	if len(inputConfigs.IsilonClusters) == 0 {
		return nil, defaultIsiClusterName, errors.New("cluster details are not provided in isilon-creds secret")
	}

	if len(inputConfigs.IsilonClusters) > 1 && s.opts.CustomTopologyEnabled {
		return nil, defaultIsiClusterName, errors.New("custom topology is enabled and it expects single cluster config in secret")
	}

	newIsiClusters := make(map[interface{}]interface{})
	for i, clusterConfig := range inputConfigs.IsilonClusters {
		config := clusterConfig
		csmlog.WithContext(ctx).Debugf("parsing config details for cluster %v", config.ClusterName)
		if config.ClusterName == "" {
			return nil, defaultIsiClusterName, fmt.Errorf("clusterName not provided in secret at index [%d]", i)
		}
		if config.User == "" {
			return nil, defaultIsiClusterName, fmt.Errorf("username not provided for cluster %s at index [%d]", config.ClusterName, i)
		}
		if config.Password == "" {
			return nil, defaultIsiClusterName, fmt.Errorf("password not provided for cluster %s  at index [%d]", config.ClusterName, i)
		}
		if config.Endpoint == "" {
			return nil, defaultIsiClusterName, fmt.Errorf("endpoint not provided for cluster %s at index [%d]", config.ClusterName, i)
		}

		// Let Endpoint be generic.
		// Take out https prefix from it, if present, and let it's consumers to use it the way they want
		config.Endpoint = strings.TrimPrefix(config.Endpoint, "https://")

		if config.EndpointPort == "" {
			csmlog.WithContext(ctx).Warnf("using default as EndpointPort not provided for cluster %s in secret at index [%d]", config.ClusterName, i)
			config.EndpointPort = s.opts.Port
		}

		if config.SkipCertificateValidation == nil {
			config.SkipCertificateValidation = &s.opts.SkipCertificateValidation
		}

		if config.IsiPath == "" {
			csmlog.WithContext(ctx).Warnf("using default as IsiPath not provided for cluster %s in secret at index [%d]", config.ClusterName, i)
			config.IsiPath = s.opts.Path
		}

		if config.IsiVolumePathPermissions == "" {
			csmlog.WithContext(ctx).Warnf("using default as IsiVolumePathPermissions not provided for cluster %s in secret at index [%d]", config.ClusterName, i)
			config.IsiVolumePathPermissions = s.opts.IsiVolumePathPermissions
		}

		if config.IgnoreUnresolvableHosts == nil {
			config.IgnoreUnresolvableHosts = &s.opts.IgnoreUnresolvableHosts
		}

		config.EndpointURL = fmt.Sprintf("https://%s:%s", config.Endpoint, config.EndpointPort)
		// clientCtx, _ := GetLogger(ctx)
		// Need to verify this part
		if !noProbeOnStart.Load() {
			config.isiSvc, err = s.GetIsiService(ctx, &config, csmlog.GetLevel())
			if err != nil {
				csmlog.WithContext(ctx).Errorf("failed to get isi client for  cluster %s, error: %v", config.ClusterName, err)
			}
		}

		if config.IsDefault == nil {
			defaultBoolValue := false
			config.IsDefault = &defaultBoolValue
		}

		if *config.IsDefault == true {
			noOfDefaultClusters++
			if noOfDefaultClusters > 1 {
				return nil, defaultIsiClusterName, fmt.Errorf("'IsDefault' attribute set for multiple isilon cluster configs in 'isilonClusters': %s. Only one cluster should be marked as default cluster", config.ClusterName)
			}
		}

		if _, ok := newIsiClusters[config.ClusterName]; ok {
			return nil, defaultIsiClusterName, fmt.Errorf("duplicate cluster name [%s] found in input isilonClusters", config.ClusterName)
		}

		newConfig := IsilonClusterConfig{}
		newConfig = config

		newIsiClusters[config.ClusterName] = &newConfig
		if *config.IsDefault {
			defaultIsiClusterName = config.ClusterName
		}

		fields := map[string]interface{}{
			"ClusterName":               config.ClusterName,
			"Endpoint":                  config.Endpoint,
			"EndpointPort":              config.EndpointPort,
			"Username":                  config.User,
			"Password":                  "*******",
			"SkipCertificateValidation": *config.SkipCertificateValidation,
			"IsiPath":                   config.IsiPath,
			"IsiVolumePathPermissions":  config.IsiVolumePathPermissions,
			"IsDefault":                 *config.IsDefault,
			"IgnoreUnresolvableHosts":   *config.IgnoreUnresolvableHosts,
		}
		// TODO: Replace logrus with log
		csmlog.WithFields(fields).Infof("new config details set for cluster %s", config.ClusterName)
	}

	return newIsiClusters, defaultIsiClusterName, nil
}

func handler(_, value interface{}) bool {
	csmlog.Debug(value.(*IsilonClusterConfig).String())
	return true
}

// Returns details of a cluster with name clusterName
func (s *service) getIsilonClusterConfig(clusterName string) *IsilonClusterConfig {
	if cluster, ok := s.isiClusters.Load(clusterName); ok {
		return cluster.(*IsilonClusterConfig)
	}
	return nil
}

// Returns details of all isilon clusters
func (s *service) getIsilonClusters() []*IsilonClusterConfig {
	list := make([]*IsilonClusterConfig, 0)
	s.isiClusters.Range(func(_ interface{}, value interface{}) bool {
		list = append(list, value.(*IsilonClusterConfig))
		return true
	})
	return list
}

// Update configurable params from configmap
func (s *service) updateDriverConfigParams(ctx context.Context, v *viper.Viper) error {
	logLevel := constants.DefaultLogLevel
	if v.IsSet(constants.ParamCSILogLevel) {
		inputLogLevel := v.GetString(constants.ParamCSILogLevel)
		if inputLogLevel != "" {
			inputLogLevel = strings.ToLower(inputLogLevel)
			var err error
			logLevel, err = csmlog.ParseLevel(inputLogLevel)
			if err != nil {
				return fmt.Errorf("input log level %q is not valid", inputLogLevel)
			}
		}
	}
	csmlog.SetLevel(logLevel)
	csmlog.WithContext(ctx).Infof("log level set to '%s'", logLevel)

	// set access zone network label interval
	s.setAzReconcileInterval(ctx, v)

	err := s.syncIsilonConfigs(ctx)
	if err != nil {
		return err
	}
	if s.opts.MetricsEnabled {
		s.reconcileMetricsCollectors(ctx)
	}
	return nil
}

func (s *service) setAzReconcileInterval(ctx context.Context, v *viper.Viper) {
	var azReconcileIntervalStr string
	if v.IsSet(constants.ParamAZReconcileInterval) {
		azReconcileIntervalStr = v.GetString(constants.ParamAZReconcileInterval)
	}

	if strings.TrimSpace(azReconcileIntervalStr) == "" || azReconcileIntervalStr == "0" {
		csmlog.WithContext(ctx).Info("disabling access zone reconcile feature")
		s.azReconcileInterval = 0
		return
	}

	interval, err := time.ParseDuration(azReconcileIntervalStr)
	if err != nil {
		csmlog.WithContext(ctx).Errorf("parsing access zone reconcile interval %s, defaulting to %s: %v", azReconcileIntervalStr, constants.DefaultAZReconcileInterval, err)
		interval = constants.DefaultAZReconcileInterval
	}
	csmlog.WithContext(ctx).Infof("access zone reconcile interval set to %s", interval)
	s.azReconcileInterval = interval
	if s.updateAZReconcileIntervalCh != nil {
		s.updateAZReconcileIntervalCh <- interval
	}
}

// GetCSINodeID gets the id of the CSI node which regards the node name as node id
func (s *service) GetCSINodeID() (string, error) {
	// if the node id has already been initialized, return it
	if s.nodeID != "" {
		return s.nodeID, nil
	}
	// node id couldn't be read from env variable while initializing service, return with error
	return "", errors.New("cannot get node id")
}

// GetCSINodeIP gets the IP of the CSI node
func (s *service) GetCSINodeIP() (string, error) {
	// if the node ip has already been initialized, return it
	if s.nodeIP != "" {
		return s.nodeIP, nil
	}
	// node id couldn't be read from env variable while initializing service, return with error
	return "", errors.New("cannot get node IP")
}

func (s *service) getVolByName(ctx context.Context, isiPath, volName string, isiConfig *IsilonClusterConfig) (isi.Volume, error) {
	return getVolByNameFunc(s, ctx, isiPath, volName, isiConfig)
}

var getVolByNameFunc = func(_ *service, ctx context.Context, isiPath, volName string, isiConfig *IsilonClusterConfig) (isi.Volume, error) {
	// The `GetVolume` API returns a slice of volumes, but when only passing
	// in a volume ID, the response will be just the one volume
	vol, err := isiConfig.isiSvc.GetVolumeWithIsiPath(ctx, isiPath, "", volName)
	if err != nil {
		return nil, err
	}
	return vol, nil
}

// Provide periodic logging of statistics like goroutines and memory
func (s *service) logStatistics() {
	if s.statisticsCounter = s.statisticsCounter + 1; (s.statisticsCounter % 100) == 0 {
		goroutines := runtime.NumGoroutine()
		memstats := new(runtime.MemStats)
		runtime.ReadMemStats(memstats)
		fields := map[string]interface{}{
			"GoRoutines":   goroutines,
			"HeapAlloc":    memstats.HeapAlloc,
			"HeapReleased": memstats.HeapReleased,
			"StackSys":     memstats.StackSys,
		}
		// TODO: Replace logrus with log
		csmlog.WithFields(fields).Debugf("resource statistics counter: %d", s.statisticsCounter)
	}
}

func (s *service) getIsiPathForVolumeFromClusterConfig(clusterConfig *IsilonClusterConfig) string {
	if clusterConfig.IsiPath == "" {
		return s.opts.Path
	}
	return clusterConfig.IsiPath
}

// GetMessageWithReqID returns message with reqID information
func GetMessageWithReqID(ReqID string, format string, args ...interface{}) string {
	str := fmt.Sprintf(format, args...)
	return fmt.Sprintf(" ReqID=%s %s", ReqID, str)
}

// LogMap logs the key-value entries of a given map
func LogMap(ctx context.Context, mapName string, m map[string]string) {
	csmlog.WithContext(ctx).Debugf("map '%s':", mapName)
	for key, value := range m {
		csmlog.WithContext(ctx).Debugf("    [%s]='%s'", key, value)
	}
}

// getIsilonConfig returns the cluster config
func (s *service) getIsilonConfig(ctx context.Context, clusterName *string) (*IsilonClusterConfig, error) {
	if *clusterName == "" {
		csmlog.WithContext(ctx).Infof("Request doesn't include cluster name. Use default cluster '%s'", s.defaultIsiClusterName)
		*clusterName = s.defaultIsiClusterName
		if s.defaultIsiClusterName == "" {
			return nil, fmt.Errorf("no default cluster config available to continue with request")
		}
	}

	isiConfig := s.getIsilonClusterConfig(*clusterName)
	if isiConfig == nil {
		return nil, fmt.Errorf("failed to get cluster config details for clusterName: '%s'", *clusterName)
	}

	return isiConfig, nil
}

func (s *service) GetNodeLabels() (map[string]string, error) {
	k8sclientset, err := k8sutils.CreateKubeClientSet(s.opts.KubeConfigPath)
	if err != nil {
		csmlog.Errorf("init client failed: '%s'", err.Error())
		return nil, err
	}
	// access the API to fetch node object
	node, err := k8sclientset.CoreV1().Nodes().Get(context.TODO(), s.nodeID, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	csmlog.Debugf("Node details: %s", node)

	return node.Labels, nil
}

var (
	getKubeClientSet = func(kubeConfigPath string) (*kubernetes.Clientset, error) {
		return k8sutils.CreateKubeClientSet(kubeConfigPath)
	}
	getK8sNodeByName = func(k8sclientset *kubernetes.Clientset, nodeName string) (*corev1.Node, error) {
		return k8sclientset.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
	}
)

func (s *service) GetNodeLabelsWithName(nodeName string) (map[string]string, error) {
	k8sclientset, err := getKubeClientSet(s.opts.KubeConfigPath)
	if err != nil {
		csmlog.Errorf("init client failed: '%s'", err.Error())
		return nil, err
	}
	// access the API to fetch node object
	node, err := getK8sNodeByName(k8sclientset, nodeName)
	if err != nil {
		return nil, err
	}
	csmlog.Debugf("Node details: %s", node)

	return node.Labels, nil
}

func (s *service) PatchNodeLabels(add map[string]string, remove []string) error {
	if s.k8sclient == nil {
		return errors.New("k8s client is not initialized")
	}
	node, err := s.k8sclient.CoreV1().Nodes().Get(context.TODO(), s.nodeID, metav1.GetOptions{})
	if err != nil {
		csmlog.Errorf("failed to get current node details: '%s'", err.Error())
		return err
	}

	currentNode, err := json.Marshal(node)
	if err != nil {
		csmlog.Errorf("failed to marshal current node details: '%s'", err.Error())
		return err
	}

	for k, v := range add {
		node.Labels[k] = v
	}

	for _, k := range remove {
		delete(node.Labels, k)
	}

	newNode, err := json.Marshal(node)
	if err != nil {
		csmlog.Errorf("failed to marshal new node details: '%s'", err.Error())
		return err
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(currentNode, newNode, node)
	if err != nil {
		csmlog.Errorf("failed to create patch: '%s'", err.Error())
		return err
	}

	node, err = s.k8sclient.CoreV1().Nodes().Patch(context.TODO(), s.nodeID, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		csmlog.Errorf("failed to patch node labels: '%s'", err.Error())
		return err
	}

	csmlog.Debugf("Node details after patching labels: %s", node)
	return err
}

func (s *service) ProbeController(ctx context.Context,
	_ *commonext.ProbeControllerRequest) (
	*commonext.ProbeControllerResponse, error,
) {
	if !strings.EqualFold(s.mode, "node") {
		csmlog.WithContext(ctx).Debugf("controllerProbe")
		if err := s.probeAllClusters(ctx); err != nil {
			csmlog.WithContext(ctx).Errorf("error in controllerProbe: %s", err.Error())
			return nil, err
		}
	}

	ready := new(wrapperspb.BoolValue)
	ready.Value = true
	rep := new(commonext.ProbeControllerResponse)
	rep.Ready = ready
	rep.Name = constants.PluginName
	rep.VendorVersion = Manifest["semver"]
	rep.Manifest = Manifest

	csmlog.WithContext(ctx).Debug(fmt.Sprintf("ProbeController returning: %v", rep.Ready.GetValue()))

	return rep, nil
}

// WithRP appends Replication Prefix to provided string
func (s *service) WithRP(key string) string {
	return s.opts.replicationPrefix + "/" + key
}

func (s *service) validateIsiPath(ctx context.Context, volName string) (string, error) {
	if s.k8sclient == nil {
		return "", errors.New("no k8s clientset")
	}

	pv, err := s.k8sclient.CoreV1().PersistentVolumes().Get(ctx, volName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("unable to get PersistentVolume: %w", err)
	}

	// check pv for IsiPath
	// will be in VolumeAttributes like:
	// Path: IsiPath/volumeName
	if pv.Spec.CSI != nil && pv.Spec.CSI.VolumeAttributes != nil {
		if pv.Spec.CSI.VolumeAttributes[ExportPathParam] != "" {
			exportPath := pv.Spec.CSI.VolumeAttributes[ExportPathParam]
			isiPath := isilonfs.GetIsiPathFromExportPath(exportPath)
			csmlog.WithContext(ctx).Debugf("Found IsiPath from PersistentVolume: %v", isiPath)
			return isiPath, nil
		}
	}

	csmlog.WithContext(ctx).Debug("IsiPath not found in PersistentVolume")

	// if we cannot find IsiPath in VolumeAttributes, check StorageClass next
	if pv.Spec.StorageClassName == "" {
		csmlog.WithContext(ctx).Debug("StorageClass not found in PersistentVolume")
		return "", nil
	}

	csmlog.WithContext(ctx).Debugf("Checking StorageClass: %v", pv.Spec.StorageClassName)

	sc, err := s.k8sclient.StorageV1().StorageClasses().Get(ctx, pv.Spec.StorageClassName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("unable to get StorageClass: %w", err)
	}

	isiPath, ok := sc.Parameters[IsiPathParam]
	if !ok || isiPath == "" {
		csmlog.WithContext(ctx).Debug("IsiPath not found in StorageClass")
		return "", nil
	}

	return isiPath, nil
}

func getExportPathFromExportID(ctx context.Context, isiConfig *IsilonClusterConfig, exportID int, accessZone string) (string, error) {
	export, err := isiConfig.isiSvc.GetExportByIDWithZone(ctx, exportID, accessZone)
	if err != nil {
		csmlog.WithContext(ctx).Error("Failed to get export with error: " + err.Error())
		return "", status.Error(codes.NotFound, err.Error())
	}
	if len(*export.Paths) == 0 {
		return "", status.Error(codes.NotFound, fmt.Sprintf("can't find paths for export with ID %d", exportID))
	}
	csmlog.WithContext(ctx).Debugf("Export paths are: %v", export.Paths)
	exportPath := (*export.Paths)[0]
	csmlog.WithContext(ctx).Debugf("Returning export path: %s", exportPath)

	return exportPath, nil
}

// RecordAPICall records an API call result for metrics tracking
func (s *service) RecordAPICall(success bool, httpStatusCode int) {
	if s.driverHealthCollector != nil {
		s.driverHealthCollector.RecordAPICall(success, httpStatusCode)
	}
}

// RecordAuthFailure records an NFS mount authentication failure
func (s *service) RecordAuthFailure() {
	if s.accessControlCollector != nil {
		s.accessControlCollector.RecordAuthFailure()
	}
}

// RecordPermissionDenial records a file access permission denial
func (s *service) RecordPermissionDenial() {
	if s.accessControlCollector != nil {
		s.accessControlCollector.RecordPermissionDenial()
	}
}

// watchNodeDeletions watches for Kubernetes Node deletion events and cleans up
// stale node IPs from shared export client lists. It automatically reconnects
// with exponential backoff when the watch expires or encounters errors.
func (s *service) watchNodeDeletions(ctx context.Context) {
	csmlog.WithContext(ctx).Info("Starting Node deletion watcher for stale IP cleanup")

	// Exponential backoff parameters
	const (
		initialBackoff = 1 * time.Second
		maxBackoff     = 60 * time.Second
		backoffFactor  = 2
	)
	currentBackoff := initialBackoff

	// Reconnection loop - keeps watching until context is cancelled
	for {
		// Check if context is cancelled before starting a new watch
		if ctx.Err() != nil {
			csmlog.WithContext(ctx).Info("Node deletion watcher stopping: context cancelled")
			return
		}

		// Create a watcher for Node deletions
		watcher, err := s.k8sclient.CoreV1().Nodes().Watch(ctx, metav1.ListOptions{})
		if err != nil {
			csmlog.WithContext(ctx).Errorf("Failed to create Node watcher: %v, retrying in %v", err, currentBackoff)
			select {
			case <-ctx.Done():
				csmlog.WithContext(ctx).Info("Node deletion watcher stopping: context cancelled during backoff")
				return
			case <-time.After(currentBackoff):
				// Increase backoff for next retry (exponential backoff)
				currentBackoff = time.Duration(float64(currentBackoff) * backoffFactor)
				if currentBackoff > maxBackoff {
					currentBackoff = maxBackoff
				}
				continue
			}
		}

		// Reset backoff on successful connection
		currentBackoff = initialBackoff
		csmlog.WithContext(ctx).Debug("Node watcher connected successfully")

		// Process watch events
		for event := range watcher.ResultChan() {
			if event.Type == watch.Error {
				// An error event means the watch stream is invalidated (e.g. resource version too old).
				// Break out to trigger a reconnect via the outer loop.
				csmlog.WithContext(ctx).Warn("Node watcher received error event, reconnecting")
				break
			}
			if event.Type == watch.Deleted {
				node, ok := event.Object.(*corev1.Node)
				if !ok {
					csmlog.WithContext(ctx).Warn("Failed to cast deleted object to Node")
					continue
				}

				csmlog.WithContext(ctx).WithFields(csmlog.Fields{
					"nodeName": node.Name,
				}).Info("Node deleted, cleaning up stale IPs from shared exports")

				// Clean up node IPs from all shared exports
				if err := s.cleanupNodeFromSharedExports(ctx, node); err != nil {
					csmlog.WithContext(ctx).Errorf("Failed to cleanup node %s from shared exports: %v", node.Name, err)
				}
			}
		}

		// Watch channel closed - this happens when watch expires (HTTP 410 Gone), connection lost, or error event received
		watcher.Stop()
		csmlog.WithContext(ctx).Warn("Node deletion watcher connection lost, reconnecting...")
	}
}

// cleanupNodeFromSharedExports removes a deleted node's IP from all shared export client lists
func (s *service) cleanupNodeFromSharedExports(ctx context.Context, node *corev1.Node) error {
	// Get node IP addresses
	var nodeIPs []string
	for _, addr := range node.Status.Addresses {
		if addr.Type == corev1.NodeInternalIP || addr.Type == corev1.NodeExternalIP {
			nodeIPs = append(nodeIPs, addr.Address)
		}
	}

	if len(nodeIPs) == 0 {
		csmlog.WithContext(ctx).Warnf("No IP addresses found for deleted node %s", node.Name)
		return nil
	}

	// List all PVs and filter by VolumeAttributes["ProvisioningMode"] in the loop below.
	// VolumeAttributes are immutable (set at CreateVolume time), so this is always correct.
	// A label selector cannot be used here because labelDirectoryBackedPV is best-effort
	// (fire-and-forget goroutine); a missed label would silently skip cleanup → stale IPs.
	pvList, err := s.k8sclient.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list PVs: %w", err)
	}

	// Track unique shared exports (exportID:accessZone)
	sharedExports := make(map[string]struct {
		exportID    int
		accessZone  string
		clusterName string
	})

	for _, pv := range pvList.Items {
		// Only process CSI PowerScale directory-backed volumes
		if pv.Spec.CSI == nil || pv.Spec.CSI.Driver != constants.PluginName {
			continue
		}
		if pv.Spec.CSI.VolumeAttributes["ProvisioningMode"] != "directory" {
			continue
		}

		// Parse export info
		_, exportID, accessZone, clusterName, parseErr := id.ParseNormalizedVolumeID(ctx, pv.Spec.CSI.VolumeHandle)
		if parseErr != nil {
			csmlog.WithContext(ctx).Debugf("Failed to parse volume ID %s: %v", pv.Spec.CSI.VolumeHandle, parseErr)
			continue
		}

		exportKey := fmt.Sprintf("%d:%s", exportID, accessZone)
		sharedExports[exportKey] = struct {
			exportID    int
			accessZone  string
			clusterName string
		}{exportID, accessZone, clusterName}
	}

	// Remove node IPs from each shared export
	cleanupCount := 0
	for exportKey, export := range sharedExports {
		// Acquire per-export mutex
		muVal, _ := s.directoryExportMu.LoadOrStore(exportKey, &sync.RWMutex{})
		mu := muVal.(*sync.RWMutex)
		mu.Lock()

		// Get cluster config
		isiConfig, err := s.getIsilonConfig(ctx, &export.clusterName)
		if err != nil {
			csmlog.WithContext(ctx).Warnf("Failed to get config for cluster %s: %v", export.clusterName, err)
			mu.Unlock()
			continue
		}

		// Remove node IPs from export client list
		removeErr := isiConfig.isiSvc.RemoveExportClientByIPsWithZone(ctx, export.exportID, export.accessZone, nodeIPs, false)
		if removeErr != nil {
			csmlog.WithContext(ctx).Warnf("Failed to remove IPs %v from export %d (zone: %s): %v", nodeIPs, export.exportID, export.accessZone, removeErr)
		} else {
			csmlog.WithContext(ctx).WithFields(csmlog.Fields{
				"nodeIPs":    nodeIPs,
				"exportID":   export.exportID,
				"accessZone": export.accessZone,
				"cluster":    export.clusterName,
			}).Info("Removed stale node IPs from shared export")
			cleanupCount += len(nodeIPs)
		}

		mu.Unlock()
	}

	csmlog.WithContext(ctx).WithFields(csmlog.Fields{
		"nodeName":     node.Name,
		"nodeIPs":      nodeIPs,
		"cleanupCount": cleanupCount,
		"exportCount":  len(sharedExports),
	}).Info("Completed stale IP cleanup for deleted node")

	return nil
}
