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

package service

import (
	"context"
	"os"
	"strings"
	"time"

	csmnamed "github.com/Ecosystems/container-storage-modules/src/csm-metrics-common/pkg/naming"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	recordPermissionDenialFunc = func() {
		// Default no-op; will be set by service if metrics enabled
	}

	// metricsEnabled indicates whether metrics collection is enabled
	// Set by service.go when metrics are initialized
	metricsEnabled = false

	// allowedOperations defines the CSI operations that should be tracked in metrics
	// All other operations will be filtered out to reduce noise
	allowedOperations = map[string]bool{
		"CreateVolume":              true,
		"DeleteVolume":              true,
		"ControllerPublishVolume":   true,
		"ControllerUnpublishVolume": true,
		"NodeStageVolume":           true,
		"NodeUnstageVolume":         true,
		"NodePublishVolume":         true,
		"NodeUnpublishVolume":       true,
	}
)

// NewOperationInterceptor returns a gRPC UnaryServerInterceptor that records
// dell_csi_operation_* metrics for specific CSI operations using cluster_name label.
// Only the following operations are tracked to reduce noise: CreateVolume, DeleteVolume,
// ControllerPublishVolume, ControllerUnpublishVolume, NodeStageVolume, NodeUnstageVolume,
// NodePublishVolume, NodeUnpublishVolume.
// It also increments dell_powerscale_auth_failure_total and
// dell_powerscale_permission_denial_total for relevant gRPC error codes.
// The cluster_name is read dynamically from X_CSI_CLUSTER_NAME env var.
func NewOperationInterceptor(reg prometheus.Registerer, _ string) grpc.UnaryServerInterceptor {
	opTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: csmnamed.MetricCSIOperationTotal,
		Help: "Total CSI operations.",
	}, []string{csmnamed.LabelClusterName, csmnamed.LabelOperation, csmnamed.LabelStatus})
	opDuration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    csmnamed.MetricCSIOperationDurationSeconds,
		Help:    "CSI operation duration.",
		Buckets: csmnamed.HistogramBuckets,
	}, []string{csmnamed.LabelClusterName, csmnamed.LabelOperation})
	opFailure := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: csmnamed.MetricCSIOperationFailureTotal,
		Help: "Total CSI operation failures.",
	}, []string{csmnamed.LabelClusterName, csmnamed.LabelOperation, csmnamed.LabelErrorCode})

	reg.MustRegister(opTotal, opDuration, opFailure)

	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Read cluster name dynamically from environment variable
		// This allows the cluster name to be updated after interceptor creation
		clusterName := os.Getenv("X_CSI_CLUSTER_NAME")
		if clusterName == "" {
			clusterName = "default"
		}

		start := time.Now()
		operation := extractPSCOperationName(info.FullMethod)

		resp, err := handler(ctx, req)
		duration := time.Since(start).Seconds()

		// Only record metrics for allowed operations to reduce noise
		if allowedOperations[operation] {
			opDuration.WithLabelValues(clusterName, operation).Observe(duration)

			if err != nil {
				if isPSCContextCancelled(err) {
					return resp, err
				}
				opTotal.WithLabelValues(clusterName, operation, "failure").Inc()
				errorCode := classifyPSCError(err)
				opFailure.WithLabelValues(clusterName, operation, errorCode).Inc()
				// Record permission denial for PermissionDenied gRPC errors
				if errorCode == "auth_failure" && metricsEnabled {
					s, ok := status.FromError(err)
					if ok && s.Code() == codes.PermissionDenied {
						recordPermissionDenialFunc()
					}
				}
			} else {
				opTotal.WithLabelValues(clusterName, operation, "success").Inc()
			}
		}
		return resp, err
	}
}

func extractPSCOperationName(fullMethod string) string {
	if fullMethod == "" {
		return "unknown"
	}
	parts := strings.Split(fullMethod, "/")
	if len(parts) == 0 {
		return "unknown"
	}
	return parts[len(parts)-1]
}

func classifyPSCError(err error) string {
	if err == nil {
		return "none"
	}
	s, ok := status.FromError(err)
	if !ok {
		return "unknown"
	}
	switch s.Code() {
	case codes.DeadlineExceeded:
		return "timeout"
	case codes.Unauthenticated, codes.PermissionDenied:
		return "auth_failure"
	case codes.NotFound:
		return "not_found"
	default:
		return "unknown"
	}
}

func isPSCContextCancelled(err error) bool {
	if err == context.Canceled {
		return true
	}
	s, ok := status.FromError(err)
	return ok && s.Code() == codes.Canceled
}
