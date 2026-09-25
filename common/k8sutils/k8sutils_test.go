/*
Copyright (c) 2025 Dell Inc, or its subsidiaries.

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
	"errors"
	"os"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	metricsv1beta1api "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsv1beta1 "k8s.io/metrics/pkg/client/clientset/versioned/typed/metrics/v1beta1"
)

var exitFunc = os.Exit

var getPodMetricsesFunc = getPodMetricses

func TestCreateKubeClientSet(t *testing.T) {
	// Test cases
	tests := []struct {
		name       string
		kubeconfig string
		configErr  error
		clientErr  error
		wantErr    bool
	}{
		{
			name:       "Valid kubeconfig",
			kubeconfig: "valid_kubeconfig",
			configErr:  nil,
			clientErr:  nil,
			wantErr:    false,
		},
		{
			name:       "Invalid kubeconfig",
			kubeconfig: "invalid_kubeconfig",
			configErr:  errors.New("config error"),
			clientErr:  nil,
			wantErr:    true,
		},
		{
			name:       "In-cluster config",
			kubeconfig: "",
			configErr:  nil,
			clientErr:  nil,
			wantErr:    false,
		},
		{
			name:       "In-cluster config error",
			kubeconfig: "",
			configErr:  errors.New("config error"),
			clientErr:  nil,
			wantErr:    true,
		},
		{
			name:       "New for config error",
			kubeconfig: "",
			configErr:  nil,
			clientErr:  errors.New("client error"),
			wantErr:    true,
		},
		{
			name:       "New for config error",
			kubeconfig: "invalid_kubeconfig",
			configErr:  nil,
			clientErr:  errors.New("client error"),
			wantErr:    true,
		},
	}

	// Save original functions
	origBuildConfigFromFlags := buildConfigFromFlags
	origInClusterConfig := inClusterConfig
	origNewForConfig := newForConfig

	// Restore original functions after tests
	defer func() {
		buildConfigFromFlags = origBuildConfigFromFlags
		inClusterConfig = origInClusterConfig
		newForConfig = origNewForConfig
	}()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock functions
			buildConfigFromFlags = func(_, _ string) (*rest.Config, error) {
				return &rest.Config{}, tt.configErr
			}
			inClusterConfig = func() (*rest.Config, error) {
				return &rest.Config{}, tt.configErr
			}
			newForConfig = func(_ *rest.Config) (*kubernetes.Clientset, error) {
				if tt.clientErr != nil {
					return nil, tt.clientErr
				}
				return &kubernetes.Clientset{}, nil
			}

			clientset, err := CreateKubeClientSet(tt.kubeconfig)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateKubeClientSet() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && clientset == nil {
				t.Errorf("CreateKubeClientSet() = nil, want non-nil")
			}
		})
	}
}

func TestGetStats(t *testing.T) {
	// Set up the necessary dependencies
	ctx := context.Background()
	volumePath := "/path/to/volume"
	availableBytes, totalBytes, usedBytes, totalInodes, freeInodes, usedInodes, _ := GetStats(ctx, volumePath)

	expectedAvailableBytes := int64(0)
	expectedTotalBytes := int64(0)
	expectedUsedBytes := int64(0)
	expectedTotalInodes := int64(0)
	expectedFreeInodes := int64(0)
	expectedUsedInodes := int64(0)
	if availableBytes != expectedAvailableBytes {
		t.Errorf("Expected availableBytes to be %d, but got %d", expectedAvailableBytes, availableBytes)
	}
	if totalBytes != expectedTotalBytes {
		t.Errorf("Expected totalBytes to be %d, but got %d", expectedTotalBytes, totalBytes)
	}
	if usedBytes != expectedUsedBytes {
		t.Errorf("Expected usedBytes to be %d, but got %d", expectedUsedBytes, usedBytes)
	}
	if totalInodes != expectedTotalInodes {
		t.Errorf("Expected totalInodes to be %d, but got %d", expectedTotalInodes, totalInodes)
	}
	if freeInodes != expectedFreeInodes {
		t.Errorf("Expected freeInodes to be %d, but got %d", expectedFreeInodes, freeInodes)
	}
	if usedInodes != expectedUsedInodes {
		t.Errorf("Expected usedInodes to be %d, but got %d", expectedUsedInodes, usedInodes)
	}
}

func TestGetStatsNoError(t *testing.T) {
	// Set up the necessary dependencies
	defaultFsInfo := fsInfo
	fsInfo = func(_ context.Context, _ string) (int64, int64, int64, int64, int64, int64, error) {
		return 1, 1, 1, 1, 1, 1, nil
	}
	defer func() {
		fsInfo = defaultFsInfo
	}()

	ctx := context.Background()
	volumePath := "/path/to/volume"
	availableBytes, totalBytes, usedBytes, totalInodes, freeInodes, usedInodes, _ := GetStats(ctx, volumePath)

	expectedAvailableBytes := int64(1)
	expectedTotalBytes := int64(1)
	expectedUsedBytes := int64(1)
	expectedTotalInodes := int64(1)
	expectedFreeInodes := int64(1)
	expectedUsedInodes := int64(1)
	if availableBytes != expectedAvailableBytes {
		t.Errorf("Expected availableBytes to be %d, but got %d", expectedAvailableBytes, availableBytes)
	}
	if totalBytes != expectedTotalBytes {
		t.Errorf("Expected totalBytes to be %d, but got %d", expectedTotalBytes, totalBytes)
	}
	if usedBytes != expectedUsedBytes {
		t.Errorf("Expected usedBytes to be %d, but got %d", expectedUsedBytes, usedBytes)
	}
	if totalInodes != expectedTotalInodes {
		t.Errorf("Expected totalInodes to be %d, but got %d", expectedTotalInodes, totalInodes)
	}
	if freeInodes != expectedFreeInodes {
		t.Errorf("Expected freeInodes to be %d, but got %d", expectedFreeInodes, freeInodes)
	}
	if usedInodes != expectedUsedInodes {
		t.Errorf("Expected usedInodes to be %d, but got %d", expectedUsedInodes, usedInodes)
	}
}

func TestGetPodRestartCount_NilClient(t *testing.T) {
	ctx := context.Background()

	// Test with nil clientset
	_, err := GetPodRestartCount(ctx, nil, "default", "test-pod", "test-container")
	if err == nil || err.Error() != "kubernetes client is uninitialized" {
		t.Errorf("Expected error for nil clientset, got: %v", err)
	}
}

func TestGetPodRestartCountAuto_NilClient(t *testing.T) {
	ctx := context.Background()

	_, err := GetPodRestartCountAuto(ctx, nil, "default", "test-pod")
	if err == nil || err.Error() != "kubernetes client is uninitialized" {
		t.Errorf("Expected error for nil clientset, got: %v", err)
	}
}

func TestGetPodMetrics_NilClient(t *testing.T) {
	ctx := context.Background()

	_, err := GetPodMetrics(ctx, nil, "default", "test-pod")
	if err == nil || err.Error() != "kubernetes client is uninitialized" {
		t.Errorf("Expected error for nil clientset, got: %v", err)
	}
}

func TestGetStats_WithError(t *testing.T) {
	ctx := context.Background()
	defaultFsInfo := fsInfo
	fsInfo = func(_ context.Context, _ string) (int64, int64, int64, int64, int64, int64, error) {
		return 0, 0, 0, 0, 0, 0, errors.New("fs error")
	}
	defer func() {
		fsInfo = defaultFsInfo
	}()

	_, _, _, _, _, _, err := GetStats(ctx, "/invalid/path")
	if err == nil {
		t.Errorf("Expected error for invalid path, got nil")
	}
}

func TestCreateKubeClientSet_WithKubeconfig(t *testing.T) {
	origBuildConfigFromFlags := buildConfigFromFlags
	origNewForConfig := newForConfig

	defer func() {
		buildConfigFromFlags = origBuildConfigFromFlags
		newForConfig = origNewForConfig
	}()

	buildConfigFromFlags = func(_, _ string) (*rest.Config, error) {
		return &rest.Config{}, nil
	}
	newForConfig = func(_ *rest.Config) (*kubernetes.Clientset, error) {
		return &kubernetes.Clientset{}, nil
	}

	clientset, err := CreateKubeClientSet("/path/to/kubeconfig")
	if err != nil {
		t.Errorf("CreateKubeClientSet() error = %v, want nil", err)
	}
	if clientset == nil {
		t.Errorf("CreateKubeClientSet() = nil, want non-nil")
	}
}

func TestCreateKubeClientSet_InCluster(t *testing.T) {
	origInClusterConfig := inClusterConfig
	origNewForConfig := newForConfig

	defer func() {
		inClusterConfig = origInClusterConfig
		newForConfig = origNewForConfig
	}()

	inClusterConfig = func() (*rest.Config, error) {
		return &rest.Config{}, nil
	}
	newForConfig = func(_ *rest.Config) (*kubernetes.Clientset, error) {
		return &kubernetes.Clientset{}, nil
	}

	clientset, err := CreateKubeClientSet("")
	if err != nil {
		t.Errorf("CreateKubeClientSet() error = %v, want nil", err)
	}
	if clientset == nil {
		t.Errorf("CreateKubeClientSet() = nil, want non-nil")
	}
}

func TestGetPodRestartCount_Success(t *testing.T) {
	ctx := context.Background()

	clientset := fake.NewSimpleClientset(&v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Status: v1.PodStatus{
			ContainerStatuses: []v1.ContainerStatus{
				{
					Name:         "test-container",
					RestartCount: 5,
				},
			},
		},
	})

	restartCount, err := GetPodRestartCount(ctx, clientset, "default", "test-pod", "test-container")
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if restartCount != 5 {
		t.Errorf("Expected restart count 5, got: %d", restartCount)
	}
}

func TestGetPodRestartCount_PodNotFound(t *testing.T) {
	ctx := context.Background()

	clientset := fake.NewSimpleClientset()

	_, err := GetPodRestartCount(ctx, clientset, "default", "nonexistent-pod", "test-container")
	if err == nil {
		t.Errorf("Expected error when pod not found, got nil")
	}
}

func TestGetPodRestartCount_ContainerNotFound(t *testing.T) {
	ctx := context.Background()

	clientset := fake.NewSimpleClientset(&v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Status: v1.PodStatus{
			ContainerStatuses: []v1.ContainerStatus{
				{
					Name:         "other-container",
					RestartCount: 3,
				},
			},
		},
	})

	_, err := GetPodRestartCount(ctx, clientset, "default", "test-pod", "test-container")
	if err == nil {
		t.Errorf("Expected error when container not found, got nil")
	}
}

func TestGetPodRestartCountAuto_KnownContainer(t *testing.T) {
	ctx := context.Background()

	clientset := fake.NewSimpleClientset(&v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Status: v1.PodStatus{
			ContainerStatuses: []v1.ContainerStatus{
				{
					Name:         "csi-isilon",
					RestartCount: 7,
				},
			},
		},
	})

	restartCount, err := GetPodRestartCountAuto(ctx, clientset, "default", "test-pod")
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if restartCount != 7 {
		t.Errorf("Expected restart count 7, got: %d", restartCount)
	}
}

func TestGetPodRestartCountAuto_DriverContainer(t *testing.T) {
	ctx := context.Background()

	clientset := fake.NewSimpleClientset(&v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Status: v1.PodStatus{
			ContainerStatuses: []v1.ContainerStatus{
				{
					Name:         "driver",
					RestartCount: 2,
				},
			},
		},
	})

	restartCount, err := GetPodRestartCountAuto(ctx, clientset, "default", "test-pod")
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if restartCount != 2 {
		t.Errorf("Expected restart count 2, got: %d", restartCount)
	}
}

func TestGetPodRestartCountAuto_FallbackToFirst(t *testing.T) {
	ctx := context.Background()

	clientset := fake.NewSimpleClientset(&v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Status: v1.PodStatus{
			ContainerStatuses: []v1.ContainerStatus{
				{
					Name:         "unknown-container",
					RestartCount: 1,
				},
			},
		},
	})

	restartCount, err := GetPodRestartCountAuto(ctx, clientset, "default", "test-pod")
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if restartCount != 1 {
		t.Errorf("Expected restart count 1, got: %d", restartCount)
	}
}

func TestGetPodRestartCountAuto_NoContainers(t *testing.T) {
	ctx := context.Background()

	clientset := fake.NewSimpleClientset(&v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Status: v1.PodStatus{
			ContainerStatuses: []v1.ContainerStatus{},
		},
	})

	_, err := GetPodRestartCountAuto(ctx, clientset, "default", "test-pod")
	if err == nil {
		t.Errorf("Expected error when no containers found, got nil")
	}
}

func TestGetPodRestartCountAuto_PodNotFound(t *testing.T) {
	ctx := context.Background()

	clientset := fake.NewSimpleClientset()

	_, err := GetPodRestartCountAuto(ctx, clientset, "default", "nonexistent-pod")
	if err == nil {
		t.Errorf("Expected error when pod not found, got nil")
	}
}

func TestGetPodMetrics_InClusterConfigError(t *testing.T) {
	ctx := context.Background()

	origInClusterConfig := inClusterConfig
	defer func() { inClusterConfig = origInClusterConfig }()

	inClusterConfig = func() (*rest.Config, error) {
		return nil, errors.New("in-cluster config error")
	}

	_, err := GetPodMetrics(ctx, &kubernetes.Clientset{}, "default", "test-pod")
	if err == nil {
		t.Errorf("Expected error for in-cluster config failure, got nil")
	}
}

// Mock PodMetricsInterface for testing
type mockPodMetricsInterface struct {
	getFunc func(ctx context.Context, name string, opts metav1.GetOptions) (*metricsv1beta1api.PodMetrics, error)
}

func (m *mockPodMetricsInterface) Get(ctx context.Context, name string, opts metav1.GetOptions) (*metricsv1beta1api.PodMetrics, error) {
	return m.getFunc(ctx, name, opts)
}

func (m *mockPodMetricsInterface) List(_ context.Context, _ metav1.ListOptions) (*metricsv1beta1api.PodMetricsList, error) {
	return nil, nil
}

func (m *mockPodMetricsInterface) Watch(_ context.Context, _ metav1.ListOptions) (watch.Interface, error) {
	return nil, nil
}

func TestGetPodMetrics_MetricsClientCreationError(t *testing.T) {
	ctx := context.Background()

	origInClusterConfig := inClusterConfig
	origNewForConfigMetrics := newForConfigMetrics
	defer func() {
		inClusterConfig = origInClusterConfig
		newForConfigMetrics = origNewForConfigMetrics
	}()

	inClusterConfig = func() (*rest.Config, error) {
		return &rest.Config{}, nil
	}

	newForConfigMetrics = func(_ *rest.Config) (*metricsv1beta1.MetricsV1beta1Client, error) {
		return nil, errors.New("metrics client creation error")
	}

	_, err := GetPodMetrics(ctx, &kubernetes.Clientset{}, "default", "test-pod")
	if err == nil {
		t.Errorf("Expected error when metrics client creation fails, got nil")
	}
}

func TestGetPodMetrics_APIError(t *testing.T) {
	ctx := context.Background()

	origInClusterConfig := inClusterConfig
	origNewForConfigMetrics := newForConfigMetrics
	origGetPodMetricses := getPodMetricses
	defer func() {
		inClusterConfig = origInClusterConfig
		newForConfigMetrics = origNewForConfigMetrics
		getPodMetricses = origGetPodMetricses
	}()

	inClusterConfig = func() (*rest.Config, error) {
		return &rest.Config{}, nil
	}

	newForConfigMetrics = func(_ *rest.Config) (*metricsv1beta1.MetricsV1beta1Client, error) {
		return &metricsv1beta1.MetricsV1beta1Client{}, nil
	}

	mockPodMetrics := &mockPodMetricsInterface{
		getFunc: func(_ context.Context, _ string, _ metav1.GetOptions) (*metricsv1beta1api.PodMetrics, error) {
			return nil, errors.New("pod not found")
		},
	}

	getPodMetricses = func(_ metricsv1beta1.MetricsV1beta1Client, _ string) metricsv1beta1.PodMetricsInterface {
		return mockPodMetrics
	}

	_, err := GetPodMetrics(ctx, &kubernetes.Clientset{}, "default", "test-pod")
	if err == nil {
		t.Errorf("Expected error when pod metrics API fails, got nil")
	}
}

func TestGetPodMetrics_Success(t *testing.T) {
	ctx := context.Background()

	origInClusterConfig := inClusterConfig
	origNewForConfigMetrics := newForConfigMetrics
	origGetPodMetricses := getPodMetricses
	defer func() {
		inClusterConfig = origInClusterConfig
		newForConfigMetrics = origNewForConfigMetrics
		getPodMetricses = origGetPodMetricses
	}()

	inClusterConfig = func() (*rest.Config, error) {
		return &rest.Config{}, nil
	}

	newForConfigMetrics = func(_ *rest.Config) (*metricsv1beta1.MetricsV1beta1Client, error) {
		return &metricsv1beta1.MetricsV1beta1Client{}, nil
	}

	podMetrics := &metricsv1beta1api.PodMetrics{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Containers: []metricsv1beta1api.ContainerMetrics{
			{
				Name: "container1",
				Usage: v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("100m"),
					v1.ResourceMemory: resource.MustParse("100Mi"),
				},
			},
		},
	}

	mockPodMetrics := &mockPodMetricsInterface{
		getFunc: func(_ context.Context, _ string, _ metav1.GetOptions) (*metricsv1beta1api.PodMetrics, error) {
			return podMetrics, nil
		},
	}

	getPodMetricses = func(_ metricsv1beta1.MetricsV1beta1Client, _ string) metricsv1beta1.PodMetricsInterface {
		return mockPodMetrics
	}

	result, err := GetPodMetrics(ctx, &kubernetes.Clientset{}, "default", "test-pod")
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if result == nil {
		t.Errorf("Expected pod metrics, got nil")
	}
	if result.Name != "test-pod" {
		t.Errorf("Expected pod name 'test-pod', got '%s'", result.Name)
	}
}

// Mock leaderElection implementation for testing
type mockLeaderElection struct {
	namespace     string
	leaseDuration time.Duration
	renewDeadline time.Duration
	retryPeriod   time.Duration
	runFunc       func(ctx context.Context)
	runCalled     bool
	runError      error
}

func (m *mockLeaderElection) Run() error {
	m.runCalled = true
	return m.runError
}

func (m *mockLeaderElection) WithNamespace(namespace string) {
	m.namespace = namespace
}

func (m *mockLeaderElection) WithLeaseDuration(leaseDuration time.Duration) {
	m.leaseDuration = leaseDuration
}

func (m *mockLeaderElection) WithRenewDeadline(renewDeadline time.Duration) {
	m.renewDeadline = renewDeadline
}

func (m *mockLeaderElection) WithRetryPeriod(retryPeriod time.Duration) {
	m.retryPeriod = retryPeriod
}

func TestLeaderElection_Success(t *testing.T) {
	// Save original functions
	origNewLeaderElection := newLeaderElection
	origExitFunc := exitFunc

	defer func() {
		newLeaderElection = origNewLeaderElection
		exitFunc = origExitFunc
	}()

	// Track if exit was called
	exitCalled := false
	exitCode := 0
	exitFunc = func(code int) {
		exitCalled = true
		exitCode = code
	}

	// Mock leader election
	mockLE := &mockLeaderElection{
		runError: nil,
	}
	newLeaderElection = func(_ kubernetes.Interface, _ string, runFunc func(ctx context.Context)) leaderElection {
		mockLE.runFunc = runFunc
		return mockLE
	}

	// Test parameters
	clientset := fake.NewSimpleClientset()
	lockName := "test-lock"
	namespace := "test-namespace"
	renewDeadline := 10 * time.Second
	leaseDuration := 15 * time.Second
	retryPeriod := 2 * time.Second

	// Run leader election in a goroutine to prevent blocking
	done := make(chan bool)
	go func() {
		LeaderElection(clientset, lockName, namespace, renewDeadline, leaseDuration, retryPeriod, func(_ context.Context) {})
		done <- true
	}()

	// Wait for the goroutine to start
	<-done

	// Verify leader election was configured
	if !mockLE.runCalled {
		t.Errorf("Expected Run() to be called")
	}
	if mockLE.namespace != namespace {
		t.Errorf("Expected namespace %s, got %s", namespace, mockLE.namespace)
	}
	if mockLE.leaseDuration != leaseDuration {
		t.Errorf("Expected lease duration %v, got %v", leaseDuration, mockLE.leaseDuration)
	}
	if mockLE.renewDeadline != renewDeadline {
		t.Errorf("Expected renew deadline %v, got %v", renewDeadline, mockLE.renewDeadline)
	}
	if mockLE.retryPeriod != retryPeriod {
		t.Errorf("Expected retry period %v, got %v", retryPeriod, mockLE.retryPeriod)
	}

	// Verify exit was not called on success
	if exitCalled {
		t.Errorf("Expected exit not to be called on success, but it was called with code %d", exitCode)
	}
}

func TestLeaderElectionForMetrics_Success(t *testing.T) {
	// Save original function
	origNewLeaderElection := newLeaderElection

	defer func() {
		newLeaderElection = origNewLeaderElection
	}()

	// Mock leader election
	mockLE := &mockLeaderElection{
		runError: nil,
	}
	newLeaderElection = func(_ kubernetes.Interface, _ string, runFunc func(ctx context.Context)) leaderElection {
		mockLE.runFunc = runFunc
		return mockLE
	}

	// Test parameters
	ctx := context.Background()
	clientset := fake.NewSimpleClientset()
	lockName := "metrics-lock"
	namespace := "test-namespace"
	renewDeadline := 10 * time.Second
	leaseDuration := 15 * time.Second
	retryPeriod := 2 * time.Second

	// Run leader election in a goroutine to prevent blocking
	done := make(chan bool)
	go func() {
		LeaderElectionForMetrics(ctx, clientset, lockName, namespace, renewDeadline, leaseDuration, retryPeriod, func(_ context.Context) {})
		done <- true
	}()

	// Wait for the goroutine to start
	<-done

	// Verify leader election was configured
	if !mockLE.runCalled {
		t.Errorf("Expected Run() to be called")
	}
	if mockLE.namespace != namespace {
		t.Errorf("Expected namespace %s, got %s", namespace, mockLE.namespace)
	}
	if mockLE.leaseDuration != leaseDuration {
		t.Errorf("Expected lease duration %v, got %v", leaseDuration, mockLE.leaseDuration)
	}
	if mockLE.renewDeadline != renewDeadline {
		t.Errorf("Expected renew deadline %v, got %v", renewDeadline, mockLE.renewDeadline)
	}
	if mockLE.retryPeriod != retryPeriod {
		t.Errorf("Expected retry period %v, got %v", retryPeriod, mockLE.retryPeriod)
	}
}

func TestLeaderElectionForMetrics_Error(t *testing.T) {
	// Save original function
	origNewLeaderElection := newLeaderElection

	defer func() {
		newLeaderElection = origNewLeaderElection
	}()

	// Mock leader election with error
	mockLE := &mockLeaderElection{
		runError: errors.New("metrics leader election failed"),
	}
	newLeaderElection = func(_ kubernetes.Interface, _ string, runFunc func(ctx context.Context)) leaderElection {
		mockLE.runFunc = runFunc
		return mockLE
	}

	// Test parameters
	ctx := context.Background()
	clientset := fake.NewSimpleClientset()
	lockName := "metrics-lock"
	namespace := "test-namespace"
	renewDeadline := 10 * time.Second
	leaseDuration := 15 * time.Second
	retryPeriod := 2 * time.Second

	// Run leader election - should not call os.Exit
	LeaderElectionForMetrics(ctx, clientset, lockName, namespace, renewDeadline, leaseDuration, retryPeriod, func(_ context.Context) {})

	// Verify leader election was attempted
	if !mockLE.runCalled {
		t.Errorf("Expected Run() to be called")
	}
	// Note: LeaderElectionForMetrics does NOT call os.Exit on error, just prints to stderr
}
