// Copyright © 2025-2026 Dell Inc. or its subsidiaries. All Rights Reserved.
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
	"errors"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func gatherPSCMetric(t *testing.T, reg prometheus.Gatherer, name string) *dto.MetricFamily {
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

func counterPSC(mf *dto.MetricFamily, labels map[string]string) float64 {
	if mf == nil {
		return 0
	}
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
			return m.GetCounter().GetValue()
		}
	}
	return 0
}

// U-PSC-01: NodeStageVolume success uses cluster_name label
func TestPSCInterceptor_NodeStageVolume_Success(t *testing.T) {
	t.Setenv("X_CSI_CLUSTER_NAME", "cluster-1")
	reg := prometheus.NewRegistry()
	interceptor := NewOperationInterceptor(reg, "cluster-1")

	info := &grpc.UnaryServerInfo{FullMethod: "/csi.v1.Node/NodeStageVolume"}
	noopH := func(_ context.Context, _ interface{}) (interface{}, error) { return nil, nil }

	_, err := interceptor(context.Background(), nil, info, noopH)
	require.NoError(t, err)

	mf := gatherPSCMetric(t, reg, "dell_csi_operation_total")
	require.NotNil(t, mf, "dell_csi_operation_total must be registered")

	v := counterPSC(mf, map[string]string{
		"cluster_name": "cluster-1", "operation": "NodeStageVolume", "status": "success",
	})
	assert.Equal(t, 1.0, v)
}

// U-PSC-02: NodePublishVolume failure records error_code
func TestPSCInterceptor_NodePublishVolume_Failure(t *testing.T) {
	t.Setenv("X_CSI_CLUSTER_NAME", "cluster-1")
	reg := prometheus.NewRegistry()
	interceptor := NewOperationInterceptor(reg, "cluster-1")

	info := &grpc.UnaryServerInfo{FullMethod: "/csi.v1.Node/NodePublishVolume"}
	errH := func(_ context.Context, _ interface{}) (interface{}, error) {
		return nil, errors.New("timeout: context deadline exceeded")
	}

	_, _ = interceptor(context.Background(), nil, info, errH)

	failMF := gatherPSCMetric(t, reg, "dell_csi_operation_failure_total")
	require.NotNil(t, failMF)
	v := counterPSC(failMF, map[string]string{
		"cluster_name": "cluster-1", "operation": "NodePublishVolume",
	})
	assert.Equal(t, 1.0, v)
}

// U-PSC-03: Non-allowed operations are not recorded (filtering)
func TestPSCInterceptor_NonAllowedOperation_NotRecorded(t *testing.T) {
	t.Setenv("X_CSI_CLUSTER_NAME", "cluster-1")
	reg := prometheus.NewRegistry()
	interceptor := NewOperationInterceptor(reg, "cluster-1")

	// First call an allowed operation to register the metric
	info := &grpc.UnaryServerInfo{FullMethod: "/csi.v1.Node/NodeStageVolume"}
	noopH := func(_ context.Context, _ interface{}) (interface{}, error) { return nil, nil }
	_, err := interceptor(context.Background(), nil, info, noopH)
	require.NoError(t, err)

	// Now test with ControllerGetCapabilities (not in allowed list)
	info = &grpc.UnaryServerInfo{FullMethod: "/csi.v1.Controller/ControllerGetCapabilities"}
	_, err = interceptor(context.Background(), nil, info, noopH)
	require.NoError(t, err)

	// Should not have any metrics for ControllerGetCapabilities
	mf := gatherPSCMetric(t, reg, "dell_csi_operation_total")
	require.NotNil(t, mf, "dell_csi_operation_total must be registered")

	v := counterPSC(mf, map[string]string{
		"cluster_name": "cluster-1", "operation": "ControllerGetCapabilities", "status": "success",
	})
	assert.Equal(t, 0.0, v, "Non-allowed operations should not be recorded")
}

// U-PSC-04: All allowed operations are recorded
func TestPSCInterceptor_AllAllowedOperations_Recorded(t *testing.T) {
	t.Setenv("X_CSI_CLUSTER_NAME", "cluster-1")
	reg := prometheus.NewRegistry()
	interceptor := NewOperationInterceptor(reg, "cluster-1")

	allowedOps := []string{
		"CreateVolume",
		"DeleteVolume",
		"ControllerPublishVolume",
		"ControllerUnpublishVolume",
		"NodeStageVolume",
		"NodeUnstageVolume",
		"NodePublishVolume",
		"NodeUnpublishVolume",
	}

	for _, op := range allowedOps {
		info := &grpc.UnaryServerInfo{FullMethod: "/csi.v1.Controller/" + op}
		noopH := func(_ context.Context, _ interface{}) (interface{}, error) { return nil, nil }

		_, err := interceptor(context.Background(), nil, info, noopH)
		require.NoError(t, err)
	}

	mf := gatherPSCMetric(t, reg, "dell_csi_operation_total")
	require.NotNil(t, mf)

	// Each allowed operation should have been recorded
	for _, op := range allowedOps {
		v := counterPSC(mf, map[string]string{
			"cluster_name": "cluster-1", "operation": op, "status": "success",
		})
		assert.Equal(t, 1.0, v, "Allowed operation %s should be recorded", op)
	}
}

func TestClassifyPSCError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: "none",
		},
		{
			name:     "deadline exceeded",
			err:      status.Error(codes.DeadlineExceeded, "timeout"),
			expected: "timeout",
		},
		{
			name:     "unauthenticated",
			err:      status.Error(codes.Unauthenticated, "auth failed"),
			expected: "auth_failure",
		},
		{
			name:     "permission denied",
			err:      status.Error(codes.PermissionDenied, "denied"),
			expected: "auth_failure",
		},
		{
			name:     "not found",
			err:      status.Error(codes.NotFound, "not found"),
			expected: "not_found",
		},
		{
			name:     "other gRPC error",
			err:      status.Error(codes.Internal, "internal error"),
			expected: "unknown",
		},
		{
			name:     "non-gRPC error",
			err:      errors.New("plain error"),
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyPSCError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractPSCOperationName(t *testing.T) {
	tests := []struct {
		name       string
		fullMethod string
		expected   string
	}{
		{
			name:       "valid method",
			fullMethod: "/csi.v1.Controller/CreateVolume",
			expected:   "CreateVolume",
		},
		{
			name:       "empty method",
			fullMethod: "",
			expected:   "unknown",
		},
		{
			name:       "no slash",
			fullMethod: "CreateVolume",
			expected:   "CreateVolume",
		},
		{
			name:       "multiple slashes",
			fullMethod: "/csi.v1.Controller/Node/NodePublishVolume",
			expected:   "NodePublishVolume",
		},
		{
			name:       "single slash",
			fullMethod: "/CreateVolume",
			expected:   "CreateVolume",
		},
		{
			name:       "trailing slash",
			fullMethod: "/csi.v1.Controller/",
			expected:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractPSCOperationName(tt.fullMethod)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsPSCContextCancelled(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "context cancelled",
			err:      context.Canceled,
			expected: true,
		},
		{
			name:     "other error",
			err:      errors.New("some error"),
			expected: false,
		},
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isPSCContextCancelled(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewOperationInterceptor(t *testing.T) {
	reg := prometheus.NewRegistry()
	clusterName := "test-cluster"

	interceptor := NewOperationInterceptor(reg, clusterName)
	assert.NotNil(t, interceptor)
}

// U-PSC-05: When X_CSI_CLUSTER_NAME is unset, cluster_name defaults to "default"
func TestPSCInterceptor_NoClusterEnvVar_DefaultsToDefault(t *testing.T) {
	t.Setenv("X_CSI_CLUSTER_NAME", "")
	reg := prometheus.NewRegistry()
	interceptor := NewOperationInterceptor(reg, "")

	info := &grpc.UnaryServerInfo{FullMethod: "/csi.v1.Node/NodeStageVolume"}
	noopH := func(_ context.Context, _ interface{}) (interface{}, error) { return nil, nil }
	_, err := interceptor(context.Background(), nil, info, noopH)
	require.NoError(t, err)

	mf := gatherPSCMetric(t, reg, "dell_csi_operation_total")
	require.NotNil(t, mf)
	v := counterPSC(mf, map[string]string{
		"cluster_name": "default", "operation": "NodeStageVolume", "status": "success",
	})
	assert.Equal(t, 1.0, v)
}

// U-PSC-06: PermissionDenied error with metricsEnabled=true triggers recordPermissionDenialFunc
func TestPSCInterceptor_PermissionDenied_RecordsPermissionDenial(t *testing.T) {
	t.Setenv("X_CSI_CLUSTER_NAME", "cluster-1")
	reg := prometheus.NewRegistry()
	interceptor := NewOperationInterceptor(reg, "cluster-1")

	// Enable metrics and set a recording function to detect invocation
	metricsEnabled = true
	recorded := false
	recordPermissionDenialFunc = func() { recorded = true }
	defer func() {
		metricsEnabled = false
		recordPermissionDenialFunc = func() {}
	}()

	info := &grpc.UnaryServerInfo{FullMethod: "/csi.v1.Node/NodePublishVolume"}
	permDeniedH := func(_ context.Context, _ interface{}) (interface{}, error) {
		return nil, status.Error(codes.PermissionDenied, "access denied")
	}
	_, _ = interceptor(context.Background(), nil, info, permDeniedH)
	assert.True(t, recorded, "recordPermissionDenialFunc should have been called for PermissionDenied error")
}

// U-PSC-07: Context cancelled error short-circuits metric recording
func TestPSCInterceptor_ContextCancelled_SkipsMetrics(t *testing.T) {
	t.Setenv("X_CSI_CLUSTER_NAME", "cluster-1")
	reg := prometheus.NewRegistry()
	interceptor := NewOperationInterceptor(reg, "cluster-1")

	info := &grpc.UnaryServerInfo{FullMethod: "/csi.v1.Node/NodePublishVolume"}
	cancelledH := func(_ context.Context, _ interface{}) (interface{}, error) {
		return nil, context.Canceled
	}
	_, err := interceptor(context.Background(), nil, info, cancelledH)
	assert.Error(t, err)

	// failure total should NOT be incremented for cancelled context
	failMF := gatherPSCMetric(t, reg, "dell_csi_operation_failure_total")
	v := counterPSC(failMF, map[string]string{
		"cluster_name": "cluster-1", "operation": "NodePublishVolume",
	})
	assert.Equal(t, 0.0, v, "context cancelled should not record failure metric")
}
