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

package collectors

import (
	"context"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/common/constants"
	"github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/common/k8sutils"
	"github.com/Ecosystems/container-storage-modules/src/csm-metrics-common/pkg/naming"
	"github.com/Ecosystems/container-storage-modules/src/csmlog"
	"github.com/prometheus/client_golang/prometheus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

const (
	unknownLabel = "unknown"
)

// PSCDriverHealthCollector collects PowerScale driver health metrics.
type PSCDriverHealthCollector struct {
	clusterName    string
	startTime      time.Time
	uptimeGauge    *prometheus.GaugeVec
	restartTotal   *prometheus.GaugeVec
	goroutineCount *prometheus.GaugeVec
	connPoolActive *prometheus.GaugeVec
	APICallsTotal  *prometheus.CounterVec
	cpuUsage       *prometheus.GaugeVec
	memUsage       *prometheus.GaugeVec
	k8sClient      kubernetes.Interface
	nodeMode       bool // true if running in node mode (only restart count)
}

// NewPSCDriverHealthCollector creates a new PSCDriverHealthCollector.
func NewPSCDriverHealthCollector(reg prometheus.Registerer, clusterName string, k8sClient kubernetes.Interface) *PSCDriverHealthCollector {
	c := &PSCDriverHealthCollector{
		clusterName: clusterName,
		startTime:   time.Now().Add(-time.Millisecond),
		uptimeGauge: registerOrGetGaugeVec(reg, prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: naming.MetricCSIDriverUptimeSeconds,
			Help: "Driver uptime in seconds since last restart.",
		}, []string{naming.LabelClusterName, "pod", "node", "namespace", "instance_type"})),
		restartTotal: registerOrGetGaugeVec(reg, prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: naming.MetricCSIDriverRestartTotal,
			Help: "Total driver restarts.",
		}, []string{naming.LabelClusterName, "pod", "node", "namespace", "instance_type"})),
		goroutineCount: registerOrGetGaugeVec(reg, prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: naming.MetricCSIGoroutineCount,
			Help: "Number of active goroutines.",
		}, []string{naming.LabelClusterName, "pod", "node", "namespace", "instance_type"})),
		connPoolActive: registerOrGetGaugeVec(reg, prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: naming.MetricCSIConnectionPoolActive,
			Help: "Active connections to OneFS.",
		}, []string{naming.LabelClusterName})),
		APICallsTotal: RegisterOrGetCounterVec(reg, prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "dell_powerscale_api_calls_total",
			Help: "Total OneFS REST API calls by status (success/failure).",
		}, []string{naming.LabelClusterName, naming.LabelStatus})),
		cpuUsage: registerOrGetGaugeVec(reg, prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: naming.MetricCSIDriverCPUUsagePercent,
			Help: "CSI driver CPU usage percentage from Kubernetes metrics API.",
		}, []string{naming.LabelClusterName, "pod", "node", "namespace", "instance_type"})),
		memUsage: registerOrGetGaugeVec(reg, prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: naming.MetricCSIDriverMemoryUsageBytes,
			Help: "CSI driver memory usage in bytes from Kubernetes metrics API.",
		}, []string{naming.LabelClusterName, "pod", "node", "namespace", "instance_type"})),
		k8sClient: k8sClient,
		nodeMode:  false, // controller mode (default)
	}

	csmlog.Info("Kubernetes metrics collection initialized (requires Metrics Server)")

	return c
}

// getPodIdentityLabels returns the pod identity labels for metrics
func (c *PSCDriverHealthCollector) GetPodIdentityLabels(podName, namespace, nodeName, mode string) []string {
	// Provide default values if parameters are empty
	if podName == "" {
		podName = unknownLabel
	}
	if nodeName == "" {
		nodeName = unknownLabel
	}
	if namespace == "" {
		namespace = unknownLabel
	}
	if mode == "" {
		mode = unknownLabel
	}

	return []string{c.clusterName, podName, nodeName, namespace, mode}
}

// CollectRestartCount fetches the restart count from Kubernetes Pod API
// Each pod instance will report only its own restart count using hostname and namespace detection
// This follows the same efficient approach as csi-powerstore
func (c *PSCDriverHealthCollector) CollectRestartCount(ctx context.Context) {
	if c.k8sClient == nil {
		csmlog.Warn("collectRestartCount: k8sClient is nil")
		return
	}

	// Get pod name, namespace, and node name from environment variables
	podName := os.Getenv(constants.EnvPodName)
	namespace := os.Getenv(constants.EnvDriverNamespace)
	nodeName := os.Getenv(constants.EnvNodeName)

	if namespace == "" {
		csmlog.Warn("collectRestartCount: X_CSI_DRIVER_NAMESPACE not set, skipping")
		return
	}

	if podName == "" {
		csmlog.Warn("collectRestartCount: POD_NAME not set, skipping")
		return
	}

	// Get pod directly using pod name and namespace
	pod, err := c.k8sClient.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		csmlog.Errorf("collectRestartCount: Failed to get pod %s: %v", podName, err)
		return
	}

	// Determine mode from pod labels
	var mode string
	if pod.Labels != nil {
		if appLabel, ok := pod.Labels["app"]; ok {
			if strings.HasSuffix(appLabel, "-controller") {
				mode = "controller"
			} else if strings.HasSuffix(appLabel, "-node") {
				mode = "node"
			}
		}
	}
	if mode == "" {
		mode = unknownLabel
	}

	// Get restart count using the actual pod name
	restartCount, err := k8sutils.GetPodRestartCountAuto(ctx, c.k8sClient, namespace, pod.Name)
	if err != nil {
		csmlog.Errorf("collectRestartCount: Failed to get restart count: %v", err)
		return // Skip on error
	}

	// Set the restart count metric with pod identity labels
	// Use nodeName from env var if available, otherwise use pod.Spec.NodeName
	finalNodeName := nodeName
	if finalNodeName == "" {
		finalNodeName = pod.Spec.NodeName
	}
	labels := c.GetPodIdentityLabels(pod.Name, pod.Namespace, finalNodeName, mode)
	c.restartTotal.WithLabelValues(labels...).Set(float64(restartCount))
}

// Collect updates driver health metrics.
func (c *PSCDriverHealthCollector) Collect(ctx context.Context) error {
	// Get pod identity from environment variables
	podName := os.Getenv(constants.EnvPodName)
	namespace := os.Getenv(constants.EnvDriverNamespace)
	nodeName := os.Getenv(constants.EnvNodeName)

	// Determine instance type from environment
	instanceType := "controller"
	if os.Getenv(constants.EnvCSIMode) == "node" {
		instanceType = "node"
	}

	// Get pod identity labels
	labels := c.GetPodIdentityLabels(podName, namespace, nodeName, instanceType)

	// Collect all metrics (both controller and node pods use the same full collector)
	c.uptimeGauge.WithLabelValues(labels...).Set(time.Since(c.startTime).Seconds())
	c.goroutineCount.WithLabelValues(labels...).Set(float64(runtime.NumGoroutine()))
	c.connPoolActive.WithLabelValues(c.clusterName).Set(1)
	c.CollectRestartCount(ctx)

	// Collect Kubernetes metrics (CPU and memory)
	c.CollectKubernetesMetrics(ctx)

	return nil
}

// Name returns the collector name.
func (c *PSCDriverHealthCollector) Name() string {
	return "PSCDriverHealthCollector"
}

// collectKubernetesMetrics fetches CPU/memory metrics from Kubernetes Metrics API

func (c *PSCDriverHealthCollector) CollectKubernetesMetrics(ctx context.Context) {
	if c.cpuUsage == nil || c.memUsage == nil {
		csmlog.Warn("collectKubernetesMetrics: Metrics not initialized, skipping")
		return
	}

	if c.k8sClient == nil {
		csmlog.Warn("collectKubernetesMetrics: k8sClient is nil, skipping")
		return
	}

	// Get pod name, namespace, and node name from environment variables
	podName := os.Getenv(constants.EnvPodName)
	namespace := os.Getenv(constants.EnvDriverNamespace)
	nodeName := os.Getenv(constants.EnvNodeName)

	if namespace == "" {
		csmlog.Warn("collectKubernetesMetrics: X_CSI_DRIVER_NAMESPACE not set, skipping")
		return
	}

	if podName == "" {
		csmlog.Warn("collectKubernetesMetrics: POD_NAME not set, skipping")
		return
	}

	// Get pod identity labels
	labels := c.GetPodIdentityLabels(podName, namespace, nodeName, "controller")

	// Query Kubernetes Metrics API
	podMetrics, err := k8sutils.GetPodMetrics(ctx, c.k8sClient, namespace, podName)
	if err != nil {
		csmlog.Errorf("collectKubernetesMetrics: Failed to get pod metrics: %v", err)
		csmlog.Info("collectKubernetesMetrics: Troubleshooting steps:")
		csmlog.Info("collectKubernetesMetrics:   1. Verify Kubernetes Metrics Server is installed: kubectl get deployment metrics-server -n kube-system")
		csmlog.Infof("collectKubernetesMetrics:   2. Check if pod metrics are available: kubectl top pod -n %s %s", namespace, podName)
		csmlog.Info("collectKubernetesMetrics:   3. Wait 30 seconds for metrics to be available after pod startup")
		csmlog.Info("collectKubernetesMetrics:   4. Check Metrics Server logs: kubectl logs -n kube-system -l k8s-app=metrics-server")
		return
	}

	// Find the CSI driver container and extract metrics
	containerFound := false
	for _, container := range podMetrics.Containers {
		if strings.Contains(container.Name, "csi-isilon") || strings.Contains(container.Name, "driver") {
			containerFound = true

			// Extract CPU usage (in millicores, convert to percentage)
			cpuUsage := container.Usage[v1.ResourceCPU]
			cpuMillis := cpuUsage.MilliValue()
			cpuPercent := float64(cpuMillis) / 10.0 // Convert millicores to percentage

			// Extract memory usage (in bytes)
			memUsage := container.Usage[v1.ResourceMemory]
			memBytes := float64(memUsage.Value())

			// Set metrics with pod identity labels
			c.cpuUsage.WithLabelValues(labels...).Set(cpuPercent)
			c.memUsage.WithLabelValues(labels...).Set(memBytes)
			return
		}
	}

	if !containerFound {
		csmlog.Warn("collectKubernetesMetrics: CSI driver container not found in pod")
		csmlog.Infof("collectKubernetesMetrics: Available containers: %v", getContainerNames(podMetrics.Containers))
		return
	}
}

// getContainerNames extracts container names from pod metrics
func getContainerNames(containers []metricsv1beta1.ContainerMetrics) []string {
	names := make([]string, 0, len(containers))
	for _, c := range containers {
		names = append(names, c.Name)
	}
	return names
}

// RecordAPICall records an API call result for success rate tracking
func (c *PSCDriverHealthCollector) RecordAPICall(success bool, _ int) {
	if c.APICallsTotal == nil {
		csmlog.Warn("RecordAPICall: APICallsTotal is nil - metric NOT incremented")
		return // Not initialized (node mode)
	}

	status := naming.LabelStatusFailure
	if success {
		status = naming.LabelStatusSuccess
	}
	c.APICallsTotal.WithLabelValues(c.clusterName, status).Inc()
}
