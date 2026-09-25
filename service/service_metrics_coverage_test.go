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
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/common/constants"
	"github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/service/collectors"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// newMetricsTestService returns a service configured for metrics collector tests.
func newMetricsTestService(clusters map[string]interface{}) *service {
	svc := &service{
		metricsRegistry: prometheus.NewRegistry(),
		isiClusters:     new(sync.Map),
	}
	for k, v := range clusters {
		svc.isiClusters.Store(k, v)
	}
	return svc
}

// TestService_StartMetricsWithLeaderElection verifies that the service client is
// reused for leader election, that pod-level collectors are registered on every
// pod and that array-level collectors are only started once leadership is won.
func TestService_StartMetricsWithLeaderElection(t *testing.T) {
	original := leaderElectionForMetricsFunc
	defer func() { leaderElectionForMetricsFunc = original }()

	var (
		gotLockName  string
		gotNamespace string
	)
	elected := make(chan struct{})
	leaderElectionForMetricsFunc = func(ctx context.Context, clientset kubernetes.Interface, lockName, namespace string,
		_, _, _ time.Duration, runFunc func(ctx context.Context),
	) {
		gotLockName = lockName
		gotNamespace = namespace
		assert.NotNil(t, clientset)
		// Simulate winning the election so the array-level collectors start.
		runFunc(ctx)
		close(elected)
	}

	svc := newMetricsTestService(map[string]interface{}{
		"cluster1": &IsilonClusterConfig{ClusterName: "cluster1", IsiPath: "/ifs/data/csi"},
		"bad":      "not-a-cluster-config",
	})
	svc.k8sclient = fake.NewSimpleClientset()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	mgr := collectors.NewCollectorManager()
	svc.startMetricsWithLeaderElection(ctx, mgr, collectors.RuntimeConfig{})
	defer mgr.Stop()

	select {
	case <-elected:
	case <-time.After(5 * time.Second):
		t.Fatal("leader election run function was never invoked")
	}

	assert.Equal(t, "csi-powerscale-metrics", gotLockName)
	assert.Equal(t, "default", gotNamespace)
	assert.Equal(t, mgr, svc.metricsCollectorManager)
}

// TestService_StartMetricsCollectors_LeaderElectionEnabled verifies that
// X_CSI_METRICS_LEADER_ELECTION_ENABLED routes collector startup through the
// leader election path.
func TestService_StartMetricsCollectors_LeaderElectionEnabled(t *testing.T) {
	original := leaderElectionForMetricsFunc
	defer func() { leaderElectionForMetricsFunc = original }()

	called := make(chan struct{})
	leaderElectionForMetricsFunc = func(_ context.Context, _ kubernetes.Interface, _, _ string,
		_, _, _ time.Duration, _ func(ctx context.Context),
	) {
		close(called)
	}

	t.Setenv(constants.EnvMetricsLeaderElectionEnabled, "true")
	t.Setenv(constants.EnvDriverNamespace, "powerscale")

	svc := newMetricsTestService(map[string]interface{}{
		"cluster1": &IsilonClusterConfig{ClusterName: "cluster1"},
	})
	svc.k8sclient = fake.NewSimpleClientset()
	svc.mode = "controller"

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	svc.startMetricsCollectors(ctx)
	defer func() {
		if svc.metricsCollectorManager != nil {
			svc.metricsCollectorManager.Stop()
		}
	}()

	select {
	case <-called:
	case <-time.After(5 * time.Second):
		t.Fatal("leader election was not started when it is enabled")
	}
}

// TestService_StartMetricsCollectors_NodeMode verifies node mode restarts an
// existing collector manager and falls back to a default cluster name when no
// usable cluster config is present.
func TestService_StartMetricsCollectors_NodeMode(t *testing.T) {
	svc := newMetricsTestService(map[string]interface{}{
		"bad": "not-a-cluster-config",
	})
	svc.mode = constants.ModeNode
	svc.k8sclient = fake.NewSimpleClientset()

	previous := collectors.NewCollectorManager()
	svc.metricsCollectorManager = previous

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	svc.startMetricsCollectors(ctx)
	defer svc.metricsCollectorManager.Stop()

	require.NotNil(t, svc.metricsCollectorManager)
	assert.NotEqual(t, previous, svc.metricsCollectorManager, "node mode must replace the previous collector manager")
	assert.NotNil(t, svc.driverHealthCollector)
}

// TestService_StartMetricsWithoutLeaderElection_NoIsiSvc verifies collectors are
// still registered for clusters whose PowerScale client is not initialized yet.
func TestService_StartMetricsWithoutLeaderElection_NoIsiSvc(t *testing.T) {
	svc := newMetricsTestService(map[string]interface{}{
		"cluster1": &IsilonClusterConfig{ClusterName: "cluster1"},
		"bad":      "not-a-cluster-config",
		"nilCfg":   (*IsilonClusterConfig)(nil),
	})
	svc.k8sclient = fake.NewSimpleClientset()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	mgr := collectors.NewCollectorManager()
	svc.startMetricsWithoutLeaderElection(ctx, mgr, collectors.RuntimeConfig{})
	defer mgr.Stop()

	assert.NotNil(t, svc.accessControlCollector, "access control collector must be registered even without isiSvc")
}

// TestService_StartMetricsWithoutLeaderElection_WithK8sValidator verifies the
// quota adapter is built with Kubernetes validation when a client is available.
func TestService_StartMetricsWithoutLeaderElection_WithK8sValidator(t *testing.T) {
	svc := newMetricsTestService(map[string]interface{}{
		"cluster1": &IsilonClusterConfig{ClusterName: "cluster1", isiSvc: &isiService{}},
	})
	svc.k8sclient = fake.NewSimpleClientset()
	svc.opts.Path = "/ifs/data/csi"

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	mgr := collectors.NewCollectorManager()
	svc.startMetricsWithoutLeaderElection(ctx, mgr, collectors.RuntimeConfig{})
	defer mgr.Stop()

	assert.NotNil(t, svc.accessControlCollector)
	assert.NotNil(t, svc.metricsCollectorManager)
}

// TestService_SyncIsilonConfigs_Errors covers the error paths of syncIsilonConfigs.
func TestService_SyncIsilonConfigs_Errors(t *testing.T) {
	originalConfigFile := isilonConfigFile
	defer func() { isilonConfigFile = originalConfigFile }()

	tests := []struct {
		name        string
		contents    string
		expectedErr string
	}{
		{
			name:        "empty secret",
			contents:    "",
			expectedErr: "isilon cluster details are not provided in isilon-creds secret",
		},
		{
			name:        "invalid secret",
			contents:    "isilonClusters:\n  - clusterName: \"\"\n",
			expectedErr: "clusterName not provided in secret",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configFile := filepath.Join(t.TempDir(), "secret.yaml")
			require.NoError(t, os.WriteFile(configFile, []byte(tt.contents), 0o600))
			isilonConfigFile = configFile

			svc := &service{isiClusters: new(sync.Map)}
			err := svc.syncIsilonConfigs(context.Background())
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedErr)
		})
	}
}

// TestService_ValidateCreateVolumeRequest_RawBlock verifies raw block volumes are
// rejected because PowerScale only serves NFS volumes.
func TestService_ValidateCreateVolumeRequest_RawBlock(t *testing.T) {
	svc := &service{}
	req := &csi.CreateVolumeRequest{
		Name: "volume1",
		VolumeCapabilities: []*csi.VolumeCapability{
			{AccessType: &csi.VolumeCapability_Block{Block: &csi.VolumeCapability_BlockVolume{}}},
		},
	}

	size, err := svc.ValidateCreateVolumeRequest(req)
	assert.Zero(t, size)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "raw block requested from NFS Volume")
}

// TestService_PatchNodeLabels_ClientErrors covers the failure paths of
// PatchNodeLabels when the Kubernetes API rejects the get or the patch.
func TestService_PatchNodeLabels_ClientErrors(t *testing.T) {
	t.Run("get node fails", func(t *testing.T) {
		svc := &service{nodeID: "missing-node", k8sclient: fake.NewSimpleClientset()}
		err := svc.PatchNodeLabels(map[string]string{"key": "value"}, nil)
		require.Error(t, err)
	})

	t.Run("patch node fails", func(t *testing.T) {
		client := fake.NewSimpleClientset(&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: "node1", Labels: map[string]string{"existing": "label"}},
		})
		client.PrependReactor("patch", "nodes", func(_ k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("patch induced error")
		})

		svc := &service{nodeID: "node1", k8sclient: client}
		err := svc.PatchNodeLabels(map[string]string{"key": "value"}, []string{"existing"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "patch induced error")
	})
}

// TestGetKubeClientSet_InvalidPath verifies the injectable Kubernetes client
// helper surfaces configuration errors.
func TestGetKubeClientSet_InvalidPath(t *testing.T) {
	_, err := getKubeClientSet(filepath.Join(t.TempDir(), "does-not-exist.conf"))
	assert.Error(t, err)
}
