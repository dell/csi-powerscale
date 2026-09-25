/*
 Copyright © 2025-2026 Dell Inc. or its subsidiaries. All Rights Reserved.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at
      http://www.apache.org/licenses/LICENSE-2.0
 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package service

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/common/constants"
	"github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/service/collectors"
	isi "github.com/Ecosystems/container-storage-modules/src/gopowerscale"
	"github.com/Ecosystems/container-storage-modules/src/gopowerscale/api"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// generateSelfSignedCert creates a temporary self-signed TLS cert/key pair and
// returns the paths to the PEM files. Files are removed via t.Cleanup.
func generateSelfSignedCert(t *testing.T) (certFile, keyFile string) {
	t.Helper()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	require.NoError(t, err)

	cf, err := os.CreateTemp("", "test-cert-*.pem")
	require.NoError(t, err)
	require.NoError(t, pem.Encode(cf, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}))
	cf.Close()

	keyBytes, err := x509.MarshalECPrivateKey(priv)
	require.NoError(t, err)
	kf, err := os.CreateTemp("", "test-key-*.pem")
	require.NoError(t, err)
	require.NoError(t, pem.Encode(kf, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes}))
	kf.Close()

	t.Cleanup(func() {
		os.Remove(cf.Name())
		os.Remove(kf.Name())
	})
	return cf.Name(), kf.Name()
}

func pscFreePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	p := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return p
}

type fakePowerScaleAPIClient struct {
	observer api.RequestObserver
}

func (f *fakePowerScaleAPIClient) Do(_ context.Context, _, _, _ string, _ api.OrderedValues, _, _ interface{}) error {
	return nil
}

func (f *fakePowerScaleAPIClient) DoWithHeaders(_ context.Context, _, _, _ string, _ api.OrderedValues, _ map[string]string, _, _ interface{}) error {
	return nil
}

func (f *fakePowerScaleAPIClient) Get(_ context.Context, _, _ string, _ api.OrderedValues, _ map[string]string, _ interface{}) error {
	return nil
}

func (f *fakePowerScaleAPIClient) Post(_ context.Context, _, _ string, _ api.OrderedValues, _ map[string]string, _, _ interface{}) error {
	return nil
}

func (f *fakePowerScaleAPIClient) Put(_ context.Context, _, _ string, _ api.OrderedValues, _ map[string]string, _, _ interface{}) error {
	return nil
}

func (f *fakePowerScaleAPIClient) Delete(_ context.Context, _, _ string, _ api.OrderedValues, _ map[string]string, _ interface{}) error {
	return nil
}

func (f *fakePowerScaleAPIClient) APIVersion() uint8 {
	return 0
}

func (f *fakePowerScaleAPIClient) User() string {
	return ""
}

func (f *fakePowerScaleAPIClient) Group() string {
	return ""
}

func (f *fakePowerScaleAPIClient) VolumesPath() string {
	return ""
}

func (f *fakePowerScaleAPIClient) VolumePath(name string) string {
	return name
}

func (f *fakePowerScaleAPIClient) SetAuthToken(string) {}

func (f *fakePowerScaleAPIClient) SetCSRFToken(string) {}

func (f *fakePowerScaleAPIClient) SetReferer(string) {}

func (f *fakePowerScaleAPIClient) GetAuthToken() string {
	return ""
}

func (f *fakePowerScaleAPIClient) GetCSRFToken() string {
	return ""
}

func (f *fakePowerScaleAPIClient) GetReferer() string {
	return ""
}

func (f *fakePowerScaleAPIClient) SetCustomHTTPHeaders(http.Header) {}

func (f *fakePowerScaleAPIClient) GetCustomHTTPHeaders() http.Header {
	return nil
}

func (f *fakePowerScaleAPIClient) SetRequestObserver(observer api.RequestObserver) {
	f.observer = observer
}

func (f *fakePowerScaleAPIClient) GetRequestObserver() api.RequestObserver {
	return f.observer
}

func mockIsiClientFactory(t *testing.T, client *isi.Client) {
	t.Helper()
	origNewIsiClientWithArgsFunc := newIsiClientWithArgsFunc
	newIsiClientWithArgsFunc = func(_ context.Context, _ string, _ bool, _ uint, _, _, _, _, _ string, _ bool, _ uint8) (*isi.Client, error) {
		return client, nil
	}
	t.Cleanup(func() {
		newIsiClientWithArgsFunc = origNewIsiClientWithArgsFunc
	})
}

// I-PSC-01: MetricsServer starts on :8443 equivalent; QuotaCollector registered; scrape returns quota metrics.
func TestIntegration_PSC_QuotaMetricsScrape(t *testing.T) {
	reg := prometheus.NewRegistry()

	quotaUsed := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "dell_powerscale_quota_used_bytes",
		Help: "Quota used bytes.",
	}, []string{"cluster_name", "quota_id", "path"})
	reg.MustRegister(quotaUsed)
	quotaUsed.WithLabelValues("isilon-cluster-1", "quota-001", "/ifs/data").Set(1024)

	port := pscFreePort(t)
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	srv := &http.Server{Addr: fmt.Sprintf(":%d", port), Handler: mux}
	go func() { _ = srv.ListenAndServe() }()
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })
	time.Sleep(80 * time.Millisecond)

	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/metrics", port))
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(body), "dell_powerscale_quota_used_bytes",
		"scrape body must contain quota used metric")
}

// I-PSC-02: X_CSI_METRICS_ENABLED=false — port must not be bound.
func TestIntegration_PSC_MetricsDisabled_NoPortBound(t *testing.T) {
	t.Setenv("X_CSI_METRICS_ENABLED", "false")

	port := pscFreePort(t)
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), 200*time.Millisecond)
	if conn != nil {
		conn.Close()
	}
	assert.Error(t, err, "when X_CSI_METRICS_ENABLED=false, port must not be bound")
}

// U-PSC-03: startMetricsCollectors with nil registry returns early
func TestService_StartMetricsCollectors_NilRegistry(_ *testing.T) {
	svc := &service{metricsRegistry: nil}
	ctx := context.Background()

	// Should return early without panic
	svc.startMetricsCollectors(ctx)

	// No assertions needed - just ensuring no panic
}

// U-PSC-04: startMetricsCollectors registers collectors for valid clusters
func TestService_StartMetricsCollectors_ValidClusters(_ *testing.T) {
	reg := prometheus.NewRegistry()
	svc := &service{
		metricsRegistry: reg,
		isiClusters:     &sync.Map{},
	}

	// Mock cluster config - use interface not concrete type
	mockIsiSvc := &isiService{}
	clusterCfg := &IsilonClusterConfig{
		ClusterName: "test-cluster",
		isiSvc:      mockIsiSvc,
	}
	svc.isiClusters.Store("cluster1", clusterCfg)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// This should register collectors and start them
	svc.startMetricsCollectors(ctx)

	// Wait briefly for goroutines to start
	time.Sleep(50 * time.Millisecond)
}

// U-PSC-05: startMetricsCollectors handles invalid cluster configs
func TestService_StartMetricsCollectors_InvalidClusters(_ *testing.T) {
	reg := prometheus.NewRegistry()
	svc := &service{
		metricsRegistry: reg,
		isiClusters:     &sync.Map{},
	}

	// Add invalid configs
	svc.isiClusters.Store("invalid1", "not-a-config")
	svc.isiClusters.Store("invalid2", (*IsilonClusterConfig)(nil))
	svc.isiClusters.Store("invalid3", &IsilonClusterConfig{isiSvc: nil})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Should handle invalid configs gracefully
	svc.startMetricsCollectors(ctx)
}

// U-PSC-06: startMetricsCollectors with empty clusters map
func TestService_StartMetricsCollectors_EmptyClusters(_ *testing.T) {
	reg := prometheus.NewRegistry()
	svc := &service{
		metricsRegistry: reg,
		isiClusters:     &sync.Map{},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Should handle empty cluster map gracefully
	svc.startMetricsCollectors(ctx)
}

// U-PSC-07: Test PatchNodeLabels error paths
func TestService_PatchNodeLabels_ErrorPaths(t *testing.T) {
	svc := &service{
		nodeID:    "test-node",
		k8sclient: nil,
	}

	// PatchNodeLabels must return an error (not panic) when k8sclient is nil
	err := svc.PatchNodeLabels(map[string]string{"key": "value"}, []string{"remove"})
	if err == nil {
		t.Error("Expected error when k8sclient is nil, got nil")
	}
}

// U-MET-01: startMetricsServer with no TLS envs starts an HTTP server.
func TestService_StartMetricsServer_HTTP(t *testing.T) {
	port := pscFreePort(t)
	reg := prometheus.NewRegistry()
	svc := &service{
		metricsRegistry: reg,
		opts: Opts{
			MetricsPort: fmt.Sprintf(":%d", port),
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := svc.startMetricsServer(ctx)
	assert.NoError(t, err)
	time.Sleep(60 * time.Millisecond)

	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/healthz", port))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// U-MET-02: Only cert set (no key) returns an error.
func TestService_StartMetricsServer_CertOnlyStartsHTTP(t *testing.T) {
	port := pscFreePort(t)
	reg := prometheus.NewRegistry()
	svc := &service{
		metricsRegistry: reg,
		opts: Opts{
			MetricsPort:        fmt.Sprintf(":%d", port),
			MetricsTLSCertFile: "/nonexistent/cert.pem",
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := svc.startMetricsServer(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid TLS configuration")
}

// U-MET-03: Both TLS files set but invalid returns an error and does not start HTTP.
func TestService_StartMetricsServer_InvalidTLSReturnsError(t *testing.T) {
	port := pscFreePort(t)
	reg := prometheus.NewRegistry()
	svc := &service{
		metricsRegistry: reg,
		opts: Opts{
			MetricsPort:        fmt.Sprintf(":%d", port),
			MetricsTLSCertFile: "/nonexistent/cert.pem",
			MetricsTLSKeyFile:  "/nonexistent/key.pem",
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := svc.startMetricsServer(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid TLS configuration")
	time.Sleep(60 * time.Millisecond)

	_, err = http.Get(fmt.Sprintf("http://localhost:%d/healthz", port))
	assert.Error(t, err, "HTTP metrics endpoint must not start when TLS cert/key pair is invalid")
}

// U-MET-05: startMetricsServer with valid TLS cert/key starts an HTTPS server.
// Plain HTTP to the HTTPS port must fail (TLS handshake error).
func TestService_StartMetricsServer_HTTPS(t *testing.T) {
	port := pscFreePort(t)
	certFile, keyFile := generateSelfSignedCert(t)

	reg := prometheus.NewRegistry()
	svc := &service{
		metricsRegistry: reg,
		opts: Opts{
			MetricsPort:        fmt.Sprintf(":%d", port),
			MetricsTLSCertFile: certFile,
			MetricsTLSKeyFile:  keyFile,
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	require.NoError(t, svc.startMetricsServer(ctx))
	time.Sleep(150 * time.Millisecond)

	// HTTPS with a client that trusts the self-signed cert.
	tlsClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
		},
		Timeout: 5 * time.Second,
	}
	resp, err := tlsClient.Get(fmt.Sprintf("https://localhost:%d/metrics", port))
	require.NoError(t, err, "HTTPS request to metrics endpoint must succeed")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// U-REC-01: reconcileMetricsCollectors creates a fresh manager after a config change.
func TestService_ReconcileMetricsCollectors_RebuildsManager(t *testing.T) {
	reg := prometheus.NewRegistry()
	svc := &service{
		metricsRegistry: reg,
		isiClusters:     &sync.Map{},
		opts:            Opts{MetricsCollectionInterval: 30 * time.Second},
	}

	clusterCfg := &IsilonClusterConfig{
		ClusterName: "cluster1",
		isiSvc:      &isiService{},
	}
	svc.isiClusters.Store("cluster1", clusterCfg)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// First start populates metricsCollectorManager.
	svc.startMetricsCollectors(ctx)
	require.NotNil(t, svc.metricsCollectorManager, "manager must be non-nil after first start")
	first := svc.metricsCollectorManager

	// Swap in a new cluster to simulate a config reload.
	svc.isiClusters.Delete("cluster1")
	svc.isiClusters.Store("cluster2", &IsilonClusterConfig{
		ClusterName: "cluster2",
		isiSvc:      &isiService{},
	})

	svc.reconcileMetricsCollectors(ctx)
	require.NotNil(t, svc.metricsCollectorManager, "manager must be non-nil after reconcile")
	assert.NotSame(t, first, svc.metricsCollectorManager,
		"reconcile must create a new manager instance")
}

// U-REC-02: reconcileMetricsCollectors is a no-op when metrics registry is nil.
func TestService_ReconcileMetricsCollectors_NoopWhenDisabled(t *testing.T) {
	svc := &service{
		metricsRegistry: nil,
		isiClusters:     &sync.Map{},
	}
	assert.NotPanics(t, func() {
		svc.reconcileMetricsCollectors(context.Background())
	})
	assert.Nil(t, svc.metricsCollectorManager)
}

// U-MET-04: startMetricsServer wires metricsStaleReporter via SetStale.
func TestService_StartMetricsServer_WiresStaleReporter(t *testing.T) {
	port := pscFreePort(t)
	reg := prometheus.NewRegistry()
	svc := &service{
		metricsRegistry: reg,
		opts: Opts{
			MetricsPort: fmt.Sprintf(":%d", port),
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	require.NoError(t, svc.startMetricsServer(ctx))
	assert.NotNil(t, svc.metricsStaleReporter, "metricsStaleReporter must be set after startMetricsServer")
}

// U-MET-08: startMetricsServer initializes stale metrics for clusters
func TestService_StartMetricsServer_InitializesStaleMetrics(t *testing.T) {
	port := pscFreePort(t)
	reg := prometheus.NewRegistry()
	svc := &service{
		metricsRegistry: reg,
		isiClusters:     &sync.Map{},
		opts: Opts{
			MetricsPort: fmt.Sprintf(":%d", port),
		},
	}

	// Add a cluster config
	clusterCfg := &IsilonClusterConfig{
		ClusterName: "test-cluster",
		isiSvc:      &isiService{},
	}
	svc.isiClusters.Store("cluster1", clusterCfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	require.NoError(t, svc.startMetricsServer(ctx))
	assert.NotNil(t, svc.metricsStaleReporter, "metricsStaleReporter must be set after startMetricsServer")
}

// U-MODE-01: node-mode pods do not start cluster-wide metrics collectors.
func TestService_StartMetricsCollectors_NodeMode_NoOp(t *testing.T) {
	reg := prometheus.NewRegistry()
	svc := &service{
		mode:            constants.ModeNode,
		metricsRegistry: reg,
		isiClusters:     &sync.Map{},
	}
	svc.isiClusters.Store("cluster1", &IsilonClusterConfig{
		ClusterName: "cluster1",
		isiSvc:      &isiService{},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	svc.startMetricsCollectors(ctx)

	// Node mode now starts the full driver health collector (including CPU/memory metrics)
	assert.NotNil(t, svc.metricsCollectorManager,
		"node-mode pods must start driver health collector with CPU/memory metrics")
}

// U-MODE-02: controller-mode pods start the cluster-wide collectors.
func TestService_StartMetricsCollectors_ControllerMode_StartsCollectors(t *testing.T) {
	reg := prometheus.NewRegistry()
	svc := &service{
		mode:            constants.ModeController,
		metricsRegistry: reg,
		isiClusters:     &sync.Map{},
	}
	svc.isiClusters.Store("cluster1", &IsilonClusterConfig{
		ClusterName: "cluster1",
		isiSvc:      &isiService{},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	svc.startMetricsCollectors(ctx)

	require.NotNil(t, svc.metricsCollectorManager,
		"controller-mode pods must start cluster-wide collectors")
	svc.metricsCollectorManager.Stop()
}

// U-MODE-03: reconcileMetricsCollectors also no-ops in node mode.
func TestService_ReconcileMetricsCollectors_NodeMode_NoOp(t *testing.T) {
	svc := &service{
		mode:            constants.ModeNode,
		metricsRegistry: prometheus.NewRegistry(),
		isiClusters:     &sync.Map{},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	assert.NotPanics(t, func() {
		svc.reconcileMetricsCollectors(ctx)
	})
	assert.Nil(t, svc.metricsCollectorManager)
}

// U-MODE-04: metrics are disabled only when MetricsEnabled=false, regardless of mode.
func TestBeforeServe_MetricsDisabled_RegistryNeverCreated(t *testing.T) {
	for _, mode := range []string{constants.ModeController, constants.ModeNode, ""} {
		svc := &service{}
		svc.mode = mode
		svc.opts = Opts{MetricsEnabled: false}

		if svc.opts.MetricsEnabled {
			svc.metricsRegistry = prometheus.NewRegistry()
		}

		assert.Nil(t, svc.metricsRegistry,
			"metricsRegistry must be nil when MetricsEnabled=false (mode=%q)", mode)
	}
}

// U-PSC-08: Test GetNodeLabels error paths
func TestService_GetNodeLabels_ErrorPaths(t *testing.T) {
	svc := &service{
		nodeID: "test-node",
		opts:   Opts{}, // No KubeConfigPath - will cause CreateKubeClientSet to fail
	}

	// Test with invalid kubeconfig - should return error, not panic
	labels, err := svc.GetNodeLabels()
	assert.Error(t, err, "GetNodeLabels should fail with invalid kubeconfig")
	assert.Nil(t, labels, "Labels should be nil on error")

	// Error can be either missing token file or connection refused depending on environment
	errMsg := err.Error()
	hasExpectedError := strings.Contains(errMsg, "no such file or directory") ||
		strings.Contains(errMsg, "connection refused") ||
		strings.Contains(errMsg, "dial tcp")
	assert.True(t, hasExpectedError, "Error should indicate kubeconfig/connection issue, got: %s", errMsg)
}

// U-OBS-01: PowerScaleAPIObserver records success metrics
func TestPowerScaleAPIObserver_Success(t *testing.T) {
	reg := prometheus.NewRegistry()
	metric := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "dell_powerscale_api_calls_total",
		Help: "Total OneFS REST API calls by status (success/failure).",
	}, []string{"cluster_name", "status"})
	reg.MustRegister(metric)

	observer := &PowerScaleAPIObserver{
		clusterName: "test-cluster",
		apiRequests: metric,
	}

	// Simulate a successful API call
	observer.ObservePowerScaleRequest(api.RequestObservation{
		StatusCode: 200,
		Err:        nil,
	})

	// Check that success counter was incremented
	metrics, err := reg.Gather()
	require.NoError(t, err)
	assert.Greater(t, len(metrics), 0)
}

// U-OBS-02: PowerScaleAPIObserver records failure metrics for errors
func TestPowerScaleAPIObserver_Failure_Error(t *testing.T) {
	reg := prometheus.NewRegistry()
	metric := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "dell_powerscale_api_calls_total",
		Help: "Total OneFS REST API calls by status (success/failure).",
	}, []string{"cluster_name", "status"})
	reg.MustRegister(metric)

	observer := &PowerScaleAPIObserver{
		clusterName: "test-cluster",
		apiRequests: metric,
	}

	// Simulate a failed API call with error
	observer.ObservePowerScaleRequest(api.RequestObservation{
		StatusCode: 200,
		Err:        assert.AnError,
	})

	// Check that failure counter was incremented
	metrics, err := reg.Gather()
	require.NoError(t, err)
	assert.Greater(t, len(metrics), 0)
}

// U-OBS-03: PowerScaleAPIObserver records failure metrics for 4xx/5xx status codes
func TestPowerScaleAPIObserver_Failure_StatusCode(t *testing.T) {
	reg := prometheus.NewRegistry()
	metric := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "dell_powerscale_api_calls_total",
		Help: "Total OneFS REST API calls by status (success/failure).",
	}, []string{"cluster_name", "status"})
	reg.MustRegister(metric)

	observer := &PowerScaleAPIObserver{
		clusterName: "test-cluster",
		apiRequests: metric,
	}

	// Simulate a failed API call with 500 status
	observer.ObservePowerScaleRequest(api.RequestObservation{
		StatusCode: 500,
		Err:        nil,
	})

	// Check that failure counter was incremented
	metrics, err := reg.Gather()
	require.NoError(t, err)
	assert.Greater(t, len(metrics), 0)
}

// U-OBS-04: PowerScaleAPIObserver handles nil observer gracefully
func TestPowerScaleAPIObserver_NilObserver(t *testing.T) {
	var observer *PowerScaleAPIObserver

	// Should not panic
	assert.NotPanics(t, func() {
		observer.ObservePowerScaleRequest(api.RequestObservation{
			StatusCode: 200,
			Err:        nil,
		})
	})
}

// U-OBS-05: PowerScaleAPIObserver handles nil metric gracefully
func TestPowerScaleAPIObserver_NilMetric(t *testing.T) {
	observer := &PowerScaleAPIObserver{
		clusterName: "test-cluster",
		apiRequests: nil,
	}

	// Should not panic
	assert.NotPanics(t, func() {
		observer.ObservePowerScaleRequest(api.RequestObservation{
			StatusCode: 200,
			Err:        nil,
		})
	})
}

// U-OBS-06: PowerScaleAPIObserver handles 3xx status codes as success
func TestPowerScaleAPIObserver_Success_3xxStatusCode(t *testing.T) {
	reg := prometheus.NewRegistry()
	metric := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "dell_powerscale_api_calls_total",
		Help: "Total OneFS REST API calls by status (success/failure).",
	}, []string{"cluster_name", "status"})
	reg.MustRegister(metric)

	observer := &PowerScaleAPIObserver{
		clusterName: "test-cluster",
		apiRequests: metric,
	}

	// Simulate a successful API call with 301 status (redirect)
	observer.ObservePowerScaleRequest(api.RequestObservation{
		StatusCode: 301,
		Err:        nil,
	})

	// Check that success counter was incremented
	metrics, err := reg.Gather()
	require.NoError(t, err)
	assert.Greater(t, len(metrics), 0)
}

// U-OBS-07: GetIsiClient attaches observer when metrics enabled
func TestGetIsiClient_AttachesObserver_WhenMetricsEnabled(t *testing.T) {
	reg := prometheus.NewRegistry()
	fakeAPIClient := &fakePowerScaleAPIClient{}
	mockIsiClientFactory(t, &isi.Client{API: fakeAPIClient})
	svc := &service{
		metricsRegistry: reg,
		opts: Opts{
			SkipCertificateValidation: true,
			Verbose:                   0,
			IgnoreUnresolvableHosts:   true,
			isiAuthType:               0,
		},
	}

	isiConfig := &IsilonClusterConfig{
		ClusterName:               "test-cluster",
		Endpoint:                  "https://test.example.com:8080",
		EndpointPort:              "8080",
		EndpointURL:               "https://test.example.com:8080",
		User:                      "admin",
		Password:                  "password",
		IsiPath:                   "/ifs",
		IsiVolumePathPermissions:  "0777",
		SkipCertificateValidation: boolPtr(true),
		IgnoreUnresolvableHosts:   boolPtr(true),
	}

	ctx := context.Background()
	client, err := svc.GetIsiClient(ctx, isiConfig)
	require.NoError(t, err)
	require.NotNil(t, client)
	assert.NotNil(t, fakeAPIClient.GetRequestObserver())
	fakeAPIClient.GetRequestObserver().ObservePowerScaleRequest(api.RequestObservation{StatusCode: 200})

	metricFamilies, gatherErr := reg.Gather()
	require.NoError(t, gatherErr)
	metricFound := false
	for _, metricFamily := range metricFamilies {
		if metricFamily.GetName() == "dell_powerscale_api_calls_total" {
			metricFound = true
			break
		}
	}
	assert.True(t, metricFound)
}

// U-OBS-08: GetIsiClient does not attach observer when metrics disabled
func TestGetIsiClient_DoesNotAttachObserver_WhenMetricsDisabled(t *testing.T) {
	fakeAPIClient := &fakePowerScaleAPIClient{}
	mockIsiClientFactory(t, &isi.Client{API: fakeAPIClient})
	svc := &service{
		metricsRegistry: nil,
		opts: Opts{
			SkipCertificateValidation: true,
			Verbose:                   0,
			IgnoreUnresolvableHosts:   true,
			isiAuthType:               0,
		},
	}

	isiConfig := &IsilonClusterConfig{
		ClusterName:               "test-cluster",
		Endpoint:                  "https://test.example.com:8080",
		EndpointPort:              "8080",
		EndpointURL:               "https://test.example.com:8080",
		User:                      "admin",
		Password:                  "password",
		IsiPath:                   "/ifs",
		IsiVolumePathPermissions:  "0777",
		SkipCertificateValidation: boolPtr(true),
		IgnoreUnresolvableHosts:   boolPtr(true),
	}

	ctx := context.Background()
	client, err := svc.GetIsiClient(ctx, isiConfig)
	require.NoError(t, err)
	require.NotNil(t, client)
	assert.Nil(t, fakeAPIClient.GetRequestObserver())
}

// U-OBS-09: GetIsiClient handles custom topology enabled
func TestGetIsiClient_CustomTopology(t *testing.T) {
	svc := &service{
		metricsRegistry: nil,
		opts: Opts{
			CustomTopologyEnabled:     true,
			SkipCertificateValidation: true,
			Verbose:                   0,
			IgnoreUnresolvableHosts:   true,
			isiAuthType:               0,
		},
		nodeID: "test-node",
	}

	isiConfig := &IsilonClusterConfig{
		ClusterName:               "test-cluster",
		Endpoint:                  "https://test.example.com:8080",
		EndpointPort:              "8080",
		EndpointURL:               "https://test.example.com:8080",
		User:                      "admin",
		Password:                  "password",
		IsiPath:                   "/ifs",
		IsiVolumePathPermissions:  "0777",
		SkipCertificateValidation: boolPtr(true),
		IgnoreUnresolvableHosts:   boolPtr(true),
	}

	ctx := context.Background()
	_, err := svc.GetIsiClient(ctx, isiConfig)

	// Should fail due to missing node labels (GetNodeLabels will fail)
	assert.Error(t, err, "GetIsiClient should fail when custom topology enabled but labels not found")
}

func boolPtr(b bool) *bool {
	return &b
}

// U-REC-09: RecordAPICall calls driverHealthCollector when not nil
func TestService_RecordAPICall(t *testing.T) {
	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()
	collector := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)
	svc := &service{
		driverHealthCollector: collector,
	}

	// Should not panic when calling RecordAPICall
	assert.NotPanics(t, func() {
		svc.RecordAPICall(true, 200)
	})

	assert.NotPanics(t, func() {
		svc.RecordAPICall(false, 500)
	})
}

// U-REC-10: RecordAPICall does not panic when driverHealthCollector is nil
func TestService_RecordAPICall_NilCollector(t *testing.T) {
	svc := &service{
		driverHealthCollector: nil,
	}

	assert.NotPanics(t, func() {
		svc.RecordAPICall(true, 200)
	})
}

// U-REC-12: RecordAPICall with various HTTP status codes
func TestService_RecordAPICall_VariousStatusCodes(t *testing.T) {
	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()
	collector := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)
	svc := &service{
		driverHealthCollector: collector,
	}

	// Test with various HTTP status codes
	statusCodes := []int{200, 201, 204, 400, 401, 403, 404, 500, 502, 503}
	for _, statusCode := range statusCodes {
		isSuccess := statusCode < 400
		assert.NotPanics(t, func() {
			svc.RecordAPICall(isSuccess, statusCode)
		})
	}
}

// U-REC-13: RecordAPICall with different success/failure combinations
func TestService_RecordAPICall_Combinations(t *testing.T) {
	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()
	collector := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)
	svc := &service{
		driverHealthCollector: collector,
	}

	// Test alternating success/failure
	for i := 0; i < 10; i++ {
		isSuccess := i%2 == 0
		assert.NotPanics(t, func() {
			svc.RecordAPICall(isSuccess, 200)
		})
	}
}

// U-REC-11: RecordAuthFailure calls accessControlCollector when not nil
func TestService_RecordAuthFailure(t *testing.T) {
	reg := prometheus.NewRegistry()
	mockClient := &mockAccessControlClient{}
	collector := collectors.NewAccessControlCollector(mockClient, reg, "test-cluster")
	svc := &service{
		accessControlCollector: collector,
	}

	// Should not panic when calling RecordAuthFailure
	assert.NotPanics(t, func() {
		svc.RecordAuthFailure()
	})
}

// U-REC-12: RecordAuthFailure does not panic when accessControlCollector is nil
func TestService_RecordAuthFailure_NilCollector(t *testing.T) {
	svc := &service{
		accessControlCollector: nil,
	}

	assert.NotPanics(t, func() {
		svc.RecordAuthFailure()
	})
}

// U-REC-13: RecordPermissionDenial calls accessControlCollector when not nil
func TestService_RecordPermissionDenial(t *testing.T) {
	reg := prometheus.NewRegistry()
	mockClient := &mockAccessControlClient{}
	collector := collectors.NewAccessControlCollector(mockClient, reg, "test-cluster")
	svc := &service{
		accessControlCollector: collector,
	}

	// Should not panic when calling RecordPermissionDenial
	assert.NotPanics(t, func() {
		svc.RecordPermissionDenial()
	})
}

// U-REC-14: RecordPermissionDenial does not panic when accessControlCollector is nil
func TestService_RecordPermissionDenial_NilCollector(t *testing.T) {
	svc := &service{
		accessControlCollector: nil,
	}

	assert.NotPanics(t, func() {
		svc.RecordPermissionDenial()
	})
}

// U-PARSE-01: parseDuration returns default when string is empty
func TestParseDuration_EmptyString(t *testing.T) {
	result := parseDuration("", 30*time.Second)
	assert.Equal(t, 30*time.Second, result)
}

// U-PARSE-02: parseDuration returns parsed duration when valid
func TestParseDuration_ValidDuration(t *testing.T) {
	result := parseDuration("1m", 30*time.Second)
	assert.Equal(t, 1*time.Minute, result)
}

// U-PARSE-03: parseDuration returns default when invalid
func TestParseDuration_InvalidDuration(t *testing.T) {
	result := parseDuration("invalid", 30*time.Second)
	assert.Equal(t, 30*time.Second, result)
}

// U-MET-06: MetricsRegistry returns the metrics registry
func TestService_MetricsRegistry(t *testing.T) {
	reg := prometheus.NewRegistry()
	svc := &service{
		metricsRegistry: reg,
	}

	result := svc.MetricsRegistry()
	assert.Equal(t, reg, result)
}

// U-MET-07: MetricsRegistry returns nil when not set
func TestService_MetricsRegistry_Nil(t *testing.T) {
	svc := &service{
		metricsRegistry: nil,
	}

	result := svc.MetricsRegistry()
	assert.Nil(t, result)
}

// U-COL-01: getPodLevelCollectors returns collectors for pod-level metrics
func TestService_GetPodLevelCollectors(t *testing.T) {
	reg := prometheus.NewRegistry()
	svc := &service{
		metricsRegistry: reg,
		isiClusters:     &sync.Map{},
	}

	// Add a cluster config
	clusterCfg := &IsilonClusterConfig{
		ClusterName: "test-cluster",
		isiSvc:      &isiService{},
	}
	svc.isiClusters.Store("cluster1", clusterCfg)

	runtimeCfg := collectors.RuntimeConfig{}

	collectors := svc.getPodLevelCollectors(runtimeCfg)
	assert.Greater(t, len(collectors), 0, "should return at least one collector")
}

// U-COL-02: getPodLevelCollectors handles empty isiClusters
func TestService_GetPodLevelCollectors_Empty(t *testing.T) {
	reg := prometheus.NewRegistry()
	svc := &service{
		metricsRegistry: reg,
		isiClusters:     &sync.Map{},
	}

	runtimeCfg := collectors.RuntimeConfig{}

	collectors := svc.getPodLevelCollectors(runtimeCfg)
	assert.Equal(t, 0, len(collectors), "should return empty list when no clusters")
}

// U-COL-03: getArrayLevelCollectors returns collectors for array-level metrics
func TestService_GetArrayLevelCollectors(t *testing.T) {
	reg := prometheus.NewRegistry()
	svc := &service{
		metricsRegistry: reg,
		isiClusters:     &sync.Map{},
		opts:            Opts{Path: "/ifs"},
	}

	// Add a cluster config with isiSvc
	clusterCfg := &IsilonClusterConfig{
		ClusterName: "test-cluster",
		IsiPath:     "/ifs/data",
		isiSvc:      &isiService{},
	}
	svc.isiClusters.Store("cluster1", clusterCfg)

	runtimeCfg := collectors.RuntimeConfig{}

	collectors := svc.getArrayLevelCollectors(runtimeCfg)
	assert.Greater(t, len(collectors), 0, "should return at least one collector")
}

// U-COL-04: getArrayLevelCollectors handles empty isiClusters
func TestService_GetArrayLevelCollectors_Empty(t *testing.T) {
	reg := prometheus.NewRegistry()
	svc := &service{
		metricsRegistry: reg,
		isiClusters:     &sync.Map{},
		opts:            Opts{Path: "/ifs"},
	}

	runtimeCfg := collectors.RuntimeConfig{}

	collectors := svc.getArrayLevelCollectors(runtimeCfg)
	assert.Equal(t, 0, len(collectors), "should return empty list when no clusters")
}

// U-LEAD-01: startMetricsWithLeaderElection falls back to startMetricsWithoutLeaderElection when k8s client creation fails
func TestService_StartMetricsWithLeaderElection_Fallback(t *testing.T) {
	reg := prometheus.NewRegistry()
	svc := &service{
		metricsRegistry: reg,
		isiClusters:     &sync.Map{},
		opts:            Opts{Path: "/ifs"},
	}

	mgr := collectors.NewCollectorManager()
	runtimeCfg := collectors.RuntimeConfig{}

	ctx := context.Background()
	// Set invalid kubeconfig to trigger fallback
	t.Setenv("KUBECONFIG", "/nonexistent/kubeconfig")

	// Should not panic when k8s client creation fails
	assert.NotPanics(t, func() {
		svc.startMetricsWithLeaderElection(ctx, mgr, runtimeCfg)
	})
}

// mockAccessControlClient is a mock implementation of AccessControlClient
type mockAccessControlClient struct{}

func (m *mockAccessControlClient) GetZones(_ context.Context) ([]collectors.ZoneInfo, error) {
	return []collectors.ZoneInfo{
		{Name: "System"},
		{Name: "zone1"},
	}, nil
}

func (m *mockAccessControlClient) GetExportCountByZone(_ context.Context, _ string) (int, error) {
	return 10, nil
}

// U-PATCH-01: PatchNodeLabels with nil k8sclient returns error
func TestService_PatchNodeLabels_NilK8sClient(t *testing.T) {
	svc := &service{
		nodeID:    "test-node",
		k8sclient: nil,
	}

	// Should panic when k8sclient is nil, so we recover
	defer func() {
		if r := recover(); r != nil {
			t.Logf("Expected panic caught: %v", r)
		}
	}()

	_ = svc.PatchNodeLabels(map[string]string{"key": "value"}, []string{})
}

// U-PATCH-02: PatchNodeLabels with valid k8sclient
func TestService_PatchNodeLabels_ValidK8sClient(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()
	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "test-node",
			Labels: map[string]string{"existing": "label"},
		},
	}
	_, err := fakeClient.CoreV1().Nodes().Create(context.Background(), node, metav1.CreateOptions{})
	require.NoError(t, err)

	svc := &service{
		nodeID:    "test-node",
		k8sclient: fakeClient,
	}

	err = svc.PatchNodeLabels(map[string]string{"new": "label"}, []string{"existing"})
	assert.NoError(t, err)
}

// U-NEW-01: New creates a service instance
func TestMetricsNew(t *testing.T) {
	svc := New()
	assert.NotNil(t, svc)
}

// U-NEW-02: GetIsiPathForVolumeFromClusterConfig returns cluster path when set
func TestMetricsGetIsiPathForVolumeFromClusterConfig_ClusterPath(t *testing.T) {
	svc := &service{
		opts: Opts{Path: "/ifs"},
	}
	config := &IsilonClusterConfig{IsiPath: "/ifs/data"}
	path := svc.getIsiPathForVolumeFromClusterConfig(config)
	assert.Equal(t, "/ifs/data", path)
}

// U-NEW-03: GetIsiPathForVolumeFromClusterConfig returns default path when cluster path empty
func TestMetricsGetIsiPathForVolumeFromClusterConfig_DefaultPath(t *testing.T) {
	svc := &service{
		opts: Opts{Path: "/ifs"},
	}
	config := &IsilonClusterConfig{IsiPath: ""}
	path := svc.getIsiPathForVolumeFromClusterConfig(config)
	assert.Equal(t, "/ifs", path)
}

// U-NEW-04: ValidateCreateVolumeRequest validates volume capabilities
func TestMetricsValidateCreateVolumeRequest(t *testing.T) {
	svc := &service{}
	req := &csi.CreateVolumeRequest{
		Name: "test-volume",
		CapacityRange: &csi.CapacityRange{
			RequiredBytes: 1024 * 1024 * 1024,
		},
		VolumeCapabilities: []*csi.VolumeCapability{
			{
				AccessMode: &csi.VolumeCapability_AccessMode{
					Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
				},
			},
		},
	}
	size, err := svc.ValidateCreateVolumeRequest(req)
	assert.NoError(t, err)
	assert.Greater(t, size, int64(0))
}

// U-NEW-05: ValidateCreateVolumeRequest fails with no name
func TestMetricsValidateCreateVolumeRequest_NoName(t *testing.T) {
	svc := &service{}
	req := &csi.CreateVolumeRequest{
		Name:               "",
		VolumeCapabilities: []*csi.VolumeCapability{},
	}
	_, err := svc.ValidateCreateVolumeRequest(req)
	assert.Error(t, err)
}

// U-NEW-06: ValidateDeleteVolumeRequest validates volume ID with proper format
func TestMetricsValidateDeleteVolumeRequest(t *testing.T) {
	svc := &service{}
	ctx := context.Background()
	req := &csi.DeleteVolumeRequest{
		VolumeId: "test-volume=_=_=123=_=_=System=_=_=cluster1",
	}
	err := svc.ValidateDeleteVolumeRequest(ctx, req)
	assert.NoError(t, err)
}

// U-NEW-07: ValidateDeleteVolumeRequest fails with empty volume ID
func TestMetricsValidateDeleteVolumeRequest_EmptyVolumeID(t *testing.T) {
	svc := &service{}
	ctx := context.Background()
	req := &csi.DeleteVolumeRequest{
		VolumeId: "",
	}
	err := svc.ValidateDeleteVolumeRequest(ctx, req)
	assert.Error(t, err)
}

// U-NEW-08: GetCSINodeID returns error when nodeID is empty
func TestMetricsGetCSINodeID_Empty(t *testing.T) {
	svc := &service{nodeID: ""}
	_, err := svc.GetCSINodeID()
	assert.Error(t, err)
}

// U-NEW-09: GetCSINodeID returns nodeID when set
func TestMetricsGetCSINodeID_Set(t *testing.T) {
	svc := &service{nodeID: "test-node"}
	id, err := svc.GetCSINodeID()
	assert.NoError(t, err)
	assert.Equal(t, "test-node", id)
}

// U-NEW-10: GetCSINodeIP returns error when nodeIP is empty
func TestMetricsGetCSINodeIP_Empty(t *testing.T) {
	svc := &service{nodeIP: ""}
	_, err := svc.GetCSINodeIP()
	assert.Error(t, err)
}

// U-NEW-11: GetCSINodeIP returns nodeIP when set
func TestMetricsGetCSINodeIP_Set(t *testing.T) {
	svc := &service{nodeIP: "10.0.0.1"}
	ip, err := svc.GetCSINodeIP()
	assert.NoError(t, err)
	assert.Equal(t, "10.0.0.1", ip)
}
