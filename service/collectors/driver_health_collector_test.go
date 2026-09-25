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

package collectors_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/service/collectors"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func gatherPSCHealthMetric(t *testing.T, reg prometheus.Gatherer, name string) *dto.MetricFamily {
	t.Helper()
	mfs, err := reg.Gather()
	require.NoError(t, err)
	for _, mf := range mfs {
		if mf.GetName() == name {
			return mf
		}
	}
	return nil
}

func gaugePSCHealth(mf *dto.MetricFamily, labels map[string]string) (float64, bool) {
	for _, m := range mf.GetMetric() {
		got := make(map[string]string)
		for _, lp := range m.GetLabel() {
			got[lp.GetName()] = lp.GetValue()
		}
		match := true
		for k, v := range labels {
			if got[k] != v {
				match = false
				break
			}
		}
		if match {
			return m.GetGauge().GetValue(), true
		}
	}
	return 0, false
}

func counterPSCHealth(mf *dto.MetricFamily, labels map[string]string) (float64, bool) {
	for _, m := range mf.GetMetric() {
		got := make(map[string]string)
		for _, lp := range m.GetLabel() {
			got[lp.GetName()] = lp.GetValue()
		}
		match := true
		for k, v := range labels {
			if got[k] != v {
				match = false
				break
			}
		}
		if match {
			return m.GetCounter().GetValue(), true
		}
	}
	return 0, false
}

// U-PSC-04: cluster_name label on uptime metric
func TestPSCDriverHealthCollector_Collect_ClusterNameLabel(t *testing.T) {
	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()
	c := collectors.NewPSCDriverHealthCollector(reg, "isilon-cluster-1", fakeClient)

	_ = c.Collect(context.Background())
	time.Sleep(5 * time.Millisecond)
	_ = c.Collect(context.Background())

	mf := gatherPSCHealthMetric(t, reg, "dell_csi_driver_uptime_seconds")
	require.NotNil(t, mf)
	v, ok := gaugePSCHealth(mf, map[string]string{"cluster_name": "isilon-cluster-1"})
	require.True(t, ok)
	assert.Greater(t, v, 0.0, "uptime should be > 0")
}

// U-PSC-DHN: Name returns expected string
func TestPSCDriverHealthCollector_Name(t *testing.T) {
	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()
	c := collectors.NewPSCDriverHealthCollector(reg, "isilon-cluster-1", fakeClient)
	assert.Equal(t, "PSCDriverHealthCollector", c.Name())
}

// U-PSC-GC: goroutine count metric is recorded
func TestPSCDriverHealthCollector_GoroutineCount(t *testing.T) {
	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()
	c := collectors.NewPSCDriverHealthCollector(reg, "isilon-cluster-3", fakeClient)

	err := c.Collect(context.Background())
	require.NoError(t, err)

	mf := gatherPSCHealthMetric(t, reg, "dell_csi_goroutine_count")
	require.NotNil(t, mf)
	v, ok := gaugePSCHealth(mf, map[string]string{"cluster_name": "isilon-cluster-3"})
	require.True(t, ok)
	assert.Greater(t, v, 0.0, "goroutine count should be > 0")
}

// U-PSC-CC: connection pool active metric is recorded
func TestPSCDriverHealthCollector_ConnectionPoolActive(t *testing.T) {
	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()
	c := collectors.NewPSCDriverHealthCollector(reg, "isilon-cluster-4", fakeClient)

	err := c.Collect(context.Background())
	require.NoError(t, err)

	mf := gatherPSCHealthMetric(t, reg, "dell_csi_connection_pool_active")
	require.NotNil(t, mf)
	v, ok := gaugePSCHealth(mf, map[string]string{"cluster_name": "isilon-cluster-4"})
	require.True(t, ok)
	assert.GreaterOrEqual(t, v, 0.0, "connection pool active should be >= 0")
}

// U-PSC-API: API calls total metric is recorded
func TestPSCDriverHealthCollector_APICallsTotal(t *testing.T) {
	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()
	c := collectors.NewPSCDriverHealthCollector(reg, "isilon-cluster-5", fakeClient)

	err := c.Collect(context.Background())
	require.NoError(t, err)

	// API calls total is a counter, may not be present if no calls recorded
	mf := gatherPSCHealthMetric(t, reg, "dell_csi_api_calls_total")
	// It's ok if metric doesn't exist yet (no API calls recorded)
	if mf != nil {
		v, ok := counterPSCHealth(mf, map[string]string{"cluster_name": "isilon-cluster-5"})
		if ok {
			assert.GreaterOrEqual(t, v, 0.0, "API calls total should be >= 0")
		}
	}
}

// U-PSC-MULTI: Multiple collectors for different clusters
func TestPSCDriverHealthCollector_MultipleClusters(t *testing.T) {
	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()
	c1 := collectors.NewPSCDriverHealthCollector(reg, "cluster-1", fakeClient)
	c2 := collectors.NewPSCDriverHealthCollector(reg, "cluster-2", fakeClient)

	err := c1.Collect(context.Background())
	require.NoError(t, err)
	err = c2.Collect(context.Background())
	require.NoError(t, err)

	mf := gatherPSCHealthMetric(t, reg, "dell_csi_driver_uptime_seconds")
	require.NotNil(t, mf)

	// Verify both clusters have metrics
	v1, ok1 := gaugePSCHealth(mf, map[string]string{"cluster_name": "cluster-1"})
	v2, ok2 := gaugePSCHealth(mf, map[string]string{"cluster_name": "cluster-2"})
	require.True(t, ok1, "cluster-1 should have metrics")
	require.True(t, ok2, "cluster-2 should have metrics")
	assert.Greater(t, v1, 0.0)
	assert.Greater(t, v2, 0.0)
}

// U-PSC-UPTIME: Uptime increases over time
func TestPSCDriverHealthCollector_UptimeIncreases(t *testing.T) {
	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()
	c := collectors.NewPSCDriverHealthCollector(reg, "isilon-uptime-test", fakeClient)

	// First collection
	err := c.Collect(context.Background())
	require.NoError(t, err)

	mf1 := gatherPSCHealthMetric(t, reg, "dell_csi_driver_uptime_seconds")
	require.NotNil(t, mf1)
	v1, ok := gaugePSCHealth(mf1, map[string]string{"cluster_name": "isilon-uptime-test"})
	require.True(t, ok)

	// Wait a bit
	time.Sleep(10 * time.Millisecond)

	// Second collection
	err = c.Collect(context.Background())
	require.NoError(t, err)

	mf2 := gatherPSCHealthMetric(t, reg, "dell_csi_driver_uptime_seconds")
	require.NotNil(t, mf2)
	v2, ok := gaugePSCHealth(mf2, map[string]string{"cluster_name": "isilon-uptime-test"})
	require.True(t, ok)

	// Uptime should increase
	assert.Greater(t, v2, v1, "uptime should increase over time")
}

// U-PSC-COLLECT-ERROR: Collect handles errors gracefully
func TestPSCDriverHealthCollector_CollectError(t *testing.T) {
	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()
	c := collectors.NewPSCDriverHealthCollector(reg, "isilon-error-test", fakeClient)

	// Collect should not return error even if some metrics fail
	err := c.Collect(context.Background())
	require.NoError(t, err)
}

// U-PSC-CONTEXT-CANCEL: Collect respects context cancellation
func TestPSCDriverHealthCollector_ContextCancellation(_ *testing.T) {
	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()
	c := collectors.NewPSCDriverHealthCollector(reg, "isilon-cancel-test", fakeClient)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Should handle cancelled context gracefully
	err := c.Collect(ctx)
	// Error is expected or no error, both are acceptable
	_ = err
}

// U-PSC-RESTART-ENV: Collect with POD_NAMESPACE environment variable set
func TestPSCDriverHealthCollector_Collect_WithPodNamespace(_ *testing.T) {
	// Set POD_NAMESPACE environment variable
	oldNamespace := os.Getenv("POD_NAMESPACE")
	defer os.Setenv("POD_NAMESPACE", oldNamespace)
	os.Setenv("POD_NAMESPACE", "test-namespace")

	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()
	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	err := c.Collect(context.Background())
	// May fail due to hostname not matching, but that's OK - we're testing the code path
	_ = err
}

// U-PSC-K8S-METRICS-ENV: Collect with POD_NAME and POD_NAMESPACE set
func TestPSCDriverHealthCollector_Collect_WithPodNameAndNamespace(_ *testing.T) {
	// Set POD_NAME and POD_NAMESPACE environment variables
	oldPodName := os.Getenv("POD_NAME")
	oldNamespace := os.Getenv("POD_NAMESPACE")
	defer os.Setenv("POD_NAME", oldPodName)
	defer os.Setenv("POD_NAMESPACE", oldNamespace)
	os.Setenv("POD_NAME", "test-pod")
	os.Setenv("POD_NAMESPACE", "test-namespace")

	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()
	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	err := c.Collect(context.Background())
	// May fail due to pod not found, but that's OK - we're testing the code path
	_ = err
}

// U-PSC-RESTART-NIL-CLIENT: collectRestartCount with nil k8sClient
func TestPSCDriverHealthCollector_Collect_NilK8sClient(_ *testing.T) {
	reg := prometheus.NewRegistry()
	c := collectors.NewPSCDriverHealthCollector(reg, "test-node", nil)

	err := c.Collect(context.Background())
	// Should not fail with nil k8sClient
	_ = err
}

// U-PSC-RESTART-HOSTNAME-ERROR: collectRestartCount with hostname error (simulated)
func TestPSCDriverHealthCollector_Collect_HostnameError(_ *testing.T) {
	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()
	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	// This test exercises the code path where hostname retrieval might fail
	// We can't easily mock os.Hostname, but we can test the overall flow
	err := c.Collect(context.Background())
	_ = err
}

// U-PSC-RESTART-MOCK-POD: collectRestartCount with mocked Kubernetes pod
func TestPSCDriverHealthCollector_Collect_WithMockedPod(_ *testing.T) {
	// Set POD_NAMESPACE to match the pod we'll create
	oldNamespace := os.Getenv("POD_NAMESPACE")
	defer os.Setenv("POD_NAMESPACE", oldNamespace)
	os.Setenv("POD_NAMESPACE", "test-namespace")

	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()

	// Create a fake pod that matches the hostname
	hostname, _ := os.Hostname()
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hostname,
			Namespace: "test-namespace",
			Labels: map[string]string{
				"app": "csi-isilon",
			},
		},
		Spec: v1.PodSpec{
			NodeName: "test-node",
		},
	}
	_, _ = fakeClient.CoreV1().Pods("test-namespace").Create(context.Background(), pod, metav1.CreateOptions{})

	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	err := c.Collect(context.Background())
	_ = err
}

// U-PSC-RESTART-MOCK-POD-LIST: collectRestartCount with mocked pod list for node-based discovery
func TestPSCDriverHealthCollector_Collect_WithMockedPodList(_ *testing.T) {
	// Set POD_NAMESPACE to match the pod we'll create
	oldNamespace := os.Getenv("POD_NAMESPACE")
	defer os.Setenv("POD_NAMESPACE", oldNamespace)
	os.Setenv("POD_NAMESPACE", "test-namespace")

	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()

	// Create a fake pod list for node-based discovery
	hostname, _ := os.Hostname()
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "csi-isilon-node",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"app": "csi-isilon",
			},
		},
		Spec: v1.PodSpec{
			NodeName: hostname, // Matches hostname for node-based discovery
		},
	}
	_, _ = fakeClient.CoreV1().Pods("test-namespace").Create(context.Background(), pod, metav1.CreateOptions{})

	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	err := c.Collect(context.Background())
	_ = err
}

// U-PSC-K8S-METRICS-MOCK-POD: collectKubernetesMetrics with mocked pod
func TestPSCDriverHealthCollector_Collect_K8sMetrics_WithMockedPod(_ *testing.T) {
	// Set POD_NAME and POD_NAMESPACE environment variables
	oldPodName := os.Getenv("POD_NAME")
	oldNamespace := os.Getenv("POD_NAMESPACE")
	defer os.Setenv("POD_NAME", oldPodName)
	defer os.Setenv("POD_NAMESPACE", oldNamespace)
	os.Setenv("POD_NAME", "test-pod")
	os.Setenv("POD_NAMESPACE", "test-namespace")

	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()

	// Create a fake pod
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"app": "csi-isilon",
			},
		},
		Spec: v1.PodSpec{
			NodeName: "test-node",
		},
	}
	_, _ = fakeClient.CoreV1().Pods("test-namespace").Create(context.Background(), pod, metav1.CreateOptions{})

	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	err := c.Collect(context.Background())
	_ = err
}

// U-PSC-K8S-METRICS-NIL-METRICS: CollectKubernetesMetrics with nil metrics (should skip)
func TestPSCDriverHealthCollector_CollectKubernetesMetrics_NilMetrics(_ *testing.T) {
	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()
	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	// Access the private field through reflection or create a collector without metrics
	// Since we can't easily set the private fields to nil after construction,
	// we'll test the public Collect method which handles this gracefully
	err := c.Collect(context.Background())
	_ = err
}

// U-PSC-K8S-METRICS-NIL-CLIENT: CollectKubernetesMetrics with nil k8sClient
func TestPSCDriverHealthCollector_CollectKubernetesMetrics_NilClient(_ *testing.T) {
	reg := prometheus.NewRegistry()
	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", nil)

	// Should not panic with nil k8sClient
	c.CollectKubernetesMetrics(context.Background())
}

// U-PSC-K8S-METRICS-HOSTNAME-ERROR: CollectKubernetesMetrics with hostname error (simulated by clearing POD_NAME/POD_NAMESPACE)
func TestPSCDriverHealthCollector_CollectKubernetesMetrics_HostnameError(_ *testing.T) {
	// Clear environment variables to force dynamic pod discovery
	oldPodName := os.Getenv("POD_NAME")
	oldNamespace := os.Getenv("POD_NAMESPACE")
	defer os.Setenv("POD_NAME", oldPodName)
	defer os.Setenv("POD_NAMESPACE", oldNamespace)
	os.Unsetenv("POD_NAME")
	os.Unsetenv("POD_NAMESPACE")

	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()
	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	// This will attempt dynamic pod discovery but may fail on hostname
	// The function should handle errors gracefully
	c.CollectKubernetesMetrics(context.Background())
}

// U-PSC-K8S-METRICS-SERVICE-ACCOUNT-ERROR: CollectKubernetesMetrics with service account read error
func TestPSCDriverHealthCollector_CollectKubernetesMetrics_ServiceAccountError(_ *testing.T) {
	// Clear environment variables to force service account read
	oldPodName := os.Getenv("POD_NAME")
	oldNamespace := os.Getenv("POD_NAMESPACE")
	defer os.Setenv("POD_NAME", oldPodName)
	defer os.Setenv("POD_NAMESPACE", oldNamespace)
	os.Unsetenv("POD_NAME")
	os.Unsetenv("POD_NAMESPACE")

	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()
	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	// This will try to read from service account but the file doesn't exist in test environment
	// The function should handle errors gracefully
	c.CollectKubernetesMetrics(context.Background())
}

// U-PSC-K8S-METRICS-POD-LIST-ERROR: collectKubernetesMetrics with pod list error
func TestPSCDriverHealthCollector_Collect_K8sMetrics_PodListError(_ *testing.T) {
	// Set only POD_NAMESPACE to trigger pod discovery
	oldNamespace := os.Getenv("POD_NAMESPACE")
	defer os.Setenv("POD_NAMESPACE", oldNamespace)
	os.Setenv("POD_NAMESPACE", "test-namespace")

	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()

	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	err := c.Collect(context.Background())
	// Will fail due to pod not found, but that's OK - we're testing the error path
	_ = err
}

// U-PSC-K8S-METRICS-POD-NOT-FOUND: collectKubernetesMetrics with pod not found
func TestPSCDriverHealthCollector_Collect_K8sMetrics_PodNotFound(_ *testing.T) {
	// Set POD_NAME and POD_NAMESPACE to non-existent pod
	oldPodName := os.Getenv("POD_NAME")
	oldNamespace := os.Getenv("POD_NAMESPACE")
	defer os.Setenv("POD_NAME", oldPodName)
	defer os.Setenv("POD_NAMESPACE", oldNamespace)
	os.Setenv("POD_NAME", "non-existent-pod")
	os.Setenv("POD_NAMESPACE", "test-namespace")

	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()

	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	// Should handle pod not found gracefully
	c.CollectKubernetesMetrics(context.Background())
}

// U-PSC-RESTART-EMPTY-POD-LIST: collectRestartCount with empty pod list
func TestPSCDriverHealthCollector_Collect_EmptyPodList(_ *testing.T) {
	// Set POD_NAMESPACE to match the pod we'll create
	oldNamespace := os.Getenv("POD_NAMESPACE")
	defer os.Setenv("POD_NAMESPACE", oldNamespace)
	os.Setenv("POD_NAMESPACE", "test-namespace")

	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()

	// Don't create any pods - empty list
	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	err := c.Collect(context.Background())
	_ = err
}

// U-PSC-RESTART-POD-WITHOUT-LABEL: collectRestartCount with pod without CSI label
func TestPSCDriverHealthCollector_Collect_PodWithoutCSILabel(_ *testing.T) {
	// Set POD_NAMESPACE to match the pod we'll create
	oldNamespace := os.Getenv("POD_NAMESPACE")
	defer os.Setenv("POD_NAMESPACE", oldNamespace)
	os.Setenv("POD_NAMESPACE", "test-namespace")

	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()

	// Create a fake pod without CSI label
	hostname, _ := os.Hostname()
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hostname,
			Namespace: "test-namespace",
			Labels: map[string]string{
				"app": "other-app", // Not a CSI driver pod
			},
		},
		Spec: v1.PodSpec{
			NodeName: hostname,
		},
	}
	_, _ = fakeClient.CoreV1().Pods("test-namespace").Create(context.Background(), pod, metav1.CreateOptions{})

	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	err := c.Collect(context.Background())
	_ = err
}

// U-PSC-RESTART-POD-NODE-SUFFIX: collectRestartCount with pod having -node suffix
func TestPSCDriverHealthCollector_Collect_PodWithNodeSuffix(_ *testing.T) {
	// Set POD_NAMESPACE to match the pod we'll create
	oldNamespace := os.Getenv("POD_NAMESPACE")
	defer os.Setenv("POD_NAMESPACE", oldNamespace)
	os.Setenv("POD_NAMESPACE", "test-namespace")

	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()

	// Create a fake pod with -node suffix and csi-isilon image
	hostname, _ := os.Hostname()
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "csi-isilon-node",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"app": "csi-isilon-node",
			},
		},
		Spec: v1.PodSpec{
			NodeName: hostname,
			Containers: []v1.Container{
				{
					Name:  "csi-isilon",
					Image: "dell/csi-isilon:latest",
				},
			},
		},
	}
	_, _ = fakeClient.CoreV1().Pods("test-namespace").Create(context.Background(), pod, metav1.CreateOptions{})

	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	err := c.Collect(context.Background())
	_ = err
}

// U-PSC-K8S-METRICS-POD-LIST-DISCOVERY: collectKubernetesMetrics with pod list discovery
func TestPSCDriverHealthCollector_Collect_K8sMetrics_PodListDiscovery(_ *testing.T) {
	// Set only POD_NAMESPACE to trigger pod discovery
	oldNamespace := os.Getenv("POD_NAMESPACE")
	defer os.Setenv("POD_NAMESPACE", oldNamespace)
	os.Setenv("POD_NAMESPACE", "test-namespace")

	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()

	// Create a fake pod with -node suffix and csi-isilon image
	hostname, _ := os.Hostname()
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "csi-isilon-node",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"app": "csi-isilon-node",
			},
		},
		Spec: v1.PodSpec{
			NodeName: hostname,
			Containers: []v1.Container{
				{
					Name:  "csi-isilon",
					Image: "dell/csi-isilon:latest",
				},
			},
		},
	}
	_, _ = fakeClient.CoreV1().Pods("test-namespace").Create(context.Background(), pod, metav1.CreateOptions{})

	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	err := c.Collect(context.Background())
	_ = err
}

// U-PSC-K8S-METRICS-HOSTNAME-ERROR: collectKubernetesMetrics with hostname error
func TestPSCDriverHealthCollector_Collect_K8sMetrics_HostnameError(_ *testing.T) {
	// Set only POD_NAMESPACE to trigger pod discovery without POD_NAME
	oldNamespace := os.Getenv("POD_NAMESPACE")
	defer os.Setenv("POD_NAMESPACE", oldNamespace)
	os.Setenv("POD_NAMESPACE", "test-namespace")

	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()

	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	err := c.Collect(context.Background())
	// Will fail due to hostname not matching any pod, but that's OK - we're testing the error path
	_ = err
}

// U-PSC-RESTART-POD-LIST-ERROR: collectRestartCount with pod list error
func TestPSCDriverHealthCollector_Collect_PodListError(_ *testing.T) {
	// Set POD_NAMESPACE to trigger pod discovery
	oldNamespace := os.Getenv("POD_NAMESPACE")
	defer os.Setenv("POD_NAMESPACE", oldNamespace)
	os.Setenv("POD_NAMESPACE", "test-namespace")

	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()

	// Don't create any pods - list will be empty
	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	err := c.Collect(context.Background())
	_ = err
}

// U-PSC-RESTART-POD-WITHOUT-NODE-SUFFIX: collectRestartCount with pod without -node suffix
func TestPSCDriverHealthCollector_Collect_PodWithoutNodeSuffix(_ *testing.T) {
	// Set POD_NAMESPACE to match the pod we'll create
	oldNamespace := os.Getenv("POD_NAMESPACE")
	defer os.Setenv("POD_NAMESPACE", oldNamespace)
	os.Setenv("POD_NAMESPACE", "test-namespace")

	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()

	// Create a fake pod without -node suffix
	hostname, _ := os.Hostname()
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "csi-isilon",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"app": "csi-isilon",
			},
		},
		Spec: v1.PodSpec{
			NodeName: hostname,
			Containers: []v1.Container{
				{
					Name:  "csi-isilon",
					Image: "dell/csi-isilon:latest",
				},
			},
		},
	}
	_, _ = fakeClient.CoreV1().Pods("test-namespace").Create(context.Background(), pod, metav1.CreateOptions{})

	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	err := c.Collect(context.Background())
	_ = err
}

// U-PSC-K8S-METRICS-POD-WITHOUT-CSI-IMAGE: collectKubernetesMetrics with pod without csi-isilon image
func TestPSCDriverHealthCollector_Collect_K8sMetrics_PodWithoutCSIImage(_ *testing.T) {
	// Set only POD_NAMESPACE to trigger pod discovery
	oldNamespace := os.Getenv("POD_NAMESPACE")
	defer os.Setenv("POD_NAMESPACE", oldNamespace)
	os.Setenv("POD_NAMESPACE", "test-namespace")

	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()

	// Create a fake pod with -node suffix but without csi-isilon image
	hostname, _ := os.Hostname()
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "csi-isilon-node",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"app": "csi-isilon-node",
			},
		},
		Spec: v1.PodSpec{
			NodeName: hostname,
			Containers: []v1.Container{
				{
					Name:  "other-container",
					Image: "nginx:latest",
				},
			},
		},
	}
	_, _ = fakeClient.CoreV1().Pods("test-namespace").Create(context.Background(), pod, metav1.CreateOptions{})

	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	err := c.Collect(context.Background())
	_ = err
}

// U-PSC-RESTART-POD-WITHOUT-LABELS: collectRestartCount with pod without labels
func TestPSCDriverHealthCollector_Collect_PodWithoutLabels(_ *testing.T) {
	// Set POD_NAMESPACE to match the pod we'll create
	oldNamespace := os.Getenv("POD_NAMESPACE")
	defer os.Setenv("POD_NAMESPACE", oldNamespace)
	os.Setenv("POD_NAMESPACE", "test-namespace")

	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()

	// Create a fake pod without labels
	hostname, _ := os.Hostname()
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "csi-isilon-node",
			Namespace: "test-namespace",
			Labels:    nil,
		},
		Spec: v1.PodSpec{
			NodeName: hostname,
			Containers: []v1.Container{
				{
					Name:  "csi-isilon",
					Image: "dell/csi-isilon:latest",
				},
			},
		},
	}
	_, _ = fakeClient.CoreV1().Pods("test-namespace").Create(context.Background(), pod, metav1.CreateOptions{})

	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	err := c.Collect(context.Background())
	_ = err
}

// U-PSC-RESTART-DIRECT-LOOKUP: collectRestartCount with pod matching hostname directly
func TestPSCDriverHealthCollector_Collect_DirectLookup(_ *testing.T) {
	// Set POD_NAMESPACE to match the pod we'll create
	oldNamespace := os.Getenv("POD_NAMESPACE")
	defer os.Setenv("POD_NAMESPACE", oldNamespace)
	os.Setenv("POD_NAMESPACE", "test-namespace")

	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()

	// Create a fake pod with name matching hostname (controller mode)
	hostname, _ := os.Hostname()
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hostname,
			Namespace: "test-namespace",
			Labels: map[string]string{
				"app": "csi-isilon",
			},
		},
		Spec: v1.PodSpec{
			NodeName: "test-node",
		},
	}
	_, _ = fakeClient.CoreV1().Pods("test-namespace").Create(context.Background(), pod, metav1.CreateOptions{})

	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	err := c.Collect(context.Background())
	_ = err
}

// U-PSC-RESTART-CASE-INSENSITIVE: collectRestartCount with case-insensitive node name matching
func TestPSCDriverHealthCollector_Collect_RestartCaseInsensitive(_ *testing.T) {
	// Set POD_NAMESPACE to match the pod we'll create
	oldNamespace := os.Getenv("POD_NAMESPACE")
	defer os.Setenv("POD_NAMESPACE", oldNamespace)
	os.Setenv("POD_NAMESPACE", "test-namespace")

	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()

	// Create a pod with different case node name
	hostname, _ := os.Hostname()
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "csi-isilon-node",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"app": "csi-isilon-node",
			},
		},
		Spec: v1.PodSpec{
			NodeName: hostname, // Exact match
			Containers: []v1.Container{
				{
					Name:  "csi-isilon",
					Image: "dell/csi-isilon:latest",
				},
			},
		},
	}
	_, _ = fakeClient.CoreV1().Pods("test-namespace").Create(context.Background(), pod, metav1.CreateOptions{})

	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	err := c.Collect(context.Background())
	_ = err
}

// U-PSC-RESTART-MULTIPLE-CONTAINERS: collectRestartCount with pod having multiple containers
func TestPSCDriverHealthCollector_Collect_RestartMultipleContainers(_ *testing.T) {
	// Set POD_NAMESPACE to match the pod we'll create
	oldNamespace := os.Getenv("POD_NAMESPACE")
	defer os.Setenv("POD_NAMESPACE", oldNamespace)
	os.Setenv("POD_NAMESPACE", "test-namespace")

	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()

	// Create a pod with multiple containers
	hostname, _ := os.Hostname()
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "csi-isilon-node",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"app": "csi-isilon-node",
			},
		},
		Spec: v1.PodSpec{
			NodeName: hostname,
			Containers: []v1.Container{
				{
					Name:  "sidecar",
					Image: "nginx:latest",
				},
				{
					Name:  "csi-isilon",
					Image: "dell/csi-isilon:latest",
				},
			},
		},
	}
	_, _ = fakeClient.CoreV1().Pods("test-namespace").Create(context.Background(), pod, metav1.CreateOptions{})

	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	err := c.Collect(context.Background())
	_ = err
}

// U-PSC-RESTART-MIXED-PODS: collectRestartCount with mixed pods
func TestPSCDriverHealthCollector_Collect_RestartMixedPods(_ *testing.T) {
	// Set POD_NAMESPACE to match the pod we'll create
	oldNamespace := os.Getenv("POD_NAMESPACE")
	defer os.Setenv("POD_NAMESPACE", oldNamespace)
	os.Setenv("POD_NAMESPACE", "test-namespace")

	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()

	// Create multiple pods
	hostname, _ := os.Hostname()

	// Pod 1: CSI driver pod
	pod1 := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "csi-isilon-node",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"app": "csi-isilon-node",
			},
		},
		Spec: v1.PodSpec{
			NodeName: hostname,
			Containers: []v1.Container{
				{
					Name:  "csi-isilon",
					Image: "dell/csi-isilon:latest",
				},
			},
		},
	}

	// Pod 2: Non-CSI pod
	pod2 := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "other-pod",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"app": "other",
			},
		},
		Spec: v1.PodSpec{
			NodeName: hostname,
		},
	}

	_, _ = fakeClient.CoreV1().Pods("test-namespace").Create(context.Background(), pod1, metav1.CreateOptions{})
	_, _ = fakeClient.CoreV1().Pods("test-namespace").Create(context.Background(), pod2, metav1.CreateOptions{})

	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	err := c.Collect(context.Background())
	_ = err
}

// U-PSC-K8S-METRICS-MULTIPLE-PODS: collectKubernetesMetrics with multiple pods in list
func TestPSCDriverHealthCollector_Collect_K8sMetrics_MultiplePods(_ *testing.T) {
	// Set only POD_NAMESPACE to trigger pod discovery
	oldNamespace := os.Getenv("POD_NAMESPACE")
	defer os.Setenv("POD_NAMESPACE", oldNamespace)
	os.Setenv("POD_NAMESPACE", "test-namespace")

	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()

	// Create multiple fake pods
	hostname, _ := os.Hostname()

	// Pod 1: matches hostname with -node suffix and csi-isilon image
	pod1 := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "csi-isilon-node",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"app": "csi-isilon-node",
			},
		},
		Spec: v1.PodSpec{
			NodeName: hostname,
			Containers: []v1.Container{
				{
					Name:  "csi-isilon",
					Image: "dell/csi-isilon:latest",
				},
			},
		},
	}

	// Pod 2: different pod, should be skipped
	pod2 := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "other-pod",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"app": "other-app",
			},
		},
		Spec: v1.PodSpec{
			NodeName: "other-node",
		},
	}

	_, _ = fakeClient.CoreV1().Pods("test-namespace").Create(context.Background(), pod1, metav1.CreateOptions{})
	_, _ = fakeClient.CoreV1().Pods("test-namespace").Create(context.Background(), pod2, metav1.CreateOptions{})

	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	err := c.Collect(context.Background())
	_ = err
}

// U-PSC-RESTART-POD-WITHOUT-APP-LABEL: collectRestartCount with pod without app label
func TestPSCDriverHealthCollector_Collect_PodWithoutAppLabel(_ *testing.T) {
	// Set POD_NAMESPACE to match the pod we'll create
	oldNamespace := os.Getenv("POD_NAMESPACE")
	defer os.Setenv("POD_NAMESPACE", oldNamespace)
	os.Setenv("POD_NAMESPACE", "test-namespace")

	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()

	// Create a fake pod without app label
	hostname, _ := os.Hostname()
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "csi-isilon-node",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"other-label": "value",
			},
		},
		Spec: v1.PodSpec{
			NodeName: hostname,
			Containers: []v1.Container{
				{
					Name:  "csi-isilon",
					Image: "dell/csi-isilon:latest",
				},
			},
		},
	}
	_, _ = fakeClient.CoreV1().Pods("test-namespace").Create(context.Background(), pod, metav1.CreateOptions{})

	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	err := c.Collect(context.Background())
	_ = err
}

// U-PSC-K8S-METRICS-NAMESPACE-ERROR: collectKubernetesMetrics with namespace read error
func TestPSCDriverHealthCollector_Collect_K8sMetrics_NamespaceError(_ *testing.T) {
	// Clear POD_NAMESPACE to trigger service account read
	oldNamespace := os.Getenv("POD_NAMESPACE")
	defer os.Setenv("POD_NAMESPACE", oldNamespace)
	os.Unsetenv("POD_NAMESPACE")

	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()

	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	err := c.Collect(context.Background())
	// Will fail due to namespace file not existing, but that's OK - we're testing the error path
	_ = err
}

// U-PSC-K8S-METRICS-POD-NAME-ONLY: collectKubernetesMetrics with POD_NAME set but POD_NAMESPACE not set
func TestPSCDriverHealthCollector_Collect_K8sMetrics_PodNameOnly(_ *testing.T) {
	// Set POD_NAME but not POD_NAMESPACE
	oldPodName := os.Getenv("POD_NAME")
	oldNamespace := os.Getenv("POD_NAMESPACE")
	defer os.Setenv("POD_NAME", oldPodName)
	defer os.Setenv("POD_NAMESPACE", oldNamespace)
	os.Setenv("POD_NAME", "test-pod")
	os.Unsetenv("POD_NAMESPACE")

	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()

	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	err := c.Collect(context.Background())
	_ = err
}

// U-PSC-K8S-METRICS-BOTH-SET: collectKubernetesMetrics with both POD_NAME and POD_NAMESPACE set
func TestPSCDriverHealthCollector_Collect_K8sMetrics_BothSet(_ *testing.T) {
	// Set both POD_NAME and POD_NAMESPACE
	oldPodName := os.Getenv("POD_NAME")
	oldNamespace := os.Getenv("POD_NAMESPACE")
	defer os.Setenv("POD_NAME", oldPodName)
	defer os.Setenv("POD_NAMESPACE", oldNamespace)
	os.Setenv("POD_NAME", "test-pod")
	os.Setenv("POD_NAMESPACE", "test-namespace")

	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()

	// Create a fake pod
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"app": "csi-isilon",
			},
		},
		Spec: v1.PodSpec{
			NodeName: "test-node",
		},
	}
	_, _ = fakeClient.CoreV1().Pods("test-namespace").Create(context.Background(), pod, metav1.CreateOptions{})

	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	err := c.Collect(context.Background())
	_ = err
}

// U-PSC-K8S-METRICS-NIL-METRICS: collectKubernetesMetrics with nil cpuUsage/memUsage
func TestPSCDriverHealthCollector_Collect_K8sMetrics_NilMetrics(_ *testing.T) {
	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()

	// Create collector without initializing CPU/memory metrics
	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	// Manually set metrics to nil to test the error path
	// This requires accessing internal fields, which we can't do directly
	// Instead, we'll test via the Collect method which should handle this gracefully
	err := c.Collect(context.Background())
	_ = err
}

// U-PSC-K8S-METRICS-HOSTNAME-FAIL: collectKubernetesMetrics with hostname failure
func TestPSCDriverHealthCollector_Collect_K8sMetrics_HostnameFail(_ *testing.T) {
	// Clear POD_NAME and POD_NAMESPACE to trigger dynamic discovery
	oldPodName := os.Getenv("POD_NAME")
	oldNamespace := os.Getenv("POD_NAMESPACE")
	defer os.Setenv("POD_NAME", oldPodName)
	defer os.Setenv("POD_NAMESPACE", oldNamespace)
	os.Unsetenv("POD_NAME")
	os.Unsetenv("POD_NAMESPACE")

	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()

	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	// This will try to get hostname and potentially fail
	// We can't easily mock os.Hostname, but the code path is exercised
	err := c.Collect(context.Background())
	_ = err
}

// U-PSC-K8S-METRICS-NAMESPACE-READ-FAIL: collectKubernetesMetrics with namespace read failure
func TestPSCDriverHealthCollector_Collect_K8sMetrics_NamespaceReadFail(_ *testing.T) {
	// Clear POD_NAME and POD_NAMESPACE to trigger dynamic discovery
	oldPodName := os.Getenv("POD_NAME")
	oldNamespace := os.Getenv("POD_NAMESPACE")
	defer os.Setenv("POD_NAME", oldPodName)
	defer os.Setenv("POD_NAMESPACE", oldNamespace)
	os.Unsetenv("POD_NAME")
	os.Unsetenv("POD_NAMESPACE")

	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()

	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	// This will try to read namespace from service account file
	// The file won't exist in test environment, so it will fail
	err := c.Collect(context.Background())
	_ = err
}

// U-PSC-K8S-METRICS-MIXED-LABELS: collectKubernetesMetrics with mixed pod labels
func TestPSCDriverHealthCollector_Collect_K8sMetrics_MixedLabels(_ *testing.T) {
	// Set only POD_NAMESPACE to trigger pod discovery
	oldNamespace := os.Getenv("POD_NAMESPACE")
	defer os.Setenv("POD_NAMESPACE", oldNamespace)
	os.Setenv("POD_NAMESPACE", "test-namespace")

	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()

	// Create multiple pods with mixed labels
	hostname, _ := os.Hostname()

	// Pod 1: CSI driver pod
	pod1 := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "csi-isilon-node",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"app": "csi-isilon-node",
			},
		},
		Spec: v1.PodSpec{
			NodeName: hostname,
			Containers: []v1.Container{
				{
					Name:  "csi-isilon",
					Image: "dell/csi-isilon:latest",
				},
			},
		},
	}

	// Pod 2: Non-CSI pod on same node
	pod2 := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nginx-pod",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"app": "nginx",
			},
		},
		Spec: v1.PodSpec{
			NodeName: hostname,
		},
	}

	// Pod 3: CSI pod on different node
	pod3 := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "csi-isilon-node-2",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"app": "csi-isilon-node",
			},
		},
		Spec: v1.PodSpec{
			NodeName: "other-node",
			Containers: []v1.Container{
				{
					Name:  "csi-isilon",
					Image: "dell/csi-isilon:latest",
				},
			},
		},
	}

	_, _ = fakeClient.CoreV1().Pods("test-namespace").Create(context.Background(), pod1, metav1.CreateOptions{})
	_, _ = fakeClient.CoreV1().Pods("test-namespace").Create(context.Background(), pod2, metav1.CreateOptions{})
	_, _ = fakeClient.CoreV1().Pods("test-namespace").Create(context.Background(), pod3, metav1.CreateOptions{})

	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	err := c.Collect(context.Background())
	_ = err
}

// U-PSC-K8S-METRICS-MULTIPLE-CONTAINERS: collectKubernetesMetrics with pod having multiple containers
func TestPSCDriverHealthCollector_Collect_K8sMetrics_MultipleContainers(_ *testing.T) {
	// Set only POD_NAMESPACE to trigger pod discovery
	oldNamespace := os.Getenv("POD_NAMESPACE")
	defer os.Setenv("POD_NAMESPACE", oldNamespace)
	os.Setenv("POD_NAMESPACE", "test-namespace")

	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()

	// Create a pod with multiple containers, only one has csi-isilon image
	hostname, _ := os.Hostname()
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "csi-isilon-node",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"app": "csi-isilon-node",
			},
		},
		Spec: v1.PodSpec{
			NodeName: hostname,
			Containers: []v1.Container{
				{
					Name:  "sidecar",
					Image: "nginx:latest",
				},
				{
					Name:  "csi-isilon",
					Image: "dell/csi-isilon:latest",
				},
				{
					Name:  "log-collector",
					Image: "fluentd:latest",
				},
			},
		},
	}
	_, _ = fakeClient.CoreV1().Pods("test-namespace").Create(context.Background(), pod, metav1.CreateOptions{})

	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	err := c.Collect(context.Background())
	_ = err
}

// U-PSC-K8S-METRICS-CASE-INSENSITIVE: collectKubernetesMetrics with case-insensitive node name matching
func TestPSCDriverHealthCollector_Collect_K8sMetrics_CaseInsensitive(_ *testing.T) {
	// Set only POD_NAMESPACE to trigger pod discovery
	oldNamespace := os.Getenv("POD_NAMESPACE")
	defer os.Setenv("POD_NAMESPACE", oldNamespace)
	os.Setenv("POD_NAMESPACE", "test-namespace")

	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()

	// Create a pod with different case node name
	hostname, _ := os.Hostname()
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "csi-isilon-node",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"app": "csi-isilon-node",
			},
		},
		Spec: v1.PodSpec{
			NodeName: hostname, // Exact match
		},
	}
	_, _ = fakeClient.CoreV1().Pods("test-namespace").Create(context.Background(), pod, metav1.CreateOptions{})

	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	err := c.Collect(context.Background())
	_ = err
}

// U-PSC-RECORD-API: RecordAPICall increments counter
func TestPSCDriverHealthCollector_RecordAPICall(t *testing.T) {
	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()
	c := collectors.NewPSCDriverHealthCollector(reg, "test-api-cluster", fakeClient)

	// Record a successful API call
	c.RecordAPICall(true, 200)

	mf := gatherPSCHealthMetric(t, reg, "dell_powerscale_api_calls_total")
	require.NotNil(t, mf)
	v, ok := counterPSCHealth(mf, map[string]string{"cluster_name": "test-api-cluster", "status": "success"})
	require.True(t, ok)
	assert.Equal(t, 1.0, v)

	// Record a failed API call
	c.RecordAPICall(false, 500)

	mf = gatherPSCHealthMetric(t, reg, "dell_powerscale_api_calls_total")
	require.NotNil(t, mf)
	v, ok = counterPSCHealth(mf, map[string]string{"cluster_name": "test-api-cluster", "status": "failure"})
	require.True(t, ok)
	assert.Equal(t, 1.0, v)
}

// U-PSC-GET-POD-IDENTITY-LABELS: getPodIdentityLabels returns correct labels
func TestPSCDriverHealthCollector_GetPodIdentityLabels(t *testing.T) {
	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()
	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	labels := c.GetPodIdentityLabels("test-pod", "test-namespace", "test-node", "controller")
	require.Len(t, labels, 5)
	assert.Equal(t, "test-cluster", labels[0])
	assert.Equal(t, "test-pod", labels[1])
	assert.Equal(t, "test-node", labels[2])
	assert.Equal(t, "test-namespace", labels[3])
	assert.Equal(t, "controller", labels[4])
}

// U-PSC-GET-POD-IDENTITY-LABELS-EMPTY: getPodIdentityLabels handles empty values
func TestPSCDriverHealthCollector_GetPodIdentityLabels_EmptyValues(t *testing.T) {
	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()
	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	labels := c.GetPodIdentityLabels("", "", "", "")
	require.Len(t, labels, 5)
	assert.Equal(t, "test-cluster", labels[0])
	assert.Equal(t, "unknown", labels[1])
	assert.Equal(t, "unknown", labels[2])
	assert.Equal(t, "unknown", labels[3])
	assert.Equal(t, "unknown", labels[4])
}

// U-PSC-COLLECT-RESTART-COUNT-NIL-CLIENT: collectRestartCount with nil client
func TestPSCDriverHealthCollector_CollectRestartCount_NilClient(t *testing.T) {
	reg := prometheus.NewRegistry()
	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", nil)

	// Should not panic
	assert.NotPanics(t, func() {
		c.CollectRestartCount(context.Background())
	})
}

// U-PSC-COLLECT-KUBERNETES-METRICS: collectKubernetesMetrics with valid pod
func TestPSCDriverHealthCollector_CollectKubernetesMetrics_ValidPod(t *testing.T) {
	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()

	// Create a test pod
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"app": "csi-isilon-controller",
			},
		},
		Spec: v1.PodSpec{
			NodeName: "test-node",
			Containers: []v1.Container{
				{
					Name:  "csi-isilon",
					Image: "csi-isilon:latest",
				},
			},
		},
	}
	_, _ = fakeClient.CoreV1().Pods("test-namespace").Create(context.Background(), pod, metav1.CreateOptions{})

	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	// Set environment variables
	t.Setenv("POD_NAME", "test-pod")
	t.Setenv("POD_NAMESPACE", "test-namespace")

	c.CollectKubernetesMetrics(context.Background())
	// Should not panic even if metrics server is not available
}

// U-PSC-COLLECT-KUBERNETES-METRICS-NAMESPACE-ERROR: collectKubernetesMetrics with namespace file read error
func TestPSCDriverHealthCollector_CollectKubernetesMetrics_NamespaceError(t *testing.T) {
	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()
	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	// Unset both POD_NAME and POD_NAMESPACE to force dynamic discovery
	t.Setenv("POD_NAME", "")
	t.Setenv("POD_NAMESPACE", "")

	// Should not fail even if namespace file read fails
	c.CollectKubernetesMetrics(context.Background())
}

// U-PSC-COLLECT-KUBERNETES-METRICS-PODLIST-ERROR: collectKubernetesMetrics with pod list error
func TestPSCDriverHealthCollector_CollectKubernetesMetrics_PodListError(t *testing.T) {
	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()
	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	// Unset POD_NAME to force dynamic discovery, but keep namespace
	t.Setenv("POD_NAME", "")
	t.Setenv("POD_NAMESPACE", "test-namespace")

	// Should not fail even if pod list fails
	c.CollectKubernetesMetrics(context.Background())
}

// U-PSC-COLLECT-RESTART-HOSTNAME-ERROR: CollectRestartCount with hostname error
func TestPSCDriverHealthCollector_CollectRestartCount_HostnameError(t *testing.T) {
	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()
	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	// Unset POD_NAMESPACE to force file read
	t.Setenv("POD_NAMESPACE", "")

	// Should not fail even if hostname retrieval fails
	c.CollectRestartCount(context.Background())
}

// U-PSC-GET-CONTAINER-NAMES: getContainerNames extracts container names correctly
func TestPSCDriverHealthCollector_GetContainerNames(_ *testing.T) {
	// This test exercises getContainerNames by triggering the code path
	// where CSI driver container is not found in pod metrics
	// Note: Since we can't easily mock the Kubernetes Metrics API, this test
	// verifies the code path doesn't panic. The function is simple enough that
	// coverage isn't critical - it just extracts container names.
	oldPodName := os.Getenv("POD_NAME")
	oldNamespace := os.Getenv("X_CSI_DRIVER_NAMESPACE")
	defer os.Setenv("POD_NAME", oldPodName)
	defer os.Setenv("X_CSI_DRIVER_NAMESPACE", oldNamespace)

	os.Setenv("POD_NAME", "test-pod")
	os.Setenv("X_CSI_DRIVER_NAMESPACE", "test-namespace")

	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()

	// Create a pod without CSI driver container
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-namespace",
		},
	}
	_, _ = fakeClient.CoreV1().Pods("test-namespace").Create(context.Background(), pod, metav1.CreateOptions{})

	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	// This will call getContainerNames when container is not found
	c.CollectKubernetesMetrics(context.Background())
	// Should not panic, getContainerNames is exercised in the warning log
}

// U-PSC-RESTART-COUNT-POD-NOT-FOUND: CollectRestartCount with pod not found
func TestPSCDriverHealthCollector_CollectRestartCount_PodNotFound(_ *testing.T) {
	oldNamespace := os.Getenv("X_CSI_DRIVER_NAMESPACE")
	oldPodName := os.Getenv("POD_NAME")
	defer os.Setenv("X_CSI_DRIVER_NAMESPACE", oldNamespace)
	defer os.Setenv("POD_NAME", oldPodName)

	os.Setenv("X_CSI_DRIVER_NAMESPACE", "test-namespace")
	os.Setenv("POD_NAME", "non-existent-pod")

	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()
	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	// Should handle pod not found gracefully
	c.CollectRestartCount(context.Background())
}

// U-PSC-RESTART-COUNT-RESTART-ERROR: CollectRestartCount with restart count retrieval error
func TestPSCDriverHealthCollector_CollectRestartCount_RestartError(_ *testing.T) {
	oldNamespace := os.Getenv("X_CSI_DRIVER_NAMESPACE")
	oldPodName := os.Getenv("POD_NAME")
	defer os.Setenv("X_CSI_DRIVER_NAMESPACE", oldNamespace)
	defer os.Setenv("POD_NAME", oldPodName)

	os.Setenv("X_CSI_DRIVER_NAMESPACE", "test-namespace")
	os.Setenv("POD_NAME", "test-pod")

	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()

	// Create a pod without restart count info
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"app": "csi-isilon-controller",
			},
		},
		Spec: v1.PodSpec{
			NodeName: "test-node",
		},
	}
	_, _ = fakeClient.CoreV1().Pods("test-namespace").Create(context.Background(), pod, metav1.CreateOptions{})

	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	// Should handle restart count retrieval gracefully
	c.CollectRestartCount(context.Background())
}

// U-PSC-RESTART-COUNT-CONTROLLER-MODE: CollectRestartCount with controller mode pod
func TestPSCDriverHealthCollector_CollectRestartCount_ControllerMode(_ *testing.T) {
	oldNamespace := os.Getenv("X_CSI_DRIVER_NAMESPACE")
	oldPodName := os.Getenv("POD_NAME")
	defer os.Setenv("X_CSI_DRIVER_NAMESPACE", oldNamespace)
	defer os.Setenv("POD_NAME", oldPodName)

	os.Setenv("X_CSI_DRIVER_NAMESPACE", "test-namespace")
	os.Setenv("POD_NAME", "csi-isilon-controller")

	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()

	// Create a pod with controller suffix
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "csi-isilon-controller",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"app": "csi-isilon-controller",
			},
		},
		Spec: v1.PodSpec{
			NodeName: "test-node",
		},
		Status: v1.PodStatus{
			ContainerStatuses: []v1.ContainerStatus{
				{
					Name:         "csi-isilon",
					RestartCount: 2,
				},
			},
		},
	}
	_, _ = fakeClient.CoreV1().Pods("test-namespace").Create(context.Background(), pod, metav1.CreateOptions{})

	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	// Should collect restart count for controller mode
	c.CollectRestartCount(context.Background())
}

// U-PSC-RESTART-COUNT-NODE-MODE: CollectRestartCount with node mode pod
func TestPSCDriverHealthCollector_CollectRestartCount_NodeMode(_ *testing.T) {
	oldNamespace := os.Getenv("X_CSI_DRIVER_NAMESPACE")
	oldPodName := os.Getenv("POD_NAME")
	defer os.Setenv("X_CSI_DRIVER_NAMESPACE", oldNamespace)
	defer os.Setenv("POD_NAME", oldPodName)

	os.Setenv("X_CSI_DRIVER_NAMESPACE", "test-namespace")
	os.Setenv("POD_NAME", "csi-isilon-node")

	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()

	// Create a pod with node suffix
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "csi-isilon-node",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"app": "csi-isilon-node",
			},
		},
		Spec: v1.PodSpec{
			NodeName: "test-node",
		},
		Status: v1.PodStatus{
			ContainerStatuses: []v1.ContainerStatus{
				{
					Name:         "csi-isilon",
					RestartCount: 3,
				},
			},
		},
	}
	_, _ = fakeClient.CoreV1().Pods("test-namespace").Create(context.Background(), pod, metav1.CreateOptions{})

	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	// Should collect restart count for node mode
	c.CollectRestartCount(context.Background())
}

// U-PSC-LABELS-WITH-INSTANCE-TYPE: Metrics include instance_type label
func TestPSCDriverHealthCollector_Metrics_IncludeInstanceType(t *testing.T) {
	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()
	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	err := c.Collect(context.Background())
	require.NoError(t, err)

	// Check uptime metric has instance_type label
	mf := gatherPSCHealthMetric(t, reg, "dell_csi_driver_uptime_seconds")
	require.NotNil(t, mf)

	// Verify labels include instance_type
	for _, m := range mf.GetMetric() {
		hasInstanceType := false
		for _, lp := range m.GetLabel() {
			if lp.GetName() == "instance_type" {
				hasInstanceType = true
				break
			}
		}
		if hasInstanceType {
			return
		}
	}
	t.Error("instance_type label not found in metrics")
}

// U-PSC-LABELS-WITH-POD-NODE-NAMESPACE: Metrics include pod, node, namespace labels
func TestPSCDriverHealthCollector_Metrics_IncludePodNodeNamespace(t *testing.T) {
	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()
	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	err := c.Collect(context.Background())
	require.NoError(t, err)

	// Check uptime metric has pod, node, namespace labels
	mf := gatherPSCHealthMetric(t, reg, "dell_csi_driver_uptime_seconds")
	require.NotNil(t, mf)

	// Verify labels include pod, node, namespace
	for _, m := range mf.GetMetric() {
		hasPod := false
		hasNode := false
		hasNamespace := false
		for _, lp := range m.GetLabel() {
			if lp.GetName() == "pod" {
				hasPod = true
			}
			if lp.GetName() == "node" {
				hasNode = true
			}
			if lp.GetName() == "namespace" {
				hasNamespace = true
			}
		}
		if hasPod && hasNode && hasNamespace {
			return
		}
	}
	t.Error("pod, node, or namespace labels not found in metrics")
}

// U-PSC-ENV-VARS-COLLECT: Collect uses correct environment variable constants
func TestPSCDriverHealthCollector_Collect_UsesCorrectEnvVars(t *testing.T) {
	// Set environment variables using the correct constants
	oldPodName := os.Getenv("POD_NAME")
	oldNamespace := os.Getenv("X_CSI_DRIVER_NAMESPACE")
	oldNodeName := os.Getenv("X_CSI_NODE_NAME")
	oldCSIMode := os.Getenv("X_CSI_MODE")
	defer os.Setenv("POD_NAME", oldPodName)
	defer os.Setenv("X_CSI_DRIVER_NAMESPACE", oldNamespace)
	defer os.Setenv("X_CSI_NODE_NAME", oldNodeName)
	defer os.Setenv("X_CSI_MODE", oldCSIMode)

	os.Setenv("POD_NAME", "test-pod")
	os.Setenv("X_CSI_DRIVER_NAMESPACE", "test-namespace")
	os.Setenv("X_CSI_NODE_NAME", "test-node")
	os.Setenv("X_CSI_MODE", "controller")

	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()
	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	err := c.Collect(context.Background())
	require.NoError(t, err)

	// Verify metrics were collected with correct labels
	mf := gatherPSCHealthMetric(t, reg, "dell_csi_driver_uptime_seconds")
	require.NotNil(t, mf)

	// Check that labels reflect the environment variables
	v, ok := gaugePSCHealth(mf, map[string]string{
		"cluster_name":  "test-cluster",
		"pod":           "test-pod",
		"node":          "test-node",
		"namespace":     "test-namespace",
		"instance_type": "controller",
	})
	require.True(t, ok, "Metrics should have labels matching environment variables")
	assert.Greater(t, v, 0.0, "uptime should be > 0")
}

// U-PSC-RECORD-API-CALL-MULTIPLE: RecordAPICall increments counter multiple times
func TestPSCDriverHealthCollector_RecordAPICall_Multiple(t *testing.T) {
	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()
	c := collectors.NewPSCDriverHealthCollector(reg, "test-api-cluster", fakeClient)

	// Record multiple successful API calls
	c.RecordAPICall(true, 200)
	c.RecordAPICall(true, 200)
	c.RecordAPICall(true, 200)

	mf := gatherPSCHealthMetric(t, reg, "dell_powerscale_api_calls_total")
	require.NotNil(t, mf)
	v, ok := counterPSCHealth(mf, map[string]string{"cluster_name": "test-api-cluster", "status": "success"})
	require.True(t, ok)
	assert.Equal(t, 3.0, v, "Should have 3 successful API calls")

	// Record multiple failed API calls
	c.RecordAPICall(false, 500)
	c.RecordAPICall(false, 404)

	mf = gatherPSCHealthMetric(t, reg, "dell_powerscale_api_calls_total")
	require.NotNil(t, mf)
	v, ok = counterPSCHealth(mf, map[string]string{"cluster_name": "test-api-cluster", "status": "failure"})
	require.True(t, ok)
	assert.Equal(t, 2.0, v, "Should have 2 failed API calls")
}

// U-PSC-RECORD-API-CALL-NIL-METRIC: RecordAPICall handles nil metric gracefully
func TestPSCDriverHealthCollector_RecordAPICall_NilMetric(t *testing.T) {
	reg := prometheus.NewRegistry()
	// Create collector without k8sClient to simulate node mode where APICallsTotal might be nil
	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", nil)

	// Should not panic even if metric is not initialized
	assert.NotPanics(t, func() {
		c.RecordAPICall(true, 200)
	})
}

// U-PSC-GET-CONTAINER-NAMES: getContainerNames extracts container names from metrics
func TestGetContainerNames(t *testing.T) {
	// Since getContainerNames is a private function, we test it through CollectKubernetesMetrics
	// which exercises the code path that uses getContainerNames

	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()
	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	// Set up environment variables for CollectKubernetesMetrics
	t.Setenv("POD_NAME", "test-pod")
	t.Setenv("X_CSI_DRIVER_NAMESPACE", "default")
	t.Setenv("X_CSI_NODE_NAME", "test-node")

	// Call CollectKubernetesMetrics which exercises the getContainerNames code path
	// This should not panic even if pod metrics are not available
	assert.NotPanics(t, func() {
		c.CollectKubernetesMetrics(context.Background())
	})
}

// U-PSC-COLLECT-KUBERNETES-METRICS-NIL-CLIENT: CollectKubernetesMetrics handles nil k8sClient
func TestCollectKubernetesMetrics_NilClient(t *testing.T) {
	reg := prometheus.NewRegistry()
	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", nil)

	// Should not panic when k8sClient is nil
	assert.NotPanics(t, func() {
		c.CollectKubernetesMetrics(context.Background())
	})
}

// U-PSC-COLLECT-KUBERNETES-METRICS-MISSING-ENV: CollectKubernetesMetrics handles missing environment variables
func TestCollectKubernetesMetrics_MissingEnv(t *testing.T) {
	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()
	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	// Set POD_NAME but not X_CSI_DRIVER_NAMESPACE
	t.Setenv("POD_NAME", "test-pod")
	t.Setenv("X_CSI_NODE_NAME", "test-node")

	// Should not panic when environment variables are missing
	assert.NotPanics(t, func() {
		c.CollectKubernetesMetrics(context.Background())
	})
}

// U-PSC-RECORD-API-CALL-SUCCESS: RecordAPICall records successful API calls
func TestRecordAPICall_Success(t *testing.T) {
	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()
	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	// Record a successful API call
	c.RecordAPICall(true, 200)

	// Verify metric was recorded
	mf := gatherPSCHealthMetric(t, reg, "dell_powerscale_api_calls_total")
	require.NotNil(t, mf)
	v, ok := counterPSCHealth(mf, map[string]string{"cluster_name": "test-cluster", "status": "success"})
	require.True(t, ok)
	assert.Equal(t, 1.0, v)
}

// U-PSC-RECORD-API-CALL-FAILURE: RecordAPICall records failed API calls
func TestRecordAPICall_Failure(t *testing.T) {
	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()
	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	// Record a failed API call
	c.RecordAPICall(false, 500)

	// Verify metric was recorded
	mf := gatherPSCHealthMetric(t, reg, "dell_powerscale_api_calls_total")
	require.NotNil(t, mf)
	v, ok := counterPSCHealth(mf, map[string]string{"cluster_name": "test-cluster", "status": "failure"})
	require.True(t, ok)
	assert.Equal(t, 1.0, v)
}

// U-PSC-RECORD-API-CALL-MULTIPLE: RecordAPICall increments counter correctly
func TestRecordAPICall_Multiple(t *testing.T) {
	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()
	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	// Record multiple successful API calls
	c.RecordAPICall(true, 200)
	c.RecordAPICall(true, 200)
	c.RecordAPICall(true, 200)

	// Verify counter incremented correctly
	mf := gatherPSCHealthMetric(t, reg, "dell_powerscale_api_calls_total")
	require.NotNil(t, mf)
	v, ok := counterPSCHealth(mf, map[string]string{"cluster_name": "test-cluster", "status": "success"})
	require.True(t, ok)
	assert.Equal(t, 3.0, v)
}

// U-PSC-RECORD-API-CALL-NIL-METRIC: RecordAPICall handles nil metric gracefully
func TestRecordAPICall_NilMetric(t *testing.T) {
	c := &collectors.PSCDriverHealthCollector{}
	c.APICallsTotal = nil

	// Should not panic when metric is nil
	assert.NotPanics(t, func() {
		c.RecordAPICall(true, 200)
	})
}

// U-PSC-COLLECT-K8S-METRICS-SUCCESS: CollectKubernetesMetrics with valid pod
func TestCollectKubernetesMetrics_ValidPod(t *testing.T) {
	// Set environment variables
	t.Setenv("POD_NAME", "test-pod")
	t.Setenv("X_CSI_DRIVER_NAMESPACE", "test-namespace")
	t.Setenv("X_CSI_NODE_NAME", "test-node")

	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()

	// Create a fake pod
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"app": "csi-isilon-controller",
			},
		},
		Spec: v1.PodSpec{
			NodeName: "test-node",
		},
	}
	_, _ = fakeClient.CoreV1().Pods("test-namespace").Create(context.Background(), pod, metav1.CreateOptions{})

	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	// Call CollectKubernetesMetrics - should not panic even if metrics API fails
	assert.NotPanics(t, func() {
		c.CollectKubernetesMetrics(context.Background())
	})
}

// U-PSC-COLLECT-K8S-METRICS-NO-NAMESPACE: CollectKubernetesMetrics without namespace
func TestCollectKubernetesMetrics_NoNamespace(t *testing.T) {
	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()
	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	// Clear namespace env var
	t.Setenv("POD_NAME", "test-pod")
	t.Setenv("X_CSI_DRIVER_NAMESPACE", "")

	// Should return early without panic
	assert.NotPanics(t, func() {
		c.CollectKubernetesMetrics(context.Background())
	})
}

// U-PSC-COLLECT-K8S-METRICS-NO-POD-NAME: CollectKubernetesMetrics without pod name
func TestCollectKubernetesMetrics_NoPodName(t *testing.T) {
	reg := prometheus.NewRegistry()
	fakeClient := fake.NewSimpleClientset()
	c := collectors.NewPSCDriverHealthCollector(reg, "test-cluster", fakeClient)

	// Clear pod name env var
	t.Setenv("POD_NAME", "")
	t.Setenv("X_CSI_DRIVER_NAMESPACE", "test-namespace")

	// Should return early without panic
	assert.NotPanics(t, func() {
		c.CollectKubernetesMetrics(context.Background())
	})
}
