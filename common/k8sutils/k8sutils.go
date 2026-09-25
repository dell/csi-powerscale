/*
 Copyright (c) 2020-2025 Dell Inc, or its subsidiaries.

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

package k8sutils

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/Ecosystems/container-storage-modules/src/gofsutil"
	"github.com/kubernetes-csi/csi-lib-utils/leaderelection"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	metricsv1beta1api "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsv1beta1 "k8s.io/metrics/pkg/client/clientset/versioned/typed/metrics/v1beta1"
)

var (
	buildConfigFromFlags = clientcmd.BuildConfigFromFlags
	newForConfig         = kubernetes.NewForConfig
	inClusterConfig      = rest.InClusterConfig
	newLeaderElection    = func(clientset kubernetes.Interface, lockName string, runFunc func(ctx context.Context)) leaderElection {
		return leaderelection.NewLeaderElection(clientset, lockName, runFunc)
	}
	newForConfigMetrics = metricsv1beta1.NewForConfig
)

var fsInfo = func(ctx context.Context, path string) (int64, int64, int64, int64, int64, int64, error) {
	return gofsutil.FsInfo(ctx, path)
}

var getPodMetricses = func(client metricsv1beta1.MetricsV1beta1Client, namespace string) metricsv1beta1.PodMetricsInterface {
	return client.PodMetricses(namespace)
}

type leaderElection interface {
	Run() error
	WithNamespace(namespace string)
	WithLeaseDuration(leaseDuration time.Duration)
	WithRenewDeadline(renewDeadline time.Duration)
	WithRetryPeriod(retryPeriod time.Duration)
}

// CreateKubeClientSet - Returns kubeclient set
func CreateKubeClientSet(kubeconfig string) (*kubernetes.Clientset, error) {
	var clientset *kubernetes.Clientset
	if kubeconfig != "" {
		// use the current context in kubeconfig
		config, err := buildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, err
		}
		// create the clientset
		clientset, err = newForConfig(config)
		if err != nil {
			return nil, err
		}
	} else {
		config, err := inClusterConfig()
		if err != nil {
			return nil, err
		}
		// creates the clientset
		clientset, err = newForConfig(config)
		if err != nil {
			return nil, err
		}
	}
	return clientset, nil
}

// LeaderElection - Initialize leader election
func LeaderElection(clientset kubernetes.Interface, lockName string, namespace string,
	leaderElectionRenewDeadline, leaderElectionLeaseDuration, leaderElectionRetryPeriod time.Duration, runFunc func(ctx context.Context),
) {
	le := newLeaderElection(clientset, lockName, runFunc)
	le.WithNamespace(namespace)
	le.WithLeaseDuration(leaderElectionLeaseDuration)
	le.WithRenewDeadline(leaderElectionRenewDeadline)
	le.WithRetryPeriod(leaderElectionRetryPeriod)
	if err := le.Run(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to initialize leader election: %v", err)
		os.Exit(1)
	}
}

// LeaderElectionForMetrics - Initialize leader election for metrics collection
// This is separate from the main driver leader election to allow array-level metrics
// to only run on the leader pod while pod-level metrics run on all pods
func LeaderElectionForMetrics(_ context.Context, clientset kubernetes.Interface, lockName string, namespace string,
	leaderElectionRenewDeadline, leaderElectionLeaseDuration, leaderElectionRetryPeriod time.Duration, runFunc func(ctx context.Context),
) {
	le := newLeaderElection(clientset, lockName, runFunc)
	le.WithNamespace(namespace)
	le.WithLeaseDuration(leaderElectionLeaseDuration)
	le.WithRenewDeadline(leaderElectionRenewDeadline)
	le.WithRetryPeriod(leaderElectionRetryPeriod)
	if err := le.Run(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to initialize metrics leader election: %v", err)
	}
}

// GetStats - Returns the stats for the volume mounted on given volume path
func GetStats(ctx context.Context, volumePath string) (int64, int64, int64, int64, int64, int64, error) {
	availableBytes, totalBytes, usedBytes, totalInodes, freeInodes, usedInodes, err := fsInfo(ctx, volumePath)
	if err != nil {
		return 0, 0, 0, 0, 0, 0, status.Error(codes.Internal, fmt.Sprintf(
			"failed to get volume stats: %s", err,
		))
	}
	return availableBytes, totalBytes, usedBytes, totalInodes, freeInodes, usedInodes, err
}

// GetPodRestartCount retrieves the restart count for a specific container in a pod
func GetPodRestartCount(ctx context.Context, clientset kubernetes.Interface, namespace, podName, containerName string) (int32, error) {
	if clientset == nil {
		return 0, fmt.Errorf("kubernetes client is uninitialized")
	}

	pod, err := clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return 0, fmt.Errorf("failed to get pod %s in namespace %s: %w", podName, namespace, err)
	}

	// Find the specified container and return its restart count
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.Name == containerName {
			return containerStatus.RestartCount, nil
		}
	}

	return 0, fmt.Errorf("container %s not found in pod %s", containerName, podName)
}

// GetPodRestartCountAuto retrieves the restart count for the CSI driver container by auto-detecting the container name
func GetPodRestartCountAuto(ctx context.Context, clientset kubernetes.Interface, namespace, podName string) (int32, error) {
	if clientset == nil {
		return 0, fmt.Errorf("kubernetes client is uninitialized")
	}

	pod, err := clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return 0, fmt.Errorf("failed to get pod %s in namespace %s: %w", podName, namespace, err)
	}

	// Auto-detect the CSI driver container by looking for common CSI driver container names
	// Priority: "csi-isilon", "driver", or the first container
	containerNames := []string{"csi-isilon", "driver"}

	// First try known container names
	for _, containerStatus := range pod.Status.ContainerStatuses {
		for _, knownName := range containerNames {
			if containerStatus.Name == knownName {
				return containerStatus.RestartCount, nil
			}
		}
	}

	// Fallback to first container if known names not found
	if len(pod.Status.ContainerStatuses) > 0 {
		return pod.Status.ContainerStatuses[0].RestartCount, nil
	}

	return 0, fmt.Errorf("no containers found in pod %s", podName)
}

// GetPodMetrics retrieves CPU and memory metrics for a pod from Kubernetes Metrics API
// NOTE: This requires Kubernetes Metrics Server to be installed in the cluster
// Install with: kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
func GetPodMetrics(ctx context.Context, clientset kubernetes.Interface, namespace, podName string) (*metricsv1beta1api.PodMetrics, error) {
	if clientset == nil {
		return nil, fmt.Errorf("kubernetes client is uninitialized")
	}

	fmt.Printf("[GetPodMetrics] Retrieving pod metrics for %s in namespace %s\n", podName, namespace)

	// Get in-cluster config for metrics client
	config, err := inClusterConfig()
	if err != nil {
		fmt.Printf("[GetPodMetrics] ERROR - Failed to get in-cluster config: %v\n", err)
		return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
	}

	// Create metrics client
	metricsClient, err := newForConfigMetrics(config)
	if err != nil {
		fmt.Printf("[GetPodMetrics] ERROR - Failed to create metrics client: %v\n", err)
		return nil, fmt.Errorf("failed to create metrics client: %w", err)
	}

	fmt.Printf("[GetPodMetrics] Metrics client created successfully\n")

	// Query Kubernetes Metrics API for pod metrics
	fmt.Printf("[GetPodMetrics] Querying Metrics API for pod %s in namespace %s\n", podName, namespace)
	podMetricses := getPodMetricses(*metricsClient, namespace)
	podMetrics, err := podMetricses.Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		fmt.Printf("[GetPodMetrics] ERROR - Failed to get pod metrics from Metrics API: %v\n", err)
		fmt.Printf("[GetPodMetrics] Troubleshooting:\n")
		fmt.Printf("[GetPodMetrics]   1. Check if Metrics Server is installed: kubectl get deployment metrics-server -n kube-system\n")
		fmt.Printf("[GetPodMetrics]   2. Check Metrics Server logs: kubectl logs -n kube-system -l k8s-app=metrics-server\n")
		fmt.Printf("[GetPodMetrics]   3. Wait 30 seconds after pod startup for metrics to be available\n")
		return nil, fmt.Errorf("failed to get pod metrics from Metrics API: %w", err)
	}

	fmt.Printf("[GetPodMetrics] Successfully retrieved metrics for pod %s with %d containers\n", podMetrics.Name, len(podMetrics.Containers))

	// Log container metrics
	for i, container := range podMetrics.Containers {
		cpuUsage := container.Usage[v1.ResourceCPU]
		memUsage := container.Usage[v1.ResourceMemory]
		cpuMillis := cpuUsage.MilliValue()
		memBytes := memUsage.Value()
		cpuPercent := float64(cpuMillis) / 10.0
		memMB := float64(memBytes) / (1024 * 1024)

		fmt.Printf("[GetPodMetrics] Container %d (%s): CPU=%d millicores (%.2f%%), Memory=%d bytes (%.2f MB)\n",
			i, container.Name, cpuMillis, cpuPercent, memBytes, memMB)
	}

	return podMetrics, nil
}
